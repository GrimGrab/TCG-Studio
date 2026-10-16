using System;
using System.Globalization;
using System.IO;
using UnityEngine;
using Object = UnityEngine.Object;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// The shared part of TCG Studio's Shop look pictures (shop sign, phone wallpaper, open/closed sign): an image setting (a file
    /// in the active setup's game\ShopSign or a full path) plus a crop setting "x,y,w,h", painted into a copy of a vanilla texture or shown
    /// as a sprite.
    /// </summary>
    internal static class ShopPicture
    {
        /// <summary>The setup's folder holding the chosen pictures and fonts (&lt;setup&gt;\game\ShopSign, written by TCG Studio).</summary>
        internal static string Folder => Core.StudioPaths.SetupFile(Path.Combine("game", "ShopSign"));

        /// <summary>The setting holds a file name in the setup's ShopSign folder or a full path; null when empty.</summary>
        internal static string ImagePath(string value)
        {
            value = value?.Trim().Trim('"');
            if (string.IsNullOrEmpty(value)) return null;
            if (Path.IsPathRooted(value)) return value;
            return Folder == null ? null : Path.Combine(Folder, value);
        }

        /// <summary>A readable RGBA32 copy of a PNG/JPG; null (logged) when missing or unreadable.</summary>
        internal static Texture2D Load(string path, string what, bool mipmaps = false)
        {
            if (!File.Exists(path))
            {
                Plugin.Log.LogWarning($"{what} image not found: {path}");
                return null;
            }
            var tex = new Texture2D(2, 2, TextureFormat.RGBA32, mipmaps) { name = "TCGCC_" + what.Replace(' ', '_') };
            try
            {
                if (tex.LoadImage(File.ReadAllBytes(path))) return tex;
                Plugin.Log.LogWarning($"{what} image {path} isn't a PNG/JPG image");
            }
            catch (Exception e) { Plugin.Log.LogWarning($"{what} image {path} failed: {e.Message}"); }
            Object.Destroy(tex);
            return null;
        }

        /// <summary>The crop setting "x,y,w,h" (fractions, top-left origin; Studio's crop box keeps it at <paramref name="aspect"/>)
        /// as a UV rect (bottom-up); empty or invalid = the largest <paramref name="aspect"/> area from the middle.</summary>
        internal static Rect Crop(string value, string setting, Texture2D src, float aspect)
        {
            return Parse(value?.Trim(), setting) ?? Centre((float)src.width / src.height, aspect);
        }

        private static Rect Centre(float have, float aspect) => have > aspect
            ? new Rect((1 - aspect / have) / 2, 0, aspect / have, 1)
            : new Rect(0, (1 - have / aspect) / 2, 1, have / aspect);

        private static Rect? Parse(string s, string setting)
        {
            if (string.IsNullOrEmpty(s)) return null;
            var parts = s.Split(',');
            var f = new float[4];
            bool ok = parts.Length == 4;
            for (int i = 0; ok && i < 4; i++)
                ok = float.TryParse(parts[i].Trim(), NumberStyles.Float, CultureInfo.InvariantCulture, out f[i]);
            if (ok)
            {
                float x = Mathf.Clamp01(f[0]), y = Mathf.Clamp01(f[1]), w = Mathf.Clamp(f[2], 0, 1 - x), h = Mathf.Clamp(f[3], 0, 1 - y);
                if (w >= 0.01f && h >= 0.01f) return new Rect(x, 1 - y - h, w, h);
            }
            Plugin.Log.LogWarning($"{setting} \"{s}\" isn't x,y,width,height (fractions 0-1); using the middle of the image");
            return null;
        }

        /// <summary>Pixels painted past each area so mipmaps/bilinear filtering don't pull in vanilla art at the edges.</summary>
        private const int Bleed = 3;

        /// <summary>
        /// Paints <paramref name="srcUv"/> of the picture into <paramref name="dstUv"/> of a texture's pixels (both UV, bottom-up),
        /// over the vanilla pixels: the picture's transparent parts keep them, and <paramref name="weight"/> (texture pixel x, y →
        /// 0..1, default 1) keeps vanilla parts such as a frame.
        /// </summary>
        internal static void Paint(Color32[] px, int w, int h, Texture2D src, Rect srcUv, Rect dstUv, Func<int, int, float> weight = null)
        {
            float x0 = dstUv.xMin * w, x1 = dstUv.xMax * w, y0 = dstUv.yMin * h, y1 = dstUv.yMax * h;
            int bleed = weight == null ? Bleed : 0; // a weighted area ends at its own mask
            int ix0 = Mathf.Max(0, Mathf.FloorToInt(x0) - bleed), ix1 = Mathf.Min(w, Mathf.CeilToInt(x1) + bleed);
            int iy0 = Mathf.Max(0, Mathf.FloorToInt(y0) - bleed), iy1 = Mathf.Min(h, Mathf.CeilToInt(y1) + bleed);
            for (int y = iy0; y < iy1; y++)
            {
                float v = srcUv.y + srcUv.height * Mathf.Clamp01((y + 0.5f - y0) / (y1 - y0));
                for (int x = ix0; x < ix1; x++)
                {
                    float a = weight?.Invoke(x, y) ?? 1f;
                    if (a <= 0f) continue;
                    float u = srcUv.x + srcUv.width * Mathf.Clamp01((x + 0.5f - x0) / (x1 - x0));
                    Color c = src.GetPixelBilinear(u, v);
                    px[y * w + x] = Color.Lerp(px[y * w + x], new Color(c.r, c.g, c.b, 1f), c.a * a);
                }
            }
        }

        /// <summary>A copy of <paramref name="vanilla"/> with <paramref name="paint"/> applied to its pixels (mipmapped, not readable);
        /// null when the vanilla texture can't be read back.</summary>
        internal static Texture2D Compose(Texture vanilla, string name, Action<Color32[], int, int> paint)
        {
            var baseTex = TextureReadback.Read(vanilla, new RectInt(0, 0, vanilla.width, vanilla.height));
            if (baseTex == null) return null;
            int w = baseTex.width, h = baseTex.height;
            var px = baseTex.GetPixels32();
            Object.Destroy(baseTex);
            paint(px, w, h);
            var dst = new Texture2D(w, h, TextureFormat.RGBA32, true) { name = name, wrapMode = TextureWrapMode.Clamp };
            dst.SetPixels32(px);
            dst.Apply(updateMipmaps: true, makeNoLongerReadable: true);
            dst.hideFlags = HideFlags.DontUnloadUnusedAsset;
            return dst;
        }

        private static MaterialPropertyBlock _block;

        /// <summary>Shows <paramref name="tex"/> instead of the material's main texture in every slot that uses it (e.g. _MainTex and
        /// _EmissionMap) through a MaterialPropertyBlock, so null restores vanilla.</summary>
        internal static void SetTexture(Renderer r, Texture tex)
        {
            if (r == null) return;
            if (tex == null) { r.SetPropertyBlock(null); return; }
            _block ??= new MaterialPropertyBlock();
            _block.Clear();
            var mat = r.sharedMaterial;
            foreach (var prop in mat.GetTexturePropertyNames())
                if (mat.GetTexture(prop) == mat.mainTexture) _block.SetTexture(prop, tex);
            r.SetPropertyBlock(_block);
        }
    }

    /// <summary>One built picture (texture or sprite) kept while its settings don't change; the old one is destroyed on a change.</summary>
    internal sealed class PictureCache<T> where T : Object
    {
        private string _key;
        private T _value;

        internal T Get(string key, Func<T> build)
        {
            if (key == _key) return _value;
            if (_value is Sprite s && s.texture != null) Object.Destroy(s.texture);
            if (_value != null) Object.Destroy(_value);
            _key = key;
            _value = null;
            _value = build();
            return _value;
        }
    }
}
