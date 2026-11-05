using System.Collections.Generic;
using System.Linq;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] Friendships: customers who get along become friends. A bond grows with finished chats (+1),
    /// fair trades (+2) and table games together (+1); at <see cref="FriendAt"/> they're friends (at most
    /// <see cref="MaxFriends"/> each, both ways), which goes into the gossip log. A noticed rip-off ends it; rivals never
    /// become friends. Friends seek each other out to chat (a choice), wave when they spot each other, cheer each other's
    /// pulls and console each other's bad luck, and a regular is likelier to visit while a friend is in the shop.
    /// Saved per persona: bonds (id, score, friend), at most <see cref="MaxBonds"/>.
    /// </summary>
    internal class Friends : IPersonaModule
    {
        public class Bond
        {
            public string id;
            public int s;     // how well they get along
            public bool f;    // friends
        }

        public class Row
        {
            public List<Bond> b;
        }

        private const int FriendAt = 3; // with 30 regulars the same two rarely meet: more and nobody ever got there
        private const int MaxFriends = 3;
        private const int MaxBonds = 6;
        private const float MeetWeight = 0.6f; // a friend in the shop: chance to go and chat with them, against vanilla's 1

        public string Id => "friends";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerFriendships.Value;

        private static readonly PersonaTable<Row> Table = new PersonaTable<Row>();

        /// <summary>Per visit: met up with a friend already, waved already.</summary>
        private class Visit
        {
            public bool Met, Waved;
        }

        public void Init()
        {
            Chatting.ChatFinished += (a, b) => Grow(a, b, 1, "chatted");
            Trading.TradedFairly += (a, b) => Grow(a, b, 2, "traded");
            ShopEvents.CustomerMatchEnded += (a, b, aWon) => Grow(a, b, 1, "played a game");
            Trading.RipOffNoticed += (victim, hustlerId) => Break(victim, hustlerId);
            // Friends seek each other out.
            CustomerChoices.Register("friend", c =>
            {
                var p = On ? Cast.Of(c) : null;
                if (p == null || Cast.PeekVisitData<Visit>(c)?.Met == true) return 0f;
                return FriendHere(p, free: true) != null ? MeetWeight : 0f;
            }, c =>
            {
                var p = Cast.Of(c);
                var friend = FriendHere(p, free: true);
                var v = Cast.VisitData<Visit>(c);
                if (friend == null || v == null) return false;
                v.Met = true;
                return Chatting.StartWith(c, friend, "friendHello");
            });
            // Spotting a friend: a wave, once a visit.
            CustomerLife.SlowTick.Add(c =>
            {
                var p = On ? Cast.Of(c) : null;
                var friend = p == null ? null : FriendHere(p, free: false);
                if (friend == null || Vector3.Distance(friend.transform.position, c.transform.position) > 10f) return;
                var v = Cast.VisitData<Visit>(c);
                if (v == null || v.Waved) return;
                v.Waved = true;
                CustomerLife.Glance(c, friend.transform.position);
                Dialogue.Say(c, "friendSpot", new Dialogue.Tokens { Other = Cast.Of(friend)?.name }, 0.8f);
            });
            // A friend's big pull: a cheer (instead of the crowd's line).
            Crowd.Overrides.Add((watcher, wp, who, p, card, price) =>
            {
                if (!On || !AreFriends(wp, p.id)) return false;
                Dialogue.Say(watcher, "friendHit", new Dialogue.Tokens { Other = p.name, Card = CardInfo.Name(card), Price = price }, 1f, Random.Range(0.3f, 1.2f));
                return true;
            });
            PackRipping.Bricked += (c, p) => Comfort(c, p, "friendBrick");
            KickOut.KickedOut += (c, p) => Comfort(c, p, "friendShamed");
            Shoplifting.CaughtStealing += (c, p) => Comfort(c, p, "friendShamed");
            // Friends come by together: a regular is likelier to visit while a friend is in the shop.
            Cast.VisitWeight.Add((p, w) => On && FriendIds(p).Any(id => Cast.Visitors().Any(v => v.Value.id == id)) ? w * 3f : w);
            UI.CustomerDebugPanel.Section("Friends", (c, p, rows) =>
            {
                if (!On) return;
                var bonds = Table.Get(p, create: false)?.b;
                if (bonds == null || bonds.Count == 0) { rows.Add(("friends", "none yet")); return; }
                foreach (var b in bonds.OrderByDescending(x => x.s))
                    rows.Add((NameOf(b.id), b.f ? $"<color=#7CFC7C>friend</color> ({b.s})" : $"getting along {b.s}/{FriendAt}"));
            });
        }

        private static string NameOf(string id) => Cast.Regular(id)?.name ?? Cast.Visitors().FirstOrDefault(v => v.Value.id == id).Value?.name ?? "a walk-in";

        private static IEnumerable<string> FriendIds(Persona p) =>
            Table.Get(p, create: false)?.b?.Where(b => b.f).Select(b => b.id) ?? Enumerable.Empty<string>();

        public static IEnumerable<string> FriendNames(Persona p) =>
            Table.Get(p, create: false)?.b?.Where(b => b.f).Select(b => Cast.Regular(b.id)?.name).Where(n => n != null) ?? Enumerable.Empty<string>();

        public static bool AreFriends(Persona p, string otherId) => Table.Get(p, create: false)?.b?.Exists(b => b.f && b.id == otherId) == true;

        /// <summary>A friend in the shop now (free to be drawn into a chat, if asked), or null.</summary>
        private static Customer FriendHere(Persona p, bool free)
        {
            var ids = new HashSet<string>(FriendIds(p));
            if (ids.Count == 0) return null;
            foreach (var v in Cast.Visitors())
                if (ids.Contains(v.Value.id) && (!free || CustomerLife.IsFree(v.Key))) return v.Key;
            return null;
        }

        private static Bond BondOf(Persona p, string otherId, bool create)
        {
            var row = Table.Get(p, create);
            if (row == null) return null;
            if (row.b == null) row.b = new List<Bond>();
            var bond = row.b.Find(b => b.id == otherId);
            if (bond != null || !create) return bond;
            if (row.b.Count >= MaxBonds)
            {
                // Full: the weakest acquaintance (not a friend) makes room.
                var weakest = row.b.Where(b => !b.f).OrderBy(b => b.s).FirstOrDefault();
                if (weakest == null) return null;
                row.b.Remove(weakest);
            }
            bond = new Bond { id = otherId };
            row.b.Add(bond);
            return bond;
        }

        private static void Grow(Customer a, Customer b, int by, string how)
        {
            if (!On) return;
            Persona pa = Cast.Of(a), pb = Cast.Of(b);
            // Only regulars: a walk-in is forgotten when they leave, and the bond with them with it.
            if (pa == null || pb == null || pa == pb || !pa.regular || !pb.regular || Rivalries.AreRivals(pa, pb)) return;
            var ba = BondOf(pa, pb.id, create: true);
            var bb = BondOf(pb, pa.id, create: true);
            if (ba == null || bb == null) return;
            by += Mathf.RoundToInt(Traits.Effect(pa, "bond") + Traits.Effect(pb, "bond")); // a Mentor hits it off with anyone
            ba.s += by;
            bb.s += by;
            if (ba.f || ba.s < FriendAt) return;
            if (FriendIds(pa).Count() >= MaxFriends || FriendIds(pb).Count() >= MaxFriends) return;
            ba.f = bb.f = true;
            News.Add("friends", pa.name, pb.name);
            Dialogue.Say(a, "friendMade", new Dialogue.Tokens { Other = pb.name }, 1f, 1f);
            Plugin.Log.LogInfo($"Customers: {pa.name} and {pb.name} are friends now ({how})");
        }

        private static void Break(Persona victim, string otherId)
        {
            var b = BondOf(victim, otherId, create: false);
            var other = Cast.Regular(otherId) ?? Cast.Visitors().FirstOrDefault(v => v.Value.id == otherId).Value;
            var back = other == null ? null : BondOf(other, victim.id, create: false);
            bool were = b?.f == true;
            if (b != null) { b.f = false; b.s = -5; }
            if (back != null) { back.f = false; back.s = -5; }
            if (were) Plugin.Log.LogInfo($"Customers: {victim.name} and {other?.name ?? "a walk-in"} aren't friends any more (a rip-off)");
        }

        private static void Comfort(Customer c, Persona p, string situation)
        {
            if (!On) return;
            foreach (var v in Cast.Visitors())
            {
                if (v.Key == c || !AreFriends(v.Value, p.id) || Vector3.Distance(v.Key.transform.position, c.transform.position) > 12f) continue;
                CustomerLife.Glance(v.Key, c.transform.position);
                Dialogue.Say(v.Key, situation, new Dialogue.Tokens { Other = p.name }, 0.9f, Random.Range(0.5f, 1.5f));
            }
        }

        public void Load(JToken state) => Table.Load(state);

        public JToken Save(ICollection<string> keep)
        {
            // Bonds to someone who isn't kept (a walk-in who left) are forgotten.
            foreach (var kv in Table.Rows) kv.Value.b?.RemoveAll(b => !keep.Contains(b.id));
            return Table.Save(keep);
        }

        public string Summary(Persona p)
        {
            if (!On) return null;
            var names = FriendIds(p).Select(NameOf).ToList();
            return names.Count == 0 ? null : "friends with " + string.Join(", ", names);
        }
    }
}
