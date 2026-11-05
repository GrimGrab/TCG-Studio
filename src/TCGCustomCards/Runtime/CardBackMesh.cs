using System.Collections.Generic;
using System.IO;
using System.Linq;
using HarmonyLib;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// 3D cards (hand, pack opening, shelves, play table) draw their back with <c>Card3dUIGroup.m_CardBackMesh</c>, a mesh using one
    /// shared material (<c>LightManager.m_CardBackMat</c>) — the per-expansion <c>CardUI.m_CardBackImage</c> sprite isn't what shows there.
    /// Custom sets with a card back get a MaterialPropertyBlock on that renderer with the back texture; vanilla cards get it cleared
    /// (CardUIs are pooled and reused). Double-faced cards (a back-face picture) show their back face there instead.
    /// </summary>
    internal static class CardBackMesh
    {
        private static readonly AccessTools.FieldRef<CardUI, Card3dUIGroup> Group =
            Core.Compat.Field<CardUI, Card3dUIGroup>(typeof(CardBackMesh), "m_Card3dUIGroup");

        /// <summary>Composed mesh texture per back image file (null = failed to load).</summary>
        private static readonly Dictionary<string, Texture2D> Textures = new Dictionary<string, Texture2D>();
        private static readonly HashSet<Renderer> Overridden = new HashSet<Renderer>();
        private static MaterialPropertyBlock _block;
        private static string[] _texProps;
        private static bool _logged;

        /// <summary>Rebuilds back textures (config change); cards already on screen update when they are next set up.</summary>
        public static void ClearCache()
        {
            foreach (var t in Textures.Values) if (t != null) Object.Destroy(t);
            Textures.Clear();
            _basePixels = null;
            foreach (var r in Overridden) if (r != null) r.SetPropertyBlock(null);
            Overridden.Clear();
        }

        /// <summary>While set, cards aren't given a back: the card pictures hide it for the shot, and composing a double-faced card's costs over 100 ms of pixel work.</summary>
        public static bool Suspended;

        public static void Apply(CardUI ui, CardData data)
        {
            if (Suspended) return;
            var group = Group(ui);
            if (group == null || group.m_CardBackMesh == null) return;
            var renderer = group.m_CardBackMesh.GetComponentInChildren<Renderer>(true);
            if (renderer == null) return;
            if (!_logged) LogOnce(ui, group, renderer);

            string path = null;
            if (Registry.IsCustom(data.expansionType) && Registry.Get(data.expansionType) is CustomSet set)
            {
                // A double-faced card has no plain back: its back panel shows the back face (racks, hand, turning it over).
                // MtgCardFaces.IsBack copies already show that face on the front, so they keep the set's back.
                path = !Mtg.MtgCardFaces.IsBack(data) && set.PosByMonster.TryGetValue(data.monsterType, out int pos) && set.HasBackFace(pos)
                    ? set.Def.Resolve(set.Card(pos).BackImage) : set.CardBackPath;
            }
            var tex = path != null ? TextureFor(path, renderer) : null;
            if (tex == null)
            {
                if (Overridden.Remove(renderer)) renderer.SetPropertyBlock(null);
                return;
            }

            _block ??= new MaterialPropertyBlock();
            _block.Clear();
            foreach (var prop in TextureProps(renderer.sharedMaterial)) _block.SetTexture(prop, tex);
            renderer.SetPropertyBlock(_block);
            Overridden.Add(renderer);
        }

        /// <summary>Texture slots of the shared material that hold the vanilla back art (main texture, and emission map if it reuses it).</summary>
        private static string[] TextureProps(Material mat)
        {
            if (_texProps != null) return _texProps;
            var main = MaterialTex.Main(mat);
            var props = new List<string>();
            if (mat != null)
                foreach (var name in mat.GetTexturePropertyNames())
                    if (main != null && mat.GetTexture(name) == main) props.Add(name);
            if (props.Count == 0) props.Add("_MainTex");
            return _texProps = props.ToArray();
        }

        private static Texture2D TextureFor(string path, Renderer renderer)
        {
            if (Textures.TryGetValue(path, out var cached)) return cached;
            Texture2D result = null;
            Debug.SpikeWatch.Mark("building a card back");
            using var measured = Debug.SpikeWatch.Measure("card back");
            try { result = Compose(path, renderer); }
            catch (System.Exception e) { Plugin.Log.LogWarning($"Card back mesh texture for {path} failed: {e.Message}"); }
            Textures[path] = result;
            return result;
        }

        // T_CardBackMesh (1024², measured 2026-09-24): card art at x78–692, y66–927 (top-down) with ≈20 px rounded corners, a dark
        // border around it and the card-edge stripes at x≈765–995. We replace only the art window, so the layout matches vanilla exactly.
        private static readonly RectInt ArtTopDown = new RectInt(78, 66, 615, 862);
        private const int ArtCornerRadius = 20;
        /// <summary>The card face (dark border included) spans x0–755; beyond that are the card-edge stripes.</summary>
        private const float FaceWidth = 755;
        // Old TCG Studio backs were 1024² in the UI back-sprite layout; their art window was x218–798, y104–920 (top-down).
        private static readonly RectInt LegacyArtTopDown = new RectInt(218, 104, 580, 816);

        /// <summary>Cover-fits the back image into the art window of a copy of the vanilla back-mesh texture.</summary>
        private static Texture2D Compose(string path, Renderer renderer)
        {
            var vanilla = MaterialTex.Main(renderer.sharedMaterial);
            if (vanilla == null) return null;
            if (!BasePixels(vanilla)) return null;
            var src = new Texture2D(2, 2, TextureFormat.RGBA32, false);
            if (!src.LoadImage(File.ReadAllBytes(path))) { Object.Destroy(src); return null; }

            // Source region (UV, bottom-up): legacy square backs → their art window; otherwise the whole image, cropped to cover.
            int w = _baseW, h = _baseH;
            float k = w / 1024f;
            float scale = Mathf.Clamp(Plugin.CardBackScale.Value, 0.9f, 1.25f);
            float cx = ArtTopDown.center.x, cy = 1024 - ArtTopDown.center.y;
            float hw = ArtTopDown.width * scale / 2, hh = ArtTopDown.height * scale / 2;
            int ax0 = Mathf.RoundToInt(Mathf.Max(0, cx - hw) * k), ax1 = Mathf.RoundToInt(Mathf.Min(FaceWidth, cx + hw) * k);
            int ay0 = Mathf.RoundToInt(Mathf.Max(0, cy - hh) * k), ay1 = Mathf.RoundToInt(Mathf.Min(1024, cy + hh) * k);
            var art = new RectInt(ax0, ay0, ax1 - ax0, ay1 - ay0);
            Rect srcUv;
            if (Mathf.Abs(src.width - src.height) <= 2)
                srcUv = new Rect(LegacyArtTopDown.x / 1024f, 1f - LegacyArtTopDown.yMax / 1024f, LegacyArtTopDown.width / 1024f, LegacyArtTopDown.height / 1024f);
            else
            {
                float want = (float)art.width / art.height, have = (float)src.width / src.height;
                srcUv = have > want ? new Rect((1 - want / have) / 2, 0, want / have, 1) : new Rect(0, (1 - have / want) / 2, 1, have / want);
            }

            var px = (Color32[])_basePixels.Clone();
            var from = src.GetPixels32();
            int sw = src.width, sh = src.height;
            int rad = Mathf.RoundToInt(ArtCornerRadius * k * scale);
            for (int y = art.yMin; y < art.yMax; y++)
                for (int x = art.xMin; x < art.xMax; x++)
                {
                    if (!InsideRounded(x, y, art, rad)) continue;
                    float u = srcUv.x + srcUv.width * ((x + 0.5f - art.xMin) / art.width);
                    float v = srcUv.y + srcUv.height * ((y + 0.5f - art.yMin) / art.height);
                    px[y * w + x] = Over(px[y * w + x], Bilinear(from, sw, sh, u, v));
                }

            var dst = new Texture2D(w, h, TextureFormat.RGBA32, true) { name = Path.GetFileNameWithoutExtension(path) + "_backmesh", wrapMode = TextureWrapMode.Clamp };
            dst.SetPixels32(px);
            dst.Apply(updateMipmaps: true, makeNoLongerReadable: true);
            dst.hideFlags = HideFlags.DontUnloadUnusedAsset;
            Object.Destroy(src);
            return dst;
        }

        private static Color32[] _basePixels;
        private static int _baseW, _baseH;

        /// <summary>The vanilla back's pixels, read from the graphics card once (a read makes the game wait for it).</summary>
        private static bool BasePixels(Texture vanilla)
        {
            if (_basePixels != null) return true;
            var read = TextureReadback.Read(vanilla, new RectInt(0, 0, vanilla.width, vanilla.height));
            if (read == null) return false;
            _basePixels = read.GetPixels32();
            _baseW = read.width;
            _baseH = read.height;
            Object.Destroy(read);
            return true;
        }

        /// <summary>Texture2D.GetPixelBilinear's result on a plain pixel array, clamped at the edges; that call per pixel took over 100 ms for one back.</summary>
        private static Color32 Bilinear(Color32[] px, int w, int h, float u, float v)
        {
            float fx = u * w - 0.5f, fy = v * h - 0.5f;
            int x0 = (int)System.Math.Floor(fx), y0 = (int)System.Math.Floor(fy);
            int tx = (int)((fx - x0) * 256f), ty = (int)((fy - y0) * 256f);
            int x1 = x0 + 1 >= w ? w - 1 : x0 + 1, y1 = y0 + 1 >= h ? h - 1 : y0 + 1;
            if (x0 < 0) x0 = 0;
            if (y0 < 0) y0 = 0;
            Color32 a = px[y0 * w + x0], b = px[y0 * w + x1], c = px[y1 * w + x0], d = px[y1 * w + x1];
            int ix = 256 - tx, iy = 256 - ty;
            return new Color32(
                (byte)(((a.r * ix + b.r * tx) * iy + (c.r * ix + d.r * tx) * ty) >> 16),
                (byte)(((a.g * ix + b.g * tx) * iy + (c.g * ix + d.g * tx) * ty) >> 16),
                (byte)(((a.b * ix + b.b * tx) * iy + (c.b * ix + d.b * tx) * ty) >> 16),
                (byte)(((a.a * ix + b.a * tx) * iy + (c.a * ix + d.a * tx) * ty) >> 16));
        }

        private static Color32 Over(Color32 under, Color32 over)
        {
            int a = over.a, ia = 255 - a;
            return new Color32((byte)((under.r * ia + over.r * a) / 255), (byte)((under.g * ia + over.g * a) / 255), (byte)((under.b * ia + over.b * a) / 255), (byte)((under.a * ia + 255 * a) / 255));
        }

        private static bool InsideRounded(int x, int y, RectInt r, int rad)
        {
            int cx = Mathf.Clamp(x, r.xMin + rad, r.xMax - 1 - rad), cy = Mathf.Clamp(y, r.yMin + rad, r.yMax - 1 - rad);
            int dx = x - cx, dy = y - cy;
            return dx * dx + dy * dy <= rad * rad;
        }

        private static Rect UvBounds(Renderer renderer)
        {
            var mesh = renderer.GetComponent<MeshFilter>()?.sharedMesh;
            if (mesh == null || !mesh.isReadable || mesh.uv.Length == 0) return new Rect(0, 0, 1, 1);
            var uvs = mesh.uv;
            return Rect.MinMaxRect(uvs.Min(p => p.x), uvs.Min(p => p.y), uvs.Max(p => p.x), uvs.Max(p => p.y));
        }

        private static void LogOnce(CardUI ui, Card3dUIGroup group, Renderer renderer)
        {
            _logged = true;
            var mat = renderer.sharedMaterial;
            var mesh = renderer.GetComponent<MeshFilter>()?.sharedMesh;
            string texInfo = mat == null ? "none" : string.Join(", ", mat.GetTexturePropertyNames()
                .Select(n => (n, t: mat.GetTexture(n))).Where(x => x.t != null).Select(x => $"{x.n}={x.t.name} {x.t.width}x{x.t.height}"));
            Plugin.Log.LogInfo($"Card back mesh: renderer={renderer.name} material={mat?.name} shader={mat?.shader?.name} textures=[{texInfo}] " +
                               $"mesh={mesh?.name} readable={mesh?.isReadable} uv={UvBounds(renderer)} " +
                               $"ui back active={ui.m_CardBack?.activeInHierarchy} ui back sprite={ui.m_CardBackImage?.sprite?.name}");
        }
    }
}
