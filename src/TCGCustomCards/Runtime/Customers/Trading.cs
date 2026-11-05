using System.Collections.Generic;
using System.Linq;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] Trading: every customer keeps a small binder (<see cref="MaxBinder"/> cards, saved for regulars):
    /// a few cards of the shop's sets to start with, then their own pack hits. During a chat (<see cref="Chatting.Exchange"/>)
    /// two customers may swap one card each — the cards they got show above their heads with their values. Fair trades are
    /// close in value and favour wishlist cards. Hustlers (trait effect "hustle") hand over a card worth half or less; the
    /// victim may notice at once or when their visit ends, and then holds a grudge: no more chats or trades with them, the
    /// hustler becomes their rival if they have none, they say so when they see them, and it's in today's gossip.
    /// Saved per persona: binder, trades, rip-offs made / suffered, grudges (persona ids, at most 3).
    /// </summary>
    internal class Trading : IPersonaModule
    {
        public class Card
        {
            public string k;  // CardInfo key
            public int b;     // ECardBorderType
            public bool f;    // foil
        }

        public class Row
        {
            public List<Card> bind;  // the binder, at most MaxBinder
            public int t;            // trades made
            public int ro;           // rip-offs they pulled
            public int sc;           // times they got ripped off
            public List<string> g;   // grudges: persona ids, at most 3
        }

        private const int MaxBinder = 10;
        private const int TradeExp = 2;  // shop XP for a trade in the shop

        /// <summary>A fair trade went through: (a, b).</summary>
        public static event System.Action<Customer, Customer> TradedFairly;

        /// <summary>A victim realised who ripped them off: (victim's persona, hustler's persona id).</summary>
        public static event System.Action<Persona, string> RipOffNoticed;

        public string Id => "trading";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerTrading.Value;

        private static readonly PersonaTable<Row> Table = new PersonaTable<Row>();

        /// <summary>Per visit: a rip-off they haven't noticed yet, and whether they already called out a hustler they saw.</summary>
        private class Visit
        {
            public string UnnoticedFrom; // hustler's persona id
            public string UnnoticedCard;   // what they gave away
            public string UnnoticedGot;    // the cheap card they got for it
            public float UnnoticedPrice;
            public bool CalledOut;
        }

        public void Init()
        {
            Chatting.Exchange = TryTrade;
            Chatting.Avoid.Add((a, b) => On && Holds(a, b));
            Cast.VisitStarted += (c, p, resumed) => { if (On) Seed(p); };
            PackRipping.PulledHit += (c, p, card, price) => { if (On) AddToBinder(p, card); };
            CustomerLife.SlowTick.Add(CallOut);
            Cast.VisitEnded += (c, p, paid) =>
            {
                var v = Cast.PeekVisitData<Visit>(c);
                if (On && v?.UnnoticedFrom != null && Random.value < 0.4f) Notice(c, p, v, sayIt: false);
            };
            UI.CustomerDebugPanel.Section("Trading", (c, p, rows) =>
            {
                if (!On) return;
                var row = Table.Get(p, create: false);
                var cards = row?.bind ?? new List<Card>();
                var best = cards.Select(Data).Where(d => d != null).OrderByDescending(CardInfo.Price).FirstOrDefault();
                rows.Add(("binder", cards.Count == 0 ? "empty" : $"{cards.Count} card(s), best {CardInfo.Name(best)} ({CardBubble.Value(CardInfo.Price(best))})"));
                rows.Add(("hustle", Traits.Effect(p, "hustle") > 0f ? $"{Traits.Effect(p, "hustle") * 100f:0}% of their trades" : "never"));
                if (row != null && (row.t > 0 || row.ro > 0 || row.sc > 0)) rows.Add(("record", $"{row.t} trade(s), ripped off {row.ro} · got ripped {row.sc}"));
                if (row?.g != null && row.g.Count > 0) rows.Add(("grudges", string.Join(", ", row.g.Select(id => Cast.Regular(id)?.name ?? "a walk-in"))));
                var v = Cast.PeekVisitData<Visit>(c);
                if (v?.UnnoticedFrom != null) rows.Add(("not noticed yet", $"ripped off by {Cast.Regular(v.UnnoticedFrom)?.name ?? "someone"} ({v.UnnoticedCard})"));
            });
        }

        private static bool Holds(Persona a, Persona b) => Table.Get(a, create: false)?.g?.Contains(b.id) == true;

        private static CardData Data(Card c)
        {
            var d = CardInfo.FromKey(c.k);
            if (d == null) return null;
            d.borderType = (ECardBorderType)c.b;
            d.isFoil = c.f;
            return d;
        }

        private static Card Entry(CardData d) => new Card { k = CardInfo.Key(d), b = (int)d.borderType, f = d.isFoil };

        /// <summary>A new binder: 3–6 cards a customer might own (common ones mostly).</summary>
        private static void Seed(Persona p)
        {
            var row = Table.Get(p);
            if (row.bind != null) return;
            row.bind = new List<Card>();
            int n = Cast.Rng.Next(3, 7);
            for (int i = 0; i < n; i++)
            {
                var d = CardInfo.RandomOwned(Cast.Rng);
                if (d != null) row.bind.Add(Entry(d));
            }
        }

        private static void AddToBinder(Persona p, CardData card)
        {
            var row = Table.Get(p);
            if (row.bind == null) row.bind = new List<Card>();
            row.bind.Add(Entry(card));
            if (row.bind.Count <= MaxBinder) return;
            // Full: the least valuable goes.
            var worst = row.bind.OrderBy(c => Data(c) is CardData d ? CardInfo.Price(d) : -1f).First();
            row.bind.Remove(worst);
        }

        /// <summary>Chatting's exchange step: maybe a trade between them. Returns the seconds its lines take (0 = nothing said).</summary>
        private const float WishTradeChance = 0.5f, PlainTradeChance = 0.15f; // of a chat ending in a fair trade, with a wanted card or without

        private static float TryTrade(Customer a, Customer b)
        {
            if (!On) return 0f;
            Persona pa = Cast.Of(a), pb = Cast.Of(b);
            if (pa == null || pb == null || Holds(pa, pb) || Holds(pb, pa)) return 0f;
            Seed(pa);
            Seed(pb);
            var ra = Table.Get(pa);
            var rb = Table.Get(pb);
            if (ra.bind.Count == 0 || rb.bind.Count == 0) return 0f;
            // Either of them may be a hustler.
            Customer hustler = null;
            if (Random.value < Traits.Effect(pa, "hustle")) hustler = a;
            else if (Random.value < Traits.Effect(pb, "hustle")) hustler = b;
            if (hustler != null)
            {
                var victim = hustler == a ? b : a;
                return RipOff(hustler, victim);
            }
            return Fair(a, pa, ra, b, pb, rb);
        }

        /// <summary>A fair swap: values within a third of each other, wishlist cards first; without a wish only sometimes. Returns its lines' seconds.</summary>
        private static float Fair(Customer a, Persona pa, Row ra, Customer b, Persona pb, Row rb)
        {
            Card bestX = null, bestY = null;
            float bestScore = 0f;
            foreach (var x in ra.bind)
            {
                var dx = Data(x);
                float vx = dx == null ? 0f : CardInfo.Price(dx);
                if (vx <= 0f) continue;
                foreach (var y in rb.bind)
                {
                    var dy = Data(y);
                    float vy = dy == null ? 0f : CardInfo.Price(dy);
                    if (vy <= 0f || vx / vy < 0.75f || vx / vy > 1.33f || x.k == y.k) continue;
                    float score = (Wishlists.Wants(pb, dx) ? 2f : 0f) + (Wishlists.Wants(pa, dy) ? 2f : 0f) + Random.value;
                    if (score > bestScore) { bestScore = score; bestX = x; bestY = y; }
                }
            }
            if (bestX == null || Random.value >= (bestScore >= 1f ? WishTradeChance : PlainTradeChance)) return 0f;
            float gap = Dialogue.Gap;
            Swap(a, pa, ra, bestX, b, pb, rb, bestY, 2f * gap);
            TradedFairly?.Invoke(a, b);
            var got = Data(bestY);
            var gave = Data(bestX);
            Dialogue.Say(a, "tradeDone", new Dialogue.Tokens { Card = CardInfo.Name(got), Price = CardInfo.Price(got), Other = pb.name }, 1f);
            Dialogue.Say(b, "tradeDone", new Dialogue.Tokens { Card = CardInfo.Name(gave), Price = CardInfo.Price(gave), Other = pa.name }, 1f, gap);
            News.Add("trade", pa.name, pb.name, CardInfo.Name(got), CardInfo.Price(got), CardInfo.Name(gave));
            Plugin.Log.LogInfo($"Customers: {pa.name} traded {CardInfo.Name(gave)} ({CardInfo.Price(gave):0.00}) to {pb.name} for {CardInfo.Name(got)} ({CardInfo.Price(got):0.00})");
            return 2f * gap;
        }

        /// <summary>The hustler's cheapest-for-most swap (their card worth at most half); the victim may say no (Hagglers often do).
        /// Returns its lines' seconds: done, hustle, maybe the victim realising.</summary>
        private static float RipOff(Customer h, Customer v)
        {
            Persona ph = Cast.Of(h), pv = Cast.Of(v);
            var rh = Table.Get(ph);
            var rv = Table.Get(pv);
            Card bestX = null, bestY = null;
            float bestGain = 0f;
            foreach (var x in rh.bind)
            {
                var dx = Data(x);
                float vx = dx == null ? 0f : CardInfo.Price(dx);
                if (vx <= 0f) continue;
                foreach (var y in rv.bind)
                {
                    var dy = Data(y);
                    float vy = dy == null ? 0f : CardInfo.Price(dy);
                    if (vy <= 0f || vx > vy * 0.5f) continue;
                    if (vy - vx > bestGain) { bestGain = vy - vx; bestX = x; bestY = y; }
                }
            }
            if (bestX == null) return 0f;
            float gap = Dialogue.Gap;
            float accept = pv.traits.Contains("haggler") ? 0.3f : Wishlists.Wants(pv, Data(bestX)) ? 0.9f : 0.65f;
            if (Random.value >= accept)
            {
                Dialogue.Say(v, "tradeRefuse", new Dialogue.Tokens { Other = ph.name }, 1f);
                return gap;
            }
            var visit = Cast.VisitData<Visit>(v);
            bool noticeNow = visit != null && Random.value < 0.5f; // half notice right away, others maybe as they leave
            float took = (noticeNow ? 3f : 2f) * gap;
            Swap(h, ph, rh, bestX, v, pv, rv, bestY, took);
            rh.ro++;
            var gotByH = Data(bestY);
            var gotByV = Data(bestX);
            Dialogue.Say(v, "tradeDone", new Dialogue.Tokens { Card = CardInfo.Name(gotByV), Price = CardInfo.Price(gotByV), Other = ph.name }, 1f);
            Dialogue.Say(h, "tradeHustle", new Dialogue.Tokens { Card = CardInfo.Name(gotByH), Price = CardInfo.Price(gotByH), Other = pv.name }, 0.7f, gap);
            Plugin.Log.LogInfo($"Customers: {ph.name} ripped off {pv.name}: gave {CardInfo.Name(gotByV)} ({CardInfo.Price(gotByV):0.00}) for {CardInfo.Name(gotByH)} ({CardInfo.Price(gotByH):0.00})");
            if (visit == null) return took;
            visit.UnnoticedFrom = ph.id;
            visit.UnnoticedCard = CardInfo.Name(gotByH);
            visit.UnnoticedGot = CardInfo.Name(gotByV);
            visit.UnnoticedPrice = CardInfo.Price(gotByH);
            if (noticeNow) Notice(v, pv, visit, sayIt: true, delay: 2f * gap);
            return took;
        }

        private static void Swap(Customer a, Persona pa, Row ra, Card x, Customer b, Persona pb, Row rb, Card y, float seconds)
        {
            ra.bind.Remove(x);
            rb.bind.Remove(y);
            ra.bind.Add(y);
            rb.bind.Add(x);
            ra.t++;
            rb.t++;
            CardBubble.Show(a, Data(y), seconds);
            CardBubble.Show(b, Data(x), seconds);
            CEventManager.QueueEvent(new CEventPlayer_AddShopExp(TradeExp));
        }

        /// <summary>The victim realises: grudge (+ rival), gossip, maybe out loud.</summary>
        private static void Notice(Customer c, Persona p, Visit v, bool sayIt, float delay = 0f)
        {
            string from = v.UnnoticedFrom;
            v.UnnoticedFrom = null;
            RipOffNoticed?.Invoke(p, from);
            var row = Table.Get(p);
            row.sc++;
            if (row.g == null) row.g = new List<string>();
            if (!row.g.Contains(from)) row.g.Add(from);
            if (row.g.Count > 3) row.g.RemoveAt(0);
            Rivalries.MakeRival(p, from, "ripoff");
            string hustlerName = Cast.Regular(from)?.name ?? "that hustler";
            News.Add("ripoff", hustlerName, p.name, v.UnnoticedCard, v.UnnoticedPrice, v.UnnoticedGot);
            if (sayIt) Dialogue.Say(c, "tradeRipped", new Dialogue.Tokens { About = hustlerName, Card = v.UnnoticedCard, Card2 = v.UnnoticedGot, Price = v.UnnoticedPrice }, 1f, delay);
            Plugin.Log.LogInfo($"Customers: {p.name} realised {hustlerName} ripped them off and holds a grudge");
        }

        /// <summary>Seeing someone they hold a grudge against: says so, once a visit.</summary>
        private static void CallOut(Customer c)
        {
            if (!On) return;
            var p = Cast.Of(c);
            var grudges = p == null ? null : Table.Get(p, create: false)?.g;
            if (grudges == null || grudges.Count == 0) return;
            var v = Cast.VisitData<Visit>(c);
            if (v == null || v.CalledOut) return;
            foreach (var other in Cast.Visitors())
            {
                if (!grudges.Contains(other.Value.id) || Vector3.Distance(other.Key.transform.position, c.transform.position) > 8f) continue;
                v.CalledOut = true;
                CustomerLife.Glance(c, other.Key.transform.position);
                Dialogue.Say(c, "seeHustler", new Dialogue.Tokens { About = other.Value.name }, 0.9f);
                return;
            }
        }

        public void Load(JToken state) => Table.Load(state);

        public JToken Save(ICollection<string> keep)
        {
            // A grudge against someone who isn't kept any more (a walk-in who left) is forgotten.
            foreach (var kv in Table.Rows) kv.Value.g?.RemoveAll(id => !keep.Contains(id));
            return Table.Save(keep);
        }

        /// <summary>What they'd part with: their most valuable card but the prized one; null with fewer than two cards.</summary>
        public static CardData CardToSell(Persona p)
        {
            if (!On) return null;
            var cards = Table.Get(p, create: false)?.bind?.Select(Data).Where(d => d != null && CardInfo.Price(d) > 0f).OrderByDescending(CardInfo.Price).ToList();
            return cards != null && cards.Count >= 2 ? cards[1] : null;
        }

        public static void TakeFromBinder(Persona p, CardData card)
        {
            var bind = Table.Get(p, create: false)?.bind;
            int i = bind?.FindIndex(c => c.k == CardInfo.Key(card) && c.b == (int)card.borderType && c.f == card.isFoil) ?? -1;
            if (i >= 0) bind.RemoveAt(i);
        }

        /// <summary>The most valuable card in their binder, or null.</summary>
        public static CardData BestCard(Persona p) =>
            On ? Table.Get(p, create: false)?.bind?.Select(Data).Where(d => d != null).OrderByDescending(CardInfo.Price).FirstOrDefault() : null;

        public string Summary(Persona p)
        {
            if (!On) return null;
            var row = Table.Get(p, create: false);
            if (row == null) return null;
            var parts = new List<string>();
            if (row.t > 0) parts.Add($"traded {row.t}×");
            if (row.ro > 0) parts.Add($"ripped off {row.ro} customer(s)");
            if (row.sc > 0) parts.Add($"got ripped off {row.sc}×");
            if (row.g != null && row.g.Count > 0) parts.Add("won't deal with " + string.Join(", ", row.g.Select(id => Cast.Regular(id)?.name ?? "a walk-in")));
            return parts.Count == 0 ? null : string.Join(", ", parts);
        }
    }
}
