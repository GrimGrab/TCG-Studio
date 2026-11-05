using System.Collections.Generic;
using System.Linq;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] Wishlists: collectors (trait effect "wish") hunt a few rarer cards of one set. They ask for them,
    /// buy them off your card shelves even when pricey, and are thrilled (mood + tip) when they find one; a new wish takes
    /// its place. Saved: up to 5 card keys + how many wishes came true, per persona.
    /// </summary>
    internal class Wishlists : IPersonaModule
    {
        public class Row
        {
            public List<string> w;  // card keys (CardInfo.Key), at most MaxWishes
            public int got;         // wishes fulfilled in this shop
        }

        private const int MaxWishes = 5;

        public string Id => "wishlists";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerWishlists.Value;

        private static readonly PersonaTable<Row> Table = new PersonaTable<Row>();

        /// <summary>The persona's wished card keys (empty when none).</summary>
        public static IReadOnlyList<string> Wishes(Persona p) => (IReadOnlyList<string>)Table.Get(p, create: false)?.w ?? new List<string>();

        public static bool Wants(Persona p, CardData card)
        {
            var w = Table.Get(p, create: false)?.w;
            return w != null && card != null && w.Contains(CardInfo.Key(card));
        }

        public void Init()
        {
            Cast.VisitStarted += (c, p, resumed) =>
            {
                if (!On || resumed) return;
                Fill(p);
            };
            Cast.EnteredShop += (c, p) =>
            {
                if (!On) return;
                var w = Table.Get(p, create: false)?.w;
                if (w == null || w.Count == 0) return;
                string name = CardInfo.NameOfKey(w[Random.Range(0, w.Count)]);
                if (name != null) Dialogue.Say(c, "wishAsk", new Dialogue.Tokens { Card = name }, 0.3f, Random.Range(4f, 10f));
            };
            CustomerLife.BuyChance.Add((ctx, chance) =>
            {
                var p = On && ctx.IsCard ? Cast.Of(ctx.Customer) : null;
                if (p == null || !Wants(p, ctx.Card)) return chance;
                // They'll pay over market for a wish, within reason.
                float over = ctx.Market > 0f ? ctx.Price / ctx.Market : 1f;
                return Mathf.Max(chance, over <= 1.5f ? 95 : over <= 2.5f ? 60 : chance);
            });
            CustomerLife.CardBought += (c, card, price) =>
            {
                var p = On ? Cast.Of(c) : null;
                if (p == null || !Wants(p, card)) return;
                var row = Table.Get(p);
                row.w.Remove(CardInfo.Key(card));
                row.got++;
                Loyalty.Change(p, Loyalty.FoundWish, "found a card they wanted");
                Dialogue.Say(c, "wishFound", new Dialogue.Tokens { Card = CardInfo.Name(card), Price = price }, 1f);
                var found = Cast.VisitData<WishesFound>(c);
                if (found != null) found.Paid += price;
                Plugin.Log.LogInfo($"Customers: {p.name} found {CardInfo.Name(card)} from their wishlist");
                Fill(p); // a new wish takes its place
            };
            // A thank-you tip at the till for each wish found: 10% of what they paid for it.
            CustomerLife.CheckedOut += c =>
            {
                var found = Cast.PeekVisitData<WishesFound>(c);
                if (found == null || found.Paid <= 0f) return;
                float total = found.Paid;
                found.Paid = 0f;
                if (On) Quirks.GiveTip(c, Mathf.Max(1f, Mathf.Round(total * 0.1f)));
            };
        }

        /// <summary>Per visit: what they paid for wish cards found (tipped on at the till).</summary>
        private class WishesFound
        {
            public float Paid;
        }

        private static void Fill(Persona p)
        {
            int want = Mathf.Clamp((int)Traits.Effect(p, "wish"), 0, MaxWishes);
            if (want == 0) return;
            var row = Table.Get(p);
            if (row.w == null) row.w = new List<string>();
            row.w.RemoveAll(k => CardInfo.FromKey(k) == null); // set uninstalled
            if (row.w.Count >= want) return;
            // Stay with the set they already chase, if any.
            var pool = CardInfo.WishPool(Cast.Rng);
            if (row.w.Count > 0)
            {
                string prefix = row.w[0].Substring(0, row.w[0].LastIndexOf(':') + 1);
                var same = pool.Where(k => k.StartsWith(prefix)).ToList();
                if (same.Count > 0) pool = same;
            }
            pool = pool.Except(row.w).ToList();
            while (row.w.Count < want && pool.Count > 0)
            {
                int i = Cast.Rng.Next(pool.Count);
                row.w.Add(pool[i]);
                pool.RemoveAt(i);
            }
        }

        public void Load(JToken state)
        {
            Table.Load(state);
        }

        public JToken Save(ICollection<string> keep) => Table.Save(keep);

        public string Summary(Persona p)
        {
            if (!On) return null;
            var row = Table.Get(p, create: false);
            return row != null && row.got > 0 ? $"{row.got} wish(es) found" : null;
        }
    }
}
