using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Newtonsoft.Json;

namespace TCGCustomCards.Core
{
    /// <summary>Reads and validates the active setup's installed sets (projects\&lt;id&gt;\set.json).</summary>
    internal static class SetLoader
    {
        /// <summary>The active TCG Studio setup's installed sets (StudioPaths), each from its project folder.</summary>
        public static List<SetDef> LoadAll()
        {
            var result = new List<SetDef>();
            string studioLibrary = StudioPaths.Library;
            foreach (var dir in StudioPaths.InstalledSetFolders())
            {
                string file = Path.Combine(dir, "set.json");
                if (!File.Exists(file)) { Plugin.Log.LogWarning($"Installed set not found: {file}"); continue; }
                try
                {
                    var set = JsonConvert.DeserializeObject<SetDef>(File.ReadAllText(file));
                    set.FolderPath = dir;
                    // TCG Studio keeps a set's files under its project folder's name (normally the same as the set id).
                    string folderId = Path.GetFileName(dir);
                    if (IsSafeId(set.Id) && set.Id != folderId) Plugin.Log.LogWarning($"Set '{set.Id}' is in project folder '{folderId}': its files are read from library\\{folderId}");
                    if (studioLibrary != null && IsSafeId(folderId)) set.LibraryPath = Path.Combine(studioLibrary, folderId);
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

        /// <summary>Ids appear in deck text lines ("custom:set:card"), side-car keys and folder names (SetDef.Resolve).</summary>
        internal static bool IsSafeId(string id) =>
            !string.IsNullOrEmpty(id) && id.IndexOfAny(new[] { ' ', '\t', ':', '/', '\\', '|' }) < 0 && id != "." && id != "..";

        private static List<string> Validate(SetDef set)
        {
            var errors = new List<string>();
            int missing = 0;
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
            ValidateVariants(set, errors);

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
                else if (!File.Exists(set.Resolve(c.Image))) MissingImage(set, $"{where}: image not found '{c.Image}'", ref missing);
                if (!string.IsNullOrEmpty(c.BackImage) && !File.Exists(set.Resolve(c.BackImage)))
                    MissingImage(set, $"{where}: back image not found '{c.BackImage}'", ref missing);
            }
            if (missing > MissingShown)
                Plugin.Log.LogWarning($"Set '{set.Id}': {missing - MissingShown} more card images not found (files are read from {set.LibraryPath ?? "nowhere: no set library"})");
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
                if (p.License == null) p.License = new LicenseDef();
                if (p.Slots == null) p.Slots = new List<PackSlot>();
                if (p.Slots.Count > 0) p.CardsPerPack = p.Slots.Sum(s => s.Count); // slots decide the pack's size
                if (p.CardsPerPack < 1 || p.CardsPerPack > Patches.PackSizePatches.MaxCards)
                    errors.Add($"{where}: a pack must have 1–{Patches.PackSizePatches.MaxCards} cards (has {p.CardsPerPack})");
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

        /// <summary>Missing card images logged one by one per set; the rest as one line (a missing library would log every card).</summary>
        private const int MissingShown = 5;

        private static void MissingImage(SetDef set, string what, ref int missing)
        {
            if (++missing <= MissingShown) Plugin.Log.LogWarning($"Set '{set.Id}' {what}");
        }

        private static readonly string[] DefaultRarities = { "Common", "Rare", "Epic", "Legendary" };

        /// <summary>
        /// Fills <see cref="SetDef.RarityById"/> and each rarity's place (the default list when the set has none; returns true then).
        /// Values come from Registry.Build.
        /// </summary>
        /// <summary>Fills <see cref="SetDef.VariantMask"/> from "variants" (empty = all 12).</summary>
        private static void ValidateVariants(SetDef set, List<string> errors)
        {
            var mask = new bool[12];
            if (set.Variants == null || set.Variants.Count == 0)
            {
                for (int i = 0; i < 12; i++) mask[i] = true;
                set.VariantMask = mask;
                return;
            }
            foreach (var v in set.Variants)
            {
                string name = v ?? "";
                bool foil = name.EndsWith("_foil", System.StringComparison.OrdinalIgnoreCase);
                if (foil) name = name.Substring(0, name.Length - 5);
                if (!System.Enum.TryParse(name, true, out ECardBorderType border) || !System.Enum.IsDefined(typeof(ECardBorderType), border) || (int)border > 5)
                    errors.Add($"variants: unknown version '{v}' (Base, FirstEdition, Silver, Gold, EX, FullArt, each optionally + _foil)");
                else mask[(int)border + (foil ? 6 : 0)] = true;
            }
            if (!mask.Any(x => x)) { errors.Add("variants: at least one version is needed"); for (int i = 0; i < 12; i++) mask[i] = true; }
            set.VariantMask = mask;
        }

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
