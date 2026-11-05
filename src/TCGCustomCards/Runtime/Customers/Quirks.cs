using System.Collections.Generic;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] TraitEffects: traits change how a customer shops: their budget (money), how readily they buy at
    /// a given price (itemChance / cardChance, added to vanilla's chance) and how keen they are to play at a table ("table":
    /// a free customer who hasn't played yet heads for one through vanilla's own way, CustomerLife.GoPlayTable). No saved state.
    /// </summary>
    internal class Quirks : IPersonaModule
    {
        public string Id => "quirks";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerTraitEffects.Value;

        public void Init()
        {
            // A restored visitor's budget comes from the save (already adjusted when they arrived).
            Cast.VisitStarted += (c, p, resumed) =>
            {
                if (!On || resumed) return;
                c.m_MaxMoney *= System.Math.Max(0.1f, Traits.Effect(p, "money"));
            };
            // Now and then they tip ("tip"): more often and more with the trait.
            CustomerLife.CheckedOut += c =>
            {
                var p = On ? Cast.Of(c) : null;
                float paid = CustomerLife.PaidThisVisit(c);
                if (p == null || paid <= 0f) return;
                float tipEffect = Traits.Effect(p, "tip");
                if (Random.value >= TipChance * tipEffect) return;
                GiveTip(c, Mathf.Max(1f, Mathf.Round(paid * Random.Range(0.05f, 0.1f) * tipEffect)));
                Dialogue.Say(c, "tip", null, 1f, 0.6f);
            };
            // Critics and gossips ("review") push the game's own rating of a visit further from the middle.
            CustomerLife.ReviewStars.Add((c, add) =>
            {
                var p = On ? Cast.Of(c) : null;
                return p == null ? add : Mathf.RoundToInt(add * Traits.Effect(p, "review"));
            });
            // Chasers ("chase"): nothing in their packs, so they walk out and come back as a new visit, through the till like anyone.
            PackRipping.Bricked += (c, p) =>
            {
                if (!On || Random.value >= Traits.Effect(p, "chase")) return;
                if (p.chaseDay != CPlayerData.m_CurrentDay) { p.chaseDay = CPlayerData.m_CurrentDay; p.chases = 0; }
                if (p.chases >= MaxChasesADay) return;
                p.chases++;
                Cast.VisitData<Chasing>(c);
                Dialogue.Say(c, "ripChase", null, 1f, 1.2f);
            };
            Cast.VisitEnded += (c, p, paid) =>
            {
                if (Cast.PeekVisitData<Chasing>(c) != null) Cast.ComeBack(p);
            };
            // Influencers ("followers"): a big pull brings people in from the street (vanilla spawn: respects the shop's customer limit).
            PackRipping.PulledHit += (c, p, card, price) =>
            {
                if (!On || price < Plugin.CustomerPackHitPrice.Value * 3f) return;
                for (int i = Mathf.RoundToInt(Traits.Effect(p, "followers")); i > 0; i--) CustomerLife.SpawnVisitor();
            };
            // A bit more shop experience from their purchase, on top of vanilla's (Customer.cs:2743, which is about what they paid).
            CustomerLife.CheckedOut += c =>
            {
                float more = On ? Traits.Effect(Cast.Of(c), "shopExp") : 0f;
                if (more > 0f) CEventManager.QueueEvent(new CEventPlayer_AddShopExp(Mathf.Max(1, Mathf.RoundToInt(CustomerLife.PaidThisVisit(c) * more))));
            };
            CustomerLife.BuyChance.Add((ctx, chance) =>
            {
                var p = On ? Cast.Of(ctx.Customer) : null;
                // Never talk anyone into a price vanilla rules out completely.
                if (p == null || chance <= 0) return chance;
                return chance + (int)Traits.Effect(p, ctx.IsCard ? "cardChance" : "itemChance");
            });
            // Keen players ("table": Grinders) pick a table more often when deciding what to do next.
            CustomerChoices.Register("table", c =>
            {
                var p = On ? Cast.Of(c) : null;
                float keen = p == null ? 0f : Traits.Effect(p, "table");
                if (keen <= 0f || Cast.PeekVisitData<TableTry>(c)?.Tried == true || !CustomerLife.CanPlayTable(c)) return 0f;
                return TableWeight * keen;
            }, c =>
            {
                var v = Cast.VisitData<TableTry>(c);
                if (v != null) v.Tried = true;
                return CustomerLife.GoPlayTable(c);
            });
        }

        private const int MaxChasesADay = 2;
        private const float TipChance = 0.05f;

        /// <summary>Pays the player a tip from this customer (coin + price popup above them).</summary>
        public static void GiveTip(Customer c, float amount)
        {
            if (amount <= 0f) return;
            CEventManager.QueueEvent(new CEventPlayer_AddCoin(amount));
            CSingleton<PricePopupSpawner>.Instance?.ShowPricePopup(amount, 2.2f, c.transform);
            Plugin.Log.LogInfo($"Customers: {Cast.Of(c)?.name} tipped {amount:0.00}");
        }

        /// <summary>Per visit: going to come back for more.</summary>
        private class Chasing { }

        private const float TableWeight = 0.6f; // × "table", against vanilla's 1 at each decision

        /// <summary>Per visit: already sent to a table once.</summary>
        private class TableTry
        {
            public bool Tried;
        }

        public void Load(JToken state) { }
        public JToken Save(ICollection<string> keep) => null;
        public string Summary(Persona p) => null;
    }
}
