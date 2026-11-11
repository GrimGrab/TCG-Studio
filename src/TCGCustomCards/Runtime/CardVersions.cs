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

        /// <summary>Editions a pack checks, rarest first (each with its own % from the pack's borderOdds).</summary>
        private static readonly ECardBorderType[] EditionCheckOrder =
            { ECardBorderType.FullArt, ECardBorderType.EX, ECardBorderType.Gold, ECardBorderType.Silver, ECardBorderType.FirstEdition };

        /// <summary>
        /// A pack's chance for each version, for <see cref="Roll"/> (one draw). The foil % keeps its meaning: how often a pulled
        /// card is foil (when the set has plain and foil versions; otherwise every card is the one kind it has). Within plain or
        /// foil, the editions follow the pack's edition % checked rarest first among the editions that exist that way, and the
        /// rest goes to the lowest one that exists. A set with every edition on or off both ways gets exactly the old odds
        /// (edition % × foil %); an edition that exists only as foil is now as rare as a foil, never forced to foil.
        /// </summary>
        public static Func<ECardBorderType, bool, double> PackWeights(ECardExpansionType exp, System.Collections.Generic.IDictionary<string, float> editionOdds, float foilChance)
        {
            var w = new double[CardStore.SlotsPerCard];
            bool hasPlain = false, hasFoil = false;
            for (int b = 0; b < CardStore.BordersPerCard; b++)
            {
                hasPlain |= IsAllowed(exp, (ECardBorderType)b, false);
                hasFoil |= IsAllowed(exp, (ECardBorderType)b, true);
            }
            double fc = Math.Min(1, Math.Max(0, foilChance / 100.0));
            double pFoil = hasPlain && hasFoil ? fc : hasFoil ? 1 : 0;
            Editions(exp, false, 1 - pFoil, editionOdds, w);
            Editions(exp, true, pFoil, editionOdds, w);
            return (b, f) => w[(int)b + (f ? CardStore.BordersPerCard : 0)];
        }

        /// <summary>Spreads <paramref name="share"/> over the editions that exist plain (or foil) by the pack's edition %.</summary>
        private static void Editions(ECardExpansionType exp, bool foil, double share, System.Collections.Generic.IDictionary<string, float> odds, double[] w)
        {
            int lowest = -1;
            for (int b = 0; b < CardStore.BordersPerCard && lowest < 0; b++)
                if (IsAllowed(exp, (ECardBorderType)b, foil)) lowest = b;
            if (lowest < 0 || share <= 0) return;
            int off = foil ? CardStore.BordersPerCard : 0;
            double left = 1;
            if (odds != null)
                foreach (var b in EditionCheckOrder)
                {
                    if (!IsAllowed(exp, b, foil) || !odds.TryGetValue(b.ToString(), out float pct)) continue;
                    double p = left * Math.Min(1, Math.Max(0, pct / 100.0));
                    w[(int)b + off] += share * p;
                    left -= p;
                }
            w[lowest + off] += share * left;
        }

        private static (ECardBorderType border, bool foil) Version(int i) =>
            ((ECardBorderType)(i % CardStore.BordersPerCard), i >= CardStore.BordersPerCard);
    }
}
