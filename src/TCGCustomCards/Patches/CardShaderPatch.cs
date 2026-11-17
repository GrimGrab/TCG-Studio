using System.Collections.Generic;
using HarmonyLib;
using TCGCustomCards.Core;
using TCGCustomCards.Runtime;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Patches
{
    // Card shaders (Runtime/CardShaders). Which shader a card version gets:
    //   1. the card's own choice, else the set's (set.json "versionShaders"; an empty entry = none);
    //   2. no choice anywhere, a full-image set and the version is foil: the stock holo of its border ([Foil] HoloFoil).
    // What a shader draws on is its own setting (ShaderDef.Draws):
    //   - the picture (default): the card's picture is drawn by an Image that is a child of the picture (same place, same
    //     sprite) with the shader's material; the picture's own Image is switched off meanwhile, so the shader alone decides
    //     what shows. The picture is our full-image overlay on full-image sets and the game's artwork Image
    //     (m_CenterFrameImage, inside the frame) on framed sets.
    //   - see-through: the same, and every other graphic of the card front is switched off while it shows, so where the
    //     shader returns no alpha you see what is behind the card (a dissolve, a ghost card).
    //   - over the card: our Image lies over the whole card front, last in drawing order, and the picture stays as it is
    //     (full-image sets: over the picture, which is the whole card there).
    // The helper lives on the CardUI and finds the picture each frame, like FullImageFoilGlow, so the order of the SetCardUI
    // postfixes doesn't matter. While a shader draws the picture, FullImageFoilGlow draws no vanilla glow copy on that card.
    // Rim ([Foil] HoloFoilRim, stock holos only): the same material on an Image inside the game's CardBorderMask (so the Mask
    // clips it to the rim), showing the border sheet; while it shows, the game's own foil layers on that CardUI are disabled
    // (they only show on the rim anyway) and re-enabled when it stops. SetCardUI resets them, so we just forget our list then.
    // What a shader knows about its card (random numbers, when it appeared, the pointer, border, foil, rarity, grade) travels
    // in the Image's vertices (CardShaderMesh), so cards keep sharing one material.

    [HarmonyPatch(typeof(CardUI), nameof(CardUI.SetCardUI))]
    internal static class CardShaderPatch
    {
        // Before the game sets the card's graphics up for the next card: give back the ones a see-through shader switched
        // off, so what the game switches on and off now is what counts.
        private static void Prefix(CardUI __instance)
        {
            if (__instance.TryGetComponent<CardShaderLayer>(out var layer)) layer.BeforeCardSet();
        }

        private static void Postfix(CardUI __instance, CardData cardData)
        {
            var layer = __instance.GetComponent<CardShaderLayer>();
            layer?.OnCardSet(); // the game just reset its foil layers for the new card
            string id = null;
            CustomSet set = null;
            int pos = -1;
            bool framed = false;
            if (Plugin.CardShaders.Value && cardData != null && Registry.IsCustom(cardData.expansionType) &&
                Registry.TryGetCard(cardData.monsterType, out set, out pos))
            {
                framed = set.Def.RenderMode != Core.RenderMode.FullImage;
                id = set.ShaderId(pos, cardData.borderType, cardData.isFoil);
                // Framed cards keep the game's own foil unless a shader was chosen: the stock holos are for whole-card pictures.
                if (id == null && !framed && cardData.isFoil && Plugin.HoloFoil.Value) id = CardShaders.StockFor(cardData.borderType);
            }
            if (string.IsNullOrEmpty(id))
            {
                if (layer != null) layer.Set(null, null, 0, null, false); // a reused CardUI: back to a normal card
                return;
            }
            if (layer == null)
            {
                layer = __instance.gameObject.AddComponent<CardShaderLayer>();
                layer.Ui = __instance;
            }
            layer.Set(id, set, pos, cardData, framed);
        }
    }

    // The pointer on a card (3D cards: binder pages, pack opening, cards on show): shaders read how long it has been there.
    [HarmonyPatch(typeof(InteractableCard3d), nameof(InteractableCard3d.OnRaycasted))]
    internal static class CardShaderHoverPatch
    {
        private static void Postfix(InteractableCard3d __instance) => CardShaderLayer.Hover(__instance, true);
    }

    [HarmonyPatch(typeof(InteractableCard3d), nameof(InteractableCard3d.OnRaycastEnded))]
    internal static class CardShaderHoverEndPatch
    {
        private static void Postfix(InteractableCard3d __instance) => CardShaderLayer.Hover(__instance, false);
    }

    internal sealed class CardShaderLayer : MonoBehaviour
    {
        private const string OverlayName = "TCGCC_FullImage";
        private const string LayerName = "TCGCC_CardShader";
        private const string RimName = "TCGCC_CardShaderRim";
        public CardUI Ui;
        private string _id, _usedBy, _draws;
        private CustomSet _set;
        private int _pos;
        private bool _framed;
        private Transform _overlay;
        private Image _overlayImage;
        private Image _image;
        private CardShaderMesh _mesh;
        private Image _rim;
        /// <summary>Game foil images we disabled while our rim shows (re-enabled when it stops).</summary>
        private readonly List<Image> _suppressed = new List<Image>();
        /// <summary>See-through shaders: the card front's other graphics we switched off (switched on again when it stops).</summary>
        private readonly List<Graphic> _hidden = new List<Graphic>();
        private bool _hiddenDone;

        // What the shader is told about the card (CardShaderMesh).
        private string _shownKey;
        private Vector4 _card, _facts;
        private bool _hover;

        /// <summary>Whether a shader draws this card's picture right now (FullImageFoilGlow then leaves the card alone).</summary>
        public static bool ActiveOn(CardUI ui) =>
            ui != null && ui.TryGetComponent<CardShaderLayer>(out var l) && l._id != null && l._draws != ShaderDef.Over &&
            Plugin.CardShaders.Value && CardShaders.Get(l._id, l._usedBy) != null;

        /// <summary>The shader's clock right now: what the template's vertex program reads as time.</summary>
        private static float ShaderTime => Shader.GetGlobalVector("_Time").y;

        /// <summary>
        /// The shader of the card this CardUI now shows (id null = none). framed: the card is drawn in the game's frame, so the
        /// picture is the game's own artwork Image inside the frame instead of our full-image overlay.
        /// </summary>
        public void Set(string id, CustomSet set, int pos, CardData data, bool framed)
        {
            string draws = id != null ? CardShaders.Draws(id) : null;
            if (id == null || framed != _framed || draws != _draws)
            {
                Hide();
                if (framed != _framed || draws != _draws)
                {
                    // Another place to draw on: our Image is a child of the old one.
                    if (_image != null) Destroy(_image.gameObject);
                    _image = null;
                    _mesh = null;
                    _overlay = null;
                    _overlayImage = null;
                    _framed = framed;
                    _draws = draws;
                }
            }
            _id = id;
            _set = set;
            _pos = pos;
            _usedBy = set?.Def.Label;
            _mat = null;
            _describe = id != null;
            _shownFrames = 0;
            if (id == null || data == null) { _shownKey = null; return; }

            // A card that wasn't showing here a moment ago has just appeared: its own random number and clock start now.
            string key = $"{(int)data.monsterType}|{(int)data.borderType}|{data.isFoil}|{data.cardGrade}";
            if (key != _shownKey)
            {
                _shownKey = key;
                _card = new Vector4(Random.value, CardNumber(set.Def.Id, set.Card(pos).Id), ShaderTime, 0f);
                _hover = false;
            }
            _facts = new Vector4((int)data.borderType, data.isFoil ? 1f : 0f, set.Rarity(pos).Rank, Mathf.Max(0, data.cardGrade));
        }

        /// <summary>A number 0..1 that belongs to the card (the same on every PC and in every session).</summary>
        private static float CardNumber(string setId, string cardId)
        {
            uint h = 2166136261;
            foreach (char c in setId + "|" + cardId) h = (h ^ c) * 16777619;
            return (h & 0xFFFFFF) / (float)0x1000000;
        }

        /// <summary>The pointer went onto or left a 3D card.</summary>
        public static void Hover(InteractableCard3d card, bool on)
        {
            var ui = card != null && card.m_Card3dUI != null ? card.m_Card3dUI.m_CardUI : null;
            if (ui == null || !ui.TryGetComponent<CardShaderLayer>(out var l) || l._id == null || l._hover == on) return;
            l._hover = on;
            l._card.w = on ? Mathf.Max(ShaderTime, 0.0001f) : 0f; // 0 = "not on it"
        }

        /// <summary>SetCardUI re-applied the game's foil state for a new card: nothing of ours to restore there.</summary>
        public void OnCardSet() => _suppressed.Clear();

        /// <summary>SetCardUI is about to run: the card front's graphics are the game's again (LateUpdate hides them anew when the next card has a see-through shader).</summary>
        public void BeforeCardSet()
        {
            Show(_hidden);
            _hiddenDone = false;
        }

        private void OnEnable() => _shownKey = null; // shown again: the next Set starts its clock
        private void OnDisable() => Hide();

        private void LateUpdate()
        {
            if (_id == null || Ui == null || Ui.m_CardFront == null) return;
            // Far away (Card3dUISpawner's distance cull): the game drops its own foil there, so the shader goes too and the
            // card shows its plain picture. Without this every custom foil card in the shop ran its shader.
            if (Ui.m_IsFarDistanceCulled) { Hide(); return; }
            if (_overlay == null)
                _overlay = _framed ? (Ui.m_CenterFrameImage != null ? Ui.m_CenterFrameImage.transform : null) : Ui.m_CardFront.transform.Find(OverlayName);
            Material mat = null;
            if (Plugin.CardShaders.Value && _overlay != null && _overlay.gameObject.activeSelf)
                mat = _mat != null ? _mat : (_mat = CardShaders.Get(_id, _usedBy, _set, _pos));
            if (mat == null) { Hide(); return; }
            if (_overlayImage == null) _overlayImage = _overlay.GetComponent<Image>();
            bool over = _draws == ShaderDef.Over;
            if (_image == null)
            {
                _image = over ? CreateOver() : Create(LayerName, _overlay);
                _mesh = _image.GetComponent<CardShaderMesh>();
            }
            if (_image.sprite != _overlayImage.sprite) _image.sprite = _overlayImage.sprite;
            if (over)
            {
                if (_image.transform.GetSiblingIndex() != _image.transform.parent.childCount - 1) _image.transform.SetAsLastSibling();
            }
            else
            {
                if (_image.preserveAspect != _overlayImage.preserveAspect) _image.preserveAspect = _overlayImage.preserveAspect;
                if (_overlayImage.enabled) _overlayImage.enabled = false;
            }
            _mesh.Tell(_card, _facts);
            Show(_image, mat);
            if (UpdateRim(mat)) Suppress(); else Restore();
            if (_draws == ShaderDef.SeeThrough && !_hiddenDone) HideCardFront();
            Describe(mat);
        }

        // ---- what a shader was drawn with, once per shader, version and place (for support: bounded, a line each)
        private static readonly HashSet<string> Described = new HashSet<string>();
        private int _shownFrames;
        private bool _describe;
        private Material _mat;

        // Runs once per card set on this CardUI (Set), on the third frame it shows, and costs nothing on other frames.
        private void Describe(Material mat)
        {
            if (!_describe) return;
            if (Described.Count >= 60) { _describe = false; return; }
            // Two frames after it first shows: the canvas has built the mesh and picked the material it really draws with.
            if (++_shownFrames < 3) return;
            _describe = false;
            var data = Ui.GetCardData();
            if (data == null) return;
            var canvas = _image.canvas;
            string masks = "";
            for (var t = _image.transform.parent; t != null && (canvas == null || t != canvas.transform.parent); t = t.parent)
            {
                if (t.TryGetComponent<Mask>(out var m) && m.enabled) masks += $" Mask:{t.name}";
                if (t.TryGetComponent<RectMask2D>(out var r) && r.enabled) masks += $" RectMask2D:{t.name}";
            }
            string key = $"{_id}|{data.borderType}|{data.isFoil}|{(canvas != null ? canvas.name : "-")}|{masks}";
            if (!Described.Add(key)) return;
            var drawn = _image.materialForRendering;
            var sprite = _image.sprite;
            var rect = _image.rectTransform.rect;
            Plugin.Log.LogInfo($"Card shader drawn: {_id} ({_draws ?? "picture"}) on {data.borderType}{(data.isFoil ? " foil" : "")} of {_usedBy}; " +
                $"canvas {(canvas != null ? $"{canvas.name} ({canvas.renderMode}, channels {canvas.additionalShaderChannels})" : "none")};" +
                $" masks:{(masks.Length == 0 ? " none" : masks)}; material {(drawn == mat ? (mat.name.EndsWith("_" + _id) ? "the shader's own" : "the card's own") : $"a copy ({(drawn != null ? drawn.name : "null")}, shader {(drawn != null && drawn.shader != null ? drawn.shader.name : "null")})")}," +
                $" slot 0 = {(drawn != null && drawn.HasProperty("_P0") ? drawn.GetVector("_P0").ToString("F3") : "missing")}, mask {(drawn != null && drawn.HasProperty("_Mask") && drawn.GetTexture("_Mask") != null ? drawn.GetTexture("_Mask").name : "none")}, keywords [{(drawn != null ? string.Join(" ", drawn.shaderKeywords) : "")}];" +
                $" card {_card.ToString("F2")}, facts {_facts.ToString("F0")}{(_hidden.Count > 0 ? $", {_hidden.Count} card graphic(s) hidden" : "")};" +
                $" image {rect.width:F0}x{rect.height:F0} {(_image.isActiveAndEnabled ? "on" : "OFF")}, alpha {_image.canvasRenderer.GetAlpha():F2}, cull {_image.canvasRenderer.cull};" +
                $" sprite {(sprite != null ? $"{sprite.rect.width:F0}x{sprite.rect.height:F0} of {sprite.texture.width}x{sprite.texture.height}" : "none")}; art image {(_overlayImage.enabled ? (_draws == ShaderDef.Over ? "on" : "STILL ON") : "off")}");
        }

        /// <summary>The shader's coordinates and card facts travel in extra UV channels, which a canvas only passes on when asked.</summary>
        private const AdditionalCanvasShaderChannels Channels =
            AdditionalCanvasShaderChannels.TexCoord1 | AdditionalCanvasShaderChannels.TexCoord2 | AdditionalCanvasShaderChannels.TexCoord3;

        private static void Show(Image image, Material mat)
        {
            if (image.material != mat) image.material = mat;
            var canvas = image.canvas;
            if (canvas != null && (canvas.additionalShaderChannels & Channels) != Channels) canvas.additionalShaderChannels |= Channels;
            if (!image.enabled) image.enabled = true;
            if (!image.gameObject.activeSelf) image.gameObject.SetActive(true);
        }

        private void Hide()
        {
            if (_image != null && _image.gameObject.activeSelf) _image.gameObject.SetActive(false);
            if (_overlayImage != null && !_overlayImage.enabled) _overlayImage.enabled = true;
            Show(_hidden);
            _hiddenDone = false;
            Restore();
        }

        private static Image Create(string name, Transform parent)
        {
            var go = new GameObject(name, typeof(RectTransform), typeof(Image), typeof(CardShaderMesh));
            go.layer = parent.gameObject.layer;
            var rt = (RectTransform)go.transform;
            rt.SetParent(parent, worldPositionStays: false);
            rt.anchorMin = Vector2.zero;
            rt.anchorMax = Vector2.one;
            rt.offsetMin = rt.offsetMax = Vector2.zero;
            var image = go.GetComponent<Image>();
            image.preserveAspect = false; // the overlay's rect is the drawn card (SetCardUIPatch.Fit): cover it exactly
            image.raycastTarget = false;
            return image;
        }

        /// <summary>
        /// The Image of an "over the card" shader: where the card's border sheet is (the whole printed card), last among its
        /// siblings so it is drawn over frame, text and stats. Without a border sheet it covers the picture.
        /// On a full-image card the picture is the whole card and is drawn after the border sheet, so there the Image lies
        /// on the picture (beside the border sheet it would be hidden behind the picture, showing on the rim only).
        /// </summary>
        private Image CreateOver()
        {
            var like = Ui.m_CardBorderMask != null ? Ui.m_CardBorderMask.rectTransform : null;
            if (!_framed || like == null || like.parent == null) return Create(LayerName, _overlay);
            var image = Create(LayerName, like.parent);
            var rt = image.rectTransform;
            rt.anchorMin = like.anchorMin;
            rt.anchorMax = like.anchorMax;
            rt.pivot = like.pivot;
            rt.anchoredPosition = like.anchoredPosition;
            rt.sizeDelta = like.sizeDelta;
            rt.localRotation = like.localRotation;
            rt.localScale = like.localScale;
            rt.SetAsLastSibling();
            return image;
        }

        // ---- see-through: nothing of the card front but the shader's picture

        /// <summary>
        /// Switches off every other graphic of the card front (frame, border, text, stats, our grade label) so the shader's
        /// see-through parts show what is behind the card. Graphics that are a Mask's shape stay: what they clip would vanish.
        /// </summary>
        private void HideCardFront()
        {
            _hiddenDone = true;
            foreach (var g in Ui.m_CardFront.GetComponentsInChildren<Graphic>(false))
            {
                if (g == null || !g.enabled || g == _image || g == _rim || g == _overlayImage || g.GetComponent<Mask>() != null) continue;
                if (g.transform.IsChildOf(_image.transform)) continue;
                g.enabled = false;
                _hidden.Add(g);
            }
        }

        private static void Show(List<Graphic> hidden)
        {
            foreach (var g in hidden) if (g != null) g.enabled = true;
            hidden.Clear();
        }

        // ---- the border rim (stock holos)

        /// <summary>The shader on the border rim: an Image inside CardBorderMask (clipped to the rim) showing the border sheet.</summary>
        private bool UpdateRim(Material mat)
        {
            var mask = Ui.m_CardBorderMask;
            var border = Ui.m_CardBorderImage;
            if (_framed || _draws != null || !CardShaders.IsStock(_id) || !Plugin.HoloFoilRim.Value || mask == null || border == null || !mask.gameObject.activeInHierarchy)
            {
                HideRim();
                return false;
            }
            if (_rim == null || _rim.transform.parent != mask.transform)
            {
                if (_rim != null) Destroy(_rim.gameObject);
                _rim = Create(RimName, mask.transform);
                var rt = _rim.rectTransform;
                var like = border.rectTransform;
                rt.anchorMin = like.anchorMin;
                rt.anchorMax = like.anchorMax;
                rt.pivot = like.pivot;
                rt.anchoredPosition = like.anchoredPosition;
                rt.sizeDelta = like.sizeDelta;
                rt.localRotation = like.localRotation;
                rt.localScale = like.localScale;
            }
            if (_rim.sprite != border.sprite) _rim.sprite = border.sprite;
            if (_rim.transform.GetSiblingIndex() != mask.transform.childCount - 1) _rim.transform.SetAsLastSibling();
            _rim.GetComponent<CardShaderMesh>().Tell(_card, _facts);
            Show(_rim, mat);
            return true;
        }

        private void HideRim()
        {
            if (_rim != null && _rim.gameObject.activeSelf) _rim.gameObject.SetActive(false);
        }

        /// <summary>Keep the game's foil layers off on this CardUI while our rim replaces them (runs after the game toggles them).</summary>
        private void Suppress()
        {
            SuppressList(Ui.m_FoilShowList);
            SuppressList(Ui.m_FoilBlendedShowList);
        }

        private void SuppressList(List<Image> list)
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
    }

    /// <summary>
    /// Gives the shader its coordinates and what it knows about the card, in the Image's vertices: uv0 = the place on the card
    /// (0..1 across the picture, whatever part of its texture the sprite is), uv1 = where the picture is in its texture
    /// (corner, size), for looking the art up; uv2 = (random number of this copy, random number of the card, the shader clock
    /// when it appeared, the shader clock when the pointer went onto it or 0); uv3 = (border, foil, rarity's place, grade).
    /// </summary>
    internal sealed class CardShaderMesh : BaseMeshEffect
    {
        private Vector4 _card, _facts;

        /// <summary>What the shader is told about the card; the mesh is only rebuilt when it changes.</summary>
        public void Tell(Vector4 card, Vector4 facts)
        {
            if (card == _card && facts == _facts) return;
            _card = card;
            _facts = facts;
            if (graphic != null) graphic.SetVerticesDirty();
        }

        public override void ModifyMesh(VertexHelper vh)
        {
            var image = graphic as Image;
            if (!IsActive() || image == null || image.sprite == null) return;
            Vector4 outer = UnityEngine.Sprites.DataUtility.GetOuterUV(image.sprite); // xMin, yMin, xMax, yMax
            float w = Mathf.Max(1e-6f, outer.z - outer.x), h = Mathf.Max(1e-6f, outer.w - outer.y);
            var art = new Vector4(outer.x, outer.y, w, h);
            var v = new UIVertex();
            for (int i = 0; i < vh.currentVertCount; i++)
            {
                vh.PopulateUIVertex(ref v, i);
                v.uv0 = new Vector4((v.uv0.x - outer.x) / w, (v.uv0.y - outer.y) / h, 0f, 0f);
                v.uv1 = art;
                v.uv2 = _card;
                v.uv3 = _facts;
                vh.SetUIVertex(v, i);
            }
        }
    }
}
