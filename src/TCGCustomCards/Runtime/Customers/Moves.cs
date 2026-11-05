using System.Collections.Generic;
using HarmonyLib;
using Newtonsoft.Json;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>The animations part of the setup's item library (&lt;setup&gt;\accessories\accessories.json, Customers → Animations).</summary>
    internal class MovesFile
    {
        [JsonProperty("animations")] public List<AnimDef> animations;
        [JsonProperty("moves")] public Dictionary<string, List<string>> moves; // the built-in file: its animations per moment
        [JsonProperty("momentAnims")] public Dictionary<string, List<string>> momentAnims; // the setup adds these per moment
        [JsonProperty("momentOff")] public Dictionary<string, List<string>> momentOff;     // and switches these built-in ones off
    }

    /// <summary>Mirror of Studio's setfmt.Animation.</summary>
    internal class AnimDef
    {
        public string id, name, mask;
        public float length;
        public Dictionary<string, List<AnimKeyDef>> bones;
        public Dictionary<string, List<AnimKeyDef>> femaleBones; // built-in ones whose hands must meet: the bodies' own keys
    }

    internal class AnimKeyDef
    {
        public float t;
        public float[] r; // x y z w
    }

    /// <summary>
    /// [Customers - Behaviour] Animations: customer animations made in TCG Studio, played at the moments they're set for
    /// (built-in + the setup's + the customer's, one at random: a line situation — <see cref="Dialogue.Say"/> asks for it — or one of the rip moments). Keys are bone
    /// rotations of the customers' skeleton (both bodies share its bone names); between keys they ease. The player runs in
    /// LateUpdate, after the Animator: each animated bone blends from the game's pose to ours (in and out over
    /// <see cref="Fade"/>), so the game's animation goes on under it. Masks keep the rest of the body on the game's animation;
    /// a whole-body animation plays on the upper body while the customer walks or sits. Only customers playing one cost anything: the
    /// runner switches itself off when nobody does.
    /// </summary>
    internal class Moves : MonoBehaviour
    {
        private class Track
        {
            public string Bone;
            public float[] T;
            public Quaternion[] R;
        }

        private class Clip
        {
            public string Id, Name, Mask;
            public float Length;
            public Track[] Tracks;
            public Track[] FemaleTracks; // null = the same as Tracks
            public bool RightHanded;     // only the right arm moves: with a bag in that hand, the left does it
        }

        private class Playing
        {
            public MonoBehaviour Owner;      // the Customer or Worker
            public Animator Anim;
            public System.Func<bool> Alive;
            public string Who;
            public Clip Clip;
            public Track[] Tracks;
            public float Start;
            public float EndAt = float.MaxValue; // replaced by another: fades out over Fade up to here
            public float End => Mathf.Min(Start + Clip.Length + Fade, EndAt);
            public Transform[] Bones;  // per track (null = not found / outside the mask)
            public bool[] Upper;       // per track: spine_01 or below it (kept while walking)
        }

        private const float Fade = 0.2f;
        /// <summary>An important line's animation waits this long (seconds) for the current one to finish rather than cutting it off.</summary>
        public const float ImportantWait = 0.8f;
        private static readonly Dictionary<string, Clip> Clips = new Dictionary<string, Clip>();
        private static readonly Dictionary<string, List<string>> Pool = new Dictionary<string, List<string>>();
        private static readonly List<string> Choice = new List<string>();
        private static Moves _runner;
        private int _appliedFrame = -1;
        private static bool _hooked;
        private readonly List<Playing> _playing = new List<Playing>();

        private static bool On => Cast.Enabled && Plugin.CustomerAnimations.Value;

        public static void Load(MovesFile file)
        {
            Clips.Clear();
            Pool.Clear();
            MovesFile builtIn = null;
            try
            {
                using (var s = typeof(Moves).Assembly.GetManifestResourceStream("TCGCustomCards.customer-default-animations.json"))
                using (var r = new System.IO.StreamReader(s))
                    builtIn = JsonConvert.DeserializeObject<MovesFile>(r.ReadToEnd());
            }
            catch (System.Exception e)
            {
                Plugin.Log.LogError($"Customer animations: the built-in ones could not be read: {e.Message}");
            }
            foreach (var a in builtIn?.animations ?? new List<AnimDef>()) Add(a);
            foreach (var a in file?.animations ?? new List<AnimDef>()) Add(a);
            foreach (var kv in builtIn?.moves ?? new Dictionary<string, List<string>>()) Pool[kv.Key] = new List<string>(kv.Value);
            foreach (var kv in file?.momentOff ?? new Dictionary<string, List<string>>())
                if (kv.Value != null && Pool.TryGetValue(kv.Key, out var list)) list.RemoveAll(kv.Value.Contains);
            foreach (var kv in file?.momentAnims ?? new Dictionary<string, List<string>>())
            {
                if (kv.Value == null) continue;
                if (!Pool.TryGetValue(kv.Key, out var list)) Pool[kv.Key] = list = new List<string>();
                list.AddRange(kv.Value);
            }
            Plugin.Log.LogInfo($"Customer animations: {Clips.Count} loaded, {Pool.Count} moment(s) have some");
            if (!_hooked)
            {
                _hooked = true;
                Cast.VisitEnded += (c, p, paid) => _runner?.Stop(c);
            }
        }

        private static void Add(AnimDef a)
        {
            var clip = Build(a);
            if (clip != null) Clips[clip.Id] = clip;
        }

        /// <summary>Checks an animation like Studio's ValidateAnimations; null (and a log line) when it can't be played.</summary>
        private static Clip Build(AnimDef a)
        {
            string label = a?.name ?? a?.id ?? "?";
            if (a == null || string.IsNullOrEmpty(a.id) || a.length < 0.1f || a.length > 30f || (a.mask != "full" && a.mask != "upper" && a.mask != "arms"))
            {
                Plugin.Log.LogWarning($"Customer animations: \"{label}\" has a bad id, length or mask; skipped");
                return null;
            }
            var tracks = Tracks(a.bones, a.length, label);
            var female = a.femaleBones == null ? null : Tracks(a.femaleBones, a.length, label);
            if (tracks == null || (a.femaleBones != null && female == null)) return null;
            return new Clip { Id = a.id, Name = a.name, Mask = a.mask, Length = a.length, Tracks = tracks, FemaleTracks = female, RightHanded = OnlyRightArmMoves(tracks) };
        }

        private static bool OnlyRightArmMoves(Track[] tracks)
        {
            bool right = false;
            foreach (var tr in tracks)
            {
                bool moves = false;
                for (int i = 1; i < tr.R.Length && !moves; i++) moves = Quaternion.Angle(tr.R[0], tr.R[i]) > 1f;
                if (!moves) continue;
                if (tr.Bone == "upperarm_l" || tr.Bone == "lowerarm_l") return false;
                if (tr.Bone == "upperarm_r" || tr.Bone == "lowerarm_r") right = true;
            }
            return right;
        }

        /// <summary>
        /// A customer with a shopping bag: it hangs from the right hand, so that arm stays on the game's animation. A one-armed animation
        /// plays on the left arm instead (the rig is mirrored: the same local rotations fit it). Animations whose hands must meet (the ones
        /// with keys per body) keep both arms out of it; the rest of the body still plays. False = leave this track out.
        /// </summary>
        private static bool BagAllows(Clip clip, string track, Dictionary<string, Transform> byName, Transform top, ref Transform bone)
        {
            bool right = Under(bone, "clavicle_r", top), left = Under(bone, "clavicle_l", top);
            if (!right && !left) return true;
            if (clip.FemaleTracks != null) return false;
            if (!clip.RightHanded) return left;
            if (left || track == "clavicle_r" || !track.EndsWith("_r")) return false;
            return byName.TryGetValue(track.Substring(0, track.Length - 1) + "l", out bone);
        }

        private static Track[] Tracks(Dictionary<string, List<AnimKeyDef>> bones, float length, string label)
        {
            var tracks = new List<Track>();
            foreach (var kv in bones ?? new Dictionary<string, List<AnimKeyDef>>())
            {
                var keys = kv.Value;
                if (keys == null || keys.Count == 0) continue;
                var t = new float[keys.Count];
                var r = new Quaternion[keys.Count];
                bool ok = true;
                for (int i = 0; i < keys.Count && ok; i++)
                {
                    var k = keys[i];
                    ok = k?.r != null && k.r.Length == 4 && k.t >= 0 && k.t <= length + 1e-4f && (i == 0 || k.t > t[i - 1]);
                    if (!ok) break;
                    t[i] = k.t;
                    r[i] = new Quaternion(k.r[0], k.r[1], k.r[2], k.r[3]).normalized;
                }
                if (!ok)
                {
                    Plugin.Log.LogWarning($"Customer animations: \"{label}\" has bad keys on {kv.Key}; skipped");
                    return null;
                }
                tracks.Add(new Track { Bone = kv.Key, T = t, R = r });
            }
            return tracks.ToArray();
        }

        /// <summary>
        /// Plays one of <paramref name="moment"/>'s animations on the customer after <paramref name="delay"/> seconds. With <paramref name="interrupt"/>
        /// it replaces what they're playing, the two blending, unless that ends within <paramref name="waitUpTo"/> seconds: then it follows on. Without, it is
        /// skipped when they'd still be busy then, so animations play to their end. False when none plays.
        /// </summary>
        public static bool Play(Customer c, string moment, float delay = 0f, bool interrupt = true, float waitUpTo = 0f)
        {
            if (c == null || moment == null || !On || c.m_Anim == null) return false;
            var p = Cast.Of(c);
            var clip = Pick(p?.anims, p?.animsOff, moment);
            if (clip == null) return false;
            EnsureRunner();
            return _runner.Begin(c, c.m_Anim, () => c.m_IsActive, p?.name ?? "someone", c.m_IsFemale, clip, delay, interrupt, waitUpTo, moment);
        }

        /// <summary>The same for an employee: def is theirs when you made them, null for the game's. Always follows on or skips, never cuts what's playing short.</summary>
        public static bool Play(Worker w, Core.EmployeeDef def, string moment, string name)
        {
            if (w == null || moment == null || !Plugin.CustomerAnimations.Value || w.m_Anim == null) return false;
            var clip = Pick(def?.Anims, def?.AnimsOff, moment);
            if (clip == null) return false;
            EnsureRunner();
            return _runner.Begin(w, w.m_Anim, () => w.m_IsActive, name, w.m_IsFemale, clip, 0f, false, 0f, moment);
        }

        private static void EnsureRunner()
        {
            if (_runner == null)
            {
                var go = new GameObject("TCGCC Customer Moves");
                DontDestroyOnLoad(go);
                _runner = go.AddComponent<Moves>();
            }
        }

        /// <summary>
        /// One of the moment's animations at random. They add up like lines do (<see cref="Traits.LinesFor"/>): built-in − the setup's
        /// switched off + the setup's + the customer's own − the customer's switched off.
        /// </summary>
        private static Clip Pick(Dictionary<string, List<string>> own, Dictionary<string, List<string>> ownOff, string moment)
        {
            Choice.Clear();
            if (Pool.TryGetValue(moment, out var pool)) Choice.AddRange(pool);
            if (ownOff != null && ownOff.TryGetValue(moment, out var off) && off != null) Choice.RemoveAll(off.Contains);
            if (own != null && own.TryGetValue(moment, out var added) && added != null) Choice.AddRange(added);
            Choice.RemoveAll(id => !Clips.ContainsKey(id));
            return Choice.Count > 0 ? Clips[Choice[Random.Range(0, Choice.Count)]] : null;
        }

        private bool Begin(MonoBehaviour c, Animator anim, System.Func<bool> alive, string who, bool female, Clip clip, float delay, bool interrupt, float waitUpTo, string moment)
        {
            using var measured = Debug.SpikeWatch.Measure("animation start");
            float now = Time.time, start = now + Mathf.Max(0f, delay);
            foreach (var other in _playing)
                if (other.Owner == c && !interrupt && other.End > start)
                {
                    Note(who, $"skipped {clip.Name} ({moment}): {other.Clip.Name} is still playing");
                    return false;
                }
            if (interrupt)
            {
                _playing.RemoveAll(o => o.Owner == c && o.Start > now); // waiting to start: this one comes instead
                float busyUntil = 0f;
                foreach (var other in _playing)
                    if (other.Owner == c) busyUntil = Mathf.Max(busyUntil, other.End);
                if (busyUntil - start <= waitUpTo) start = Mathf.Max(start, busyUntil);
                else
                    foreach (var other in _playing)
                    {
                        if (other.Owner != c || other.End <= start) continue;
                        if (start - other.Start < other.Clip.Length) Note(who, $"{other.Clip.Name} cut short at {start - other.Start:0.0}/{other.Clip.Length:0.0} s by {clip.Name} ({moment})");
                        other.EndAt = start + Fade;
                    }
            }
            Note(who, $"plays {clip.Name} ({moment})");
            var tracks = female && clip.FemaleTracks != null ? clip.FemaleTracks : clip.Tracks;
            bool bag = anim.GetBool("HoldingBag");
            var byName = new Dictionary<string, Transform>();
            Collect(anim.transform, byName);
            var p = new Playing { Owner = c, Anim = anim, Alive = alive, Who = who, Clip = clip, Tracks = tracks, Start = start, Bones = new Transform[tracks.Length], Upper = new bool[tracks.Length] };
            for (int i = 0; i < tracks.Length; i++)
            {
                if (!byName.TryGetValue(tracks[i].Bone, out var bone)) continue;
                if (bag && !BagAllows(clip, tracks[i].Bone, byName, anim.transform, ref bone)) continue;
                bool upper = Under(bone, "spine_01", anim.transform);
                bool inMask = clip.Mask == "full" || (clip.Mask == "upper" ? upper : Under(bone, "clavicle_l", anim.transform) || Under(bone, "clavicle_r", anim.transform));
                if (!inMask) continue;
                p.Bones[i] = bone;
                p.Upper[i] = upper;
            }
            _playing.Add(p);
            enabled = true;
            return true;
        }

        /// <summary>[Debug] CustomerDebug: what each customer's animations do, to check them in game.</summary>
        private static void Note(string who, string what)
        {
            if (Plugin.CustomerDebug.Value) Plugin.Log.LogInfo($"Customer animations: {who} {what}");
        }

        private static void Collect(Transform t, Dictionary<string, Transform> into)
        {
            if (!into.ContainsKey(t.name)) into[t.name] = t;
            for (int i = 0; i < t.childCount; i++) Collect(t.GetChild(i), into);
        }

        /// <summary>The bone is <paramref name="name"/> or below it (up to the Animator's object).</summary>
        private static bool Under(Transform bone, string name, Transform top)
        {
            for (var t = bone; t != null && t != top; t = t.parent)
                if (t.name == name) return true;
            return false;
        }

        private void Stop(Customer c)
        {
            for (int i = _playing.Count - 1; i >= 0; i--)
                if (_playing[i].Owner == c) _playing.RemoveAt(i);
        }

        private void LateUpdate() => ApplyOnce();

        // Hair and some clothes have their own bones, which the game's CopyPose copies from the body in its LateUpdate: animations must
        // be on the body first, in whichever order Unity runs the two.
        [HarmonyPatch(typeof(CC.CopyPose), "LateUpdate")]
        private static class BeforeCopyPose
        {
            private static void Prefix()
            {
                if (_runner != null && _runner.enabled) _runner.ApplyOnce();
            }
        }

        private void ApplyOnce()
        {
            if (_appliedFrame == Time.frameCount) return;
            _appliedFrame = Time.frameCount;
            using var measured = Debug.SpikeWatch.Measure("animations");
            float now = Time.time;
            // Oldest first: one that is replacing another blends in over it.
            for (int i = 0; i < _playing.Count; i++)
            {
                var p = _playing[i];
                if (p.Owner == null || !p.Alive() || !p.Owner.gameObject.activeInHierarchy || p.Anim == null || now >= p.EndAt) { _playing.RemoveAt(i--); continue; }
                float t = now - p.Start;
                if (t < 0f) continue; // delayed
                var clip = p.Clip;
                if (t > clip.Length + Fade)
                {
                    Note(p.Who, $"finished {clip.Name}");
                    _playing.RemoveAt(i--);
                    continue;
                }
                float w = Mathf.Min(Mathf.Clamp01(t / Fade), Mathf.Clamp01(1f - (t - clip.Length) / Fade));
                if (p.EndAt != float.MaxValue) w = Mathf.Min(w, Mathf.Clamp01((p.EndAt - now) / Fade));
                // The legs are the game's while they walk or sit.
                bool walking = clip.Mask == "full" && (p.Anim.GetFloat("MoveSpeed") > 0.05f || (p.Owner is Customer sitting && Dialogue.SitDrop(sitting) > 0f));
                float at = Mathf.Min(t, clip.Length);
                for (int k = 0; k < p.Tracks.Length; k++)
                {
                    var bone = p.Bones[k];
                    if (bone == null || (walking && !p.Upper[k])) continue;
                    bone.localRotation = Quaternion.Slerp(bone.localRotation, Sample(p.Tracks[k], at), w);
                }
            }
            if (_playing.Count == 0) enabled = false;
        }

        /// <summary>The track's rotation at time t: held before the first key and after the last, eased between keys.</summary>
        private static Quaternion Sample(Track tr, float t)
        {
            var T = tr.T;
            if (t <= T[0]) return tr.R[0];
            for (int i = 1; i < T.Length; i++)
            {
                if (t > T[i]) continue;
                float u = (t - T[i - 1]) / Mathf.Max(1e-6f, T[i] - T[i - 1]);
                return Quaternion.Slerp(tr.R[i - 1], tr.R[i], u * u * (3f - 2f * u));
            }
            return tr.R[T.Length - 1];
        }
    }
}
