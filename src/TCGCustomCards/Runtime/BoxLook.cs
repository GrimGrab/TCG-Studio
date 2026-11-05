using System;
using System.Collections.Generic;
using System.Linq;
using BepInEx.Configuration;
using HarmonyLib;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// [Visuals - Game look - Boxes] and [Visuals - Game look - Bags &amp; billboards]: the cardboard boxes (restock big/small, the
    /// box furniture comes in, the graded-card return box), the bulk card boxes, the paper shopping bags and the street
    /// billboards, painted in TCG Studio's Game look page. Each setting names a whole texture in the vanilla UV layout (the
    /// editor paints over the vanilla UVs), so it simply replaces the vanilla texture through a MaterialPropertyBlock — the
    /// boxes' static open/closed models and their animated rigs all keep working, and an empty setting gives the vanilla look back.
    /// Cardboard boxes, bags and billboards are found by their vanilla texture (each kind has its own; outline shells have none)
    /// when their owner wakes: a box, a customer (the bag in its hand), a cashier counter or play table (the open bag), the
    /// shop scene (the billboards, static-batched scenery). Bulk boxes (game 1.03+: BulkBoxNG…OG, one texture array) get the
    /// design as a slice of that array once per session (AddBulkSlices): the shelves, hands and the workbench's closing box all
    /// draw the item's slice, so a changed bulk box design shows after a restart. Restock boxes stored on warehouse shelves are
    /// drawn by the game's StoredBoxVisualBatcher from a snapshot of the box prefabs' materials (instanced, no renderers): it
    /// gets a copy of each cardboard material with the design through its own material override list (Warehouse).
    /// Since the Unity 6 update the street billboards are part of building 32, drawn from the street atlas shared by all the
    /// scenery: billboard designs stay in the earlier layout and their ads are painted into a copy of that atlas (StreetAds).
    /// </summary>
    internal static class BoxLook
    {
        private const string Section = "Visuals - Game look - Boxes";
        private const string SceneSection = "Visuals - Game look - Bags & billboards";

        /// <summary>Setting keys by vanilla texture (game files 1.02; Studio's gameextract boxes.json lists the same).</summary>
        private static readonly Dictionary<string, string> ByTexture = new Dictionary<string, string>
        {
            { "CardboardBox_20x20x20_Albedo", "RestockBoxBigTexture" },
            { "CardboardBox_20x20x10_Albedo", "RestockBoxSmallTexture" },
            { "CardboardBox_30x20x20_Albedo", "FurnitureBoxTexture" },
            { "CardboardBoxRed_20x10x7_Albedo", "GradedBoxTexture" },
            { "T_PaperBagAlbedoClosed", "ShoppingBagTexture" },
            { "T_PaperBagAlbedoOpen", "ShoppingBagOpenTexture" },
            { "mcp_building_32_billboards_d", "StreetBillboardTexture" },
            { StreetAtlas, StreetAtlasKey },
        };

        /// <summary>The bulk box items (game 1.03+: one mesh JankCardBox_Closed, slices of the BulkCardBox texture array; the
        /// earlier BulkBox_Tetramon* items are no longer used) and how the settings name them.</summary>
        private static readonly (string item, string label)[] BulkBoxes =
        {
            ("BulkBoxNG", "without a grade"), ("BulkBoxHG", "HG"), ("BulkBoxMG", "MG"), ("BulkBoxPG", "PG"), ("BulkBoxUG", "UG"), ("BulkBoxOG", "OG"),
        };

        /// <summary>Settings by key (cardboard keys, and bulk boxes as "&lt;EItemType name&gt;Texture").</summary>
        private static readonly Dictionary<string, ConfigEntry<string>> Settings = new Dictionary<string, ConfigEntry<string>>();
        private static readonly Dictionary<string, PictureCache<Texture2D>> Textures = new Dictionary<string, PictureCache<Texture2D>>();

        /// <summary>Renderers showing a box look: renderer → (material slot, setting key). Pooled items move between keys.</summary>
        private static readonly Dictionary<Renderer, (int slot, string key)> Shown = new Dictionary<Renderer, (int, string)>();


        internal static void Bind(ConfigFile config)
        {
            const string how = "a file name in the active setup's Game look folder (TCG Studio's Game look page, Textures, saves the texture " +
                "you paint there) or a full path to a PNG laid out like the vanilla texture. Empty = the vanilla look. Applies live.";
            Settings["RestockBoxBigTexture"] = config.Bind(Section, "RestockBoxBigTexture", "", "Texture of the big restock box: " + how);
            Settings["RestockBoxSmallTexture"] = config.Bind(Section, "RestockBoxSmallTexture", "", "Texture of the small restock box: " + how);
            Settings["FurnitureBoxTexture"] = config.Bind(Section, "FurnitureBoxTexture", "",
                "Texture of the box furniture comes in (bought or packed up): " + how);
            Settings["GradedBoxTexture"] = config.Bind(Section, "GradedBoxTexture", "", "Texture of the box graded cards come back in: " + how);
            Settings["ShoppingBagTexture"] = config.Bind(SceneSection, "ShoppingBagTexture", "",
                "Texture of the paper bag customers carry out: " + how);
            Settings["ShoppingBagOpenTexture"] = config.Bind(SceneSection, "ShoppingBagOpenTexture", "",
                "Texture of the open paper bag on the cashier counter (Studio paints it with the same design as the carried bag, " +
                "over the open bag's own vanilla texture): " + how);
            Settings["StreetBillboardTexture"] = config.Bind(SceneSection, "StreetBillboardTexture", "",
                "Texture of the advertising billboards on the street (the building across the road): " + how);
            foreach (var (item, label) in BulkBoxes)
            {
                string key = item + "Texture";
                Settings[key] = config.Bind(Section, key, "", $"Texture of the bulk card box {label}: " +
                    how.Replace("Applies live.", "Applies after restarting the game."));
            }
            foreach (var s in Settings.Values) s.SettingChanged += (_, __) => Reapply();
        }

        /// <summary>The built texture of a setting; null = vanilla (empty, or the file can't be read).</summary>
        private static Texture2D TextureFor(string key)
        {
            if (key == StreetAtlasKey) return StreetAtlasTexture();
            if (!Settings.TryGetValue(key, out var setting)) return null;
            string path = ShopPicture.ImagePath(setting.Value);
            if (!Textures.TryGetValue(key, out var cache)) Textures[key] = cache = new PictureCache<Texture2D>();
            return cache.Get(path ?? "", () =>
            {
                if (path == null) return null;
                var tex = ShopPicture.Load(path, key, mipmaps: true);
                if (tex == null) return null;
                tex.Apply(updateMipmaps: true, makeNoLongerReadable: true);
                tex.hideFlags = HideFlags.DontUnloadUnusedAsset;
                Plugin.Log.LogInfo($"Box look {key}: using {path}");
                return tex;
            });
        }

        private static void Show(Renderer r, int slot, string key)
        {
            var tex = key == null ? null : TextureFor(key);
            if (tex == null)
            {
                if (Shown.TryGetValue(r, out var was)) ShopPicture.SetTexture(r, null, was.slot);
                if (key == null) Shown.Remove(r);
                else Shown[r] = (slot, key); // kept: set when the setting gets a texture
                return;
            }
            if (Shown.TryGetValue(r, out var old) && old.slot != slot) ShopPicture.SetTexture(r, null, old.slot);
            Shown[r] = (slot, key);
            ShopPicture.SetTexture(r, tex, slot);
        }

        /// <summary>The material slot drawing one of <paramref name="textures"/> (by vanilla texture name), or -1.</summary>
        private static int Slot(Renderer r, Func<string, bool> textures, out string texture)
        {
            var mats = r.sharedMaterials;
            for (int i = 0; i < mats.Length; i++)
            {
                var t = MaterialTex.Main(mats[i]);
                if (t != null && textures(t.name)) { texture = t.name; return i; }
            }
            texture = null;
            return -1;
        }

        private static void Prune()
        {
            foreach (var dead in Shown.Keys.Where(r => r == null).ToList()) Shown.Remove(dead); // destroyed (e.g. a new shop scene)
        }

        /// <summary>An owner woke: every renderer under it drawing a known vanilla texture (a box's static models and animated
        /// rig, a customer's carried bag, a counter's open bag, the street billboards).</summary>
        private static void Track(Component owner)
        {
            Prune();
            foreach (var r in owner.GetComponentsInChildren<Renderer>(true))
            {
                int slot = Slot(r, ByTexture.ContainsKey, out var texture);
                if (slot < 0) continue;
                if (texture == StreetAtlas && _streetAtlas == null) _streetAtlas = MaterialTex.Main(r.sharedMaterials[slot]);
                Show(r, slot, ByTexture[texture]);
            }
        }

        // ---------------------------------------------------------------- street ads in the atlas (game 1.03+)
        // Buildings4_AlbedoOpacity (4096²) holds the two ads left on building 32 next to shop fronts, cars and signs (the middle
        // top board is gone). Rectangles: in the earlier 1024² billboard layout and in the atlas, pixels from the top-left;
        // turned = the atlas holds the ad a quarter turn clockwise (its left edge at the top, its top edge at the right).
        // Measured on the game files 2026-10-29; Studio's gameextract streetads.go rebuilds its template from the same table.

        private const string StreetAtlas = "Buildings4_AlbedoOpacity";
        private const string StreetAtlasKey = "StreetBillboardTexture|atlas";
        private static Texture _streetAtlas; // the vanilla atlas (an asset: the same object in every shop scene)
        private static readonly PictureCache<Texture2D> StreetAtlasCache = new PictureCache<Texture2D>();

        private static readonly (RectInt old, RectInt atlas, bool turned)[] StreetAds =
        {
            (new RectInt(39, 404, 941, 377), new RectInt(0, 0, 614, 1295), true), // big board (Spotlight Pizzeria)
            (new RectInt(31, 37, 458, 316), new RectInt(0, 1295, 614, 423), false), // top side boards (Euphoria)
        };

        /// <summary>A copy of the street atlas with the billboard design's ads in it (compressed like the original); null = vanilla.</summary>
        private static Texture2D StreetAtlasTexture()
        {
            string path = ShopPicture.ImagePath(Settings["StreetBillboardTexture"].Value);
            return StreetAtlasCache.Get(path ?? "", () =>
            {
                if (path == null || _streetAtlas == null) return null;
                var design = ShopPicture.Load(path, "StreetBillboardTexture");
                if (design == null) return null;
                var baseTex = TextureReadback.Read(_streetAtlas, new RectInt(0, 0, _streetAtlas.width, _streetAtlas.height));
                if (baseTex == null) { UnityEngine.Object.Destroy(design); return null; }
                int w = baseTex.width, h = baseTex.height;
                var px = baseTex.GetPixels32();
                UnityEngine.Object.Destroy(baseTex);
                float scale = w / 4096f;
                foreach (var (old, atlas, turned) in StreetAds)
                {
                    int x0 = Mathf.RoundToInt(atlas.xMin * scale), x1 = Mathf.RoundToInt(atlas.xMax * scale);
                    int y0 = Mathf.RoundToInt(atlas.yMin * scale), y1 = Mathf.RoundToInt(atlas.yMax * scale);
                    for (int y = y0; y < y1; y++)
                    for (int x = x0; x < x1; x++)
                    {
                        float fx = (x + 0.5f - x0) / (x1 - x0), fy = (y + 0.5f - y0) / (y1 - y0); // within the atlas rect
                        if (turned) (fx, fy) = (fy, 1 - fx);
                        float u = (old.x + fx * old.width) / 1024f, v = 1 - (old.y + fy * old.height) / 1024f;
                        Color c = design.GetPixelBilinear(u, v);
                        int i = (h - 1 - y) * w + x; // pixel rows run bottom-up
                        px[i] = Color.Lerp(px[i], new Color(c.r, c.g, c.b, 1f), c.a);
                    }
                }
                UnityEngine.Object.Destroy(design);
                var tex = new Texture2D(w, h, TextureFormat.RGBA32, true) { name = "TCGCC_StreetAtlas", wrapMode = _streetAtlas.wrapMode };
                tex.SetPixels32(px);
                tex.Apply(updateMipmaps: true);
                tex.Compress(highQuality: false); // 4096² RGBA32 + mips would be 85 MB of video memory
                tex.Apply(updateMipmaps: false, makeNoLongerReadable: true);
                tex.hideFlags = HideFlags.DontUnloadUnusedAsset;
                Plugin.Log.LogInfo($"Box look StreetBillboardTexture: using {path} (street atlas)");
                return tex;
            });
        }

        [HarmonyPatch(typeof(InteractablePackagingBox_Item), "Awake")]
        private static class ItemBoxAwakePatch
        {
            private static void Postfix(InteractablePackagingBox_Item __instance) => Track(__instance);
        }

        [HarmonyPatch(typeof(InteractablePackagingBox_Shelf), "Awake")]
        private static class ShelfBoxAwakePatch
        {
            private static void Postfix(InteractablePackagingBox_Shelf __instance) => Track(__instance);
        }

        [HarmonyPatch(typeof(InteractablePackagingBox_Card), "Awake")]
        private static class CardBoxAwakePatch
        {
            private static void Postfix(InteractablePackagingBox_Card __instance) => Track(__instance);
        }

        [HarmonyPatch(typeof(Customer), "Start")]
        private static class CustomerStartPatch
        {
            private static void Postfix(Customer __instance) => Track(__instance);
        }

        [HarmonyPatch(typeof(InteractableCashierCounter), "Awake")]
        private static class CashierCounterAwakePatch
        {
            private static void Postfix(InteractableCashierCounter __instance) => Track(__instance);
        }

        [HarmonyPatch(typeof(InteractablePlayTable), "Awake")]
        private static class PlayTableAwakePatch
        {
            private static void Postfix(InteractablePlayTable __instance) => Track(__instance);
        }

        /// <summary>The shop scene loaded (LightManager lives in it): the street billboards are scenery under Level_Environment_Grp.</summary>
        [HarmonyPatch(typeof(LightManager), "Awake")]
        private static class SceneryPatch
        {
            private static void Postfix(LightManager __instance)
            {
                foreach (var root in __instance.gameObject.scene.GetRootGameObjects())
                    if (root.name == "Level_Environment_Grp") Track(root.transform);
            }
        }

        /// <summary>Once per session, before the item art is committed (ItemInjector): each bulk box with a design gets it as a
        /// new slice of its texture array, and its ItemMeshData points there.</summary>
        internal static void AddBulkSlices()
        {
            foreach (var (item, _) in BulkBoxes)
            {
                string path = ShopPicture.ImagePath(Settings[item + "Texture"].Value);
                if (path == null) continue;
                if (!Enum.TryParse(item, out EItemType type))
                {
                    Core.Compat.Report("Box looks", $"bulk box item {item} isn't in this game");
                    continue;
                }
                var md = InventoryBase.GetItemMeshData(type);
                int slice = md != null ? Hooks.ItemVisuals.AddSlice(md.material, path) : -1;
                if (slice < 0)
                {
                    Core.Compat.Report("Box looks", $"bulk box {item} doesn't draw from a texture array");
                    continue;
                }
                md.textureIndex = (ushort)slice;
                Plugin.Log.LogInfo($"Box look {item}Texture: using {path}");
            }
        }

        // ---------------------------------------------------------------- warehouse-stored restock boxes (game 1.03+)

        private static readonly AccessTools.FieldRef<StoredBoxVisualBatcher, bool> SnapshotBuilt =
            Core.Compat.Field<StoredBoxVisualBatcher, bool>("Box looks in the warehouse", "m_SnapshotBuilt");
        // Our replacements in the batcher's list → the game's own replacement each one took the place of (null = none).
        private static readonly Dictionary<Material, Material> WarehouseCopies = new Dictionary<Material, Material>();

        /// <summary>The batcher's override list maps each cardboard material of the restock box prefabs to a copy drawn with its
        /// design (none for a box without one), and the batcher takes a new snapshot. The game's list already maps them to cheap
        /// stored-box materials (shader StoredBoxCheapLit): the copy is made of that material, so stored boxes keep its lighting.</summary>
        private static void Warehouse()
        {
            var batcher = StoredBoxVisualBatcher.Instance;
            if (batcher == null || SnapshotBuilt == null) return;
            var list = batcher.m_MaterialOverrideList;
            for (int i = list.Count - 1; i >= 0; i--) // back to the game's own list
            {
                if (list[i].replacement == null || !WarehouseCopies.TryGetValue(list[i].replacement, out var own)) continue;
                if (own != null) list[i] = new StoredBoxVisualBatcher.MaterialOverride { source = list[i].source, replacement = own };
                else list.RemoveAt(i);
            }
            foreach (var copy in WarehouseCopies.Keys) UnityEngine.Object.Destroy(copy);
            WarehouseCopies.Clear();
            foreach (bool big in new[] { true, false })
            {
                var prefab = RestockManager.GetPackageBoxPrefab(big);
                if (prefab == null || prefab.m_ClosedBox == null) continue;
                foreach (var r in prefab.m_ClosedBox.GetComponentsInChildren<MeshRenderer>(true))
                foreach (var mat in r.sharedMaterials)
                {
                    var main = MaterialTex.Main(mat);
                    if (main == null || !ByTexture.TryGetValue(main.name, out var key)) continue;
                    int at = list.FindIndex(o => o.source == mat);
                    if (at >= 0 && list[at].replacement != null && WarehouseCopies.ContainsKey(list[at].replacement)) continue; // the same material on another part
                    var own = at >= 0 ? list[at].replacement : null;
                    var tex = TextureFor(key);
                    if (tex == null) continue;
                    var from = own != null ? own : mat;
                    var fromMain = MaterialTex.Main(from);
                    var copy = new Material(from) { name = from.name + " (TCGCC)" };
                    foreach (var prop in from.GetTexturePropertyNames())
                    {
                        var t = from.GetTexture(prop);
                        if (t != null && (t == main || t == fromMain)) copy.SetTexture(prop, tex);
                    }
                    WarehouseCopies[copy] = own;
                    var entry = new StoredBoxVisualBatcher.MaterialOverride { source = mat, replacement = copy };
                    if (at >= 0) list[at] = entry;
                    else list.Add(entry);
                }
            }
            SnapshotBuilt(batcher) = false;
            Plugin.Log.LogInfo($"Box look: {WarehouseCopies.Count} design(s) on boxes stored on warehouse shelves");
        }

        /// <summary>The batcher woke: it sits in the shop scene, and RestockManager.Awake can run first, when it has no Instance yet.</summary>
        [HarmonyPatch(typeof(StoredBoxVisualBatcher), "Awake")]
        private static class WarehousePatch
        {
            private static void Postfix() => Warehouse();
        }

        /// <summary>A setting changed: every tracked box shows its (new) texture or vanilla.</summary>
        private static void Reapply()
        {
            Prune();
            foreach (var kv in Shown.ToList()) Show(kv.Key, kv.Value.slot, kv.Value.key);
            Warehouse();
        }
    }
}
