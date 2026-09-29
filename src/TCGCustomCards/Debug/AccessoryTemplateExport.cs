using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using Newtonsoft.Json;
using TCGCustomCards.Core;
using TCGCustomCards.Runtime;
using UnityEngine;
using UnityEngine.Rendering;

namespace TCGCustomCards.Debug
{
    /// <summary>
    /// Exports what TCG Studio needs to author custom accessories (deck boxes, playmats, sleeves, dice, comics, collection books, battle decks) into &lt;plugin&gt;\templates\accessories\:
    /// per vanilla Deckbox/Playmat item <c>{type}_texture.png</c>, <c>{type}_icon.png</c>; per distinct mesh <c>{mesh}.obj</c>
    /// (positions, normals, UVs, one group per submesh — captured on the GPU because the game's meshes aren't CPU-readable;
    /// fallback <c>{mesh}_uvpos.png</c>/<c>_uvnormal.png</c>);
    /// and <c>accessories.json</c> describing items (category, mesh, materials, texture properties, license rows, prices) and the
    /// fixed play-table playmat meshes (prefab meshes that only get the playmat item's material).
    /// </summary>
    internal static class AccessoryTemplateExport
    {
        public static int Run(string templatesDir)
        {
            string dir = Path.Combine(templatesDir, "accessories");
            Directory.CreateDirectory(dir);
            var so = CSingleton<InventoryBase>.Instance.m_StockItemData_SO;
            int n = 0;
            var items = new List<object>();
            var meshes = new Dictionary<Mesh, string>();

            for (int i = 0; i < (int)EItemType.Max && i < so.m_ItemDataList.Count; i++)
            {
                var data = so.m_ItemDataList[i];
                var type = (EItemType)i;
                if (data == null || !IsAccessory(type, data)) continue;
                var md = so.m_ItemMeshDataList[i];
                var tex = md?.material != null ? md.material.mainTexture : null;
                if (tex != null) n += TemplateExport.SaveTexture(tex, Path.Combine(dir, $"{type}_texture.png"));
                if (data.icon != null) n += TemplateExport.SaveSprite(data.icon, Path.Combine(dir, $"{type}_icon.png"));
                string meshFile = md?.mesh != null ? ExportMesh(md.mesh, dir, meshes, ref n) : null;
                string meshFile2 = md?.meshSecondary != null ? ExportMesh(md.meshSecondary, dir, meshes, ref n) : null;
                // Secondary material / material list textures (e.g. a box with something inside).
                var tex2 = md?.materialSecondary != null ? md.materialSecondary.mainTexture : null;
                if (tex2 != null && tex2 != tex) n += TemplateExport.SaveTexture(tex2, Path.Combine(dir, $"{type}_texture2.png"));
                var listTex = new List<string>();
                if (md?.materialList != null)
                    for (int m = 0; m < md.materialList.Count; m++)
                    {
                        var lt = md.materialList[m] != null ? md.materialList[m].mainTexture : null;
                        if (lt == null) { listTex.Add(null); continue; }
                        n += TemplateExport.SaveTexture(lt, Path.Combine(dir, $"{type}_list{m}_texture.png"));
                        listTex.Add($"{type}_list{m}_texture.png");
                    }
                var shown = new List<string>();
                if (so.m_ShownItemType.Contains(type)) shown.Add("items");
                if (so.m_ShownAccessoryItemType.Contains(type)) shown.Add("accessories");
                if (so.m_ShownFigurineItemType.Contains(type)) shown.Add("figurines");
                if (so.m_ShownBoardGameItemType.Contains(type)) shown.Add("boardgames");
                if (so.m_ShownAllItemType.Contains(type)) shown.Add("all");

                var rows = new List<object>();
                for (int r = 0; r < so.m_RestockDataList.Count; r++)
                {
                    var rd = so.m_RestockDataList[r];
                    if (rd.itemType != type) continue;
                    rows.Add(new { index = r, big = rd.isBigBox, level = rd.licenseShopLevelRequired, price = rd.licensePrice, rd.amount });
                }
                items.Add(new
                {
                    type = type.ToString(), id = i, category = data.category.ToString(), name = data.name,
                    data.baseCost, data.marketPriceMinPercent, data.marketPriceMaxPercent, data.isTallItem,
                    itemDimension = V(data.itemDimension),
                    // v11: how the item sits in a shelf slot / in the hand (figurine size reference).
                    data.posYOffsetInBox, data.scaleOffsetInBox, data.itemHandScaleOffset, data.iconScale,
                    colliderPosOffset = V(data.colliderPosOffset), colliderScale = V(data.colliderScale),
                    texture = tex != null ? $"{type}_texture.png" : null, textureName = tex?.name,
                    textureSize = tex != null ? new[] { tex.width, tex.height } : null,
                    icon = data.icon != null ? $"{type}_icon.png" : null,
                    iconRect = data.icon != null ? new[] { data.icon.rect.width, data.icon.rect.height } : null,
                    mesh = meshFile, meshName = md?.mesh?.name, bounds = md?.mesh != null ? Bounds(md.mesh.bounds) : null,
                    meshSecondary = md?.meshSecondary?.name, meshSecondaryFile = meshFile2,
                    boundsSecondary = md?.meshSecondary != null ? Bounds(md.meshSecondary.bounds) : null,
                    texture2 = tex2 != null && tex2 != tex ? $"{type}_texture2.png" : null, materialListTextures = listTex,
                    shownIn = shown,
                    material = Mat(md?.material), materialSecondary = Mat(md?.materialSecondary),
                    materialList = md?.materialList?.Select(Mat).ToList(),
                    restockRows = rows,
                });
            }

            // Play-table playmat surfaces: the playmat item only supplies the material; the mesh belongs to the table prefab.
            var tables = new List<object>();
            void Table(string owner, Renderer r)
            {
                if (r == null) return;
                var mf = r.GetComponent<MeshFilter>();
                if (mf == null || mf.sharedMesh == null) return;
                tables.Add(new
                {
                    owner, path = PathOf(r.transform), mesh = ExportMesh(mf.sharedMesh, dir, meshes, ref n), meshName = mf.sharedMesh.name,
                    bounds = Bounds(mf.sharedMesh.bounds), lossyScale = V(r.transform.lossyScale), material = Mat(r.sharedMaterial),
                });
            }
            foreach (var t in Resources.FindObjectsOfTypeAll<TableGameItemSet>())
            {
                Table($"TableGameItemSet({t.name}).m_PlayMatMesh", t.m_PlayMatMesh);
                Table($"TableGameItemSet({t.name}).m_DeckBoxMesh", t.m_DeckBoxMesh);
            }
            foreach (var p in Resources.FindObjectsOfTypeAll<PlayCardSet>())
            {
                Table($"PlayCardSet({p.name}).m_Mesh_Playmat", p.m_Mesh_Playmat);
                Table($"PlayCardSet({p.name}).m_Mesh_Deckbox", p.m_Mesh_Deckbox);
            }

            // v11: shop item prefab (mesh child transform) and shelves, so TCG Studio can show a figurine on a real shelf.
            object itemPrefab = null, shelves = null;
            try { itemPrefab = ShelfExport.ItemPrefab(); }
            catch (Exception e) { Plugin.Log.LogWarning($"Item prefab export failed: {e.Message}"); }
            try { shelves = ShelfExport.Run(dir, ref n); }
            catch (Exception e) { Plugin.Log.LogWarning($"Shelf export failed: {e}"); }

            File.WriteAllText(Path.Combine(dir, "accessories.json"),
                JsonConvert.SerializeObject(new { version = 2, items, tables, itemPrefab, shelves }, Formatting.Indented));
            Plugin.Log.LogInfo($"Accessory templates: {items.Count} items, {meshes.Count} meshes, {tables.Count} table renderers → {dir}");
            return n + 1;
        }

        /// <summary>Items TCG Studio can make custom versions of: deck boxes, playmats, sleeves, dice, comics, collection books.</summary>
        private static bool IsAccessory(EItemType type, ItemData data)
        {
            // Everything the mod supports, plus the other "Sleeve"-category card supplies (UP_*) for reference.
            return AccessoryKinds.KindOf(type).HasValue || data.category == EItemCategory.Sleeve || data.category == EItemCategory.Dice;
        }

        internal static string ExportMesh(Mesh mesh, string dir, Dictionary<Mesh, string> done, ref int n)
        {
            if (done.TryGetValue(mesh, out var file)) return file;
            file = Safe(mesh.name) + ".obj";
            // Distinct meshes can share a name.
            while (done.ContainsValue(file)) file = Safe(mesh.name) + "_" + done.Count + ".obj";
            done[mesh] = file;
            try
            {
                string obj = MeshReader.ToObj(mesh);
                if (obj == null)
                {
                    n += MeshReader.SaveUvMaps(mesh, dir, Safe(mesh.name));
                    done[mesh] = null;
                    return null;
                }
                File.WriteAllText(Path.Combine(dir, file), obj);
                n++;
                return file;
            }
            catch (Exception e)
            {
                Plugin.Log.LogWarning($"Mesh export failed for {mesh.name}: {e.Message}");
                done[mesh] = null;
                return null;
            }
        }

        internal static object Mat(Material m)
        {
            if (m == null) return null;
            var tex = new Dictionary<string, string>();
            var sh = m.shader;
            for (int i = 0; i < sh.GetPropertyCount(); i++)
            {
                if (sh.GetPropertyType(i) != ShaderPropertyType.Texture) continue;
                string prop = sh.GetPropertyName(i);
                var t = m.GetTexture(prop);
                if (t != null) tex[prop] = $"{t.name} {t.width}x{t.height}";
            }
            return new { name = m.name, shader = sh.name, textures = tex, mainTextureScale = V(m.mainTextureScale), mainTextureOffset = V(m.mainTextureOffset) };
        }

        internal static object Bounds(Bounds b) => new { center = V(b.center), size = V(b.size) };
        internal static float[] V(Vector3 v) => new[] { v.x, v.y, v.z };
        private static float[] V(Vector2 v) => new[] { v.x, v.y };
        private static string Safe(string s) => string.Concat((s ?? "mesh").Select(c => char.IsLetterOrDigit(c) || c == '_' || c == '-' ? c : '_'));

        internal static string PathOf(Transform t)
        {
            var sb = new StringBuilder(t.name);
            for (var p = t.parent; p != null; p = p.parent) sb.Insert(0, p.name + "/");
            return sb.ToString();
        }
    }

    /// <summary>
    /// Reads mesh geometry. Readable meshes use the normal API. The game's meshes aren't readable, and on DX11 GetData on their
    /// vertex buffers returns zeros, so they are drawn once with our capture shader instead (bundle tcgcc_foil).
    /// </summary>
    internal static class MeshReader
    {
        /// <summary>Geometry of any mesh (readable or captured on the GPU). False when it can't be read.</summary>
        public static bool Read(Mesh mesh, out Vector3[] pos, out Vector3[] nrm, out Vector2[] uv, out List<int[]> subs)
        {
            if (mesh.isReadable)
            {
                pos = mesh.vertices; nrm = mesh.normals; uv = mesh.uv;
                subs = Enumerable.Range(0, mesh.subMeshCount).Select(s => mesh.GetTriangles(s)).ToList();
                return true;
            }
            return CaptureGpu(mesh, out pos, out nrm, out uv, out subs);
        }

        public static string ToObj(Mesh mesh)
        {
            if (!Read(mesh, out var pos, out var nrm, out var uv, out var subs)) return null;

            var inv = System.Globalization.CultureInfo.InvariantCulture;
            var sb = new StringBuilder();
            sb.AppendLine($"# {mesh.name} — exported by TCGCustomCards (Unity space: left-handed, Y up; UV origin bottom-left)");
            sb.AppendLine($"# vertices {pos.Length}, submeshes {subs.Count}, readable {mesh.isReadable}");
            foreach (var v in pos) sb.AppendLine(string.Format(inv, "v {0} {1} {2}", v.x, v.y, v.z));
            bool hasUv = uv != null && uv.Length == pos.Length, hasN = nrm != null && nrm.Length == pos.Length;
            if (hasUv) foreach (var t in uv) sb.AppendLine(string.Format(inv, "vt {0} {1}", t.x, t.y));
            if (hasN) foreach (var t in nrm) sb.AppendLine(string.Format(inv, "vn {0} {1} {2}", t.x, t.y, t.z));
            for (int s = 0; s < subs.Count; s++)
            {
                sb.AppendLine($"g submesh{s}");
                var tri = subs[s];
                for (int i = 0; i + 2 < tri.Length; i += 3)
                {
                    sb.Append('f');
                    for (int k = 0; k < 3; k++)
                    {
                        int x = tri[i + k] + 1;
                        sb.Append(' ').Append(x);
                        if (hasUv || hasN) sb.Append('/').Append(hasUv ? x.ToString() : "");
                        if (hasN) sb.Append('/').Append(x);
                    }
                    sb.AppendLine();
                }
            }
            return sb.ToString();
        }

        private static Material _capture;
        // Matches struct Tri in TCGCC_MeshCapture.shader (structured buffers are tightly packed): 3 float3 positions,
        // 3 float3 normals, 3 float2 UVs = 9 + 9 + 6 floats = 96 bytes.
        private const int TriFloats = 24;

        /// <summary>
        /// Draws the mesh with Hidden/TCGCC/MeshCapture pass 0, whose geometry shader writes every triangle into a UAV buffer, and
        /// reads that back. Returns a triangle soup (3 vertices per triangle). False when the shader/UAV path isn't available.
        /// </summary>
        private static bool CaptureGpu(Mesh mesh, out Vector3[] pos, out Vector3[] nrm, out Vector2[] uv, out List<int[]> subs)
        {
            pos = null; nrm = null; uv = null; subs = null;
            var mat = CaptureMaterial();
            if (mat == null) return false;
            var counts = Enumerable.Range(0, mesh.subMeshCount)
                .Select(s => mesh.GetTopology(s) == MeshTopology.Triangles ? (int)mesh.GetIndexCount(s) / 3 : 0).ToArray();
            int total = counts.Sum();
            if (total == 0) return false;

            var data = new float[total * TriFloats];
            var buf = new ComputeBuffer(total, TriFloats * 4);
            var rt = RenderTexture.GetTemporary(64, 64, 0, RenderTextureFormat.ARGB32);
            var prev = RenderTexture.active;
            try
            {
                buf.SetData(data);
                Graphics.SetRenderTarget(rt);
                GL.Clear(true, true, Color.clear);
                Graphics.ClearRandomWriteTargets();
                Graphics.SetRandomWriteTarget(1, buf, false);
                int offset = 0;
                for (int s = 0; s < counts.Length; s++)
                {
                    if (counts[s] == 0) continue;
                    mat.SetInt("_TriOffset", offset);
                    mat.SetPass(0);
                    Graphics.DrawMeshNow(mesh, Matrix4x4.identity, s);
                    offset += counts[s];
                }
                Graphics.ClearRandomWriteTargets();
                buf.GetData(data);
            }
            finally
            {
                Graphics.ClearRandomWriteTargets();
                RenderTexture.active = prev;
                RenderTexture.ReleaseTemporary(rt);
                buf.Release();
            }

            // Every triangle must have been written: an all-zero record means the UAV path didn't run.
            int missing = 0;
            for (int t = 0; t < total; t++)
            {
                bool any = false;
                for (int k = 0; k < TriFloats && !any; k++) any = data[t * TriFloats + k] != 0f;
                if (!any) missing++;
            }
            if (missing == total) { Plugin.Log.LogWarning($"Mesh {mesh.name}: GPU triangle capture returned nothing (no UAV support?)"); return false; }
            if (missing > 0) Plugin.Log.LogWarning($"Mesh {mesh.name}: {missing}/{total} triangles came back empty");

            pos = new Vector3[total * 3]; nrm = new Vector3[total * 3]; uv = new Vector2[total * 3];
            for (int t = 0; t < total; t++)
            {
                int b = t * TriFloats;
                for (int k = 0; k < 3; k++)
                {
                    pos[t * 3 + k] = new Vector3(data[b + k * 3], data[b + k * 3 + 1], data[b + k * 3 + 2]);
                    nrm[t * 3 + k] = new Vector3(data[b + 9 + k * 3], data[b + 9 + k * 3 + 1], data[b + 9 + k * 3 + 2]);
                    uv[t * 3 + k] = new Vector2(data[b + 18 + k * 2], data[b + 18 + k * 2 + 1]);
                }
            }
            subs = new List<int[]>();
            int start = 0;
            foreach (int c in counts)
            {
                subs.Add(Enumerable.Range(start * 3, c * 3).ToArray());
                start += c;
            }
            // Sanity check (a layout mismatch shows up as non-unit normals / UVs far outside 0..1).
            int unitNormals = nrm.Count(v => Mathf.Abs(v.magnitude - 1f) < 0.05f);
            int uvIn = uv.Count(v => v.x >= -0.01f && v.x <= 1.01f && v.y >= -0.01f && v.y <= 1.01f);
            var bounds = mesh.bounds;
            int posIn = pos.Count(v => bounds.Contains(v) || (bounds.ClosestPoint(v) - v).sqrMagnitude < 1e-6f);
            Plugin.Log.LogInfo($"Mesh {mesh.name}: captured {total} triangles on the GPU — unit normals {unitNormals * 100 / pos.Length}%, " +
                               $"UVs in 0..1 {uvIn * 100 / pos.Length}%, positions in bounds {posIn * 100 / pos.Length}%");
            return true;
        }

        private static Material CaptureMaterial()
        {
            if (_capture != null) return _capture;
            var sh = ShaderBundle.Get("Hidden/TCGCC/MeshCapture");
            if (sh == null) return null;
            return _capture = new Material(sh) { hideFlags = HideFlags.HideAndDontSave };
        }

        /// <summary>
        /// Fallback when triangles can't be captured: renders the mesh in UV space (passes 1/2) and saves object-space position
        /// (normalized to the bounds) and normal maps as <c>{name}_uvpos.png</c> / <c>{name}_uvnormal.png</c>. Returns files written.
        /// </summary>
        public static int SaveUvMaps(Mesh mesh, string dir, string name, int size = 1024)
        {
            var mat = CaptureMaterial();
            if (mat == null) return 0;
            int n = 0;
            var b = mesh.bounds;
            for (int pass = 1; pass <= 2; pass++)
            {
                var rt = RenderTexture.GetTemporary(size, size, 0, RenderTextureFormat.ARGBFloat, RenderTextureReadWrite.Linear);
                var prev = RenderTexture.active;
                Graphics.SetRenderTarget(rt);
                GL.Clear(true, true, new Color(0, 0, 0, 0));
                mat.SetPass(pass);
                for (int s = 0; s < mesh.subMeshCount; s++) Graphics.DrawMeshNow(mesh, Matrix4x4.identity, s);
                var read = new Texture2D(size, size, TextureFormat.RGBAFloat, false, true);
                read.ReadPixels(new Rect(0, 0, size, size), 0, 0);
                read.Apply();
                RenderTexture.active = prev;
                RenderTexture.ReleaseTemporary(rt);

                var px = read.GetPixels();
                var outPx = new Color32[px.Length];
                for (int i = 0; i < px.Length; i++)
                {
                    var c = px[i];
                    if (c.a <= 0f) continue;
                    Vector3 v = pass == 1
                        ? new Vector3(Norm(c.r, b.min.x, b.size.x), Norm(c.g, b.min.y, b.size.y), Norm(c.b, b.min.z, b.size.z))
                        : new Vector3(c.r * 0.5f + 0.5f, c.g * 0.5f + 0.5f, c.b * 0.5f + 0.5f);
                    outPx[i] = new Color32((byte)(v.x * 255), (byte)(v.y * 255), (byte)(v.z * 255), 255);
                }
                var png = new Texture2D(size, size, TextureFormat.RGBA32, false);
                png.SetPixels32(outPx);
                png.Apply();
                File.WriteAllBytes(Path.Combine(dir, $"{name}_{(pass == 1 ? "uvpos" : "uvnormal")}.png"), png.EncodeToPNG());
                UnityEngine.Object.Destroy(read);
                UnityEngine.Object.Destroy(png);
                n++;
            }
            Plugin.Log.LogInfo($"Mesh {mesh.name}: wrote UV-space position/normal maps (fallback)");
            return n;
        }

        private static float Norm(float v, float min, float size) => size > 0 ? Mathf.Clamp01((v - min) / size) : 0.5f;
    }
}
