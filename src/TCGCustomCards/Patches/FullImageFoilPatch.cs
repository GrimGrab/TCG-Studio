using HarmonyLib;
using TCGCustomCards.Runtime;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Patches
{
    // The game's own foil glow on full-image cards, exactly as vanilla draws it over the monster art, for foil cards that no
    // card shader draws (holo switched off, or a version given "no shader"). In the vanilla card front the rainbow shine and
    // blended layers sit UNDER the art (they colour frame/background only); the only foil drawn OVER the art is
    // CardGrp/AnimGrp/FoilGrp/GlowMask (CardFullMask) → LegendaryShine (M_CardLegendGlowUI, the drifting glow animated by
    // LegendCardGlowTexturePanner). Our full-image overlay covers FoilGrp, so a copy of GlowMask is drawn right above the overlay,
    // mirroring the original's visibility/transform/materials every frame. Kept separate from VisualPatches on purpose.
    // The helper lives on the CardUI (not the overlay) and finds the overlay itself each frame, so it doesn't depend on the
    // order Harmony runs the SetCardUI postfixes in. Holo foil itself is a card shader now (CardShaderPatch).

    [HarmonyPatch(typeof(CardUI), nameof(CardUI.SetCardUI))]
    internal static class FullImageFoilGlowPatch
    {
        private static void Postfix(CardUI __instance, CardData cardData)
        {
            if (cardData == null || !Registry.IsCustom(cardData.expansionType)) return;
            if (!Registry.TryGetCard(cardData.monsterType, out var set, out _) || set.Def.RenderMode != Core.RenderMode.FullImage) return;
            if (__instance.GetComponent<FullImageFoilGlow>() != null) return;
            __instance.gameObject.AddComponent<FullImageFoilGlow>().Ui = __instance;
        }
    }

    internal sealed class FullImageFoilGlow : MonoBehaviour
    {
        private const string OverlayName = "TCGCC_FullImage";
        private const string CopyName = "TCGCC_FoilGlow";
        public CardUI Ui;
        private Transform _overlay;
        private Transform _src;
        private Transform _copy;

        private void OnDisable() => Hide();

        private void LateUpdate()
        {
            if (Ui == null || Ui.m_FoilGrp == null || Ui.m_CardFront == null) return;
            // Cheap exit for the common case: FoilGrp is only active on foil cards (the game toggles it in SetCardUI).
            if (!Ui.m_FoilGrp.activeInHierarchy) { Hide(); return; }
            // Lookups run once and are cached (the overlay/GlowMask objects live as long as the CardUI).
            if (_src == null) _src = Ui.m_FoilGrp.transform.Find("GlowMask");
            if (_overlay == null) _overlay = Ui.m_CardFront.transform.Find(OverlayName);
            if (_src == null || _overlay == null) { Hide(); return; }

            // Vanilla shows it when the card is foil (FoilGrp active) and not culled; we also need our art to be showing, and
            // no card shader drawing it.
            bool on = Plugin.FullImageFoilGlow.Value && _overlay.gameObject.activeSelf && _src.gameObject.activeInHierarchy &&
                !CardShaderLayer.ActiveOn(Ui);
            if (!on) { Hide(); return; }
            if (_copy == null) CreateCopy();

            // Same place/size as the original, drawn right above the overlay.
            _copy.SetPositionAndRotation(_src.position, _src.rotation);
            Vector3 p = _copy.parent.lossyScale, w = _src.lossyScale;
            _copy.localScale = new Vector3(w.x / Mathf.Max(1e-5f, p.x), w.y / Mathf.Max(1e-5f, p.y), w.z / Mathf.Max(1e-5f, p.z));
            int idx = _overlay.GetSiblingIndex() + 1;
            if (_copy.GetSiblingIndex() != idx) _copy.SetSiblingIndex(idx);
            Mirror(_src, _copy);
            if (!_copy.gameObject.activeSelf) _copy.gameObject.SetActive(true);
        }

        private void Hide()
        {
            if (_copy != null && _copy.gameObject.activeSelf) _copy.gameObject.SetActive(false);
        }

        private void CreateCopy()
        {
            var go = Instantiate(_src.gameObject, _overlay.parent, worldPositionStays: true);
            go.name = CopyName;
            _copy = go.transform;
            foreach (var g in go.GetComponentsInChildren<Graphic>(true)) g.raycastTarget = false;
        }

        /// <summary>Copies active state and Image state from the original subtree (the game toggles/animates these).</summary>
        private static void Mirror(Transform src, Transform dst)
        {
            for (int i = 0; i < src.childCount && i < dst.childCount; i++)
            {
                Transform s = src.GetChild(i), d = dst.GetChild(i);
                if (d.gameObject.activeSelf != s.gameObject.activeSelf) d.gameObject.SetActive(s.gameObject.activeSelf);
                var si = s.GetComponent<Image>();
                var di = d.GetComponent<Image>();
                if (si != null && di != null)
                {
                    if (di.enabled != si.enabled) di.enabled = si.enabled;
                    if (di.material != si.material) di.material = si.material;
                    if (di.sprite != si.sprite) di.sprite = si.sprite;
                    if (di.color != si.color) di.color = si.color;
                }
                if (s.childCount > 0) Mirror(s, d);
            }
        }
    }
}
