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

        /// <summary>
        /// Custom furniture from the same library file. Checks that only need the def happen here; the base piece's type is checked
        /// against its prefab when injecting (<see cref="Runtime.FurnitureInjector"/>).
        /// </summary>
        public static List<FurnitureDef> LoadFurniture(string pluginDir)
        {
            var result = new List<FurnitureDef>();
            string dir = Path.Combine(pluginDir, FolderName);
            string file = Path.Combine(dir, FileName);
            if (!File.Exists(file)) return result;
            AccessoryLibraryDef lib;
            try
            {
                lib = JsonConvert.DeserializeObject<AccessoryLibraryDef>(File.ReadAllText(file));
            }
            catch (Exception)
            {
                return result; // already reported by Load
            }
            if (lib?.Furniture == null || lib.SchemaVersion > AccessoryLibraryDef.CurrentSchemaVersion) return result;

            var ids = new HashSet<string>();
            foreach (var f in lib.Furniture)
            {
                if (f == null) continue;
                string where = $"furniture '{f.Id}'";
                var errors = new List<string>();
                if (string.IsNullOrWhiteSpace(f.Id)) errors.Add("missing id");
                else if (f.Id.IndexOfAny(new[] { ' ', '\t', ':', '/', '|' }) >= 0) errors.Add("id may not contain spaces, ':', '/' or '|'");
                else if (!ids.Add(f.Id)) errors.Add("duplicate id");
                if (string.IsNullOrWhiteSpace(f.Name)) f.Name = f.Id;
                string baseName = string.IsNullOrWhiteSpace(f.Base) ? FurnitureKinds.DefaultBase(f.Type) : f.Base;
                if (!Enum.TryParse(baseName, out EObjectType baseObj) || baseObj < 0 || baseObj >= EObjectType.MAX)
                    errors.Add($"base '{baseName}' is not a vanilla furniture piece");
                else f.BaseObject = baseObj;
                if (f.Price.HasValue && f.Price.Value <= 0) errors.Add("price must be > 0");
                if (f.Level.HasValue && f.Level.Value < 0) errors.Add("level must be ≥ 0");
                if (!string.IsNullOrEmpty(f.Tint) && !UnityEngine.ColorUtility.TryParseHtmlString(f.Tint, out _)) errors.Add($"tint '{f.Tint}' is not a colour (#RRGGBB)");
                foreach (var img in new[] { f.Texture, f.Icon })
                    if (!string.IsNullOrEmpty(img) && !File.Exists(Path.Combine(dir, img)))
                        Plugin.Log.LogWarning($"Accessory library {where}: image not found '{img}' (base art used)");
                if (!string.IsNullOrEmpty(f.Mesh) && !File.Exists(Path.Combine(dir, f.Mesh)))
                {
                    Plugin.Log.LogWarning($"Accessory library {where}: model not found '{f.Mesh}' (base model used)");
                    f.Mesh = null;
                }
                if (f.Paint != null)
                {
                    bool missing = string.IsNullOrEmpty(f.Paint.Texture) || !File.Exists(Path.Combine(dir, f.Paint.Texture)) || f.Paint.Parts == null
                        || f.Paint.Parts.Count == 0 || f.Paint.Parts.Exists(p => p == null || string.IsNullOrEmpty(p.Mesh) || !File.Exists(Path.Combine(dir, p.Mesh)));
                    if (missing)
                    {
                        Plugin.Log.LogWarning($"Accessory library {where}: painted look incomplete (texture or part model missing) — base look used");
                        f.Paint = null;
                    }
                    else if (!string.IsNullOrEmpty(f.Mesh)) errors.Add("a painted piece can't also have its own model");
                }
                if (f.Spots != null && f.Spots.Count > 0)
                {
                    for (int i = 0; i < f.Spots.Count; i++)
                    {
                        var s = f.Spots[i];
                        string sw = $"spot {i + 1}";
                        if (s == null) { errors.Add($"{sw} is empty"); continue; }
                        if (s.Kind == FurnitureSpotKind.Items && !FurnitureKinds.HasItemSpots(f.Type)) errors.Add($"{sw}: a {f.Type} has no item spots");
                        if (s.Kind == FurnitureSpotKind.Card && !FurnitureKinds.HasCardSpots(f.Type)) errors.Add($"{sw}: a {f.Type} has no card spots");
                        if (!Vec(s.Pos, 3) || !Vec(s.Rot, 3)) errors.Add($"{sw}: pos and rot need 3 numbers");
                        if (s.Customer != null && s.Customer.Length != 2) errors.Add($"{sw}: customer needs 2 numbers (x, z)");
                        else if (s.Customer == null)
                            Plugin.Log.LogWarning($"Accessory library {where} {sw}: no customer point — the base compartment's is used (TCG Studio requires one)");
                        if (s.PriceTag != null && s.PriceTag.Length != 3) errors.Add($"{sw}: priceTag needs 3 numbers");
                        if (s.Kind == FurnitureSpotKind.Items)
                        {
                            if (!Vec(s.Size, 3) || s.Size[0] <= 0 || s.Size[1] <= 0 || s.Size[2] < 0) errors.Add($"{sw}: size needs 3 numbers: width, depth > 0, height ≥ 0");
                            if (s.Grid == null) s.Grid = new[] { 4, 8, 1 };
                            if (s.Grid.Length != 3 || s.Grid[0] < 1 || s.Grid[1] < 1 || s.Grid[2] < 1 || s.Grid[0] * s.Grid[1] * s.Grid[2] > 512)
                                errors.Add($"{sw}: grid needs 3 whole numbers ≥ 1 (at most 512 units)");
                        }
                    }
                    if (f.Spots.Count > 64) errors.Add("at most 64 spots");
                }
                else if (f.Spots != null && f.Spots.Count == 0) f.Spots = null;
                if (f.Points != null)
                {
                    var roles = new HashSet<string>(System.Linq.Enumerable.Select(FurnitureKinds.PointRoles(f.Type), r => r.Role));
                    for (int i = 0; i < f.Points.Count; i++)
                    {
                        var pt = f.Points[i];
                        if (pt == null || !roles.Contains(pt.Role ?? "")) errors.Add($"point {i + 1}: a {f.Type} has no '{pt?.Role}' points");
                        else if (!Vec(pt.Pos, 3) || !Vec(pt.Rot, 3)) errors.Add($"point {i + 1}: pos and rot need 3 numbers");
                        else if (pt.Scale.HasValue && (pt.Scale.Value < 0 || pt.Scale.Value > 20)) errors.Add($"point {i + 1}: scale must be between 0 and 20");
                    }
                    if (f.Points.Count == 0) f.Points = null;
                }
                if (f.Area != null && (!Vec(f.Area.Pos, 2) || !Vec(f.Area.Size, 2) || f.Area.Size[0] <= 0 || f.Area.Size[1] <= 0))
                    errors.Add("area needs pos [x, z] and size [width, depth] > 0");
                if (errors.Count > 0)
                {
                    Plugin.Log.LogError($"Accessory library {where} rejected: " + string.Join("; ", errors));
                    continue;
                }
                f.FolderPath = dir;
                result.Add(f);
            }
            if (result.Count > 0) Plugin.Log.LogInfo($"Loaded {result.Count} custom furniture piece(s) from {file}");
            return result;
        }

        /// <summary>Custom decorations (surfaces, posters, objects) from the same library file.</summary>
        public static List<DecorationDef> LoadDecorations(string pluginDir)
        {
            var result = new List<DecorationDef>();
            string dir = Path.Combine(pluginDir, FolderName);
            string file = Path.Combine(dir, FileName);
            if (!File.Exists(file)) return result;
            AccessoryLibraryDef lib;
            try
            {
                lib = JsonConvert.DeserializeObject<AccessoryLibraryDef>(File.ReadAllText(file));
            }
            catch (Exception)
            {
                return result; // already reported by Load
            }
            if (lib?.Decorations == null || lib.SchemaVersion > AccessoryLibraryDef.CurrentSchemaVersion) return result;

            var ids = new HashSet<string>();
            foreach (var d in lib.Decorations)
            {
                if (d == null) continue;
                string where = $"decoration '{d.Id}'";
                var errors = new List<string>();
                if (string.IsNullOrWhiteSpace(d.Id)) errors.Add("missing id");
                else if (d.Id.IndexOfAny(new[] { ' ', '\t', ':', '/', '|' }) >= 0) errors.Add("id may not contain spaces, ':', '/' or '|'");
                else if (!ids.Add(d.Id)) errors.Add("duplicate id");
                if (string.IsNullOrWhiteSpace(d.Name)) d.Name = d.Id;
                if (d.Price < 0) errors.Add("price must be ≥ 0");
                if (!string.IsNullOrEmpty(d.Icon) && !File.Exists(Path.Combine(dir, d.Icon)))
                    Plugin.Log.LogWarning($"Accessory library {where}: icon not found '{d.Icon}'");
                if (d.IsSurface)
                {
                    if (string.IsNullOrEmpty(d.Texture) || !File.Exists(Path.Combine(dir, d.Texture))) errors.Add($"texture not found '{d.Texture}'");
                    foreach (var map in new[] { d.NormalMap, d.RoughnessMap })
                        if (!string.IsNullOrEmpty(map) && !File.Exists(Path.Combine(dir, map)))
                            Plugin.Log.LogWarning($"Accessory library {where}: map not found '{map}' (left out)");
                    if (!string.IsNullOrEmpty(d.Color) && !UnityEngine.ColorUtility.TryParseHtmlString(d.Color, out _)) errors.Add($"color '{d.Color}' is not a colour (#RRGGBB)");
                    if (d.Smoothness < 0f || d.Smoothness > 1f) errors.Add("smoothness must be between 0 and 1");
                }
                else
                {
                    if (string.IsNullOrEmpty(d.Mesh) || !File.Exists(Path.Combine(dir, d.Mesh))) errors.Add($"model not found '{d.Mesh}'");
                    if (!string.IsNullOrEmpty(d.Texture) && !File.Exists(Path.Combine(dir, d.Texture)))
                        Plugin.Log.LogWarning($"Accessory library {where}: texture not found '{d.Texture}' (white used)");
                }
                if (errors.Count > 0)
                {
                    Plugin.Log.LogError($"Accessory library {where} rejected: " + string.Join("; ", errors));
                    continue;
                }
                d.FolderPath = dir;
                result.Add(d);
            }
            if (result.Count > 0) Plugin.Log.LogInfo($"Loaded {result.Count} custom decoration(s) from {file}");
            return result;
        }

        private static bool Vec(float[] v, int n) => v != null && v.Length == n;
    }
}
