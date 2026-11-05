using System.Collections.Generic;
using System.Diagnostics;
using HarmonyLib;
using UnityEngine;
using UnityEngine.Rendering;

namespace TCGCustomCards.Hooks
{
    /// <summary>
    /// Single owner of how the game draws our custom items (game 1.03+, Unity 6). The game draws items from shared data, and we
    /// add our art to that data so the game's own code draws it — no patches on the drawing paths:
    /// <list type="bullet">
    /// <item>Item art: packs, boxes and most accessories use one texture-array material per model; each item picks its slice
    /// with "_TexIndex" (ItemMeshData.textureIndex). <see cref="ApplyArt"/> gives a custom item the next slice of its base's
    /// array; <see cref="CommitArt"/> builds each array again with our slices after the vanilla ones (DXT5 like vanilla) and
    /// puts it on every material that used the vanilla array (items, and the pack/box being opened, which copy only the
    /// item's slice index). <see cref="UseOurArt"/> repeats that for materials a reloaded scene brought back.</item>
    /// <item><see cref="AddPriceTagIcons"/>: our icons in the price-tag atlas and map.</item>
    /// <item>One patch, for a gap in the game: items restored from a save get no slice (<see cref="SetMeshSlicePatch"/>).</item>
    /// </list>
    /// </summary>
    internal static class ItemVisuals
    {
        // ---- item art: our slices in the game's texture arrays

        /// <summary>One vanilla texture array and the slices we add to it.</summary>
        private sealed class Stack
        {
            public int Prop;                        // the materials' Texture2DArray property
            public Texture2DArray Vanilla, Ours;    // Ours = vanilla slices, then Added
            public readonly List<Art> Added = new List<Art>();
        }

        /// <summary>One added slice: the picture's file (loaded when the arrays are built) and/or the picture already loaded.</summary>
        private struct Art
        {
            public string Path;
            public Texture2D Tex;
        }

        private static readonly Dictionary<Texture2DArray, Stack> Stacks = new Dictionary<Texture2DArray, Stack>();
        private static bool _committed;
        private static int _usedInScene = -1; // the scene UseOurArt last ran in

        /// <summary>
        /// Gives a custom item the picture at <paramref name="path"/> when its base is drawn with a texture-array material:
        /// <paramref name="md"/> keeps the base's shared material and gets the slice the picture will have (built by
        /// <see cref="CommitArt"/>). False = the base uses a plain material (or there is no picture): the caller does the
        /// plain-material way.
        /// </summary>
        internal static bool ApplyArt(ItemMeshData md, Material arrayMat, string path)
        {
            if (md.arrayMaterialType == ItemMeshData.ItemShareTexture2DArrayMaterial.None) return false;
            int slice = AddSlice(arrayMat, path);
            if (slice < 0) return false;
            md.material = arrayMat;
            md.textureIndex = (ushort)slice;
            md.secondTextureIndex = md.textureIndex;
            return true;
        }

        /// <summary>
        /// The slice a picture will have in <paramref name="arrayMat"/>'s texture array (built by <see cref="CommitArt"/>; draw
        /// with arrayMat itself and "_TexIndex" = the slice), or -1 when the material samples no array (or no picture, or too
        /// late: the arrays are built). The picture is its file <paramref name="path"/> and/or <paramref name="tex"/> already
        /// loaded. Used for items and for own models on array bases (<see cref="Runtime.ModelSwap"/>).
        /// </summary>
        internal static int AddSlice(Material arrayMat, string path, Texture2D tex = null)
        {
            if (string.IsNullOrEmpty(path) || !System.IO.File.Exists(path)) path = null;
            if (_committed || (path == null && tex == null)) return -1;
            int prop = ArrayProperty(arrayMat);
            if (prop < 0 || !(arrayMat.GetTexture(prop) is Texture2DArray vanilla)) return -1;
            if (!Stacks.TryGetValue(vanilla, out var stack)) Stacks[vanilla] = stack = new Stack { Prop = prop, Vanilla = vanilla };
            int k = stack.Added.FindIndex(a => path != null ? a.Path == path : a.Tex == tex);
            if (k < 0) { k = stack.Added.Count; stack.Added.Add(new Art { Path = path, Tex = tex }); }
            return vanilla.depth + k;
        }

        /// <summary>Name id of the material's Texture2DArray property, or -1 when it samples none.</summary>
        private static int ArrayProperty(Material m)
        {
            var s = m != null ? m.shader : null;
            if (s == null) return -1;
            for (int i = 0; i < s.GetPropertyCount(); i++)
                if (s.GetPropertyType(i) == ShaderPropertyType.Texture && s.GetPropertyTextureDimension(i) == TextureDimension.Tex2DArray)
                    return s.GetPropertyNameId(i);
            return -1;
        }

        /// <summary>Once per session, after every custom item got its slice (end of ItemInjector.EnsureInjected).</summary>
        internal static void CommitArt()
        {
            if (_committed) return;
            _committed = true;
            var watch = Stopwatch.StartNew();
            int slices = 0;
            foreach (var stack in Stacks.Values)
            {
                stack.Ours = Build(stack);
                slices += stack.Added.Count;
            }
            UseOurArt();
            if (Stacks.Count > 0)
                Plugin.Log.LogInfo($"Item art: {slices} custom slice(s) added to {Stacks.Count} texture array(s) in {watch.ElapsedMilliseconds} ms");
        }

        /// <summary>
        /// Puts our arrays on every loaded material that still uses a vanilla one (assets once; scene materials again after a
        /// save loads). Cheap: no texture work.
        /// </summary>
        internal static void UseOurArt()
        {
            if (!_committed || Stacks.Count == 0) return;
            // Once per loaded scene: EnsureInjected (the caller) also runs on every worker's start, and a scene's materials are
            // all loaded by its first call.
            int scene = UnityEngine.SceneManagement.SceneManager.GetActiveScene().handle;
            if (scene == _usedInScene) return;
            _usedInScene = scene;
            foreach (var m in Resources.FindObjectsOfTypeAll<Material>())
                foreach (var stack in Stacks.Values)
                    if (stack.Ours != null && m.HasProperty(stack.Prop) && m.GetTexture(stack.Prop) == stack.Vanilla)
                        m.SetTexture(stack.Prop, stack.Ours);
        }

        /// <summary>The vanilla slices (GPU copy) followed by ours, each compressed like vanilla; null if that failed (logged).</summary>
        private static Texture2DArray Build(Stack stack)
        {
            var v = stack.Vanilla;
            try
            {
                var ours = new Texture2DArray(v.width, v.height, v.depth + stack.Added.Count, v.format, v.mipmapCount, !v.isDataSRGB)
                {
                    name = v.name + " (TCGCC)", wrapMode = v.wrapMode, filterMode = v.filterMode, anisoLevel = v.anisoLevel,
                    hideFlags = HideFlags.DontUnloadUnusedAsset,
                };
                for (int i = 0; i < v.depth; i++) Graphics.CopyTexture(v, i, ours, i);
                for (int k = 0; k < stack.Added.Count; k++)
                {
                    var art = stack.Added[k];
                    // Loaded only for its slice: freed after. Already in the cache = something else draws with it (same file, same path).
                    bool onlyForSlice = art.Tex == null && !Runtime.ImageCache.Has(art.Path);
                    var tex = art.Tex != null ? art.Tex : Runtime.ImageCache.Get(art.Path)?.texture;
                    var slice = tex != null ? Slice(tex, v) : null;
                    if (onlyForSlice) Runtime.ImageCache.Forget(art.Path);
                    if (slice == null) continue; // that item shows black; logged
                    Graphics.CopyTexture(slice, 0, ours, v.depth + k);
                    Object.Destroy(slice);
                }
                return ours;
            }
            catch (System.Exception e)
            {
                Core.Compat.Report(ArtFeature, $"couldn't build texture array {v.name} ({v.width}x{v.height} {v.format}, {v.mipmapCount} mips): {e.Message}");
                return null;
            }
        }

        /// <summary>
        /// <paramref name="tex"/> at the array's slice size with its mips, compressed to the array's format (the game's are DXT5);
        /// null if the format can't be matched (logged).
        /// </summary>
        private static Texture2D Slice(Texture2D tex, Texture2DArray like)
        {
            var rt = RenderTexture.GetTemporary(like.width, like.height, 0, RenderTextureFormat.ARGB32, RenderTextureReadWrite.sRGB);
            var was = RenderTexture.active;
            try
            {
                Graphics.Blit(tex, rt);
                RenderTexture.active = rt;
                var slice = new Texture2D(like.width, like.height, TextureFormat.RGBA32, like.mipmapCount, false);
                slice.ReadPixels(new Rect(0, 0, like.width, like.height), 0, 0, false);
                slice.Apply(updateMipmaps: true);
                if (slice.format != like.format) slice.Compress(highQuality: false);
                if (slice.format != like.format || slice.mipmapCount != like.mipmapCount)
                {
                    Plugin.Log.LogError($"Item art: {tex.name} became {slice.format}/{slice.mipmapCount} mips, the array needs {like.format}/{like.mipmapCount}");
                    Object.Destroy(slice);
                    return null;
                }
                return slice;
            }
            finally
            {
                RenderTexture.active = was;
                RenderTexture.ReleaseTemporary(rt);
            }
        }

        // ---- items restored from a save

        /// <summary>
        /// Item.SetMesh (the game's restore paths: customers' bags and hands loaded from a save, the auto pack opener, the
        /// workbench) sets an item's material but never its texture-array slice, so a restored item showed slice 0 or whatever its
        /// pooled object showed last (vanilla items too). Sets the slice the way the game's shelf code does.
        /// </summary>
        [HarmonyPatch(typeof(Item), nameof(Item.SetMesh))]
        private static class SetMeshSlicePatch
        {
            private static void Postfix(Item __instance, EItemType itemType)
            {
                var md = InventoryBase.GetItemMeshData(itemType);
                if (md == null) return;
                switch (md.arrayMaterialType)
                {
                    case ItemMeshData.ItemShareTexture2DArrayMaterial.FirstMesh:
                    case ItemMeshData.ItemShareTexture2DArrayMaterial.FirstMeshArraySecondDefault:
                        __instance.SetPropertyBlock(ref __instance.m_Mesh, ref md.textureIndex);
                        break;
                    case ItemMeshData.ItemShareTexture2DArrayMaterial.SecondMesh:
                        __instance.SetPropertyBlock(ref __instance.m_MeshSecondary, ref md.textureIndex);
                        break;
                    case ItemMeshData.ItemShareTexture2DArrayMaterial.BothMeshes:
                        __instance.SetPropertyBlock(ref __instance.m_Mesh, ref md.textureIndex);
                        __instance.SetPropertyBlock(ref __instance.m_MeshSecondary, ref md.textureIndex);
                        break;
                }
            }
        }

        // ---- price-tag icons

        // Price tags (shelves, boxes, furniture) are one instanced mesh whose shader draws icon, price and amount from a 4096²
        // atlas of _AtlasGridSize × _AtlasGridSize cells (vanilla 16: cells 0-25 glyphs, items 32-155/206-213, furniture
        // 156-205); UI_PriceTag looks a type's cell up in its PriceTagAtlasMap. We add our types to that map and give the tag
        // material a finer-grid copy of the atlas with every vanilla cell under its own index and our icons after them, so the
        // game's own tag code shows them. Map and material are assets: changed once per session.
        private const int VanillaGrid = 16, VanillaCells = 256, AtlasSize = 4096;
        private static readonly int GridId = Shader.PropertyToID("_AtlasGridSize");
        private const string TagFeature = "Price-tag icons", ArtFeature = "Item art";
        private static bool _tagsDone;

        /// <summary>Once per session, when every custom item and furniture piece exists (end of ItemInjector.EnsureInjected).</summary>
        internal static void AddPriceTagIcons(StockItemData_ScriptableObject so)
        {
            if (_tagsDone) return;
            _tagsDone = true;
            var itemCache = Core.Compat.Field<PriceTagAtlasMap, Dictionary<int, int>>(TagFeature, "m_ItemCache");
            var objectCache = Core.Compat.Field<PriceTagAtlasMap, Dictionary<int, int>>(TagFeature, "m_ObjectCache");
            if (itemCache == null || objectCache == null) return;
            var maps = new HashSet<PriceTagAtlasMap>();
            var materials = new HashSet<Material>();
            foreach (var t in Resources.FindObjectsOfTypeAll<UI_PriceTag>())
            {
                if (t.m_AtlasMap != null) maps.Add(t.m_AtlasMap);
                if (t.m_InstancedRenderer != null && t.m_InstancedRenderer.sharedMaterial != null) materials.Add(t.m_InstancedRenderer.sharedMaterial);
            }
            Texture vanillaAtlas = null;
            foreach (var m in materials) if (m.mainTexture != null) { vanillaAtlas = m.mainTexture; break; }
            if (maps.Count == 0 || vanillaAtlas == null) { Core.Compat.Report(TagFeature, "no price-tag atlas found"); return; }
            foreach (var m in materials)
                if (m.HasProperty(GridId) && Mathf.RoundToInt(m.GetFloat(GridId)) != VanillaGrid)
                { Core.Compat.Report(TagFeature, $"the tag atlas grid is {m.GetFloat(GridId)}, not {VanillaGrid}"); return; }
            foreach (var map in maps)
                if (map.itemTypeValues == null || map.itemCellIndices == null || map.objectTypeValues == null || map.objectCellIndices == null)
                { Core.Compat.Report(TagFeature, "the tag atlas map has no type/cell lists"); return; }

            // Our icons in a fixed order: custom items (packs, boxes, accessories), then custom furniture.
            var icons = new List<Sprite>();
            var items = new List<int>();
            var objects = new List<int>();
            for (int i = (int)EItemType.Max; i < so.m_ItemDataList.Count; i++)
                if (Runtime.Registry.IsCustomItem((EItemType)i)) { items.Add(i); icons.Add(so.m_ItemDataList[i].icon); }
            foreach (var f in Runtime.Registry.Furniture)
                if (Runtime.Registry.IsCustomFurniture(f.Object)) { objects.Add((int)f.Object); icons.Add(InventoryBase.GetFurniturePurchaseData(f.Object)?.icon); }
            if (icons.Count == 0) return;

            int grid = Mathf.Max(VanillaGrid + 1, Mathf.CeilToInt(Mathf.Sqrt(VanillaCells + icons.Count)));
            var atlas = BuildAtlas(vanillaAtlas, icons, grid);
            if (atlas == null) return;

            foreach (var map in maps)
            {
                map.itemTypeValues = Append(map.itemTypeValues, items, null);
                map.itemCellIndices = Append(map.itemCellIndices, items, i => VanillaCells + i);
                map.objectTypeValues = Append(map.objectTypeValues, objects, null);
                map.objectCellIndices = Append(map.objectCellIndices, objects, i => VanillaCells + items.Count + i);
                itemCache(map) = null; // rebuilt from the arrays on the next lookup
                objectCache(map) = null;
            }
            int changed = 0;
            foreach (var m in materials)
            {
                if (m.mainTexture != vanillaAtlas) { Plugin.Log.LogWarning($"Price tags: material {m.name} uses another atlas ({m.mainTexture?.name}) — left as is"); continue; }
                m.mainTexture = atlas;
                m.SetFloat(GridId, grid);
                changed++;
            }
            Plugin.Log.LogInfo($"Price tags: {items.Count} item and {objects.Count} furniture icon(s) added; atlas grid {grid}×{grid} " +
                               $"(vanilla {VanillaGrid}), {maps.Count} map(s), {changed} material(s)");
        }

        /// <summary>The array with one entry per <paramref name="add"/> appended: the value itself, or <paramref name="value"/>(index).</summary>
        private static int[] Append(int[] array, List<int> add, System.Func<int, int> value)
        {
            var list = new List<int>(array ?? new int[0]);
            for (int i = 0; i < add.Count; i++) list.Add(value == null ? add[i] : value(i));
            return list.ToArray();
        }

        /// <summary>
        /// Finer-grid copy of the vanilla atlas (vanilla cell i, rows from the top, → our cell i) with <paramref name="icons"/> in
        /// cells 256+, drawn on the GPU; null if that failed (logged).
        /// </summary>
        private static Texture2D BuildAtlas(Texture vanillaAtlas, List<Sprite> icons, int grid)
        {
            float cell = AtlasSize / (float)grid;
            var rt = RenderTexture.GetTemporary(new RenderTextureDescriptor(AtlasSize, AtlasSize, RenderTextureFormat.ARGB32, 0)
                { sRGB = true, useMipMap = true, autoGenerateMips = false });
            var was = RenderTexture.active;
            try
            {
                RenderTexture.active = rt;
                GL.Clear(true, true, Color.clear);
                GL.PushMatrix();
                GL.LoadPixelMatrix(0, AtlasSize, AtlasSize, 0); // y down: cell 0 top-left, like the vanilla atlas
                Rect At(int i) => new Rect(i % grid * cell, i / grid * cell, cell, cell);
                for (int i = 0; i < VanillaCells; i++)
                {
                    int col = i % VanillaGrid, row = i / VanillaGrid;
                    var src = new Rect(col / (float)VanillaGrid, 1f - (row + 1) / (float)VanillaGrid, 1f / VanillaGrid, 1f / VanillaGrid);
                    Graphics.DrawTexture(At(i), vanillaAtlas, src, 0, 0, 0, 0);
                }
                for (int k = 0; k < icons.Count; k++)
                {
                    var s = icons[k];
                    if (s == null || s.texture == null) continue;
                    var tr = s.textureRect;
                    var src = new Rect(tr.x / s.texture.width, tr.y / s.texture.height, tr.width / s.texture.width, tr.height / s.texture.height);
                    var dst = At(VanillaCells + k);
                    float scale = Mathf.Min(dst.width / tr.width, dst.height / tr.height); // fit, keep the icon's shape
                    float w = tr.width * scale, h = tr.height * scale;
                    Graphics.DrawTexture(new Rect(dst.x + (dst.width - w) / 2, dst.y + (dst.height - h) / 2, w, h), s.texture, src, 0, 0, 0, 0);
                }
                GL.PopMatrix();
                rt.GenerateMips();
                var atlas = new Texture2D(AtlasSize, AtlasSize, TextureFormat.RGBA32, rt.mipmapCount, false)
                    { name = "TCGCC_PriceTagAtlas", filterMode = vanillaAtlas.filterMode, wrapMode = TextureWrapMode.Clamp, hideFlags = HideFlags.DontUnloadUnusedAsset };
                Graphics.CopyTexture(rt, atlas);
                if (Plugin.DumpDiagnostics.Value) DumpAtlas(rt);
                return atlas;
            }
            catch (System.Exception e)
            {
                Core.Compat.Report(TagFeature, $"couldn't build the icon atlas: {e.Message}");
                return null;
            }
            finally
            {
                RenderTexture.active = was;
                RenderTexture.ReleaseTemporary(rt);
            }
        }

        /// <summary>Diagnostics: the built atlas, quarter size, next to diagnostics.txt.</summary>
        private static void DumpAtlas(RenderTexture rt)
        {
            var small = RenderTexture.GetTemporary(1024, 1024, 0, RenderTextureFormat.ARGB32, RenderTextureReadWrite.sRGB);
            var was = RenderTexture.active;
            try
            {
                Graphics.Blit(rt, small);
                RenderTexture.active = small;
                var read = new Texture2D(1024, 1024, TextureFormat.RGBA32, false);
                read.ReadPixels(new Rect(0, 0, 1024, 1024), 0, 0);
                read.Apply();
                System.IO.File.WriteAllBytes(System.IO.Path.Combine(Plugin.DumpDir, "price-tag-atlas.png"), read.EncodeToPNG());
                Object.Destroy(read);
            }
            catch (System.Exception e) { Plugin.Log.LogWarning($"Price tags: atlas dump failed: {e.Message}"); }
            finally { RenderTexture.active = was; RenderTexture.ReleaseTemporary(small); }
        }
    }
}
