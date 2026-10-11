using HarmonyLib;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// [Visuals - Shop look] PhoneWallpaperImage: the phone's home screen background, the Image
    /// <c>UI_PhoneScreen_Grp/ScreenGrp/AnimGrp/PhoneBG</c> (sprite PhoneBG2, 850×1512) stretched over the 24.63 × 41.896 screen.
    /// The picture's cropped part becomes that Image's sprite; clearing the setting puts the vanilla sprite back.
    /// </summary>
    internal static class PhoneWallpaper
    {
        /// <summary>Width ÷ height of the phone screen (measured from the game files 2026-10-10; Studio reads it from phone.json).</summary>
        internal const float ScreenAspect = 24.63f / 41.896f;

        private static Image _image;
        private static Sprite _vanillaSprite;
        private static bool _vanillaPreserveAspect;
        private static readonly PictureCache<Sprite> Sprite = new PictureCache<Sprite>();

        [HarmonyPatch(typeof(PhoneManager), "Awake")]
        private static class PhoneManagerAwakePatch
        {
            private static void Postfix(PhoneManager __instance)
            {
                var screen = __instance.m_UI_PhoneScreen;
                _image = screen != null ? screen.transform.Find("ScreenGrp/AnimGrp/PhoneBG")?.GetComponent<Image>() : null;
                if (_image == null)
                {
                    Plugin.Log.LogWarning("Phone wallpaper: the phone's background image wasn't found; PhoneWallpaperImage has no effect");
                    return;
                }
                _vanillaSprite = _image.sprite;
                _vanillaPreserveAspect = _image.preserveAspect;
                Apply();
            }
        }

        internal static void Init()
        {
            Plugin.PhoneWallpaperImage.SettingChanged += (_, __) => Apply();
            Plugin.PhoneWallpaperCrop.SettingChanged += (_, __) => Apply();
        }

        internal static void Apply()
        {
            if (_image == null) return;
            string path = ShopPicture.ImagePath(Plugin.PhoneWallpaperImage.Value), crop = Plugin.PhoneWallpaperCrop.Value;
            var sprite = Sprite.Get(path + "|" + crop, () =>
            {
                if (path == null) return null;
                var tex = ShopPicture.Load(path, "Phone wallpaper", mipmaps: true);
                if (tex == null) return null;
                tex.wrapMode = TextureWrapMode.Clamp;
                tex.Apply(updateMipmaps: true, makeNoLongerReadable: true);
                var c = ShopPicture.Crop(crop, "PhoneWallpaperCrop", tex, ScreenAspect);
                float x = c.x * tex.width, y = c.y * tex.height; // Sprite.Create throws on a rect past the edge
                var rect = new Rect(x, y, Mathf.Min(c.width * tex.width, tex.width - x), Mathf.Min(c.height * tex.height, tex.height - y));
                Plugin.Log.LogInfo($"Phone wallpaper: using {path}");
                return UnityEngine.Sprite.Create(tex, rect, new Vector2(0.5f, 0.5f), 100f, 0, SpriteMeshType.FullRect);
            });
            _image.sprite = sprite != null ? sprite : _vanillaSprite;
            _image.preserveAspect = sprite == null && _vanillaPreserveAspect;
        }
    }
}
