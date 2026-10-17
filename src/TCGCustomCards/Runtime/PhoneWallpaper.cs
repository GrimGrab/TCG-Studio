using HarmonyLib;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// [Visuals - Game look] PhoneWallpaperImage: the phone's home screen background, the Image
    /// <c>UI_PhoneScreen_Grp/ScreenGrp/AnimGrp/PhoneBG</c> (sprite PhoneBG2, 850×1512) stretched over the 24.63 × 41.896 screen.
    /// The picture's cropped part becomes that Image's sprite; clearing the setting puts the vanilla sprite back.
    /// </summary>
    internal static class PhoneWallpaper
    {
        /// <summary>Width ÷ height of the phone screen (measured from the game files 2026-10-10; Studio reads it from phone.json).</summary>
        internal const float ScreenAspect = 24.63f / 41.896f;

        private static readonly ImageSwap Image = new ImageSwap();
        private static readonly PictureCache<Sprite> Sprite = new PictureCache<Sprite>();

        [HarmonyPatch(typeof(PhoneManager), "Awake")]
        private static class PhoneManagerAwakePatch
        {
            private static void Postfix(PhoneManager __instance)
            {
                var screen = __instance.m_UI_PhoneScreen;
                var image = screen != null ? screen.transform.Find("ScreenGrp/AnimGrp/PhoneBG")?.GetComponent<Image>() : null;
                if (image == null)
                {
                    Plugin.Log.LogWarning("Phone wallpaper: the phone's background image wasn't found; PhoneWallpaperImage has no effect");
                    return;
                }
                Image.Capture(image);
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
            if (!Image.Valid) return;
            string path = ShopPicture.ImagePath(Plugin.PhoneWallpaperImage.Value), crop = Plugin.PhoneWallpaperCrop.Value;
            Image.Show(Sprite.Get(path + "|" + crop,
                () => ShopPicture.CropSprite(path, crop, "PhoneWallpaperCrop", ScreenAspect, "Phone wallpaper")));
        }
    }
}
