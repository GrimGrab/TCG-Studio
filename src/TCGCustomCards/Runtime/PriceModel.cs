using System;
using TCGCustomCards.Core;

namespace TCGCustomCards.Runtime
{
    /// <summary>Mod-defined market prices: card base × border multiplier × foil multiplier, or an exact override.</summary>
    internal static class PriceModel
    {
        public static string VariantKey(ECardBorderType border, bool foil) => foil ? $"{border}_foil" : border.ToString();

        public static float Compute(SetDef set, CardDef card, ECardBorderType border, bool foil)
        {
            var p = card.Price;
            if (p.Overrides != null && p.Overrides.TryGetValue(VariantKey(border, foil), out float exact))
                return Round(exact);

            float[] borderMult = p.BorderMultipliers ?? set.PriceDefaults.BorderMultipliers;
            float foilMult = p.FoilMultiplier ?? set.PriceDefaults.FoilMultiplier;
            float price = p.Base * borderMult[(int)border] * (foil ? foilMult : 1f);
            return Round(Math.Max(price, set.PriceDefaults.Minimum));
        }

        private static float Round(float v) => (float)Math.Round(v, 2, MidpointRounding.AwayFromZero);
    }
}
