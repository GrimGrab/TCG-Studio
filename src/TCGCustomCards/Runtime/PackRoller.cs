using System;
using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Core;
using Random = UnityEngine.Random;

namespace TCGCustomCards.Runtime
{
    /// <summary>Rolls a custom pack's cards from its PackDef (slots, rarity weights, foil and border odds).</summary>
    internal static class PackRoller
    {
        /// <summary>Vanilla-like odds used when a pack defines no slots (≈ vanilla non-guaranteed slot: Rare 10%, Epic 2%, Legendary 0.1%).</summary>
        private static readonly Dictionary<ERarity, float> DefaultWeights = new Dictionary<ERarity, float>
        {
            [ERarity.Common] = 87.9f, [ERarity.Rare] = 10f, [ERarity.Epic] = 2f, [ERarity.Legendary] = 0.1f
        };

        private static readonly ECardBorderType[] BorderCheckOrder =
            { ECardBorderType.FullArt, ECardBorderType.EX, ECardBorderType.Gold, ECardBorderType.Silver, ECardBorderType.FirstEdition };

        /// <summary>Fills <paramref name="pool"/> entries [0..count) and appends them to <paramref name="output"/>. Returns true if any card is foil.</summary>
        public static bool Roll(CustomPack pack, List<CardData> pool, List<CardData> output)
        {
            var def = pack.Def;
            var set = pack.Set;
            var buckets = BuildBuckets(pack);
            bool anyFoil = false;

            var slotWeights = new List<Dictionary<ERarity, float>>();
            if (def.Slots.Count == 0)
                for (int i = 0; i < def.CardsPerPack; i++) slotWeights.Add(DefaultWeights);
            else
                foreach (var s in def.Slots)
                {
                    var w = s.Weights.ToDictionary(kv => (ERarity)Enum.Parse(typeof(ERarity), kv.Key), kv => kv.Value);
                    for (int i = 0; i < s.Count; i++) slotWeights.Add(w);
                }

            for (int j = 0; j < slotWeights.Count && j < pool.Count; j++)
            {
                int pos = PickCard(pack, buckets, slotWeights[j]);
                var card = pool[j];
                card.expansionType = set.Expansion;
                card.monsterType = set.Shown[pos];
                card.isDestiny = false;
                card.isChampionCard = false;
                card.cardGrade = 0;
                card.isFoil = Random.Range(0f, 100f) < def.FoilChance;
                card.borderType = RollBorder(def);
                anyFoil |= card.isFoil;
                output.Add(card);
            }
            return anyFoil;
        }

        private static Dictionary<ERarity, List<int>> BuildBuckets(CustomPack pack)
        {
            var buckets = new Dictionary<ERarity, List<int>>();
            foreach (int pos in pack.Pool)
            {
                var r = pack.Set.Card(pos).Rarity;
                if (!buckets.TryGetValue(r, out var list)) buckets[r] = list = new List<int>();
                list.Add(pos);
            }
            return buckets;
        }

        private static int PickCard(CustomPack pack, Dictionary<ERarity, List<int>> buckets, Dictionary<ERarity, float> weights)
        {
            // Only rarities that still have cards take part; if the wanted rarities are all empty, fall back to any rarity.
            var candidates = weights.Where(w => w.Value > 0f && buckets.TryGetValue(w.Key, out var b) && b.Count > 0).ToList();
            if (candidates.Count == 0) candidates = buckets.Where(b => b.Value.Count > 0).Select(b => new KeyValuePair<ERarity, float>(b.Key, 1f)).ToList();
            if (candidates.Count == 0)
            {
                // Pack pool exhausted (no-duplicate pack with fewer cards than slots): refill and allow repeats.
                foreach (var kv in BuildBuckets(pack)) buckets[kv.Key] = kv.Value;
                candidates = buckets.Select(b => new KeyValuePair<ERarity, float>(b.Key, 1f)).ToList();
            }

            float roll = Random.Range(0f, candidates.Sum(c => c.Value));
            ERarity rarity = candidates[candidates.Count - 1].Key;
            foreach (var c in candidates)
            {
                if (roll < c.Value) { rarity = c.Key; break; }
                roll -= c.Value;
            }

            var bucket = buckets[rarity];
            int idx = Random.Range(0, bucket.Count);
            int pos = bucket[idx];
            if (!pack.Def.AllowDuplicates) bucket.RemoveAt(idx);
            return pos;
        }

        private static ECardBorderType RollBorder(PackDef def)
        {
            if (def.BorderOdds == null) return ECardBorderType.Base;
            foreach (var border in BorderCheckOrder)
                if (def.BorderOdds.TryGetValue(border.ToString(), out float pct) && Random.Range(0f, 100f) < pct)
                    return border;
            return ECardBorderType.Base;
        }
    }
}
