using System.Linq;
using HarmonyLib;
using TCGCustomCards.Runtime;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Patches
{
    // Foil on full-image cards, exactly as vanilla draws it over the monster art. In the vanilla card front the rainbow shine and
    // blended layers sit UNDER the art (they colour frame/background only); the only foil drawn OVER the art is
    // CardGrp/AnimGrp/FoilGrp/GlowMask (CardFullMask) → LegendaryShine (M_CardLegendGlowUI, the drifting glow animated by
    // LegendCardGlowTexturePanner). Our full-image overlay covers FoilGrp, so a copy of GlowMask is drawn right above the overlay,
    // mirroring the original's visibility/transform/materials every frame. Kept separate from VisualPatches on purpose.
    // The helper lives on the CardUI (not the overlay) and finds the overlay itself each frame, so it doesn't depend on the
    // order Harmony runs the two SetCardUI postfixes in.
    // Custom holo ([Foil] HoloFoil): our own shader (Runtime/HoloFoil, bundle tcgcc_foil) drawn by a child of the overlay that uses
    // the artwork sprite itself, additive over the art, per-grade material. Shown with the same checks (foil card, FoilGrp showing).
    // Rim ([Foil] HoloFoilRim): the same material on an Image inside the game's CardBorderMask (so the Mask clips it to the rim),
    // using the grade's border sheet as sprite; while it shows, the game's own foil layers on that CardUI are disabled (they only
    // show on the rim anyway) and re-enabled when our holo stops. SetCardUI resets them, so we just forget our list then.

    [HarmonyPatch(typeof(CardUI), nameof(CardUI.SetCardUI))]
    internal static class FullImageFoilGlowPatch
    {
        private static int _attached;

        private static void Postfix(CardUI __instance, CardData cardData)
        {
            var existing = __instance.GetComponent<FullImageFoilGlow>();
            existing?.OnCardSet(); // the game just reset its foil layers for the new card
            if (cardData == null || !Registry.IsCustom(cardData.expansionType)) return;
            if (!Registry.TryGetCard(cardData.monsterType, out var set, out _) || set.Def.RenderMode != Core.RenderMode.FullImage) return;
            if (existing != null) return;
            __instance.gameObject.AddComponent<FullImageFoilGlow>().Ui = __instance;
            if (_attached++ < 3) Plugin.Log.LogInfo($"Foil glow: attached to {__instance.name} (foil={cardData.isFoil})");
        }
    }

    internal sealed class FullImageFoilGlow : MonoBehaviour
    {
        private const string OverlayName = "TCGCC_FullImage";
        private const string CopyName = "TCGCC_FoilGlow";
        private const string HoloName = "TCGCC_HoloFoil";
        private const string RimName = "TCGCC_HoloRim";
        public CardUI Ui;
        private Transform _overlay;
        private Transform _src;
        private Transform _copy;
        private Image _overlayImage;
        private Image _holo;
        private Image _rim;
        /// <summary>Game foil images we disabled while our rim holo shows (re-enabled when it stops).</summary>
        private readonly System.Collections.Generic.List<Image> _suppressed = new System.Collections.Generic.List<Image>();
        private static bool _logged;
        private static int _diag;
        private CardData _lastDiagCard;

        private void OnDisable()
        {
            Hide();
            HideHolo();
        }

        private void LateUpdate()
        {
            if (Ui == null || Ui.m_FoilGrp == null || Ui.m_CardFront == null) return;
            // Cheap exit for the common case: FoilGrp is only active on foil cards (the game toggles it in SetCardUI).
            if (!Ui.m_FoilGrp.activeInHierarchy) { Hide(); HideHolo(); return; }
            // Lookups run once and are cached (the overlay/GlowMask objects live as long as the CardUI).
            if (_src == null) _src = Ui.m_FoilGrp.transform.Find("GlowMask");
            if (_overlay == null) _overlay = Ui.m_CardFront.transform.Find(OverlayName);
            if (_src == null || _overlay == null) { Hide(); HideHolo(); return; }
            // Our holo replaces the vanilla glow on this card; the glow only shows when the holo is off/missing for it.
            bool holo = UpdateHolo();

            // Vanilla shows it when the card is foil (FoilGrp active) and not culled; we also need our art to be showing.
            bool on = !holo && Plugin.FullImageFoilGlow.Value && _overlay.gameObject.activeSelf && _src.gameObject.activeInHierarchy;
            var data = Ui.GetCardData();
            if (data != null && data.isFoil && _overlay.gameObject.activeSelf && _diag < 8 && _lastDiagCard != data)
            {
                _diag++;
                _lastDiagCard = data;
                var shine = _src.childCount > 0 ? _src.GetChild(0) : null;
                Plugin.Log.LogInfo($"Foil glow check: border={data.borderType} on={on} FoilGrp self={Ui.m_FoilGrp.activeSelf} hier={Ui.m_FoilGrp.activeInHierarchy} " +
                                   $"GlowMask hier={_src.gameObject.activeInHierarchy} shine={shine?.name} active={shine?.gameObject.activeSelf} " +
                                   $"enabled={shine?.GetComponent<Image>()?.enabled} color={shine?.GetComponent<Image>()?.color} mat={shine?.GetComponent<Image>()?.material?.name}");
            }
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

        // ---- custom holo ----

        /// <summary>Shows our holo when enabled for this card's grade; returns whether it is showing.</summary>
        private bool UpdateHolo()
        {
            var data = Ui.GetCardData();
            bool on = Plugin.HoloFoil.Value && data != null && data.isFoil && _overlay.gameObject.activeSelf;
            var mat = on ? HoloFoil.Get(data.borderType) : null;
            if (mat == null) { HideHolo(); Restore(); return false; }
            if (_overlayImage == null) _overlayImage = _overlay.GetComponent<Image>();
            if (_holo == null) CreateHolo();
            if (_holo.sprite != _overlayImage.sprite) _holo.sprite = _overlayImage.sprite;
            if (_holo.material != mat) _holo.material = mat;
            if (!_holo.enabled) _holo.enabled = true;
            if (!_holo.gameObject.activeSelf) _holo.gameObject.SetActive(true);
            if (UpdateRim(mat)) Suppress(); else Restore();
            return true;
        }

        /// <summary>Holo on the border rim: an Image inside CardBorderMask (clipped to the rim) showing the grade's border sheet.</summary>
        private bool UpdateRim(Material mat)
        {
            var mask = Ui.m_CardBorderMask;
            var border = Ui.m_CardBorderImage;
            if (!Plugin.HoloFoilRim.Value || mask == null || border == null || !mask.gameObject.activeInHierarchy) { HideRim(); return false; }
            if (_rim == null || _rim.transform.parent != mask.transform) CreateRim(mask.transform, border.rectTransform);
            if (_rim.sprite != border.sprite) _rim.sprite = border.sprite;
            if (_rim.material != mat) _rim.material = mat;
            if (_rim.transform.GetSiblingIndex() != mask.transform.childCount - 1) _rim.transform.SetAsLastSibling();
            if (!_rim.enabled) _rim.enabled = true;
            if (!_rim.gameObject.activeSelf) _rim.gameObject.SetActive(true);
            return true;
        }

        private void CreateRim(Transform parent, RectTransform like)
        {
            if (_rim != null) Destroy(_rim.gameObject);
            var go = new GameObject(RimName, typeof(RectTransform), typeof(Image));
            go.layer = parent.gameObject.layer;
            var rt = (RectTransform)go.transform;
            rt.SetParent(parent, worldPositionStays: false);
            rt.anchorMin = like.anchorMin;
            rt.anchorMax = like.anchorMax;
            rt.pivot = like.pivot;
            rt.anchoredPosition = like.anchoredPosition;
            rt.sizeDelta = like.sizeDelta;
            rt.localRotation = like.localRotation;
            rt.localScale = like.localScale;
            _rim = go.GetComponent<Image>();
            _rim.raycastTarget = false;
        }

        private void HideRim()
        {
            if (_rim != null && _rim.gameObject.activeSelf) _rim.gameObject.SetActive(false);
        }

        /// <summary>Keep the game's foil layers off on this CardUI while our holo replaces them (runs after the game toggles them).</summary>
        private void Suppress()
        {
            SuppressList(Ui.m_FoilShowList);
            SuppressList(Ui.m_FoilBlendedShowList);
        }

        private void SuppressList(System.Collections.Generic.List<Image> list)
        {
            if (list == null) return;
            foreach (var img in list)
            {
                if (img == null || !img.enabled) continue;
                img.enabled = false;
                if (!_suppressed.Contains(img)) _suppressed.Add(img);
            }
        }

        private void Restore()
        {
            HideRim();
            foreach (var img in _suppressed) if (img != null) img.enabled = true;
            _suppressed.Clear();
        }

        /// <summary>SetCardUI re-applied the game's foil state for a new card: nothing of ours to restore.</summary>
        public void OnCardSet() => _suppressed.Clear();

        private void CreateHolo()
        {
            var go = new GameObject(HoloName, typeof(RectTransform), typeof(Image));
            go.layer = _overlay.gameObject.layer;
            var rt = (RectTransform)go.transform;
            rt.SetParent(_overlay, worldPositionStays: false);
            rt.anchorMin = Vector2.zero;
            rt.anchorMax = Vector2.one;
            rt.offsetMin = rt.offsetMax = Vector2.zero;
            _holo = go.GetComponent<Image>();
            _holo.preserveAspect = true; // same drawn rect as the overlay's artwork
            _holo.raycastTarget = false;
        }

        private void HideHolo()
        {
            if (_holo != null && _holo.gameObject.activeSelf) _holo.gameObject.SetActive(false);
            HideRim();
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
            if (!_logged)
            {
                _logged = true;
                var shine = _src.childCount > 0 ? _src.GetChild(0).GetComponent<Image>() : null;
                Plugin.Log.LogInfo($"Foil glow: copied {_src.name} ({_src.childCount} children, shine mat={shine?.material?.name}) above the overlay; " +
                                   $"CardFront children: {string.Join(" | ", Enumerable.Range(0, _overlay.parent.childCount).Select(i => _overlay.parent.GetChild(i).name))}");
            }
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
