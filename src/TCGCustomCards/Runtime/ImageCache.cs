using System.Collections.Generic;
using System.IO;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>Lazy PNG/JPG → Sprite loader. Textures are uploaded once and made non-readable to save memory.</summary>
    internal static class ImageCache
    {
        private static readonly Dictionary<string, Sprite> Sprites = new Dictionary<string, Sprite>();
        private static readonly HashSet<string> Failed = new HashSet<string>();

        public static Sprite Get(string absolutePath)
        {
            if (string.IsNullOrEmpty(absolutePath)) return null;
            if (Sprites.TryGetValue(absolutePath, out var cached) && cached != null) return cached;
            if (Failed.Contains(absolutePath)) return null;

            if (!File.Exists(absolutePath))
            {
                Failed.Add(absolutePath);
                Plugin.Log.LogWarning($"Image not found: {absolutePath}");
                return null;
            }

            var tex = new Texture2D(2, 2, TextureFormat.RGBA32, true)
            {
                name = Path.GetFileNameWithoutExtension(absolutePath),
                wrapMode = TextureWrapMode.Clamp,
                filterMode = FilterMode.Trilinear,
                anisoLevel = 4
            };
            if (!tex.LoadImage(File.ReadAllBytes(absolutePath), markNonReadable: true))
            {
                Object.Destroy(tex);
                Failed.Add(absolutePath);
                Plugin.Log.LogWarning($"Image could not be decoded: {absolutePath}");
                return null;
            }

            return Store(absolutePath, tex);
        }

        private static readonly Dictionary<string, int> Trims = new Dictionary<string, int>();

        /// <summary>
        /// Card face without the scan's own printed border (the black/white/yellow rim of a photographed card), so the game's
        /// border frames the card content directly. The border is detected per image (borderless and full-art printings have
        /// none) and only the sprite's rectangle is shrunk — same texture, files untouched. Callers stretch the sprite back to
        /// the untrimmed shape (texture size). Falls back to the whole image when nothing is detected.
        /// </summary>
        public static Sprite GetTrimmedCard(string absolutePath, float maxFraction, out int borderPx)
        {
            borderPx = 0;
            if (string.IsNullOrEmpty(absolutePath)) return null;
            string key = $"{absolutePath}#trim{maxFraction:F3}";
            if (Sprites.TryGetValue(key, out var cached) && cached != null)
            {
                Trims.TryGetValue(key, out borderPx);
                return cached;
            }
            if (Failed.Contains(absolutePath) || !File.Exists(absolutePath)) return Get(absolutePath);

            var tex = new Texture2D(2, 2, TextureFormat.RGBA32, true)
            {
                name = Path.GetFileNameWithoutExtension(absolutePath) + "_face",
                wrapMode = TextureWrapMode.Clamp,
                filterMode = FilterMode.Trilinear,
                anisoLevel = 4
            };
            if (!tex.LoadImage(File.ReadAllBytes(absolutePath), markNonReadable: false))
            {
                Object.Destroy(tex);
                return Get(absolutePath);
            }
            int w = tex.width, h = tex.height;
            int b = DetectBorder(tex.GetPixels32(), w, h);
            if (b > maxFraction * w || b * 2 >= w || b * 2 >= h) b = 0; // wider than any printed border: artwork
            tex.Apply(updateMipmaps: true, makeNoLongerReadable: true);
            var sprite = Sprite.Create(tex, new Rect(b, b, w - 2 * b, h - 2 * b), new Vector2(0.5f, 0.5f), 100f);
            sprite.name = tex.name;
            tex.hideFlags = HideFlags.DontUnloadUnusedAsset;
            sprite.hideFlags = HideFlags.DontUnloadUnusedAsset;
            Sprites[key] = sprite;
            Trims[key] = borderPx = b;
            return sprite;
        }

        /// <summary>
        /// Width in pixels of a uniform-colour rim around the image (0 = none): per side, the median run of edge-coloured pixels
        /// on 7 scanlines (25–75 %) after skipping transparent rounded corners and the anti-aliased outer pixel; the second-smallest
        /// side wins; +1 px for the anti-aliased inner edge. Pixel rows start at the bottom (Unity order).
        /// </summary>
        internal static int DetectBorder(Color32[] px, int w, int h)
        {
            int Run(int x0, int y0, int dx, int dy, int limit)
            {
                int i = 0;
                while (i < limit && px[(y0 + dy * i) * w + x0 + dx * i].a < 200) i++; // transparent corner / anti-aliasing
                if (i + 2 >= limit) return 0;
                // Reference = the rim colour one pixel in (the outermost pixel is often a lighter anti-aliased edge).
                i++;
                var r = px[(y0 + dy * i) * w + x0 + dx * i];
                int start = i;
                for (; i < limit; i++)
                {
                    var c = px[(y0 + dy * i) * w + x0 + dx * i];
                    if (System.Math.Abs(c.r - r.r) + System.Math.Abs(c.g - r.g) + System.Math.Abs(c.b - r.b) > 60) break;
                }
                return i - start < 2 ? 0 : i; // measured from the image edge
            }
            int Median(System.Func<float, int> line)
            {
                var runs = new List<int>();
                for (int k = 0; k < 7; k++) runs.Add(line(0.25f + k * 0.5f / 6f));
                runs.Sort();
                return runs[3];
            }
            int lw = w / 4, lh = h / 4;
            int left = Median(f => Run(0, (int)(f * (h - 1)), 1, 0, lw));
            int right = Median(f => Run(w - 1, (int)(f * (h - 1)), -1, 0, lw));
            int bottom = Median(f => Run((int)(f * (w - 1)), 0, 0, 1, lh));
            int top = Median(f => Run((int)(f * (w - 1)), h - 1, 0, -1, lh));
            // Second-smallest side: one side may run long (art in the rim colour) or short (a detail line in the rim,
            // e.g. Pokémon's silver border) without moving the result.
            var sides = new List<int> { left, right, top, bottom };
            sides.Sort();
            int b = sides[1];
            return b < 2 ? 0 : b + 1;
        }

        /// <summary>
        /// Loads an image and pads it with transparency (centered, not scaled) to <paramref name="targetAspect"/> (width/height).
        /// UI slots such as shop icons are sized for the vanilla sprite's shape, so an unpadded image would be stretched.
        /// </summary>
        public static Sprite GetPadded(string absolutePath, float targetAspect)
        {
            if (string.IsNullOrEmpty(absolutePath) || targetAspect <= 0f) return Get(absolutePath);
            string key = $"{absolutePath}#pad{targetAspect:F3}";
            if (Sprites.TryGetValue(key, out var cached) && cached != null) return cached;
            if (Failed.Contains(absolutePath) || !File.Exists(absolutePath)) return Get(absolutePath);

            var src = new Texture2D(2, 2, TextureFormat.RGBA32, false);
            if (!src.LoadImage(File.ReadAllBytes(absolutePath)))
            {
                Object.Destroy(src);
                return Get(absolutePath);
            }
            float aspect = src.width / (float)src.height;
            if (Mathf.Abs(aspect - targetAspect) < 0.01f)
            {
                Object.Destroy(src);
                return Get(absolutePath);
            }

            int w = src.width, h = src.height;
            if (aspect < targetAspect) w = Mathf.RoundToInt(h * targetAspect);
            else h = Mathf.RoundToInt(w / targetAspect);
            var canvas = new Texture2D(w, h, TextureFormat.RGBA32, true)
            {
                name = Path.GetFileNameWithoutExtension(absolutePath) + "_padded",
                wrapMode = TextureWrapMode.Clamp,
                filterMode = FilterMode.Trilinear
            };
            canvas.SetPixels32(new Color32[w * h]);
            canvas.SetPixels32((w - src.width) / 2, (h - src.height) / 2, src.width, src.height, src.GetPixels32());
            canvas.Apply(updateMipmaps: true, makeNoLongerReadable: true);
            Object.Destroy(src);
            return Store(key, canvas);
        }

        /// <summary>
        /// Card back in the vanilla back-sprite shape: vanilla backs are 1024² sprites with the card ≈87% of the height, centered.
        /// Square images are used as-is (e.g. made from the exported template); card-shaped images get transparent padding.
        /// </summary>
        public static Sprite GetCardBack(string absolutePath)
        {
            if (string.IsNullOrEmpty(absolutePath)) return null;
            string key = absolutePath + "#cardback";
            if (Sprites.TryGetValue(key, out var cached) && cached != null) return cached;
            if (Failed.Contains(absolutePath) || !File.Exists(absolutePath)) return null;

            var src = new Texture2D(2, 2, TextureFormat.RGBA32, false);
            if (!src.LoadImage(File.ReadAllBytes(absolutePath)))
            {
                Object.Destroy(src);
                Failed.Add(absolutePath);
                return null;
            }
            if (Mathf.Abs(src.width - src.height) <= 2)
            {
                Object.Destroy(src);
                return Get(absolutePath);
            }
            int side = Mathf.Max(Mathf.RoundToInt(src.height / 0.867f), Mathf.RoundToInt(src.width / 0.62f));
            var canvas = new Texture2D(side, side, TextureFormat.RGBA32, true)
            {
                name = Path.GetFileNameWithoutExtension(absolutePath) + "_cardback",
                wrapMode = TextureWrapMode.Clamp,
                filterMode = FilterMode.Trilinear
            };
            canvas.SetPixels32(new Color32[side * side]);
            canvas.SetPixels32((side - src.width) / 2, (side - src.height) / 2, src.width, src.height, src.GetPixels32());
            canvas.Apply(updateMipmaps: true, makeNoLongerReadable: true);
            Object.Destroy(src);
            return Store(key, canvas);
        }

        private static Sprite Store(string key, Texture2D tex)
        {
            var sprite = Sprite.Create(tex, new Rect(0, 0, tex.width, tex.height), new Vector2(0.5f, 0.5f), 100f);
            sprite.name = tex.name;
            tex.hideFlags = HideFlags.DontUnloadUnusedAsset;
            sprite.hideFlags = HideFlags.DontUnloadUnusedAsset;
            Sprites[key] = sprite;
            return sprite;
        }
    }
}
