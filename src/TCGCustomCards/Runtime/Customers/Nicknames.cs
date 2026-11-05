using System.Collections.Generic;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] Nicknames: regulars earn one for what the shop saw them do (deeds in customer-defaults.json `nicknames`), rarely, and
    /// have one at most. A "title" (Moneybags) has one holder: someone with a better number takes it from whoever holds it with the worst. A
    /// "named" one (Lucky Sam) can be held by <see cref="NamedCap"/> people. When a deed's words are all held and can't be taken, there's nothing to
    /// earn. Someone nearby coins it out loud, and it becomes gossip. A nickname set in TCG Studio is never replaced. Saved: per persona their
    /// nickname and counters, and the last <see cref="LogMax"/> changes (the whiteboard's).
    /// </summary>
    internal class Nicknames : IPersonaModule
    {
        public class Row
        {
            public string n;    // their nickname's word
            public string deed; // what earned it
            public int d;       // the day they got it
            public int huge, bricks, caught, ripoffs; // counters the other modules don't keep (bricks = in a row)
        }

        public class Change
        {
            public int d;
            public string t;
        }

        private const int NamedCap = 2;
        private const int LogMax = 8;
        private const float CoinRange = 12f;

        private static readonly PersonaTable<Row> Table = new PersonaTable<Row>();
        private static List<Change> _log = new List<Change>();

        /// <summary>A nickname was given, taken or lost (the whiteboard redraws).</summary>
        public static event System.Action Changed;

        public string Id => "nicknames";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerNicknames.Value;

        public static IReadOnlyList<Change> Log => _log;

        /// <summary>Their name or nickname changed outside this module.</summary>
        public static void NotifyChanged() => Changed?.Invoke();

        public static int HugePulls(Persona p) => Table.Get(p, create: false)?.huge ?? 0;
        public static int RipOffs(Persona p) => Table.Get(p, create: false)?.ripoffs ?? 0;

        /// <summary>The player names a regular from the whiteboard; empty takes the name away, and they can earn one again. An earned word is freed.</summary>
        public static void SetByPlayer(Persona p, string word)
        {
            string before = Of(p);
            word = string.IsNullOrEmpty(word) ? null : word;
            if (word == before) return;
            var row = Table.Get(p, create: false);
            if (row != null) { row.n = null; row.deed = null; }
            p.nickname = word;
            p.edited = true;
            AddLog(word != null ? $"{p.name} is now {word}" : $"{p.name} is no longer {before}");
            Changed?.Invoke();
        }

        /// <summary>What they're called: the word for a title, "Lucky Sam" for a named one; a nickname from TCG Studio as written. Null when they have none.</summary>
        public static string Of(Persona p)
        {
            if (p == null) return null;
            if (!string.IsNullOrEmpty(p.nickname)) return p.nickname;
            if (!On) return null;
            var row = Table.Get(p, create: false);
            return row?.n == null ? null : Display(p, row.n, Traits.Deed(row.deed));
        }

        private static string Display(Persona p, string word, Traits.NickDeed deed) =>
            deed != null && deed.style == "named" ? word + " " + FirstName(p) : word;

        private static string FirstName(Persona p)
        {
            int space = p.name.IndexOf(' ');
            return space > 0 ? p.name.Substring(0, space) : p.name;
        }

        /// <summary>
        /// How <paramref name="speaker"/> says this name in a line: the nickname's word and the name ("The Shark Sam Rossi", "Lucky Sam Rossi"), for anyone who
        /// has one. Names that aren't a regular's stay as they are.
        /// </summary>
        public static string Spoken(Persona speaker, string name)
        {
            if (!On || string.IsNullOrEmpty(name)) return name;
            foreach (var o in Cast.AllRegulars)
            {
                if (o.name != name || o == speaker) continue;
                string word = !string.IsNullOrEmpty(o.nickname) ? o.nickname : Table.Get(o, create: false)?.n;
                return word != null ? word + " " + name : name;
            }
            return name;
        }

        /// <summary>Every nickname held now, for the whiteboard: (word as shown, who, what earned it, their number). One given by hand has no deed: label null.</summary>
        public static List<(string nick, string who, string label, string number)> Holders()
        {
            var list = new List<(string, string, string, string)>();
            foreach (var p in Cast.AllRegulars)
                if (!string.IsNullOrEmpty(p.nickname)) list.Add((p.nickname, p.name, null, null));
            foreach (var kv in Table.Rows)
            {
                var p = Cast.Regular(kv.Key);
                var deed = Traits.Deed(kv.Value.deed);
                if (p == null || !string.IsNullOrEmpty(p.nickname) || kv.Value.n == null || deed == null) continue;
                float value = Stat(p, kv.Value, deed.stat);
                list.Add((Display(p, kv.Value.n, deed), p.name, deed.label, deed.stat == "spent" ? "$" + value.ToString("0") : value.ToString("0")));
            }
            return list;
        }

        public void Init()
        {
            Whiteboard.Init();
            Cast.EnteredShop += (c, p) => { Fade(p); Check(c, p); };
            Cast.VisitEnded += (c, p, paid) => Check(null, p);
            PackRipping.PulledHit += (c, p, card, price) =>
            {
                if (!On || !p.regular) return;
                var row = Table.Get(p);
                row.bricks = 0;
                if (price / Mathf.Max(0.01f, Plugin.CustomerPackHitPrice.Value) >= 10f) row.huge++;
                if (Traits.Deed(row.deed)?.fadeOnHit == true) Lose(p, row);
                Check(c, p);
            };
            PackRipping.Bricked += (c, p) =>
            {
                if (!On || !p.regular) return;
                Table.Get(p).bricks++;
                Check(c, p);
            };
            Shoplifting.CaughtStealing += (c, p) =>
            {
                if (!On || !p.regular) return;
                Table.Get(p).caught++;
                Check(c, p);
            };
            Trading.RipOffNoticed += (victim, hustlerId) =>
            {
                var hustler = Cast.Regular(hustlerId);
                if (!On || hustler == null) return;
                Table.Get(hustler).ripoffs++;
                Check(null, hustler);
            };
        }

        private static float Stat(Persona p, Row row, string stat)
        {
            switch (stat)
            {
                case "spent": return p.spent;
                case "visits": return p.visits;
                case "wins": return Rivalries.Wins(p);
                case "packs": return PackRipping.PacksOpened(p);
                case "huge": return row.huge;
                case "bricks": return row.bricks;
                case "caught": return row.caught;
                case "ripoffs": return row.ripoffs;
                default: return 0f;
            }
        }

        private static void Check(Customer c, Persona p)
        {
            if (!On || p == null || !p.regular || !string.IsNullOrEmpty(p.nickname)) return;
            var row = Table.Get(p);
            if (row.n != null) return;
            foreach (var deed in Traits.Deeds)
            {
                float value = Stat(p, row, deed.stat);
                if (value < deed.at) continue;
                string word = FreeWord(deed);
                Persona from = null;
                if (word == null && deed.style != "named") word = TakeFromWeakest(deed, value, out from);
                if (word == null) continue;
                row.n = word;
                row.deed = deed.id;
                row.d = CPlayerData.m_CurrentDay;
                Announce(p, word, deed, from);
                return;
            }
        }

        private static int Holding(string word)
        {
            int n = 0;
            foreach (var kv in Table.Rows)
                if (kv.Value.n == word && Cast.Regular(kv.Key) != null) n++; // a regular who's gone frees theirs
            return n;
        }

        private static string FreeWord(Traits.NickDeed deed)
        {
            int cap = deed.style == "named" ? NamedCap : 1;
            var free = new List<string>();
            foreach (var w in Traits.DeedWords(deed))
                if (Holding(w) < cap) free.Add(w);
            return free.Count > 0 ? free[Random.Range(0, free.Count)] : null;
        }

        /// <summary>All this deed's titles are held: the one with the worst number loses theirs, if that's worse than <paramref name="value"/>.</summary>
        private static string TakeFromWeakest(Traits.NickDeed deed, float value, out Persona from)
        {
            from = null;
            Row weakest = null;
            float worst = value;
            foreach (var kv in Table.Rows)
            {
                var o = Cast.Regular(kv.Key);
                if (o == null || kv.Value.deed != deed.id || kv.Value.n == null) continue;
                float v = Stat(o, kv.Value, deed.stat);
                if (v < worst) { worst = v; weakest = kv.Value; from = o; }
            }
            if (weakest == null) return null;
            string word = weakest.n;
            weakest.n = null;
            weakest.deed = null;
            return word;
        }

        private static void Fade(Persona p)
        {
            if (!On || p == null) return;
            var row = Table.Get(p, create: false);
            var deed = Traits.Deed(row?.deed);
            if (deed != null && deed.fadeDays > 0 && CPlayerData.m_CurrentDay - row.d >= deed.fadeDays) Lose(p, row);
        }

        private static void Lose(Persona p, Row row)
        {
            string nick = Display(p, row.n, Traits.Deed(row.deed));
            row.n = null;
            row.deed = null;
            AddLog($"{p.name} is no longer {nick}");
            Changed?.Invoke();
        }

        private static void AddLog(string text)
        {
            _log.Add(new Change { d = CPlayerData.m_CurrentDay, t = text });
            if (_log.Count > LogMax) _log.RemoveAt(0);
        }

        private static void Announce(Persona p, string word, Traits.NickDeed deed, Persona from)
        {
            string nick = Display(p, word, deed);
            AddLog(from != null ? $"{p.name} took {nick} from {from.name}" : $"{p.name} is now {nick}");
            Plugin.Log.LogInfo($"Customers: {p.name} earned the nickname {nick} ({deed.id}){(from != null ? $", taking it from {from.name}" : "")}");
            News.Add(from != null ? "nickTaken" : deed.bad ? "nickBad" : "nick", p.name, from?.name, nick);

            Customer holder = null, loser = null;
            foreach (var v in Cast.Visitors())
            {
                if (v.Value == p) holder = v.Key;
                if (v.Value == from) loser = v.Key;
            }
            if (loser != null) Dialogue.Say(loser, "nickLost", new Dialogue.Tokens { Other = p.name, Nick = nick }, 1f, Dialogue.Gap);
            if (holder != null)
            {
                Customer nearest = null;
                float best = CoinRange;
                foreach (var v in Cast.Visitors())
                {
                    if (v.Key == holder || v.Key == loser || v.Key.m_CurrentState == ECustomerState.ExitingShop) continue;
                    float d = Vector3.Distance(v.Key.transform.position, holder.transform.position);
                    if (d < best) { best = d; nearest = v.Key; }
                }
                if (nearest != null) Dialogue.Say(nearest, from != null ? "nickTaken" : deed.bad ? "nickCoinedBad" : "nickCoined", new Dialogue.Tokens { Other = FirstName(p), Nick = nick }, 1f, 0.5f);
            }
            Changed?.Invoke();
        }

        public void Load(JToken state)
        {
            Table.Load(state?["rows"]);
            _log = state?["log"]?.ToObject<List<Change>>() ?? new List<Change>();
            Whiteboard.LoadViews(state?["boards"]?.ToObject<List<Whiteboard.SavedView>>());
            Changed?.Invoke();
        }

        public JToken Save(ICollection<string> keep)
        {
            var rows = Table.Save(keep);
            var boards = Whiteboard.SaveViews();
            if (rows == null && _log.Count == 0 && boards == null) return null;
            var o = new JObject();
            if (rows != null) o["rows"] = rows;
            if (_log.Count > 0) o["log"] = JToken.FromObject(_log);
            if (boards != null) o["boards"] = JToken.FromObject(boards);
            return o;
        }

        public string Summary(Persona p) => null; // shown beside their name
    }
}
