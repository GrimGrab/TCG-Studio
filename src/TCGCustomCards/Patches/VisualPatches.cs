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
        private static bool _loggedLayout;

        private static void Prefix(CardUI __instance)
        {
            var overlay = Find(__instance);
            if (overlay != null && overlay.gameObject.activeSelf) overlay.gameObject.SetActive(false);
        }

        private static void Postfix(CardUI __instance, CardData cardData)
        {
            CardBackMesh.Apply(__instance, cardData); // also clears our back for vanilla cards on reused CardUIs
            if (!Registry.IsCustom(cardData.expansionType)) return;
            if (!Registry.TryGetCard(cardData.monsterType, out var set, out int pos)) return;
            if (set.Def.RenderMode != Core.RenderMode.FullImage) return;

            // MTG table: a transformed double-faced card shows its back face (MtgCardFaces marks that copy)
            var sprite = Runtime.Mtg.MtgCardFaces.IsBack(cardData) && set.HasBackFace(pos) ? set.BackFace(pos) ?? set.CardFace(pos) : set.CardFace(pos);
            if (sprite == null) return;
            var overlay = Find(__instance) ?? Create(__instance);
            overlay.sprite = sprite;
            Fit(__instance, overlay, sprite);
            bool fullArt = cardData.borderType == ECardBorderType.FullArt;
            float inset = fullArt ? 1f : Mathf.Clamp(Plugin.FullImageInset.Value, 0.7f, 1f);
            overlay.rectTransform.localScale = Vector3.one * (FullImageScale * inset);
            overlay.gameObject.SetActive(true);
        }

        /// <summary>
        /// Sizes the overlay to the card's printed area at the shape of the whole image (texture), not the sprite: a sprite
        /// trimmed of its printed border is stretched back to the full card's size, so the game's border sits right against
        /// the card content instead of leaving the gap the border used to fill. Untrimmed sprites get the same rect that
        /// preserveAspect gave them. Done on every SetCardUI because CardUIs are reused.
        /// </summary>
        private static void Fit(CardUI ui, Image overlay, Sprite sprite)
        {
            var rt = overlay.rectTransform;
            var parentRt = rt.parent as RectTransform;
            bool parentSized = parentRt != null && parentRt.rect.width > 1f;
            Vector2 area = parentSized ? parentRt.rect.size
                : ui.m_CardBorderMask != null ? ui.m_CardBorderMask.rectTransform.rect.size : rt.rect.size;
            float aspect = sprite.texture.width / (float)sprite.texture.height;
            if (area.x <= 1f || area.y <= 1f || aspect <= 0f) { overlay.preserveAspect = true; return; }
            overlay.preserveAspect = false;
            if (parentSized)
            {
                rt.anchorMin = rt.anchorMax = new Vector2(0.5f, 0.5f);
                rt.anchoredPosition = Vector2.zero;
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

            if (!_loggedLayout)
            {
                _loggedLayout = true;
                Plugin.Log.LogInfo($"FullImage overlay: parent={parent.name} parentRect={(parentRt != null ? parentRt.rect.ToString() : "n/a")} " +
                                   $"borderMaskRect={(reference != null ? reference.rect.ToString() : "n/a")} sibling={rt.GetSiblingIndex()}/{parent.childCount}");
                string Describe(Component c) => c == null ? "null" :
                    $"{PathOf(c.transform)} active={c.gameObject.activeInHierarchy} rect={(c.transform as RectTransform)?.rect} localPos={c.transform.localPosition} scale={c.transform.localScale}";
                Plugin.Log.LogInfo("CardFront children: " + string.Join(" | ", Enumerable.Range(0, parent.childCount).Select(i =>
                    { var ch = parent.GetChild(i); return $"{i}:{ch.name}{(ch.gameObject.activeSelf ? "" : "(off)")}"; })));
                Plugin.Log.LogInfo($"Border image: {Describe(ui.m_CardBorderImage)} sprite={ui.m_CardBorderImage?.sprite?.name}");
                Plugin.Log.LogInfo($"Border mask: {Describe(ui.m_CardBorderMask)}");
                Plugin.Log.LogInfo($"Edition text: {Describe(ui.m_FirstEditionText)}");
                Plugin.Log.LogInfo($"Foil group: {Describe(ui.m_FoilGrp != null ? ui.m_FoilGrp.transform : null)}");
            }
            return img;
        }

        private static string PathOf(Transform t)
        {
            string p = t.name;
            int depth = 0;
            for (var x = t.parent; x != null && depth < 6; x = x.parent, depth++) p = x.name + "/" + p;
            return p;
        }
    }
}
