using System;
using System.Collections.Generic;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] Loyalty: how far you've come with a regular, in points and named tiers (defaults.json loyaltyTiers). A record only: it changes nothing
    /// they do, only what's shown and how they greet you. Walk-ins have none. Saved: points, per regular.
    /// </summary>
    internal class Loyalty : IPersonaModule
    {
        public class Row
        {
            public int p;
        }

        private const int Lowest = -100, Highest = 200;
        private const int BoughtSomething = 2, LeftEmpty = -1, PlayedTournament = 3, BigPull = 5;
        public const int FoundWish = 5, GaveUpWaiting = -5, CaughtStealing = -20, KickedOut = -30;
        private const float PopupAboveFeet = 2.3f, PopupSeconds = 1.6f;
        private const float BigPullTimes = 3f; // × a hit's price (PackHitPrice)

        public string Id => "loyalty";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerLoyalty.Value;

        private static readonly PersonaTable<Row> Table = new PersonaTable<Row>();

        public static int Points(Persona p) => Table.Get(p, create: false)?.p ?? 0;

        /// <summary>Their tier (null: Loyalty is off, or a walk-in).</summary>
        public static Traits.LoyaltyTier TierOf(Persona p) => On && p != null && p.regular ? TierAt(Points(p)) : null;

        private static Traits.LoyaltyTier TierAt(int points)
        {
            Traits.LoyaltyTier tier = null;
            foreach (var t in Traits.LoyaltyTiers)
                if (tier == null || points >= t.at) tier = t;
            return tier;
        }

        /// <summary>How far through their tier they are, 0 to 1 (the top tier stays full).</summary>
        public static float Progress(Persona p)
        {
            int points = Points(p);
            var tiers = Traits.LoyaltyTiers;
            int i = tiers.IndexOf(TierAt(points));
            if (i < 0 || i == tiers.Count - 1) return 1f;
            int from = i == 0 ? Lowest : tiers[i].at;
            return Mathf.Clamp01((points - from) / (float)(tiers[i + 1].at - from));
        }

        /// <summary>Changes a regular's points; gains by their "loyaltyGain" effect. Crossing into another tier is a moment.</summary>
        public static void Change(Persona p, int delta, string why)
        {
            if (!On || p == null || !p.regular || delta == 0) return;
            var row = Table.Get(p);
            var before = TierAt(row.p);
            if (delta > 0) delta = Mathf.Max(1, Mathf.RoundToInt(delta * Traits.Effect(p, "loyaltyGain")));
            row.p = Mathf.Clamp(row.p + delta, Lowest, Highest);
            Plugin.Log.LogDebug($"Customers: {p.name} loyalty {row.p} ({delta:+0;-0}, {why})");
            var c = Cast.CustomerOf(p);
            if (c != null) Dialogue.Popup(c, $"{delta:+0;-0} Loyalty", PopupAboveFeet, PopupSeconds, delta > 0 ? new Color(1f, 0.78f, 0.28f) : new Color(1f, 0.4f, 0.35f));
            var now = TierAt(row.p);
            if (now == null || before == null || now == before) return;
            bool up = now.at > before.at;
            Plugin.Log.LogInfo($"Customers: {p.name} is now {now.name} ({why})");
            if (c != null) Dialogue.Say(c, up ? "loyaltyUp" : "loyaltyDown", new Dialogue.Tokens { Nick = now.name }, 1f, 0.8f);
            if (now.notify) UI.CustomerNameplates.Announce($"{p.name} is now {now.name}");
        }

        public void Init()
        {
            UI.CustomerDebugPanel.Section("Loyalty", (c, p, rows) =>
            {
                if (TierOf(p) is Traits.LoyaltyTier t) rows.Add(("loyalty", $"{Points(p)} points, {t.name}"));
            });
            Cast.VisitEnded += (c, p, paid) =>
            {
                if (CustomerLife.IsTournamentEntrant(c)) Change(p, PlayedTournament, "played your tournament");
            };
            // As they turn for the door, while you can still see them. A thief or someone thrown out has lost more than this already.
            CustomerLife.ExitingShop += c =>
            {
                // Playing at a table or trading with you is what they came for, even with nothing bought.
                if (CustomerLife.PaidThisVisit(c) > 0f || CustomerLife.PlayedAtTable(c) || CustomerLife.TradedWithYou(c)) return;
                if (!KickOut.WasKicked(c) && !Shoplifting.IsStealing(c)) Change(Cast.Of(c), LeftEmpty, "left empty-handed");
            };
            CustomerLife.CheckedOut += c => Change(Cast.Of(c), BoughtSomething, "bought something");
            PackRipping.PulledHit += (c, p, card, price) =>
            {
                if (price >= Plugin.CustomerPackHitPrice.Value * BigPullTimes) Change(p, BigPull, "a big pull in your shop");
            };
            TableSession.PlayerMatchEnded += (opponent, playerWon, draw) =>
            {
                var p = Cast.Of(opponent);
                if (p == null || draw) return;
                Change(p, Mathf.RoundToInt(Traits.Effect(p, playerWon ? "lostToYou" : "beatYou")), playerWon ? "lost to you at a table" : "beat you at a table");
            };
        }

        public void Load(JToken state) => Table.Load(state);
        public JToken Save(ICollection<string> keep) => Table.Save(keep);
        public string Summary(Persona p) => null; // the panel has it as a bar
    }
}
