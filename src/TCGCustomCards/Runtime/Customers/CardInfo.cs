using System.Collections.Generic;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// Cards as customers think of them: by card, any version. Saved as stable keys ("custom:&lt;set&gt;:&lt;card&gt;" like deck
    /// text, "v:&lt;expansion&gt;:&lt;monster&gt;" for vanilla), never as runtime ints of custom cards.
    /// </summary>
    internal static class CardInfo
    {
        public static string Key(CardData c)
        {
            if (c == null) return null;
            if (Registry.TryGetCard(c.monsterType, out var set, out int pos)) return $"custom:{set.Def.Id}:{set.Card(pos).Id}";
            return $"v:{(int)c.expansionType}:{(int)c.monsterType}";
        }

        /// <summary>A plain (base, non-foil) card for a key, or null when its set isn't installed.</summary>
        public static CardData FromKey(string key)
        {
            var parts = key?.Split(':');
            if (parts == null || parts.Length != 3) return null;
            if (parts[0] == "custom")
            {
                var set = Registry.Get(parts[1]);
                if (set == null || !set.PosByCardId.TryGetValue(parts[2], out int pos)) return null;
                return new CardData { expansionType = set.Expansion, monsterType = set.Monsters[pos].MonsterType };
            }
            if (parts[0] == "v" && int.TryParse(parts[1], out int exp) && int.TryParse(parts[2], out int mon))
                return new CardData { expansionType = (ECardExpansionType)exp, monsterType = (EMonsterType)mon };
            return null;
        }

        public static string Name(CardData c)
        {
            if (c == null) return "?";
            if (Registry.TryGetCard(c.monsterType, out var set, out int pos)) return set.Card(pos).Name;
            try { return InventoryBase.GetMonsterData(c.monsterType)?.GetName() ?? c.monsterType.ToString(); }
            catch { return c.monsterType.ToString(); } // GetMonsterData throws for some unknown ints
        }

        public static string NameOfKey(string key) => FromKey(key) is CardData c ? Name(c) : null;

        public static float Price(CardData c)
        {
            try { return CPlayerData.GetCardMarketPrice(c); }
            catch { return 0f; }
        }

        /// <summary>
        /// Rarity step of the card: 0 = the lowest of its set (vanilla: Common), higher = rarer. Custom sets count their own
        /// rarity list in order.
        /// </summary>
        public static int RarityStep(CardData c)
        {
            if (Registry.TryGetCard(c.monsterType, out var set, out int pos))
                return System.Math.Max(0, set.Def.Rarities.IndexOf(set.Rarity(pos)));
            try { return (int)(InventoryBase.GetMonsterData(c.monsterType)?.Rarity ?? ERarity.Common); }
            catch { return 0; }
        }

        /// <summary>
        /// A random card a customer might own, from an installed custom set (or vanilla Tetramon when vanilla cards are shown):
        /// common cards far more often than rare ones, mostly base versions. Null when no cards are installed.
        /// </summary>
        public static CardData RandomOwned(System.Random rng)
        {
            var keys = new List<(string key, int step)>();
            foreach (var set in Registry.Sets)
                for (int pos = 0; pos < set.Def.Cards.Count; pos++)
                    keys.Add(($"custom:{set.Def.Id}:{set.Card(pos).Id}", System.Math.Max(0, set.Def.Rarities.IndexOf(set.Rarity(pos)))));
            if (Plugin.ShowVanillaCards.Value)
                foreach (var m in InventoryBase.GetShownMonsterList(ECardExpansionType.Tetramon) ?? new List<EMonsterType>())
                {
                    var data = InventoryBase.GetMonsterData(m);
                    if (data != null) keys.Add(($"v:{(int)ECardExpansionType.Tetramon}:{(int)m}", (int)data.Rarity));
                }
            if (keys.Count == 0) return null;
            double total = 0;
            foreach (var k in keys) total += Weight(k.step);
            double roll = rng.NextDouble() * total;
            string pick = keys[keys.Count - 1].key;
            foreach (var k in keys)
                if ((roll -= Weight(k.step)) < 0) { pick = k.key; break; }
            var card = FromKey(pick);
            if (card == null) return null;
            card.isFoil = rng.NextDouble() < 0.1;
            double b = rng.NextDouble();
            card.borderType = b < 0.02 ? ECardBorderType.Gold : b < 0.08 ? ECardBorderType.Silver : b < 0.25 ? ECardBorderType.FirstEdition : ECardBorderType.Base;
            return card;
        }

        private static double Weight(int rarityStep) => 1.0 / System.Math.Pow(1 + rarityStep, 2.2);

        /// <summary>Cards a collector could want: rarer cards of an installed custom set, or of vanilla Tetramon when vanilla cards are shown.</summary>
        public static List<string> WishPool(System.Random rng)
        {
            var sources = new List<List<string>>();
            foreach (var set in Registry.Sets)
            {
                var keys = new List<string>();
                int top = set.Def.Rarities.Count - 1;
                for (int pos = 0; pos < set.Def.Cards.Count; pos++)
                {
                    int step = set.Def.Rarities.IndexOf(set.Rarity(pos));
                    if (step >= 1 || top <= 0) keys.Add($"custom:{set.Def.Id}:{set.Card(pos).Id}");
                }
                if (keys.Count > 0) sources.Add(keys);
            }
            if (Plugin.ShowVanillaCards.Value)
            {
                var keys = new List<string>();
                foreach (var m in InventoryBase.GetShownMonsterList(ECardExpansionType.Tetramon) ?? new List<EMonsterType>())
                {
                    var data = InventoryBase.GetMonsterData(m);
                    if (data != null && data.Rarity >= ERarity.Rare) keys.Add($"v:{(int)ECardExpansionType.Tetramon}:{(int)m}");
                }
                if (keys.Count > 0) sources.Add(keys);
            }
            // A collector chases one set at a time.
            return sources.Count == 0 ? new List<string>() : sources[rng.Next(sources.Count)];
        }
    }
}
