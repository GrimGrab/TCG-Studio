using System.Collections.Generic;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// What collectors do with their binders (trait effects "showOff", "sell"; binders are Trading's, so off with it). Show off: walk up to someone free, hold their
    /// prized card up (<see cref="CardBubble"/>), the other reacts. Sell: offer the player a card from the binder through the game's own sell screen; a deal takes it
    /// out of the binder. Each once a visit. No saved state of its own.
    /// </summary>
    internal class Collecting : IPersonaModule
    {
        private const float ShowOffWeight = 1f, SellWeight = 1f; // × the effect, against vanilla's 1 at each decision
        private const float Reach = 10f, StandOff = 1.3f;
        private const float ShowSeconds = 6f, WalkTimeout = 25f, ReactDelay = 1.6f;
        private const float MaxMarkup = 1.15f;

        public string Id => "collecting";
        public bool Enabled => Cast.Enabled;

        /// <summary>Per visit.</summary>
        private class Visit
        {
            public bool ShowedOff, Offered;
            public CardData OfferedCard;
        }

        private class Show
        {
            public Customer Shower, Listener;
            public CardData Card;
            public float Until;
        }

        public void Init()
        {
            CustomerStates.Register(new ShowerState());
            CustomerStates.Register(new ListenerState());
            CustomerChoices.Register("showOff", c => Weight(c, "showOff", ShowOffWeight, v => v.ShowedOff), TryShowOff);
            CustomerChoices.Register("sellCard", c => Weight(c, "sell", SellWeight, v => v.Offered), TrySell);
            Cast.VisitEnded += (c, p, paid) =>
            {
                var offered = Cast.PeekVisitData<Visit>(c)?.OfferedCard;
                var sold = CustomerLife.SoldCard(c);
                if (offered != null && sold != null && CardInfo.Key(sold) == CardInfo.Key(offered)) Trading.TakeFromBinder(p, offered);
            };
        }

        private static float Weight(Customer c, string effect, float weight, System.Func<Visit, bool> done)
        {
            var p = Cast.Of(c);
            if (p == null) return 0f;
            float wants = Traits.Effect(p, effect);
            if (wants <= 0f || Trading.BestCard(p) == null) return 0f;
            var v = Cast.PeekVisitData<Visit>(c);
            return v != null && done(v) ? 0f : wants * weight;
        }

        private static bool TryShowOff(Customer c)
        {
            var p = Cast.Of(c);
            var visit = Cast.VisitData<Visit>(c);
            var card = Trading.BestCard(p);
            if (visit == null || card == null) return false;
            Customer best = null;
            float bestDistance = Reach;
            foreach (var v in Cast.Visitors())
            {
                if (v.Key == c || !CustomerLife.IsFree(v.Key) || Rivalries.AreRivals(p, v.Value)) continue;
                float d = Vector3.Distance(c.transform.position, v.Key.transform.position);
                if (d < bestDistance) { bestDistance = d; best = v.Key; }
            }
            if (best == null) return false;
            visit.ShowedOff = true;
            var show = new Show { Shower = c, Listener = best, Card = card };
            CustomerStates.Goto(best, ListenerState.StateId, show);
            return CustomerStates.Goto(c, ShowerState.StateId, show);
        }

        private static bool TrySell(Customer c)
        {
            var p = Cast.Of(c);
            var visit = Cast.VisitData<Visit>(c);
            var card = Trading.CardToSell(p);
            if (visit == null || card == null) return false;
            visit.Offered = true;
            if (!CustomerLife.GoSellCard(c, card, CardInfo.Price(card) * Random.Range(1f, MaxMarkup))) return false;
            visit.OfferedCard = card;
            Dialogue.Say(c, "sellOffer", new Dialogue.Tokens { Card = CardInfo.Name(card) }, 1f);
            return true;
        }

        private class ShowerState : ModState
        {
            public const string StateId = "collecting.show";
            public override string Id => StateId;

            public override void Enter(Customer c, StateContext ctx)
            {
                var show = (Show)ctx.Data;
                var from = show.Listener.transform.position;
                var dir = c.transform.position - from;
                dir.y = 0f;
                ctx.WalkTo(from + (dir.sqrMagnitude < 0.01f ? Vector3.forward : dir.normalized) * StandOff);
            }

            public override void Tick(Customer c, StateContext ctx)
            {
                var show = (Show)ctx.Data;
                bool gone = show.Listener == null || !CustomerStates.IsIn(show.Listener, ListenerState.StateId);
                if (gone || (show.Until <= 0f && ctx.Elapsed > WalkTimeout) || (show.Until > 0f && Time.time >= show.Until))
                {
                    show.Until = -1f; // the listener goes back too
                    CustomerStates.ToVanilla(c, CustomerLife.Shop);
                    return;
                }
                if (!ctx.HasArrived) return;
                CustomerLife.FaceTowards(c, show.Listener.transform.position);
                if (show.Until > 0f) return;
                show.Until = Time.time + ShowSeconds;
                CustomerLife.Stand(c);
                float price = CardInfo.Price(show.Card);
                string name = CardInfo.Name(show.Card);
                CardBubble.Show(c, show.Card, ShowSeconds);
                Dialogue.Say(c, "showOffCard", new Dialogue.Tokens { Other = Cast.Of(show.Listener)?.name, Card = name, Price = price }, 1f);
                Dialogue.Say(show.Listener, "showOffReact", new Dialogue.Tokens { Other = Cast.Of(c)?.name, Card = name, Price = price }, 1f, ReactDelay);
            }
        }

        /// <summary>Stands and waits for them; if they never arrive, carries on.</summary>
        private class ListenerState : ModState
        {
            public const string StateId = "collecting.listen";
            public override string Id => StateId;

            public override void Enter(Customer c, StateContext ctx) => CustomerLife.Stand(c);

            public override void Tick(Customer c, StateContext ctx)
            {
                var show = (Show)ctx.Data;
                if (show.Shower == null || !CustomerStates.IsIn(show.Shower, ShowerState.StateId) || show.Until < 0f || ctx.Elapsed > WalkTimeout + ShowSeconds || (show.Until > 0f && Time.time >= show.Until))
                {
                    CustomerStates.ToVanilla(c, CustomerLife.Shop);
                    return;
                }
                CustomerLife.FaceTowards(c, show.Shower.transform.position);
            }
        }

        public void Load(JToken state) { }
        public JToken Save(ICollection<string> keep) => null;
        public string Summary(Persona p) => null;
    }
}
