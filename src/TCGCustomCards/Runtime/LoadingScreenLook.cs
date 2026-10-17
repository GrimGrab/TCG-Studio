using System;
using System.Collections.Generic;
using System.Linq;
using HarmonyLib;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Runtime
{
    /// <summary>What the loading screen's spinning card shows ([Visuals - Game look] LoadingScreenCard).</summary>
    internal enum LoadingCardMode { Vanilla, RandomFromMySets, GlobalCardBack, Picture, Hidden }

    /// <summary>
    /// [Visuals - Game look] the loading screen (game files 2026-10-17). <c>LoadingScreen/ScreenGrp</c> has a full-screen Image
    /// <c>BG</c> (no sprite, black) under "Now Loading…", the percent and <c>CardGroup</c>, a CardUI whose front is switched off:
    /// what spins is the card back (<c>AnimGrp/CardBack</c> and <c>CardBack (1)</c>, sprite CardBack 1024² with the card at
    /// x 190–825, y 68–955). Each scene has its own copy: the title's opens the screen (OpenScreen), the shop's shows the rest of
    /// the load (HideLoadingScreen) — both get the same background and card.
    /// <list type="bullet">
    /// <item>LoadingScreenImages / LoadingScreenCrops: a background picked at random each time the screen opens (16:9, covering).</item>
    /// <item>LoadingScreenCard: the vanilla card back, a random card from the installed sets, the setup's global card back,
    /// LoadingScreenCardImage, or no card; drawn
    /// as a card-shaped Image over the card back's card area (the card back itself hidden).</item>
    /// </list>
    /// </summary>
    internal static class LoadingScreenLook
    {
        /// <summary>Width ÷ height of the card (the card back's card area, 635 × 887 px).</summary>
        internal const float CardAspect = 635f / 887f;
        // The card back's card area in its 1024² sprite as anchors (bottom-up).
        private static readonly Vector2 CardMin = new Vector2(190f / 1024, 1 - 955f / 1024), CardMax = new Vector2(825f / 1024, 1 - 68f / 1024);

        private static readonly System.Random Random = new System.Random();
        private static bool _warned;

        [HarmonyPatch(typeof(LoadingScreen), nameof(LoadingScreen.OpenScreen))]
        private static class OpenScreenPatch
        {
            private static void Postfix() => Attach(CSingleton<LoadingScreen>.Instance, pickNew: true);
        }

        [HarmonyPatch(typeof(LoadingScreen), "HideLoadingScreen")]
        private static class HideLoadingScreenPatch
        {
            private static void Postfix(LoadingScreen __instance) => Attach(__instance, pickNew: false);
        }

        /// <summary>Takes over a loading screen copy; <paramref name="pickNew"/>: the screen opens (new random background and card).</summary>
        private static void Attach(LoadingScreen screen, bool pickNew)
        {
            var grp = screen != null ? screen.m_ScreenGrp : null;
            var bg = grp != null ? grp.transform.Find("BG")?.GetComponent<Image>() : null;
            if (bg == null)
            {
                if (!_warned) Plugin.Log.LogWarning("Loading screen: its background wasn't found; the Game look loading screen has no effect");
                _warned = true;
                return;
            }
            Background.Capture(bg);
            Cards.RemoveAll(c => !c.Valid); // destroyed with their scene
            foreach (var name in new[] { "CardGroup/AnimGrp/CardBack", "CardGroup/AnimGrp/CardBack (1)" })
            {
                var back = grp.transform.Find(name)?.GetComponent<Image>();
                if (back != null && !Cards.Any(c => c.Back == back)) Cards.Add(new CardFace(back));
            }
            ShowBackground(pickNew);
            ShowCard(pickNew);
        }

        // ---- background: one of the pictures, picked at random each time the screen opens

        private static readonly ImageSwap Background = new ImageSwap(white: true);
        private static readonly PictureSet<Sprite> BackgroundSprites = new PictureSet<Sprite>();
        private static string _shownBackground; // path|crop on screen, null = vanilla

        /// <summary>The background pictures as path|crop keys, in the settings' order.</summary>
        private static List<(string key, string path, string crop)> BackgroundEntries()
        {
            var images = (Plugin.LoadingScreenImages.Value ?? "").Split(ScreenPictures.ListSeparator);
            var crops = (Plugin.LoadingScreenCrops.Value ?? "").Split(ScreenPictures.ListSeparator);
            var list = new List<(string, string, string)>();
            for (int i = 0; i < images.Length; i++)
            {
                string path = ShopPicture.ImagePath(images[i]);
                if (path == null) continue;
                string crop = i < crops.Length ? crops[i] : "";
                list.Add((path + "|" + crop, path, crop));
            }
            return list;
        }

        /// <param name="pickNew">A random picture (any of them, repeats too: user's choice). Otherwise (the shop's copy, settings
        /// changed) the shown one stays while it's still in the list.</param>
        private static void ShowBackground(bool pickNew)
        {
            var entries = BackgroundEntries();
            BackgroundSprites.Keep(new HashSet<string>(entries.Select(e => e.key)));
            if (!Background.Valid) return;
            if (entries.Count == 0) { _shownBackground = null; Background.Show(null); return; }
            int pick = pickNew || _shownBackground == null ? -1 : entries.FindIndex(e => e.key == _shownBackground);
            if (pick < 0) pick = Random.Next(entries.Count);
            var (key, path, crop) = entries[pick];
            _shownBackground = key;
            Background.Show(BackgroundSprites.Get(key, () => ShopPicture.CropSprite(path, crop, "LoadingScreenCrops", ScreenPictures.Aspect, "Loading screen")),
                ScreenPictures.Aspect);
        }

        // ---- the spinning card

        /// <summary>One card back Image: hidden while the card shows something else, which a child Image over its card area draws.</summary>
        private sealed class CardFace
        {
            internal readonly Image Back;
            private readonly Color _color;
            private Image _face;

            internal CardFace(Image back) { Back = back; _color = back.color; }
            internal bool Valid => Back != null;

            /// <param name="vanilla">the card back as the game draws it</param>
            /// <param name="sprite">the card to draw over it (null with !vanilla = no card)</param>
            internal void Show(bool vanilla, Sprite sprite)
            {
                if (Back == null) return;
                Back.color = vanilla ? _color : new Color(_color.r, _color.g, _color.b, 0f); // invisible, still spins its children
                if (sprite == null || vanilla)
                {
                    if (_face != null) _face.enabled = false;
                    return;
                }
                if (_face == null)
                {
                    var go = new GameObject("TCGCC_LoadingCard", typeof(RectTransform), typeof(Image));
                    var rt = (RectTransform)go.transform;
                    rt.SetParent(Back.transform, false);
                    rt.anchorMin = CardMin;
                    rt.anchorMax = CardMax;
                    rt.offsetMin = rt.offsetMax = Vector2.zero;
                    _face = go.GetComponent<Image>();
                    _face.raycastTarget = false;
                    _face.preserveAspect = true;
                }
                _face.sprite = sprite;
                _face.enabled = true;
            }
        }

        private static readonly List<CardFace> Cards = new List<CardFace>();
        private static readonly PictureCache<Sprite> CardPicture = new PictureCache<Sprite>();
        private static string _shownCard; // image file of the random card on screen

        private static void ShowCard(bool pickNew)
        {
            var mode = Plugin.LoadingScreenCard.Value;
            Sprite sprite = null;
            if (mode == LoadingCardMode.Picture)
            {
                string path = ShopPicture.ImagePath(Plugin.LoadingScreenCardImage.Value), crop = Plugin.LoadingScreenCardCrop.Value;
                sprite = CardPicture.Get(path + "|" + crop, () => ShopPicture.CropSprite(path, crop, "LoadingScreenCardCrop", CardAspect, "Loading screen card"));
            }
            else if (mode == LoadingCardMode.GlobalCardBack)
                sprite = ImageCache.Get(CustomSet.GlobalCardBackPath); // Game look → Card back (none: the vanilla back stays)
            else if (mode == LoadingCardMode.RandomFromMySets)
            {
                if (pickNew || _shownCard == null) _shownCard = RandomCardImage();
                sprite = ImageCache.Get(_shownCard);
            }
            // Nothing to show (no sets, no picture): the card back stays.
            bool vanilla = mode == LoadingCardMode.Vanilla || (mode != LoadingCardMode.Hidden && sprite == null);
            foreach (var c in Cards) c.Show(vanilla, sprite);
        }

        /// <summary>The image of a random card from the installed sets (full-image sets first: their image is the whole card); null when none.</summary>
        private static string RandomCardImage()
        {
            var sets = Registry.Sets.Where(s => s.Def.Cards.Any(c => !string.IsNullOrEmpty(c.Image))).ToList();
            var full = sets.Where(s => s.Def.RenderMode == Core.RenderMode.FullImage).ToList();
            if (full.Count > 0) sets = full;
            if (sets.Count == 0) return null;
            var set = sets[Random.Next(sets.Count)];
            var cards = set.Def.Cards.Where(c => !string.IsNullOrEmpty(c.Image)).ToList();
            return set.ImagePath(cards[Random.Next(cards.Count)]);
        }

        internal static void Init()
        {
            EventHandler background = (_, __) => ShowBackground(pickNew: false);
            Plugin.LoadingScreenImages.SettingChanged += background;
            Plugin.LoadingScreenCrops.SettingChanged += background;
            EventHandler card = (_, __) => ShowCard(pickNew: false);
            Plugin.LoadingScreenCard.SettingChanged += card;
            Plugin.LoadingScreenCardImage.SettingChanged += card;
            Plugin.LoadingScreenCardCrop.SettingChanged += card;
        }
    }
}
