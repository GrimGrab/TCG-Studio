using System;
using System.Collections.Generic;
using System.IO;
using Newtonsoft.Json;

namespace TCGCustomCards.Core
{
    /// <summary>Reads and validates &lt;plugin&gt;\Accessories\accessories.json (custom deck boxes / playmats, written by TCG Studio).</summary>
    internal static class AccessoryLoader
    {
        public const string FolderName = "Accessories";
        public const string FileName = "accessories.json";

        public static List<AccessoryDef> Load(string pluginDir)
        {
            var result = new List<AccessoryDef>();
            string dir = Path.Combine(pluginDir, FolderName);
            string file = Path.Combine(dir, FileName);
            if (!File.Exists(file)) return result;
            AccessoryLibraryDef lib;
            try
            {
                lib = JsonConvert.DeserializeObject<AccessoryLibraryDef>(File.ReadAllText(file));
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Accessory library '{file}' failed to parse: {e.Message}");
                return result;
            }
            if (lib == null) return result;
            if (lib.SchemaVersion > AccessoryLibraryDef.CurrentSchemaVersion)
            {
                Plugin.Log.LogError($"Accessory library '{file}': schemaVersion {lib.SchemaVersion} is newer than supported ({AccessoryLibraryDef.CurrentSchemaVersion})");
                return result;
            }
            lib.FolderPath = dir;

            var ids = new HashSet<string>();
            foreach (var a in lib.Accessories ?? new List<AccessoryDef>())
            {
                string where = $"accessory '{a?.Id}'";
                var errors = new List<string>();
                if (a == null) continue;
                if (string.IsNullOrWhiteSpace(a.Id)) errors.Add("missing id");
                else if (a.Id.IndexOfAny(new[] { ' ', '\t', ':', '/', '|' }) >= 0) errors.Add("id may not contain spaces, ':', '/' or '|'");
                else if (!ids.Add(a.Id)) errors.Add("duplicate id");
                if (string.IsNullOrWhiteSpace(a.Name)) a.Name = a.Id;
                string baseName = string.IsNullOrWhiteSpace(a.Base) ? AccessoryKinds.DefaultBase(a.Kind) : a.Base;
                if (!Enum.TryParse(baseName, out EItemType baseItem) || baseItem < 0 || baseItem >= EItemType.Max)
                    errors.Add($"base '{baseName}' is not a vanilla item");
                else if (AccessoryKinds.KindOf(baseItem) != a.Kind)
                    errors.Add($"base '{baseName}' is not a vanilla {a.Kind}");
                else a.BaseItem = baseItem;
                if (a.License == null) a.License = new AccessoryLicenseDef();
                if (a.Cost.HasValue && a.Cost.Value <= 0) errors.Add("cost must be > 0");
                foreach (var img in new[] { a.Texture, a.Icon })
                    if (!string.IsNullOrEmpty(img) && !File.Exists(Path.Combine(dir, img)))
                        Plugin.Log.LogWarning($"Accessory library {where}: image not found '{img}' (vanilla art used)");
                if (!string.IsNullOrEmpty(a.Mesh))
                {
                    if (a.Kind != AccessoryKind.Figurine)
                    {
                        Plugin.Log.LogWarning($"Accessory library {where}: 'mesh' is only used by figurines (ignored)");
                        a.Mesh = null;
                    }
                    else if (!File.Exists(Path.Combine(dir, a.Mesh)))
                    {
                        Plugin.Log.LogWarning($"Accessory library {where}: model not found '{a.Mesh}' (base toy's model used)");
                        a.Mesh = null;
                    }
                }
                if (errors.Count > 0)
                {
                    Plugin.Log.LogError($"Accessory library {where} rejected: " + string.Join("; ", errors));
                    continue;
                }
                a.FolderPath = dir;
                result.Add(a);
            }
            Plugin.Log.LogInfo($"Loaded {result.Count} custom accessor{(result.Count == 1 ? "y" : "ies")} from {file}");
            return result;
        }
    }
}
