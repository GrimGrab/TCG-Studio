using System.Collections.Generic;
using System.IO;
using HarmonyLib;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] TalkSounds: customers speak Animal Crossing style, the way animalese.js does it (its letter recordings,
    /// CC BY 4.0): a whole line is rendered into one sound, every character getting the same short slot, read from the start of its
    /// letter's recording. Pitch is how fast the recording is read. Spaces and punctuation are silence. Each person has their own
    /// pitch, in a range for their body; a line's mood changes its speed, pitch, loudness and tune. The game's own customer lines are
    /// voiced too, with or without Personalities.
    /// TCG Studio's preview (lib/talkVoice.ts) renders the same way: change both together.
    /// </summary>
    internal static class TalkSounds
    {
        private class MoodStyle
        {
            public float Pitch, Speed, Loud, Swing, Fall;
        }

        private static readonly Dictionary<string, MoodStyle> Moods = new Dictionary<string, MoodStyle>
        {
            ["neutral"] = new MoodStyle { Pitch = 1f, Speed = 1f, Loud = 1f, Swing = 0.03f, Fall = 0.05f },
            ["happy"] = new MoodStyle { Pitch = 1.08f, Speed = 1.05f, Loud = 1f, Swing = 0.08f, Fall = 0f },
            ["angry"] = new MoodStyle { Pitch = 0.9f, Speed = 1.12f, Loud = 1.3f, Swing = 0.06f, Fall = 0.1f },
            ["sad"] = new MoodStyle { Pitch = 0.82f, Speed = 0.82f, Loud = 0.7f, Swing = 0.02f, Fall = 0.15f },
            ["excited"] = new MoodStyle { Pitch = 1.15f, Speed = 1.12f, Loud = 1.25f, Swing = 0.1f, Fall = -0.08f },
        };

        private const int Rate = 44100;
        private const int WavHeader = 44;
        private const int LetterSamples = 6615; // 0.15 s per letter in the recording
        private const int SlotSamples = 3307;   // 0.075 s per character spoken
        private const string DigitLetters = "zwttffssen"; // a digit sounds like the start of its name
        private const string VoiceObject = "TCGCC Voice";
        private static readonly List<AudioSource> Speaking = new List<AudioSource>();

        private static byte[] _letters;
        private static bool _failed;

        /// <summary>Speaks <paramref name="text"/> in <paramref name="mood"/>, replacing what this customer was still saying.</summary>
        public static void Say(Customer c, Persona p, string text, string mood)
        {
            if (c != null) Say(c.transform, p?.voice, p?.id ?? c.GetInstanceID().ToString(), c.m_IsFemale, text, mood);
        }

        /// <summary>The same for anyone: a voice of their own, or one worked out from the id.</summary>
        public static void Say(Transform who, CustomerVoice voice, string id, bool female, string text, string mood)
        {
            using var measured = Debug.SpikeWatch.Measure("talk sound");
            if (who == null || !Plugin.CustomerTalkSounds.Value || string.IsNullOrEmpty(text) || !Load()) return;
            var source = SourceOn(who);
            if (!TakeTurn(source)) return;
            var style = mood != null && Moods.TryGetValue(mood, out var m) ? m : Moods["neutral"];
            var data = Render(text, voice ?? AutomaticVoice(id, female), style);
            if (data.Length == 0) return;
            var clip = AudioClip.Create("TCGCC line", data.Length, 1, Rate, false);
            clip.SetData(data, 0);
            source.Stop();
            source.clip = clip;
            source.volume = Mathf.Min(1f, Plugin.CustomerTalkVolume.Value * style.Loud) * SoundManager.SFXVolume;
            source.Play();
            Object.Destroy(clip, data.Length / (float)Rate + 1f);
        }

        /// <summary>
        /// Whether this voice may be heard. With all places taken it replaces the farthest voice if it is closer to the player than that
        /// one; otherwise this line stays silent (its text still shows). Someone already talking keeps their place for their next line.
        /// </summary>
        private static bool TakeTurn(AudioSource source)
        {
            Speaking.RemoveAll(s => s == null || !s.isPlaying);
            if (Speaking.Contains(source)) return true;
            if (Speaking.Count >= Plugin.CustomerMaxVoices.Value)
            {
                var player = Camera.main;
                if (player == null) return false;
                Vector3 at = player.transform.position;
                AudioSource farthest = null;
                float most = -1f;
                foreach (var s in Speaking)
                {
                    float d = (s.transform.position - at).sqrMagnitude;
                    if (d > most) { most = d; farthest = s; }
                }
                if ((source.transform.position - at).sqrMagnitude >= most) return false;
                farthest.Stop();
                Speaking.Remove(farthest);
            }
            Speaking.Add(source);
            return true;
        }

        // Moves with them and goes with them: customers are pooled objects.
        private static AudioSource SourceOn(Transform who)
        {
            var t = who.Find(VoiceObject);
            if (t != null) return t.GetComponent<AudioSource>();
            var go = new GameObject(VoiceObject);
            go.transform.SetParent(who, false);
            go.transform.localPosition = Vector3.up * 1.6f;
            var s = go.AddComponent<AudioSource>();
            s.playOnAwake = false;
            s.spatialBlend = 1f;
            s.rolloffMode = AudioRolloffMode.Linear;
            s.minDistance = 1.5f;
            s.maxDistance = 14f;
            s.dopplerLevel = 0f;
            return s;
        }

        /// <summary>The voice of someone without a saved one, the same every visit: everything from their id, the pitch within their body's range.</summary>
        private static CustomerVoice AutomaticVoice(string id, bool female)
        {
            float Roll(string salt)
            {
                uint h = 2166136261;
                foreach (char ch in salt + id) h = (h ^ ch) * 16777619;
                return (h % 1000) / 1000f;
            }
            return new CustomerVoice
            {
                pitch = (female ? 1.25f : 0.8f) + Roll("") * (female ? 0.45f : 0.25f),
                tone = 0.3f + Roll("t") * 0.4f,
            };
        }

        private static float[] Render(string text, CustomerVoice voice, MoodStyle mood)
        {
            int slot = (int)(SlotSamples / mood.Speed), upper = 0, letters = 0;
            foreach (char ch in text)
            {
                if (!char.IsLetterOrDigit(ch)) continue;
                letters++;
                if (char.IsUpper(ch)) upper++;
            }
            string end = text.TrimEnd();
            bool question = end.EndsWith("?") || end.EndsWith("?!");
            bool shout = end.EndsWith("!") || (letters > 3 && upper * 2 > letters);
            var data = new float[text.Length * slot];
            int count = 0;
            for (int c = 0; c < text.Length; c++)
            {
                char ch = char.ToLowerInvariant(text[c]);
                if (!char.IsLetterOrDigit(ch)) continue; // silence
                if (ch >= '0' && ch <= '9') ch = DigitLetters[ch - '0'];
                else if (ch < 'a' || ch > 'z') ch = "aeiou"[ch % 5]; // other alphabets
                float progress = letters > 1 ? count / (float)(letters - 1) : 0f;
                float tune = 1f - mood.Fall * progress + mood.Swing * Mathf.Sin(count * 0.6f) + Random.Range(-0.02f, 0.02f);
                if (question && progress > 0.7f) tune *= 1f + (progress - 0.7f) * 0.7f;
                if (shout && progress > 0.8f) tune *= 1.05f;
                count++;
                float rate = voice.pitch * mood.Pitch * tune;
                int start = LetterSamples * (ch - 'a');
                for (int i = 0; i < slot; i++)
                {
                    int at = (int)(i * rate);
                    if (at >= LetterSamples) break;
                    data[c * slot + i] = (_letters[WavHeader + start + at] - 128) / 128f;
                }
            }
            ApplyTone(data, voice.tone);
            return data;
        }

        /// <summary>The middle leaves the recording alone; below it the sound gets rounder, above it crisper. Keeps its loudness.</summary>
        private static void ApplyTone(float[] d, float tone)
        {
            if (Mathf.Abs(tone - 0.5f) < 0.02f) return;
            float before = 0.001f, after = 0.001f, last = 0f;
            foreach (float v in d) before = Mathf.Max(before, Mathf.Abs(v));
            if (tone < 0.5f)
            {
                float alpha = Mathf.Lerp(0.08f, 1f, Mathf.Max(tone, 0f) * 2f);
                for (int pass = 0; pass < 2; pass++)
                {
                    last = 0f;
                    for (int i = 0; i < d.Length; i++) d[i] = last += alpha * (d[i] - last);
                }
            }
            else
            {
                float push = (Mathf.Min(tone, 1f) - 0.5f) * 3.6f;
                for (int i = 0; i < d.Length; i++)
                {
                    float v = d[i];
                    d[i] = (1f + push) * v - push * last;
                    last = v;
                }
            }
            foreach (float v in d) after = Mathf.Max(after, Mathf.Abs(v));
            for (int i = 0; i < d.Length; i++) d[i] *= before / after;
        }

        private static bool Load()
        {
            if (_letters != null) return true;
            if (_failed) return false;
            try
            {
                using (var s = typeof(TalkSounds).Assembly.GetManifestResourceStream("TCGCustomCards.talk-letters.wav"))
                using (var m = new MemoryStream())
                {
                    s.CopyTo(m);
                    _letters = m.ToArray();
                }
                return true;
            }
            catch (System.Exception e)
            {
                _failed = true;
                Plugin.Log.LogError($"Customer talking sounds could not be read: {e.Message}. Customers stay silent.");
                return false;
            }
        }

        /// <summary>Picks a mood for one of the game's own lines by its words (English; any other language is neutral). Its lines come without a situation.</summary>
        private static string MoodOfGameLine(string text)
        {
            string t = text.ToLowerInvariant();
            foreach (var w in new[] { "expensive", "stink", "stench", "smell", "ugh", "rip", "scam", "robbery", "crazy", "too much", "let me in", "hurry", "can't find", "where's" })
                if (t.Contains(w)) return "angry";
            foreach (var w in new[] { "cheap", "steal", "bargain", "good price", "great", "wow", "buy buy", "tempting" })
                if (t.Contains(w)) return "happy";
            return "neutral";
        }

        [HarmonyPatch(typeof(PricePopupSpawner), nameof(PricePopupSpawner.ShowTextPopup))]
        private static class GameLinePatch
        {
            private static readonly AccessTools.FieldRef<PricePopupSpawner, bool> CanShow =
                Core.Compat.Field<PricePopupSpawner, bool>(typeof(TalkSounds), "m_CanShowPopup");

            private static void Postfix(PricePopupSpawner __instance, string text, Transform followTransform)
            {
                if (followTransform == null || !CanShow(__instance)) return;
                var c = followTransform.GetComponent<Customer>();
                if (c != null) Say(c, Cast.Of(c), text, MoodOfGameLine(text));
            }
        }
    }
}
