using System;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// The one owner of "which versions (border × foil) a card may be" (set.json "variants"). Everything that makes up a
    /// card's version — pack rolls, trade customers, customers' own binders, dev grants — rolls it with <see cref="Roll"/>,
    /// which only ever picks a version the set has. Code that copies a card that already exists (the player's collection,
    /// decks, rolled draft pools) needs nothing.
    /// </summary>
    internal static class CardVersions
    {
        /// <summary>The custom set of an expansion when it has some versions switched off; null otherwise (vanilla too).</summary>
        public static CustomSet Limited(ECardExpansionType exp)
        {
            if (!Registry.IsCustom(exp)) return null;
            var set = Registry.Get(exp);
            return set != null && !set.Def.AllVariants ? set : null;
        }

        /// <summary>Whether this version exists for cards of the expansion (vanilla: every version).</summary>
        public static bool IsAllowed(ECardExpansionType exp, ECardBorderType border, bool foil) =>
            !(Limited(exp) is CustomSet set) || set.Def.Allows(border, foil);

        public static bool IsAllowed(CardData card) => card == null || IsAllowed(card.expansionType, card.borderType, card.isFoil);

        /// <summary>
        /// Rolls a version among the ones the expansion has, by weight (any scale; versions the set doesn't have are never
        /// asked). When every allowed version weighs 0 (say the odds only name switched-off versions), all allowed versions are
        /// equally likely. next01 = the caller's random source, 0 (incl.) to 1 (excl.).
        /// </summary>
        public static (ECardBorderType border, bool foil) Roll(ECardExpansionType exp, Func<ECardBorderType, bool, double> weight, Func<double> next01)
        {
            var w = new double[CardStore.SlotsPerCard];
            double total = 0;
            int allowed = 0;
            for (int i = 0; i < w.Length; i++)
            {
                var (b, f) = Version(i);
                if (!IsAllowed(exp, b, f)) continue;
                allowed++;
                w[i] = Math.Max(0, weight(b, f));
                total += w[i];
            }
            if (allowed == 0) return (ECardBorderType.Base, false); // SetLoader never leaves a set without versions
            bool uniform = total <= 0;
            double roll = next01() * (uniform ? allowed : total);
            int last = -1;
            for (int i = 0; i < w.Length; i++)
            {
                var (b, f) = Version(i);
                if (!IsAllowed(exp, b, f)) continue;
                last = i;
                if ((roll -= uniform ? 1 : w[i]) < 0) return (b, f);
            }
            return Version(last); // rounding at the very end
        }

        /// <summary>Any version the expansion has, all equally likely (vanilla's own made-up cards are uniform over the 12).</summary>
        public static (ECardBorderType border, bool foil) RollAny(ECardExpansionType exp, Func<double> next01) =>
            Roll(exp, (b, f) => 1, next01);

        private static (ECardBorderType border, bool foil) Version(int i) =>
            ((ECardBorderType)(i % CardStore.BordersPerCard), i >= CardStore.BordersPerCard);
    }
}
