using HarmonyLib;
using TCGCustomCards.Runtime;
using TMPro;
using UnityEngine;

namespace TCGCustomCards.Patches
{
    // Grade label ("1st Edition", "Silver", "Gold", "EX") on full-image cards. Vanilla shows it on those borders (not Base/Full Art)
    // via CardUI.m_FirstEditionText, which our overlay covers. We draw a copy of that text above the overlay. Visibility follows the
    // card's border (not whether the game's own text object happens to be active — on 3D cards it often isn't), position defaults
    // to the game's own spot and can be moved/resized live via [Visuals] GradeLabelOffsetX/Y/Size.
    // Like the foil glow, the helper lives on the CardUI and finds the overlay itself (no dependence on postfix order).

    [HarmonyPatch(typeof(CardUI), nameof(CardUI.SetCardUI))]
    internal static class FullImageGradeLabelPatch
    {
        private static void Postfix(CardUI __instance, CardData cardData)
        {
            var label = __instance.GetComponent<FullImageGradeLabel>();
            bool fullImage = cardData != null && Registry.IsCustom(cardData.expansionType) &&
                             Registry.TryGetCard(cardData.monsterType, out var set, out _) && set.Def.RenderMode == Core.RenderMode.FullImage;
            if (!fullImage)
            {
                if (label != null) label.Text = null; // CardUI reused for another card
                return;
            }
            if (label == null) (label = __instance.gameObject.AddComponent<FullImageGradeLabel>()).Ui = __instance;
            label.Text = HasLabel(cardData.borderType) ? GradeName(__instance, cardData.borderType) : null;
        }

        private static bool HasLabel(ECardBorderType b) =>
            b == ECardBorderType.FirstEdition || b == ECardBorderType.Silver || b == ECardBorderType.Gold || b == ECardBorderType.EX;

        /// <summary>The game's own text (SetCardUI fills it for these borders even when the object is inactive), else the enum name.</summary>
        private static string GradeName(CardUI ui, ECardBorderType border)
        {
            var src = ui.m_FirstEditionText;
            return src != null && !string.IsNullOrEmpty(src.text) ? src.text : border.ToString();
        }
    }

    internal sealed class FullImageGradeLabel : MonoBehaviour
    {
        private const string OverlayName = "TCGCC_FullImage";
        private const string LabelName = "TCGCC_GradeLabel";
        /// <summary>Card size in CardFront units (the card is ≈ 645×900 inside the 900² front) — offsets are % of this.</summary>
        private static readonly Vector2 CardSize = new Vector2(645f, 900f);

        public CardUI Ui;
        /// <summary>Label text for the current card, or null for none (set in SetCardUI).</summary>
        public string Text;
        private Transform _overlay;
        private TextMeshProUGUI _label;
        private static bool _logged;
        private bool _outlinePending;

        private void OnDisable() => Hide();

        private void LateUpdate()
        {
            if (Ui == null || Ui.m_CardFront == null) return;
            if (string.IsNullOrEmpty(Text) || !Plugin.GradeLabel.Value) { Hide(); return; }
            if (_overlay == null) _overlay = Ui.m_CardFront.transform.Find(OverlayName);
            var src = Ui.m_FirstEditionText;
            if (_overlay == null || !_overlay.gameObject.activeSelf || src == null) { Hide(); return; }
            if (_label == null) Create(src);

            // Game's own spot (the text keeps its transform even while inactive) + user offset, in CardFront space.
            var parent = _label.transform.parent;
            var basePos = parent.InverseTransformPoint(src.transform.position);
            var offset = new Vector3(Plugin.GradeLabelOffsetX.Value / 100f * CardSize.x, Plugin.GradeLabelOffsetY.Value / 100f * CardSize.y, 0f);
            _label.transform.localPosition = basePos + offset;
            _label.transform.rotation = src.transform.rotation;
            Vector3 p = parent.lossyScale, w = src.transform.lossyScale;
            float size = Mathf.Clamp(Plugin.GradeLabelSize.Value, 0.3f, 4f);
            _label.transform.localScale = new Vector3(w.x / Mathf.Max(1e-5f, p.x), w.y / Mathf.Max(1e-5f, p.y), w.z / Mathf.Max(1e-5f, p.z)) * size;

            if (_label.text != Text) _label.text = Text;
            if (_label.color != src.color) _label.color = src.color;
            if (!_label.enabled) _label.enabled = true;
            if (!_label.gameObject.activeSelf) _label.gameObject.SetActive(true);
            if (_outlinePending && _label.isActiveAndEnabled)
            {
                try
                {
                    // Dark outline so the label reads over any artwork (instanced material, one per pooled CardUI).
                    _label.outlineWidth = 0.25f;
                    _label.outlineColor = new Color32(0, 0, 0, 220);
                    _outlinePending = false;
                }
                catch (System.NullReferenceException) { /* material not ready yet; retry next frame */ }
            }
            if (_label.transform.GetSiblingIndex() != parent.childCount - 1) _label.transform.SetAsLastSibling();
        }

        private void Hide()
        {
            if (_label != null && _label.gameObject.activeSelf) _label.gameObject.SetActive(false);
        }

        private void Create(TextMeshProUGUI src)
        {
            // A copy of the game's own label object: same font, material and styling as vanilla.
            var go = Instantiate(src.gameObject, Ui.m_CardFront.transform, worldPositionStays: true);
            go.name = LabelName;
            for (int i = go.transform.childCount - 1; i >= 0; i--) Destroy(go.transform.GetChild(i).gameObject);
            _label = go.GetComponent<TextMeshProUGUI>();
            _label.raycastTarget = false;
            _outlinePending = true; // set once the label is active (TMP's material isn't ready on an inactive copy)
            if (!_logged)
            {
                _logged = true;
                Plugin.Log.LogInfo($"Grade label: created for {Ui.name} ('{Text}', font={_label.font?.name}, size={_label.fontSize})");
            }
        }
    }
}
