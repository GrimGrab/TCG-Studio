using System;
using HarmonyLib;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// [Visuals - Game look] MainMenuImage: the main menu's background. The title screen draws a stretched full-screen Image
    /// <c>TitleScreen/ScreenGrp/BG</c> (sprite GameStartBG_Blur, 2048×1024; game files 2026-10-17) under the logo and buttons. A
    /// picture becomes that Image's sprite and covers the screen at 16:9 (wider or taller screens cut a little off). The loading
    /// screen's background works the same way (LoadingScreenLook).
    /// </summary>
    internal static class ScreenPictures
    {
        /// <summary>Width ÷ height the crop box keeps for full-screen pictures (the game's 1920×1080 reference resolution).</summary>
        internal const float Aspect = 16f / 9f;

        /// <summary>The value separator of list settings such as LoadingScreenImages (TCG Studio strips it from file names).</summary>
        internal const char ListSeparator = '|';

        private static readonly ImageSwap Menu = new ImageSwap();
        private static readonly PictureCache<Sprite> MenuSprite = new PictureCache<Sprite>();

        [HarmonyPatch(typeof(TitleScreen), "Start")]
        private static class TitleScreenStartPatch
        {
            private static void Postfix(TitleScreen __instance)
            {
                var image = __instance.transform.Find("ScreenGrp/BG")?.GetComponent<Image>();
                if (image == null)
                {
                    Plugin.Log.LogWarning("Main menu: the background image wasn't found; MainMenuImage has no effect");
                    return;
                }
                Menu.Capture(image);
                ApplyMenu();
            }
        }

        private static void ApplyMenu()
        {
            if (!Menu.Valid) return;
            string path = ShopPicture.ImagePath(Plugin.MainMenuImage.Value), crop = Plugin.MainMenuCrop.Value;
            Menu.Show(MenuSprite.Get(path + "|" + crop, () => ShopPicture.CropSprite(path, crop, "MainMenuCrop", Aspect, "Main menu")), Aspect);
        }

        internal static void Init()
        {
            EventHandler menu = (_, __) => ApplyMenu();
            Plugin.MainMenuImage.SettingChanged += menu;
            Plugin.MainMenuCrop.SettingChanged += menu;
        }
    }
}
