using System.Linq;
using HarmonyLib;
using TCGCustomCards.Runtime;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Patches
{
    // Layer 3 — visuals. Framed sets need nothing here: art comes from the MonsterData.GetIcon patch and frames
    // from the cloned CardUISetting. FullImage sets get one overlay Image per CardUI that covers the card front.
    // CardUIs are pooled and reused for vanilla cards, so the overlay is hidden at the start of every SetCardUI.
    // Border grades stay visible like vanilla: for Base/1st/Silver/Gold/EX the art is inset so the grade's border rim
    // (m_CardBorderImage, already set by the game) shows around it (grade label: FullImageGradeLabelPatch; foil: FullImageFoilPatch);
    // Full Art keeps the art at full card size (the vanilla Full Art style hides the border too).

    [HarmonyPatch(typeof(CardUI), nameof(CardUI.SetCardUI))]
    internal static class SetCardUIPatch
    {
        private const string OverlayName = "TCGCC_FullImage";
        /// <summary>Full-image cards relative to the 900×900 card front (tuned in game, 2026-09-23).</summary>
        private const float FullImageScale = 0.9f;

        private static void Prefix(CardUI __instance)
        {
            var overlay = Find(__instance);
            if (overlay != null && overlay.gameObject.activeSelf) overlay.gameObject.SetActive(false);
            CardBorderView.BeforeCardSet(__instance);
        }

        private static void Postfix(CardUI __instance, CardData cardData)
        {
            CardBackMesh.Apply(__instance, cardData); // also clears our back for vanilla cards on reused CardUIs
            if (!Registry.IsCustom(cardData.expansionType)) return;
            if (!Registry.TryGetCard(cardData.monsterType, out var set, out int pos)) return;
            if (set.Def.RenderMode != Core.RenderMode.FullImage)
            {
                // Framed: the game put the main art (MonsterData.GetIcon) in the frame; a version with its own picture swaps it.
                // Vanilla sets the sprite again on every SetCardUI, so reused CardUIs need no reset.
                if (set.HasVersionImages(pos) && __instance.m_CenterFrameImage != null &&
                    set.VersionImagePath(pos, cardData.borderType, cardData.isFoil) != null)
                {
                    var art = set.CardImage(pos, cardData.borderType, cardData.isFoil);
                    if (art != null) __instance.m_CenterFrameImage.sprite = art;
                }
                return;
            }

            // MTG table: a transformed double-faced card shows its back face (MtgCardFaces marks that copy).
            // Otherwise the version's own picture (set.json "versionImages"), else the main image.
            var front = set.CardFace(pos, cardData.borderType, cardData.isFoil);
            var sprite = Runtime.Mtg.MtgCardFaces.IsBack(cardData) && set.HasBackFace(pos) ? set.BackFace(pos) ?? front : front;
            if (sprite == null) return;
            var overlay = Find(__instance) ?? Create(__instance);
            overlay.sprite = sprite;
            // A custom border (set.json "borders"): the border places the picture in its window.
            if (CardBorderView.Apply(__instance, overlay, set, pos, cardData))
            {
                overlay.gameObject.SetActive(true);
                return;
            }
            // Every card is the same shape: the picture fills the card's box. A scan of a real card (MTG 0.716, YGO 0.686,
            // Pokémon 0.717) is close to it and is stretched the few percent; any other picture (square-ish, landscape, tall)
            // is cropped from its middle to the card's shape, as a custom border crops it to its opening.
            float aspect = sprite.texture.width / (float)sprite.texture.height;
            if (Mathf.Abs(aspect / CardShape - 1f) > StretchLimit) overlay.sprite = PictureCrop.Fill(sprite, CardShape);
            Fit(__instance, overlay, CardShape);
            bool fullArt = cardData.borderType == ECardBorderType.FullArt;
            float inset = fullArt ? 1f : Mathf.Clamp(Plugin.FullImageInset.Value, 0.7f, 1f);
            overlay.rectTransform.localScale = Vector3.one * (FullImageScale * inset);
            overlay.gameObject.SetActive(true);
        }

        /// <summary>Pictures off the card's shape by at most this (relative) are stretched to it; others are cropped to it.</summary>
        private const float StretchLimit = 0.06f;

        /// <summary>
        /// The shape every full-image card's picture gets, width / height: a real card's 63 × 88 mm, the shape of the scans every other card
        /// is (Studio's lib/cardLooks CARD_SHAPE). The game's own card mask is a little narrower (1273×1791 = 0.711): cropped to
        /// that, the picture left a sliver of the game's card showing at its side (2026-11-21).
        /// </summary>
        private const float CardShape = 63f / 88f;

        /// <summary>
        /// Sizes the overlay to the card's printed area at the given shape (the card's): a sprite trimmed of its printed border
        /// is stretched back to the full card's size, so the game's border sits right against the card content instead of
        /// leaving the gap the border used to fill. Done on every SetCardUI because CardUIs are reused.
        /// </summary>
        private static void Fit(CardUI ui, Image overlay, float aspect)
        {
            var rt = overlay.rectTransform;
            var parentRt = rt.parent as RectTransform;
            bool parentSized = parentRt != null && parentRt.rect.width > 1f;
            Vector2 area = parentSized ? parentRt.rect.size
                : ui.m_CardBorderMask != null ? ui.m_CardBorderMask.rectTransform.rect.size : rt.rect.size;
            if (area.x <= 1f || area.y <= 1f || aspect <= 0f) { overlay.preserveAspect = true; return; }
            overlay.preserveAspect = false;
            if (parentSized)
            {
                rt.anchorMin = rt.anchorMax = new Vector2(0.5f, 0.5f);
                // On the game's card, a little left of the front's centre (CardBorderView.CardCentreX); a border may have
                // moved it into its window before.
                rt.anchoredPosition = new Vector2(CardBorderView.CardCentreX * area.x, 0f);
            }
            rt.sizeDelta = area.x / area.y > aspect ? new Vector2(area.y * aspect, area.y) : new Vector2(area.x, area.x / aspect);
        }

        private static Image Find(CardUI ui)
        {
            if (ui.m_CardFront == null) return null;
            var t = ui.m_CardFront.transform.Find(OverlayName);
            return t != null ? t.GetComponent<Image>() : null;
        }

        private static Image Create(CardUI ui)
        {
            var parent = ui.m_CardFront.transform;
            var go = new GameObject(OverlayName, typeof(RectTransform), typeof(Image));
            go.layer = ui.m_CardFront.layer;
            var rt = (RectTransform)go.transform;
            rt.SetParent(parent, worldPositionStays: false);

            // Match the card's printed area: copy the frame image's rect if the front root has no usable size.
            var parentRt = parent as RectTransform;
            var reference = ui.m_CardBorderMask != null ? ui.m_CardBorderMask.rectTransform : null;
            if (parentRt != null && parentRt.rect.width > 1f)
            {
                rt.anchorMin = Vector2.zero;
                rt.anchorMax = Vector2.one;
                rt.offsetMin = rt.offsetMax = Vector2.zero;
            }
            else if (reference != null)
            {
                rt.position = reference.position;
                rt.rotation = reference.rotation;
                rt.sizeDelta = reference.rect.size;
                rt.localScale = Vector3.one;
            }

            // Sit just below the foil group so foil shine still renders on top.
            if (ui.m_FoilGrp != null && ui.m_FoilGrp.transform.parent == parent)
                rt.SetSiblingIndex(ui.m_FoilGrp.transform.GetSiblingIndex());
            else
                rt.SetAsLastSibling();

            var img = go.GetComponent<Image>();
            img.raycastTarget = false;
            // CardFront is a 900×900 square with the card centered inside; keep the image's own aspect (MTG ≈ 0.716).
            img.preserveAspect = true;

            return img;
        }

    }
}
