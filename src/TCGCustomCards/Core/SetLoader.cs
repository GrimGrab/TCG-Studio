using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Newtonsoft.Json;

namespace TCGCustomCards.Core
{
    /// <summary>Reads and validates every Sets\&lt;folder&gt;\set.json.</summary>
    internal static class SetLoader
    {
        public static List<SetDef> LoadAll(string setsRoot)
        {
            var result = new List<SetDef>();
            if (!Directory.Exists(setsRoot))
            {
                Directory.CreateDirectory(setsRoot);
                Plugin.Log.LogInfo($"Created empty sets folder: {setsRoot}");
                return result;
            }

            foreach (var dir in Directory.GetDirectories(setsRoot).OrderBy(d => d, StringComparer.OrdinalIgnoreCase))
            {
                string file = Path.Combine(dir, "set.json");
                if (!File.Exists(file)) continue;
                try
                {
                    var set = JsonConvert.DeserializeObject<SetDef>(File.ReadAllText(file));
                    set.FolderPath = dir;
                    var errors = Validate(set);
                    if (errors.Count > 0)
                    {
                        Plugin.Log.LogError($"Set '{file}' rejected:\n  " + string.Join("\n  ", errors));
                        continue;
                    }
                    if (result.Any(s => s.Id == set.Id))
                    {
                        Plugin.Log.LogError($"Set '{file}' rejected: duplicate set id '{set.Id}'");
                        continue;
                    }
                    result.Add(set);
                    Plugin.Log.LogInfo($"Loaded set '{set.Id}' ({set.Cards.Count} cards, {set.RenderMode})");
                }
                catch (Exception e)
                {
                    Plugin.Log.LogError($"Set '{file}' failed to parse: {e.Message}");
                }
            }
            return result;
        }

        /// <summary>Ids appear in deck text lines ("custom:set:card") and side-car keys.</summary>
        private static bool IsSafeId(string id) => id.IndexOfAny(new[] { ' ', '\t', ':', '/', '|' }) < 0;

        private static List<string> Validate(SetDef set)
        {
            var errors = new List<string>();
            if (set.SchemaVersion > SetDef.CurrentSchemaVersion)
                errors.Add($"schemaVersion {set.SchemaVersion} is newer than supported ({SetDef.CurrentSchemaVersion})");
            if (string.IsNullOrWhiteSpace(set.Id)) errors.Add("missing id");
            else if (!IsSafeId(set.Id)) errors.Add($"set id '{set.Id}' may not contain spaces or ':'");
            if (string.IsNullOrWhiteSpace(set.Name)) set.Name = set.Id;
            if ((int)set.FrameTemplate < 0 || set.FrameTemplate >= ECardExpansionType.MAX || set.FrameTemplate == ECardExpansionType.FoodieGO)
                errors.Add($"frameTemplate '{set.FrameTemplate}' is not a usable vanilla expansion");
            if (set.PriceDefaults?.BorderMultipliers?.Length != 6) errors.Add("priceDefaults.borderMultipliers must have 6 entries");
            if (set.Cards == null || set.Cards.Count == 0) errors.Add("no cards");
            bool defaultList = ValidateRarities(set, errors);

            var ids = new HashSet<string>();
            foreach (var c in set.Cards ?? new List<CardDef>())
            {
                string where = $"card '{c.Id}'";
                if (string.IsNullOrWhiteSpace(c.Id)) { errors.Add("card with missing id"); continue; }
                if (!ids.Add(c.Id)) errors.Add($"{where}: duplicate id");
                if (!IsSafeId(c.Id)) errors.Add($"{where}: id may not contain spaces or ':'");
                if (string.IsNullOrWhiteSpace(c.Name)) c.Name = c.Id;
                // Before custom rarities the field was an ERarity (names, any case, or 0–4).
                if (string.IsNullOrWhiteSpace(c.Rarity)) c.Rarity = set.Rarities[0]?.Id ?? "Common";
                else if (int.TryParse(c.Rarity, out int n) && n >= 0 && n <= (int)ERarity.SuperLegend) c.Rarity = ((ERarity)n).ToString();
                // SuperLegend never had an icon in game (it showed Common's); the default list ends at Legendary.
                if (defaultList && string.Equals(c.Rarity, "SuperLegend", StringComparison.OrdinalIgnoreCase)) c.Rarity = "Legendary";
                if (set.RarityOf(c) == null) errors.Add($"{where}: unknown rarity '{c.Rarity}'");
                if (c.Price == null) c.Price = new CardPrice();
                if (c.Price.BorderMultipliers != null && c.Price.BorderMultipliers.Length != 6) errors.Add($"{where}: price.borderMultipliers must have 6 entries");
                if (c.Play == null) c.Play = new PlayDef();
                if (c.Play.LaneAttack == null || c.Play.LaneAttack.Length != 4) errors.Add($"{where}: play.laneAttack must have 4 entries");
                if (string.IsNullOrEmpty(c.Image)) Plugin.Log.LogWarning($"Set '{set.Id}' {where}: no image");
                else if (!File.Exists(set.Resolve(c.Image))) Plugin.Log.LogWarning($"Set '{set.Id}' {where}: image not found '{c.Image}'");
            }
            foreach (var c in set.Cards ?? new List<CardDef>())
                if (!string.IsNullOrEmpty(c.Play?.EvolvesFrom) && !ids.Contains(c.Play.EvolvesFrom))
                    errors.Add($"card '{c.Id}': evolvesFrom '{c.Play.EvolvesFrom}' not found in set");
            var packIds = new HashSet<string>();
            foreach (var p in set.Packs ?? new List<PackDef>())
            {
                string where = $"pack '{p.Id}'";
                if (string.IsNullOrWhiteSpace(p.Id)) { errors.Add("pack with missing id"); continue; }
                if (!packIds.Add(p.Id)) errors.Add($"{where}: duplicate id");
                if (!IsSafeId(p.Id)) errors.Add($"{where}: id may not contain spaces, ':', '/' or '|'");
                if (string.IsNullOrWhiteSpace(p.Name)) p.Name = $"{set.Name} Pack";
                if (string.IsNullOrWhiteSpace(p.BoxName)) p.BoxName = p.Name.EndsWith(" Pack") ? p.Name.Substring(0, p.Name.Length - 5) + " Box" : p.Name + " Box";
                if (p.CardsPerPack != 7) errors.Add($"{where}: cardsPerPack must be 7 (N-card packs are not supported yet)");
                if (p.License == null) p.License = new LicenseDef();
                if (p.Slots == null) p.Slots = new List<PackSlot>();
                if (p.Slots.Count > 0 && p.Slots.Sum(s => s.Count) != p.CardsPerPack)
                    errors.Add($"{where}: slot counts add up to {p.Slots.Sum(s => s.Count)}, expected {p.CardsPerPack}");
                foreach (var s in p.Slots)
                    foreach (var w in s.Weights.Keys)
                        if (!set.RarityById.ContainsKey(w)) errors.Add($"{where}: unknown rarity '{w}' in slot weights");
                foreach (var b in (p.BorderOdds ?? new Dictionary<string, float>()).Keys)
                    if (!Enum.TryParse<ECardBorderType>(b, out _)) errors.Add($"{where}: unknown border '{b}' in borderOdds");
                foreach (var id in p.Cards ?? new List<string>())
                    if (!ids.Contains(id)) errors.Add($"{where}: card '{id}' not in set");
                foreach (var img in new[] { p.PackTexture, p.PackIcon, p.BoxTexture, p.BoxIcon })
                    if (!string.IsNullOrEmpty(img) && !File.Exists(set.Resolve(img)))
                        Plugin.Log.LogWarning($"Set '{set.Id}' {where}: image not found '{img}' (vanilla art used)");
            }
            return errors;
        }

        private static readonly string[] DefaultRarities = { "Common", "Rare", "Epic", "Legendary" };

        /// <summary>
        /// Fills <see cref="SetDef.RarityById"/> and each rarity's place (the default list when the set has none; returns true then).
        /// Values come from Registry.Build.
        /// </summary>
        private static bool ValidateRarities(SetDef set, List<string> errors)
        {
            bool defaultList = set.Rarities == null || set.Rarities.Count == 0;
            if (defaultList) set.Rarities = DefaultRarities.Select(r => new RarityDef { Id = r, Name = r }).ToList();
            int count = set.Rarities.Count;
            for (int i = 0; i < set.Rarities.Count; i++)
            {
                var r = set.Rarities[i];
                if (r == null || string.IsNullOrWhiteSpace(r.Id)) { errors.Add($"rarity #{i + 1}: missing id"); continue; }
                if (!IsSafeId(r.Id)) errors.Add($"rarity '{r.Id}': id may not contain spaces, ':', '/' or '|'");
                if (!set.RarityById.ContainsKey(r.Id)) set.RarityById[r.Id] = r;
                else errors.Add($"rarity '{r.Id}': duplicate id");
                if (string.IsNullOrWhiteSpace(r.Name)) r.Name = r.Id;
                r.Rank = i;
                r.Tier = (ERarity)(count <= 4 ? i : (int)Math.Round(i * 3.0 / (count - 1)));
            }
            return defaultList;
        }
    }
}
