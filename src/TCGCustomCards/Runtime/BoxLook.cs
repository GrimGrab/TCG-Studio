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
    /// shop scene (the billboards, static-batched scenery). Bulk boxes are Items (pooled, so every SetMesh sets or clears the
    /// texture) plus the workbench's closing animation.
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
        };

        /// <summary>The bulk box items (one mesh JankCardBox_Closed, a texture per variant).</summary>
        private static readonly string[] BulkBoxes =
        {
            "BulkBox_TetramonBase", "BulkBox_TetramonDestiny",
            "BulkBox_TetramonBaseGradeHG", "BulkBox_TetramonBaseGradeMG", "BulkBox_TetramonBaseGradePG", "BulkBox_TetramonBaseGradeUG",
            "BulkBox_TetramonDestinyGradeHG", "BulkBox_TetramonDestinyGradeMG", "BulkBox_TetramonDestinyGradePG", "BulkBox_TetramonDestinyGradeUG",
        };

        /// <summary>Settings by key (cardboard keys, and bulk boxes as "&lt;EItemType name&gt;Texture").</summary>
        private static readonly Dictionary<string, ConfigEntry<string>> Settings = new Dictionary<string, ConfigEntry<string>>();
        private static readonly Dictionary<string, PictureCache<Texture2D>> Textures = new Dictionary<string, PictureCache<Texture2D>>();

        /// <summary>Renderers showing a box look: renderer → (material slot, setting key). Pooled items move between keys.</summary>
        private static readonly Dictionary<Renderer, (int slot, string key)> Shown = new Dictionary<Renderer, (int, string)>();

        private static readonly AccessTools.FieldRef<InteractableWorkbench, EItemType> WorkbenchItemType =
            AccessTools.FieldRefAccess<InteractableWorkbench, EItemType>("m_CurrentItemTypeSpawn");

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
            foreach (var name in BulkBoxes)
            {
                string key = name + "Texture";
                Settings[key] = config.Bind(Section, key, "", $"Texture of the bulk card box {name.Substring("BulkBox_".Length)}: " + how);
            }
            foreach (var s in Settings.Values) s.SettingChanged += (_, __) => Reapply();
        }

        /// <summary>The built texture of a setting; null = vanilla (empty, or the file can't be read).</summary>
        private static Texture2D TextureFor(string key)
        {
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
                if (slot >= 0) Show(r, slot, ByTexture[texture]);
            }
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

        private static string BulkKey(EItemType type)
        {
            string name = type.ToString();
            return name.StartsWith("BulkBox_", StringComparison.Ordinal) ? name + "Texture" : null;
        }

        /// <summary>A (pooled) item got its mesh: a bulk box shows its look, anything else drops one it showed before.</summary>
        [HarmonyPatch(typeof(Item), nameof(Item.SetMesh))]
        private static class ItemSetMeshPatch
        {
            private static void Postfix(Item __instance, Material material, EItemType itemType)
            {
                var r = __instance.m_Mesh;
                if (r == null) return;
                string key = BulkKey(itemType);
                if (key == null)
                {
                    if (Shown.ContainsKey(r)) Show(r, 0, null);
                    return;
                }
                Prune();
                var main = MaterialTex.Main(material);
                string vanilla = main != null ? main.name : null;
                int slot = Slot(r, n => n == vanilla, out _);
                Show(r, slot < 0 ? 0 : slot, key);
            }
        }

        /// <summary>The workbench's closing bulk box (an animated copy that takes the item's material).</summary>
        [HarmonyPatch(typeof(InteractableWorkbench), nameof(InteractableWorkbench.PlayBundlingCardBoxSequence))]
        private static class WorkbenchPatch
        {
            private static void Postfix(InteractableWorkbench __instance)
            {
                var r = __instance.m_JankBoxSkinMesh;
                if (r == null) return;
                Prune();
                Show(r, 0, BulkKey(WorkbenchItemType(__instance)));
            }
        }

        /// <summary>A setting changed: every tracked box shows its (new) texture or vanilla.</summary>
        private static void Reapply()
        {
            Prune();
            foreach (var kv in Shown.ToList()) Show(kv.Key, kv.Value.slot, kv.Value.key);
        }
    }
}
