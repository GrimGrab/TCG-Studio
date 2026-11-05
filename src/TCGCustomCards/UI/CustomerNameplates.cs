using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Runtime.Customers;
using UnityEngine;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// [Customers] on-screen parts, drawn by the hooks runner (hidden while a game screen has the mouse):
    /// names above nearby customers (regulars gold, thieves red from afar), the panel about the customer you look at (<see cref="CustomerCard"/>), the regulars
    /// list (RegularsListKey) and thief alerts. Left click stops a thief; the kick key only ever kicks, and not a thief.
    /// </summary>
    internal static class CustomerNameplates
    {
        private static bool _listOpen;
        private static Persona _profile;
        private static Customer _target;
        private static string _feedback;
        private static float _feedbackUntil;
        private static GUIStyle _plate, _list, _alert;
        private static readonly Color Gold = new Color(1f, 0.82f, 0.3f);
        private static readonly Color Red = new Color(1f, 0.3f, 0.25f);

        private const float TargetDistance = 2f;
        private const float ThiefDistance = 7f; // they walk fast
        private const float BodyHeight = 1.8f, BodyStep = 0.2f, BodyRadius = 0.3f;
        private const float ThiefRadius = 0.55f; // a thief on the move is hard to keep in the crosshair
        private const float LingerSeconds = 1f;   // the panel stays a moment after you look away

        private static readonly HarmonyLib.AccessTools.FieldRef<InteractionPlayerController, bool> IgnoreClick =
            Core.Compat.Field<InteractionPlayerController, bool>(typeof(CustomerNameplates), "m_IsResetMousePress");

        private static Persona _lastSeen;
        private static Customer _lastSeenCustomer;
        private static float _seenUntil;
        private static Worker _lastWorker;
        private const float AnnounceSeconds = 5f;
        private static string _announcement;
        private static float _announcedUntil;

        public static void Tick()
        {
            if (!Cast.Enabled || !Core.GameScene.InLoadedShop) { _target = null; _profile = null; CustomerCard.Show(null, null, null); return; }
            var player = Core.GameScene.Existing<InteractionPlayerController>();
            bool free = player != null && !player.IsInUIMode() && player.m_Cam != null;
            _target = free ? FindTarget(player.m_Cam) : null;
            if (!free) { CustomerCard.Show(null, null, null); return; }
            if (Plugin.RegularsListKey.Value.IsDown()) _listOpen = !_listOpen;
            bool padProfile = PadInput.Down(Plugin.ProfilePadButton.Value);
            if (Plugin.ProfileKey.Value.IsDown() || padProfile)
            {
                var p = Cast.Of(_target);
                if (p == null && padProfile && _profile == null) _listOpen = !_listOpen; // pad: nobody aimed at = the list
                else _profile = p == null || p == _profile ? null : p;
            }
            if (_target != null && Shoplifting.IsStealing(_target) && InputManager.GetKeyDownAction(EGameAction.InteractLeft))
            {
                var thief = Cast.Of(_target);
                Shoplifting.Catch(_target);
                IgnoreClick(player) = true; // this click was for them, not for what's behind them
                Say($"Caught {thief?.name}! They're going back to pay.");
            }
            if (_target != null && (Plugin.KickOutKey.Value.IsDown() || PadInput.Down(Plugin.KickOutPadButton.Value))) TryKick(_target);
            ShowCard();
        }

        /// <summary>Whoever you look at, for a moment after you look away; someone you kept on screen before them.</summary>
        private static void ShowCard()
        {
            var looked = Plugin.CustomerInspectOnLook.Value ? _target : null;
            var player = Core.GameScene.Existing<InteractionPlayerController>();
            var worker = _profile == null && Plugin.CustomerInspectOnLook.Value && player?.m_Cam != null ? FindWorker(player.m_Cam, looked) : null;
            if (worker != null)
            {
                _lastWorker = worker;
                _lastSeen = null;
                _seenUntil = Time.time + LingerSeconds;
            }
            else if (looked != null) _lastWorker = null;
            if (_profile == null && _lastWorker != null && _lastWorker.m_IsActive && Time.time < _seenUntil)
            {
                CustomerCard.ShowEmployee(_lastWorker, null);
                return;
            }
            if (looked != null)
            {
                _lastSeen = Cast.Of(looked);
                _lastSeenCustomer = looked;
                _seenUntil = Time.time + LingerSeconds;
            }
            var feedback = _feedback != null && Time.time < _feedbackUntil ? _feedback : null;
            var person = _profile ?? (Time.time < _seenUntil ? _lastSeen : null);
            var customer = person == null ? null : person == _lastSeen && _lastSeenCustomer != null ? _lastSeenCustomer : Cast.CustomerOf(person);
            if (customer != null && Cast.Of(customer) != person) customer = Cast.CustomerOf(person); // the object has a new visitor by now
            CustomerCard.Show(person, customer, feedback ?? Footer(person, customer));
        }

        /// <summary>The employee you aim at, when they're nearer than the customer you look at.</summary>
        private static Worker FindWorker(Camera cam, Customer customer)
        {
            Worker best = null;
            float nearest = float.MaxValue;
            if (customer != null && Aimed(cam, customer.transform.position, BodyHeight, BodyRadius, ThiefDistance, out float theirs)) nearest = theirs;
            foreach (var w in Hooks.WorkerLife.Active())
                if (Aimed(cam, w.transform.position, BodyHeight, BodyRadius, TargetDistance, out float along) && along < nearest) { nearest = along; best = w; }
            return best;
        }

        private static string Footer(Persona person, Customer customer)
        {
            if (person == null || customer == null) return null;
            if (Shoplifting.IsStealing(customer)) return GameStyle.Warning(PadInput.Active ? "Stealing! Press interact to stop them." : "Stealing! Left click to stop them.");
            string kick = KeyLabel(Plugin.KickOutKey.Value.ToString(), Plugin.KickOutPadButton.Value);
            string keep = KeyLabel(Plugin.ProfileKey.Value.ToString(), Plugin.ProfilePadButton.Value);
            return $"{kick}: kick out   ·   {keep}: {(_profile == person ? "let go" : "keep on screen")}";
        }

        private static void TryKick(Customer c)
        {
            var p = Cast.Of(c);
            if (Shoplifting.IsStealing(c))
            {
                Say(PadInput.Active ? "They're stealing: press interact." : "They're stealing: left click them.");
                return;
            }
            string why = KickOut.Refusal(c);
            if (why != null) { Say($"Can't kick {p?.name} out right now ({why})."); return; }
            string how = KickOut.Kick(c);
            if (how == null) return;
            Say(KickOut.Kicks(p) >= KickOut.BanAfter ? $"{p?.name} {how} and is banned from the shop." : $"{p?.name} {how}.");
        }

        /// <summary>A short message at the top of the screen, for something worth noticing.</summary>
        public static void Announce(string text)
        {
            _announcement = text;
            _announcedUntil = Time.time + AnnounceSeconds;
        }

        private static void Say(string text)
        {
            _feedback = text;
            _feedbackUntil = Time.time + 3f;
        }

        /// <summary>
        /// How far along your aim this body is, when your aim passes close enough to any part of it, feet to head: a person is tall and thin, so a single point on
        /// them is easy to miss up close. The game has no collider on customers to ask.
        /// </summary>
        private static bool Aimed(Camera cam, Vector3 feet, float height, float radius, float reach, out float along)
        {
            var origin = cam.transform.position;
            var forward = cam.transform.forward;
            float closest = float.MaxValue;
            along = 0f;
            for (float y = 0f; ; y += BodyStep)
            {
                float at = Mathf.Min(y, height);
                var to = feet + Vector3.up * at - origin;
                float depth = Vector3.Dot(to, forward);
                float miss = (to - forward * depth).magnitude;
                if (depth > 0f && miss < closest) { closest = miss; along = depth; }
                if (at >= height) break;
            }
            return closest <= radius && along <= reach;
        }

        /// <summary>The named customer you aim at, the nearest if several; a thief comes first, so nobody in between takes the click.</summary>
        private static Customer FindTarget(Camera cam)
        {
            foreach (var t in Shoplifting.Thieves)
                if (Aimed(cam, t.Key.transform.position, BodyHeight, ThiefRadius, ThiefDistance, out _)) return t.Key;
            Customer best = null;
            float nearest = float.MaxValue;
            foreach (var kv in Cast.Visitors())
            {
                float height = BodyHeight - Runtime.Customers.Dialogue.SitDrop(kv.Key);
                if (Aimed(cam, kv.Key.transform.position, height, BodyRadius, TargetDistance, out float along) && along < nearest) { nearest = along; best = kv.Key; }
            }
            return best;
        }

        public static void Draw()
        {
            if (!Cast.Enabled || !Core.GameScene.InLoadedShop) return;
            var player = Core.GameScene.Existing<InteractionPlayerController>();
            if (player == null || player.IsInUIMode()) return;
            var cam = player.m_Cam;
            if (cam == null) return;
            Styles();
            DrawPlates(cam);
            DrawThiefAlert();
            if (_profile != null)
            {
                var c = Cast.CustomerOf(_profile);
                var card = CustomerCard.ScreenRect();
                if (Plugin.CustomerDebug.Value && c != null && card.width > 0f) CustomerDebugPanel.Draw(c, _profile, new Rect(card.x - card.width - 12f, card.y, card.width, 0f));
            }
            else if (_listOpen) DrawList();
        }

        private static void Styles()
        {
            if (_plate != null) return;
            _plate = new GUIStyle(GUI.skin.label) { alignment = TextAnchor.MiddleCenter, fontStyle = FontStyle.Bold };
            _list = new GUIStyle(GUI.skin.label) { richText = true, wordWrap = true };
            _alert = new GUIStyle(GUI.skin.label) { alignment = TextAnchor.MiddleCenter, fontStyle = FontStyle.Bold, richText = true };
        }

        private static void Shadowed(Rect r, string text, GUIStyle style, Color color)
        {
            style.normal.textColor = new Color(0f, 0f, 0f, 0.85f);
            GUI.Label(new Rect(r.x + 1.5f, r.y + 1.5f, r.width, r.height), text, style);
            style.normal.textColor = color;
            GUI.Label(r, text, style);
        }

        private static void DrawPlates(Camera cam)
        {
            float maxDist = Plugin.NameplateDistance.Value;
            foreach (var kv in Cast.Visitors())
            {
                bool thief = Shoplifting.IsStealing(kv.Key);
                if (!thief && !Plugin.CustomerNameplates.Value && kv.Key != _target) continue;
                if (!thief && Plugin.NameplatesLookedAtOnly.Value && kv.Key != _target) continue; // thieves always show: you must find them
                Vector3 head = kv.Key.transform.position + Vector3.up * (2.05f - Runtime.Customers.Dialogue.SitDrop(kv.Key));
                float dist = Vector3.Distance(cam.transform.position, head);
                if (dist > (thief ? 30f : maxDist) && kv.Key != _target) continue;
                Vector3 s = cam.WorldToScreenPoint(head);
                if (s.z <= 0f) continue;
                _plate.fontSize = Mathf.RoundToInt(Mathf.Lerp(Screen.height / 40f, Screen.height / 70f, Mathf.Clamp01(dist / maxDist)));
                var p = kv.Value;
                string nick = Nicknames.Of(p);
                string text = (thief ? p.name + "\nStealing!" : p.name) + (nick != null ? $"\n\"{nick}\"" : "");
                float h = _plate.fontSize * ((thief ? 3.6f : 2f) + (nick != null ? 1.6f : 0f)); // a fixed height cut big names off at the top and bottom
                Shadowed(new Rect(s.x - 300f, Screen.height - s.y - h / 2f, 600f, h), text, _plate, thief ? Red : p.regular ? Gold : Color.white);
            }
        }

        private static void DrawThiefAlert()
        {
            _alert.fontSize = Mathf.RoundToInt(Screen.height / 38f);
            var r = new Rect(0f, Screen.height * 0.08f, Screen.width, Screen.height / 18f);
            var thief = Shoplifting.Thieves.Select(t => Cast.Of(t.Key)).FirstOrDefault(p => p != null);
            if (thief != null)
            {
                Shadowed(r, $"{thief.name} is walking out without paying! Look at them and {(PadInput.Active ? "press interact" : "left click")}.", _alert, Red);
            }
            else if (Shoplifting.Notice != null && Time.time < Shoplifting.NoticeUntil)
            {
                Shadowed(r, Shoplifting.Notice, _alert, Red);
            }
            else if (_announcement != null && Time.time < _announcedUntil)
            {
                Shadowed(r, _announcement, _alert, Gold);
            }
        }

        /// <summary>The controller button's name while a controller is in use (and one is set), else the key.</summary>
        private static string KeyLabel(string key, CustomerPadButton pad) =>
            PadInput.Active && PadInput.Label(pad) is string label ? label : key;

        /// <summary>The lines in a box at the left; a picture (a portrait) at its top right, the text beside it.</summary>
        private static Rect Panel(List<string> lines, Sprite picture = null)
        {
            _list.fontSize = Mathf.RoundToInt(Screen.height / 60f);
            float width = Screen.width * 0.42f, pic = picture != null ? Screen.height * 0.13f : 0f;
            float textWidth = width - 20f - (pic > 0 ? pic + 10f : 0f);
            float y = 8f;
            var heights = lines.Select(l => _list.CalcHeight(new GUIContent(l), textWidth)).ToList();
            var box = new Rect(12f, Screen.height * 0.12f, width, Mathf.Max(heights.Sum(), pic) + 16f);
            GUI.Box(box, GUIContent.none);
            GUI.Box(box, GUIContent.none); // twice = darker
            if (pic > 0)
            {
                var t = picture.texture;
                var r = picture.textureRect;
                GUI.DrawTextureWithTexCoords(new Rect(box.xMax - pic - 8f, box.y + 8f, pic, pic), t,
                    new Rect(r.x / t.width, r.y / t.height, r.width / t.width, r.height / t.height));
            }
            for (int i = 0; i < lines.Count; i++)
            {
                GUI.Label(new Rect(box.x + 10f, box.y + y, textWidth, heights[i]), lines[i], _list);
                y += heights[i];
            }
            return box;
        }

        public static IEnumerable<string> Summaries(Persona p)
        {
            foreach (var m in Cast.Modules)
            {
                string s = null;
                try { s = m.Summary(p); }
                catch (System.Exception e) { Plugin.Log.LogError($"Customers: {m.Id} summary failed: {e.Message}"); }
                if (!string.IsNullOrEmpty(s)) yield return s;
            }
        }

        private static void DrawList()
        {
            var regulars = Cast.AllRegulars.OrderByDescending(p => Cast.IsInShop(p)).ThenByDescending(p => p.lastDay).ThenBy(p => p.name).ToList();
            var lines = new List<string> { $"<b>Regulars ({regulars.Count}/{Plugin.MaxRegulars.Value})</b>" };
            if (regulars.Count == 0) lines.Add("No regulars yet: customers who spend well may come back.");
            foreach (var p in regulars.Take(15))
            {
                string seen = Cast.IsInShop(p) ? "<color=#7CFC7C>in the shop</color>" : p.lastDay < 0 ? "not seen yet" : $"last day {p.lastDay}";
                string extra = string.Join(" · ", Summaries(p).Take(2));
                lines.Add($"<color=#FFD24D>{p.name}</color>{(Nicknames.Of(p) is string nick ? $" \"{nick}\"" : "")} ({string.Join(", ", p.traits.Select(Traits.NameOf))}) · {p.visits} visit(s) · {seen}" +
                          (extra.Length > 0 ? "\n    " + extra : ""));
            }
            if (regulars.Count > 15) lines.Add($"… and {regulars.Count - 15} more");
            string profileKey = KeyLabel(Plugin.ProfileKey.Value.ToString(), Plugin.ProfilePadButton.Value);
            lines.Add($"<size={Mathf.RoundToInt(Screen.height / 75f)}>Look at a customer to see more about them.</size>");
            Panel(lines);
        }
    }
}
