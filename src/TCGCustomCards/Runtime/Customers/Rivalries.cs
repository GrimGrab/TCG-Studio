using System.Collections.Generic;
using System.Linq;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] Rivalries: table games between named customers are decided by table skill (instead of vanilla's
    /// coin flip, unless a tournament format decides), records are kept, and repeated losses to the same opponent (more for sore
    /// losers: effect "grudge") make a rival. Beating a customer often makes you their rival: they ask for rematches.
    /// Saved per persona: wins/losses, record against you, the 3 opponents played most, and the rival's id ("player" = you).
    /// </summary>
    internal class Rivalries : IPersonaModule
    {
        public class Opp
        {
            public string id;
            public int w, l;
        }

        public class Row
        {
            public int w, l;      // all table games against customers
            public int pw, pl;    // against you: they won / they lost
            public List<Opp> o;   // most played opponents, at most 3
            public string rival;  // persona id, "player", or null
            public string why;    // how it started: "tables", "ripoff" (a trade), "you" (lost to you)
        }

        public const string PlayerRival = "player";

        public string Id => "rivalries";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerRivalries.Value;

        private static readonly PersonaTable<Row> Table = new PersonaTable<Row>();

        public void Init()
        {
            ShopEvents.CustomerMatchDecider = (c0, c1) =>
            {
                var p0 = On ? Cast.Of(c0) : null;
                var p1 = On ? Cast.Of(c1) : null;
                if (p0 == null || p1 == null) return null;
                Cast.EnsureSkill(p0);
                Cast.EnsureSkill(p1);
                float chance0 = Mathf.Clamp(0.5f + (p0.skill - p1.skill) / 120f, 0.2f, 0.8f);
                return Random.value < chance0;
            };
            ShopEvents.CustomerMatchEnded += (c0, c1, seat0Won) =>
            {
                var p0 = On ? Cast.Of(c0) : null;
                var p1 = On ? Cast.Of(c1) : null;
                if (p0 == null || p1 == null) return;
                var (wc, wp, lc, lp) = seat0Won ? (c0, p0, c1, p1) : (c1, p1, c0, p0);
                bool rivals = Table.Get(wp, false)?.rival == lp.id || Table.Get(lp, false)?.rival == wp.id;
                Record(wp, lp, won: true);
                Record(lp, wp, won: false);
                if (rivals)
                {
                    Dialogue.Say(wc, "rivalWin", new Dialogue.Tokens { Rival = lp.name }, 1f);
                    Dialogue.Say(lc, "rivalLose", new Dialogue.Tokens { Rival = wp.name }, 1f, Dialogue.Gap);
                    News.Add("duel", wp.name, lp.name);
                }
                else
                {
                    Dialogue.Say(wc, "winTable", null, 0.5f);
                    Dialogue.Say(lc, "loseTable", null, 0.6f, Dialogue.Gap);
                }
            };
            TableSession.PlayerMatchEnded += (opponent, playerWon, draw) =>
            {
                var p = On ? Cast.Of(opponent) : null;
                if (p == null || draw) return;
                var row = Table.Get(p);
                if (playerWon)
                {
                    row.pl++;
                    Dialogue.Say(opponent, "lostToPlayer", null, 0.9f, 1f);
                    if (row.rival == null && row.pl >= 3 && row.pl * Traits.Effect(p, "grudge") > row.pw + 2) SetRival(p, row, PlayerRival, "you");
                }
                else
                {
                    row.pw++;
                    Dialogue.Say(opponent, "beatPlayer", null, 0.9f, 1f);
                    Cast.Promote(p, "beat you at a table");
                }
            };
            Cast.EnteredShop += (c, p) =>
            {
                if (!On) return;
                if (Table.Get(p, false)?.rival == PlayerRival) Dialogue.Say(c, "challenge", null, 0.4f, Random.Range(4f, 12f));
            };
            InitRivals();
        }

        // ---- Rivals in the shop: they seek each other out (a duel at the tables) and react to each other.

        private const float DuelWeight = 0.7f; // against vanilla's 1 when deciding what next, both free and a table open

        /// <summary>Per visit: challenged their rival already, spotted them already.</summary>
        private class RivalVisit
        {
            public bool Challenged, Spotted;
        }

        private class Duel
        {
            public Customer A, B; // A challenges B
            public Persona PA, PB;
            public int Phase;
            public float NextAt;
            public bool Ended;
        }

        private class Side
        {
            public Duel Duel;
            public bool Challenger;
        }

        private static void InitRivals()
        {
            CustomerStates.Register(new ChallengeState());
            CustomerStates.Register(new FaceState());
            CustomerChoices.Register("rival", c =>
            {
                var p = On ? Cast.Of(c) : null;
                var rival = RivalHere(p);
                if (rival == null || Cast.PeekVisitData<RivalVisit>(c)?.Challenged == true) return 0f;
                if (!CustomerLife.IsFree(rival) || !CustomerLife.CanPlayTable(c) || !CustomerLife.CanPlayTable(rival)) return 0f;
                return DuelWeight;
            }, c =>
            {
                var p = Cast.Of(c);
                var rival = RivalHere(p);
                var v = Cast.VisitData<RivalVisit>(c);
                if (rival == null || v == null || !CustomerLife.IsFree(rival)) return false;
                v.Challenged = true;
                var duel = new Duel { A = c, B = rival, PA = p, PB = Cast.Of(rival) };
                CustomerStates.Goto(rival, FaceState.StateId, new Side { Duel = duel, Challenger = false });
                return CustomerStates.Goto(c, ChallengeState.StateId, new Side { Duel = duel, Challenger = true });
            });
            // Spotting their rival: a jab, once a visit.
            CustomerLife.SlowTick.Add(c =>
            {
                var p = On ? Cast.Of(c) : null;
                var rival = RivalHere(p);
                if (rival == null || Vector3.Distance(rival.transform.position, c.transform.position) > 8f) return;
                var v = Cast.VisitData<RivalVisit>(c);
                if (v == null || v.Spotted) return;
                v.Spotted = true;
                CustomerLife.Glance(c, rival.transform.position);
                Dialogue.Say(c, "rivalSpot", new Dialogue.Tokens { Rival = Cast.Of(rival)?.name }, 0.8f);
            });
            // Their rival pulls a big card: jealousy instead of congratulations.
            Crowd.Overrides.Add((watcher, wp, who, p, card, price) =>
            {
                if (!On || CustomerRival(wp) != p.id) return false;
                Dialogue.Say(watcher, "rivalHit", new Dialogue.Tokens { Rival = p.name, Card = CardInfo.Name(card), Price = price }, 0.9f, Random.Range(0.4f, 1.4f));
                return true;
            });
            // Their rival bricks, gets kicked or gets caught stealing: they gloat.
            PackRipping.Bricked += (c, p) => Gloat(c, p, "rivalBrick");
            KickOut.KickedOut += (c, p) => Gloat(c, p, "rivalShamed");
            Shoplifting.CaughtStealing += (c, p) => Gloat(c, p, "rivalShamed");
            UI.CustomerDebugPanel.Section("Rivalry", (c, p, rows) =>
            {
                if (!On) return;
                var row = Table.Get(p, false);
                if (row?.rival == null) { rows.Add(("rival", "none")); return; }
                rows.Add(("rival", (row.rival == PlayerRival ? "you" : NameOf(row.rival)) + $" ({Why(row.why)})"));
                var opp = row.o?.Find(o => o.id == row.rival);
                if (opp != null) rows.Add(("head to head", $"{opp.w}-{opp.l}"));
                if (row.rival == PlayerRival) rows.Add(("vs you", $"{row.pw}-{row.pl}"));
                if (RivalHere(p) != null) rows.Add(("in the shop", Cast.PeekVisitData<RivalVisit>(c)?.Challenged == true ? "yes, challenged them" : "yes"));
            });
        }

        private static string Why(string why) => why == "ripoff" ? "ripped them off in a trade" : why == "you" ? "lost to you too often" : why == "tables" ? "lost to them at the tables" : "a grudge";

        private static void Gloat(Customer c, Persona p, string situation)
        {
            if (!On) return;
            foreach (var v in Cast.Visitors())
            {
                if (v.Key == c || CustomerRival(v.Value) != p.id || Vector3.Distance(v.Key.transform.position, c.transform.position) > 12f) continue;
                CustomerLife.Glance(v.Key, c.transform.position);
                Dialogue.Say(v.Key, situation, new Dialogue.Tokens { Rival = p.name }, 0.9f, Random.Range(0.5f, 1.5f));
            }
        }

        private static void EndDuel(Duel d, bool toTables)
        {
            if (d.Ended) return;
            d.Ended = true;
            // The challenger books a table first; the game seats the next player at a table with someone waiting (80%).
            foreach (var x in new[] { d.A, d.B })
            {
                if (x == null || !(CustomerStates.Of(x)?.Data is Side s) || s.Duel != d) continue;
                CustomerStates.ToVanilla(x, toTables ? (System.Action<Customer>)(c => { if (!CustomerLife.GoPlayTable(c)) CustomerLife.Shop(c); }) : CustomerLife.Shop);
            }
            if (toTables) Plugin.Log.LogInfo($"Customers: {d.PA.name} challenged their rival {d.PB.name} to a game");
        }

        /// <summary>Walks up to the rival, calls them out, hears the answer, then both head for the tables.</summary>
        private class ChallengeState : ModState
        {
            public const string StateId = "rivalries.challenge";
            public override string Id => StateId;

            public override void Enter(Customer c, StateContext ctx)
            {
                var d = ((Side)ctx.Data).Duel;
                var to = d.B.transform.position;
                var dir = c.transform.position - to;
                dir.y = 0f;
                dir = dir.sqrMagnitude < 0.01f ? Vector3.forward : dir.normalized;
                ctx.WalkTo(to + dir * 0.9f);
            }

            public override void Tick(Customer c, StateContext ctx)
            {
                var d = ((Side)ctx.Data).Duel;
                if (d.Ended) return;
                if (d.B == null || !d.B.m_IsActive || !(CustomerStates.Of(d.B)?.Data is Side s) || s.Duel != d) { EndDuel(d, false); return; }
                if (!ctx.HasArrived || (d.Phase == 0 && !CustomerLife.StepCloser(c, d.B.transform.position, 0.9f)))
                {
                    if (ctx.Elapsed > 15f) EndDuel(d, false);
                    return;
                }
                CustomerLife.FaceTowards(c, d.B.transform.position);
                if (Time.time < d.NextAt) return;
                switch (d.Phase++)
                {
                    case 0:
                        Dialogue.Say(d.A, "rivalMeet", new Dialogue.Tokens { Rival = d.PB.name }, 1f);
                        d.NextAt = Time.time + Dialogue.Gap;
                        break;
                    case 1:
                        Dialogue.Say(d.B, "rivalAccept", new Dialogue.Tokens { Rival = d.PA.name }, 1f);
                        d.NextAt = Time.time + 2.2f;
                        break;
                    default:
                        EndDuel(d, true);
                        break;
                }
            }

            public override void Exit(Customer c, StateContext ctx) => EndDuel(((Side)ctx.Data).Duel, false);
        }

        /// <summary>The challenged rival stops and faces the challenger.</summary>
        private class FaceState : ModState
        {
            public const string StateId = "rivalries.face";
            public override string Id => StateId;

            public override void Enter(Customer c, StateContext ctx)
            {
                CustomerLife.Stand(c);
                CustomerLife.FaceTowards(c, ((Side)ctx.Data).Duel.A.transform.position);
            }

            public override void Tick(Customer c, StateContext ctx)
            {
                var d = ((Side)ctx.Data).Duel;
                if (d.Ended) return;
                if (d.A == null || !d.A.m_IsActive) { EndDuel(d, false); return; }
                CustomerLife.FaceTowards(c, d.A.transform.position);
                if (ctx.Elapsed > 40f) EndDuel(d, false);
            }

            public override void Exit(Customer c, StateContext ctx) => EndDuel(((Side)ctx.Data).Duel, false);
        }

        private static void Record(Persona me, Persona them, bool won)
        {
            var row = Table.Get(me);
            if (won) row.w++; else row.l++;
            if (!me.regular || !them.regular) return; // a walk-in is forgotten when they leave, and a grudge against them with it
            if (row.o == null) row.o = new List<Opp>();
            var opp = row.o.Find(o => o.id == them.id);
            if (opp == null)
            {
                // Keep the 3 most played: a new opponent replaces the least played one.
                if (row.o.Count >= 3)
                {
                    var least = row.o.OrderBy(o => o.w + o.l).First();
                    if (least.id == row.rival) least = row.o.Where(o => o.id != row.rival).OrderBy(o => o.w + o.l).First();
                    row.o.Remove(least);
                }
                opp = new Opp { id = them.id };
                row.o.Add(opp);
            }
            if (won) opp.w++; else opp.l++;
            if (row.rival == null && !won && opp.l >= 2 && opp.l * Traits.Effect(me, "grudge") >= opp.w + 1) SetRival(me, row, them.id, "tables");
        }

        /// <summary>Makes <paramref name="otherId"/> their rival when they have none yet (a grudge from elsewhere, e.g. a trade rip-off).</summary>
        public static void MakeRival(Persona p, string otherId, string why)
        {
            if (!On || p == null || otherId == null || !p.regular || (otherId != PlayerRival && Cast.Regular(otherId) == null)) return;
            var row = Table.Get(p);
            if (row.rival == null) SetRival(p, row, otherId, why);
        }

        private static void SetRival(Persona p, Row row, string id, string why)
        {
            row.rival = id;
            row.why = why;
            string who = id == PlayerRival ? "you" : NameOf(id);
            Plugin.Log.LogInfo($"Customers: {p.name} now sees {who} as their rival ({why})");
            if (id != PlayerRival) News.Add("rivalry", p.name, who);
        }

        private static string NameOf(string id) => Cast.Regular(id)?.name ?? Cast.Visitors().FirstOrDefault(v => v.Value.id == id).Value?.name ?? "a walk-in";

        /// <summary>The persona id of their customer rival (not you), or null.</summary>
        private static string CustomerRival(Persona p)
        {
            var id = p == null ? null : Table.Get(p, false)?.rival;
            return id == PlayerRival ? null : id;
        }

        /// <summary>Their rival when they're in the shop now, or null.</summary>
        private static Customer RivalHere(Persona p)
        {
            var id = CustomerRival(p);
            if (id == null) return null;
            foreach (var v in Cast.Visitors()) if (v.Value.id == id) return v.Key;
            return null;
        }

        /// <summary>Their table wins against customers.</summary>
        public static int Wins(Persona p) => Table.Get(p, false)?.w ?? 0;

        /// <summary>Their table record: all games against customers, and against you.</summary>
        public static (int wins, int losses, int beatYou, int lostToYou) Record(Persona p)
        {
            var row = Table.Get(p, false);
            return row == null ? (0, 0, 0, 0) : (row.w, row.l, row.pw, row.pl);
        }

        /// <summary>Who they see as their rival ("you" for the player), or null.</summary>
        public static string RivalName(Persona p)
        {
            var id = Table.Get(p, false)?.rival;
            return id == null ? null : id == PlayerRival ? "you" : Cast.Regular(id)?.name;
        }

        public static bool AreRivals(Persona a, Persona b) => CustomerRival(a) == b.id || CustomerRival(b) == a.id;

        public void Load(JToken state) => Table.Load(state);

        public JToken Save(ICollection<string> keep)
        {
            // A rival who isn't kept any more (a walk-in who left) is forgotten.
            foreach (var kv in Table.Rows)
            {
                if (kv.Value.rival != null && kv.Value.rival != PlayerRival && !keep.Contains(kv.Value.rival)) kv.Value.rival = null;
                kv.Value.o?.RemoveAll(o => !keep.Contains(o.id));
            }
            return Table.Save(keep);
        }

        public string Summary(Persona p)
        {
            if (!On) return null;
            var row = Table.Get(p, false);
            if (row == null) return $"skill {p.skill}";
            var parts = new List<string> { $"skill {p.skill}", $"tables {row.w}-{row.l}" };
            if (row.pw + row.pl > 0) parts.Add($"vs you {row.pw}-{row.pl}");
            if (row.rival != null)
            {
                var opp = row.o?.Find(o => o.id == row.rival);
                parts.Add("rival: " + (row.rival == PlayerRival ? "YOU" : NameOf(row.rival)) + $" ({Why(row.why)}" +
                          (opp != null ? $", {opp.w}-{opp.l} against them)" : ")"));
            }
            return string.Join(", ", parts);
        }
    }
}
