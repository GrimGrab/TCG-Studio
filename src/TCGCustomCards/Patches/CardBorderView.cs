using System.Collections.Generic;
using TCGCustomCards.Core;
using TCGCustomCards.Runtime;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Patches
{
    /// <summary>
    /// A custom border on a full-image card (Runtime/CardBorders decides which). Called by SetCardUIPatch, the owner of the
    /// full-image picture, right after it placed the picture:
    ///   - the game's border rim (m_CardBorderMask) is switched off for this card (SetCardUI switches it on again for the next);
    ///   - a group "TCGCC_Border" right after the picture holds the border's layers, back to front, at the card's size (the
    ///     900×900 card front fitted to the shape of the border's first picture, scaled to the vanilla card's height
    ///     (<see cref="CardScale"/>);
    ///   - the picture covers the whole card (cropped to its shape), or is resized into the box the user set for the
    ///     border (window); the border's layers lie on top and their transparency decides what of it shows;
    ///   - an image layer is an Image; a shader layer is an Image with the shader's material showing the image layer under it
    ///     (the card picture when there is none). Clipped shader layers are children of that image layer, which is then a
    ///     UI Mask (the template card shader has the stencil properties for it).
    /// The group is hidden at the start of every SetCardUI (the CardUI may show a vanilla card next). Far-culled cards keep
    /// the border's pictures but stop its shaders, like the game drops its foil there.
    /// </summary>
    internal sealed class CardBorderView : MonoBehaviour
    {
        private const string GroupName = "TCGCC_Border";
        // The vanilla card in its 900² card front, read from the game files 2026-11-22 (the solid part of CardBGMask's
        // CardFullMask, x 379–1652 / y 133–1913 of 2048, over a 902.7 square). TCG Studio measures the same in every read of
        // the game files and its TestEditionLabel fails when these two literals no longer match it (keep them plain literals).
        /// <summary>The vanilla card's height ÷ the front's (785 of 900). A border card is that tall, like every other card.</summary>
        private const float CardScale = 0.8722f;
        /// <summary>
        /// The vanilla card's centre is a little left of the front's (pixel centre 1016 of 2048, y centred), as a fraction of
        /// the front's width. Border cards and full-image pictures sit there (else the game's card showed at a left edge).
        /// </summary>
        internal const float CardCentreX = -0.00392f;
        /// <summary>The shape of a card without a border picture to go by (MTG, 63×88 mm).</summary>
        private const float DefaultAspect = 63f / 88f;

        public CardUI Ui;
        private RectTransform _group;
        private string _builtFor;
        private readonly List<Image> _shaders = new List<Image>();
        private readonly List<Material> _materials = new List<Material>();
        /// <summary>Per shader layer: it shows the card picture (no image layer under it).</summary>
        private readonly List<bool> _onCard = new List<bool>();
        private BorderDef _def;

        /// <summary>The border shown right now on this CardUI (null = none).</summary>
        public static BorderDef On(CardUI ui) =>
            ui != null && ui.TryGetComponent<CardBorderView>(out var v) && v._def != null && v._group != null && v._group.gameObject.activeSelf ? v._def : null;

        /// <summary>SetCardUI is about to run: no border until the postfix says otherwise.</summary>
        public static void BeforeCardSet(CardUI ui)
        {
            if (ui != null && ui.TryGetComponent<CardBorderView>(out var v)) v.Hide();
        }

        /// <summary>
        /// Draws the card's border, if its version has one, and places the picture for it. Returns false when there is none
        /// (the picture keeps the game's placement).
        /// </summary>
        public static bool Apply(CardUI ui, Image picture, CustomSet set, int pos, CardData data)
        {
            var def = CardBorders.For(set, pos, data.borderType, data.isFoil);
            if (def == null)
            {
                BeforeCardSet(ui);
                return false;
            }
            if (!ui.TryGetComponent<CardBorderView>(out var view))
            {
                view = ui.gameObject.AddComponent<CardBorderView>();
                view.Ui = ui;
            }
            return view.Show(def, picture, set, pos, data);
        }

        private bool Show(BorderDef def, Image picture, CustomSet set, int pos, CardData data)
        {
            var front = Ui.m_CardFront != null ? (RectTransform)Ui.m_CardFront.transform : null;
            if (front == null || picture == null) return false;
            Vector2 area = front.rect.size;
            if (area.x <= 1f || area.y <= 1f) return false;

            var first = FirstPicture(def);
            float aspect = first != null ? first.rect.width / Mathf.Max(1f, first.rect.height) : DefaultAspect;
            Vector2 card = area.x / area.y > aspect ? new Vector2(area.y * aspect, area.y) : new Vector2(area.x, area.x / aspect);

            if (_group == null) _group = CreateRect(GroupName, front);
            if (_builtFor != def.Id) Build(def);
            _def = def;
            _group.anchorMin = _group.anchorMax = _group.pivot = new Vector2(0.5f, 0.5f);
            var shift = new Vector2(CardCentreX * area.x, 0f);
            _group.anchoredPosition = shift;
            _group.sizeDelta = card;
            _group.localScale = Vector3.one * CardScale;
            _group.SetSiblingIndex(picture.transform.GetSiblingIndex() + 1);

            PlacePicture(def, picture, card, shift);
            Tell(set, pos, data, picture.sprite);

            // The game's rim would show around a picture in a window; the border is the rim now.
            if (Ui.m_CardBorderMask != null && Ui.m_CardBorderMask.gameObject.activeSelf) Ui.m_CardBorderMask.gameObject.SetActive(false);
            _group.gameObject.SetActive(true);
            return true;
        }

        /// <summary>
        /// The picture as it would cover the whole card (cropped from its middle to the card's shape), then resized into the
        /// border's window: a box only scales it (stretched to the box's shape), it never shows another part of the picture.
        /// </summary>
        private static void PlacePicture(BorderDef def, Image picture, Vector2 card, Vector2 shift)
        {
            var w = def.Window;
            Vector2 size = w != null ? new Vector2(Mathf.Clamp01(w[2]) * card.x, Mathf.Clamp01(w[3]) * card.y) : card;
            Vector2 centre = w != null ? new Vector2((w[0] + w[2] / 2f - 0.5f) * card.x, (0.5f - (w[1] + w[3] / 2f)) * card.y) : Vector2.zero;
            if (size.x <= 1f || size.y <= 1f) return;
            picture.sprite = PictureCrop.Fill(picture.sprite, card.x / card.y);
            var rt = picture.rectTransform;
            picture.preserveAspect = false;
            rt.anchorMin = rt.anchorMax = rt.pivot = new Vector2(0.5f, 0.5f);
            rt.anchoredPosition = centre * CardScale + shift;
            rt.sizeDelta = size;
            rt.localScale = Vector3.one * CardScale;
        }

        private static Sprite FirstPicture(BorderDef def)
        {
            foreach (var l in def.Layers)
                if (l.Kind == BorderDef.ImageLayer)
                {
                    var s = CardBorders.Picture(def, l);
                    if (s != null) return s;
                }
            return null;
        }

        /// <summary>The border's layers as Images under the group (once per border shown on this CardUI).</summary>
        private void Build(BorderDef def)
        {
            for (int i = _group.childCount - 1; i >= 0; i--) Destroy(_group.GetChild(i).gameObject);
            _shaders.Clear();
            _materials.Clear();
            _onCard.Clear();
            _builtFor = def.Id;
            Image under = null;
            for (int i = 0; i < def.Layers.Count; i++)
            {
                var layer = def.Layers[i];
                if (layer.Kind == BorderDef.ImageLayer)
                {
                    var sprite = CardBorders.Picture(def, layer);
                    if (sprite == null) continue;
                    var img = CreateRect($"Layer{i}", _group).gameObject.AddComponent<Image>();
                    Stretch(img.rectTransform);
                    img.sprite = sprite;
                    img.preserveAspect = false;
                    img.raycastTarget = false;
                    img.color = new Color(1f, 1f, 1f, Mathf.Clamp01(layer.Opacity));
                    under = img;
                    continue;
                }
                var mat = CardBorders.Material(def, i);
                if (mat == null) continue; // reported by CardShaders
                bool clip = layer.Clip && under != null;
                if (clip && under.GetComponent<Mask>() == null) under.gameObject.AddComponent<Mask>().showMaskGraphic = true;
                var shader = CreateRect($"Layer{i}", clip ? under.rectTransform : _group).gameObject.AddComponent<Image>();
                shader.gameObject.AddComponent<CardShaderMesh>();
                Stretch(shader.rectTransform);
                shader.sprite = under != null ? under.sprite : null; // null = the card picture, set in Tell
                shader.preserveAspect = false;
                shader.raycastTarget = false;
                shader.color = new Color(1f, 1f, 1f, Mathf.Clamp01(layer.Opacity));
                _shaders.Add(shader);
                _materials.Add(mat);
                _onCard.Add(under == null);
            }
            Plugin.Log.LogInfo($"Card border {def.Label} drawn ({def.Layers.Count} layer(s), {_shaders.Count} shader(s))");
        }

        // What the border's shaders know about the card (the same as a card shader is told, see CardShaderMesh).
        private string _shownKey;
        private Vector4 _card, _facts;

        private void Tell(CustomSet set, int pos, CardData data, Sprite cardPicture)
        {
            string key = $"{(int)data.monsterType}|{(int)data.borderType}|{data.isFoil}|{data.cardGrade}";
            if (key != _shownKey)
            {
                _shownKey = key;
                _card = new Vector4(Random.value, CardShaderLayer.CardNumber(set.Def.Id, set.Card(pos).Id), CardShaderLayer.ShaderTime, 0f);
            }
            _facts = new Vector4((int)data.borderType, data.isFoil ? 1f : 0f, set.Rarity(pos).Rank, Mathf.Max(0, data.cardGrade));
            for (int i = 0; i < _shaders.Count; i++)
            {
                if (_shaders[i] == null) continue;
                if (_onCard[i] && _shaders[i].sprite != cardPicture) _shaders[i].sprite = cardPicture;
                _shaders[i].GetComponent<CardShaderMesh>().Tell(_card, _facts);
            }
        }

        private void OnEnable() => _shownKey = null;

        private void LateUpdate()
        {
            if (_def == null || _group == null || !_group.gameObject.activeSelf) return;
            bool run = Plugin.CardShaders.Value && !Ui.m_IsFarDistanceCulled;
            for (int i = 0; i < _shaders.Count; i++)
            {
                var s = _shaders[i];
                if (s == null) continue;
                if (run && s.sprite != null) CardShaderLayer.Show(s, _materials[i]);
                else if (s.enabled) s.enabled = false;
            }
        }

        private void Hide()
        {
            _def = null;
            if (_group != null && _group.gameObject.activeSelf) _group.gameObject.SetActive(false);
        }

        private static RectTransform CreateRect(string name, Transform parent)
        {
            var go = new GameObject(name, typeof(RectTransform));
            go.layer = parent.gameObject.layer;
            var rt = (RectTransform)go.transform;
            rt.SetParent(parent, worldPositionStays: false);
            return rt;
        }

        private static void Stretch(RectTransform rt)
        {
            rt.anchorMin = Vector2.zero;
            rt.anchorMax = Vector2.one;
            rt.offsetMin = rt.offsetMax = Vector2.zero;
            rt.localScale = Vector3.one;
        }
    }

    /// <summary>
    /// A picture cropped from its middle to a shape (sprites made once per picture and shape). The picture's texture shape is
    /// what counts: a sprite trimmed of its printed border is meant to be stretched back to it (SetCardUIPatch.Fit).
    /// </summary>
    internal static class PictureCrop
    {
        private static readonly Dictionary<(Sprite, int), Sprite> Crops = new Dictionary<(Sprite, int), Sprite>();

        public static Sprite Fill(Sprite sprite, float aspect)
        {
            if (sprite == null || aspect <= 0f) return sprite;
            float shown = sprite.texture.width / (float)sprite.texture.height;
            if (Mathf.Abs(shown - aspect) < 0.005f) return sprite;
            var key = (sprite, Mathf.RoundToInt(aspect * 1000f));
            if (Crops.TryGetValue(key, out var cached) && cached != null) return cached;
            var r = sprite.rect;
            // Fraction of the sprite kept across (wider than the window) or down (taller).
            Rect crop = shown > aspect
                ? new Rect(r.x + r.width * (1f - aspect / shown) / 2f, r.y, r.width * aspect / shown, r.height)
                : new Rect(r.x, r.y + r.height * (1f - shown / aspect) / 2f, r.width, r.height * shown / aspect);
            var s = Sprite.Create(sprite.texture, crop, new Vector2(0.5f, 0.5f), sprite.pixelsPerUnit, 0, SpriteMeshType.FullRect);
            s.name = sprite.name + "_window";
            return Crops[key] = s;
        }
    }
}
