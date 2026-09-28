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
