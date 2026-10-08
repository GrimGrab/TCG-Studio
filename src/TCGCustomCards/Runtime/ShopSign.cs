using System.Collections.Generic;
using System.IO;
using HarmonyLib;
using TMPro;
using UnityEngine;
using UnityEngine.TextCore.LowLevel;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// [Visuals] ShopSignImage / ShowShopNameOnSign: the billboard above the shop entrance. The sign is one mesh
    /// (<c>Level_Environment_Grp/ShopGrp/Billboard</c>, mesh <c>BillboardMesh</c>) with material <c>Billboard</c>, whose 2048²
    /// texture is both _MainTex and _EmissionMap (it glows at night). The shop name is a separate world-space text
    /// (<c>LightManager.m_BillboardText</c>) drawn over it. We put the player's banner image into a copy of the vanilla texture
    /// (MaterialPropertyBlock, so clearing the setting restores vanilla) and keep the frame.
    /// </summary>
    internal static class ShopSign
    {
        // Measured from BillboardMesh's UVs (game files, 2026-10-08), UV space (bottom-up). The front face is split at its middle:
        // the banner's left half reads from the top strip, its right half from the bottom strip, both left→right and upright.
        // The rest of the texture (u > 0.82) is the frame and edges.
        private static readonly Rect LeftHalfUv = Rect.MinMaxRect(0.0027f, 0.505f, 0.823f, 0.9937f);
        private static readonly Rect RightHalfUv = Rect.MinMaxRect(0.0084f, 0.0051f, 0.812f, 0.4934f);
        /// <summary>World width ÷ height of the sign face (4.645 × 1.2 mesh units, scaled 1.348 × 1.270 in the scene).</summary>
        internal const float FaceAspect = 4.11f;
        /// <summary>Pixels painted past each strip so mipmaps/bilinear filtering don't pull in vanilla art at the edges.</summary>
        private const int Bleed = 3;
        internal const string Folder = "ShopSign";

        private static LightManager _lights;
        private static readonly List<Renderer> Renderers = new List<Renderer>();
        private static Texture2D _texture;
        private static string _textureKey; // image path + crop the texture was built from
        private static MaterialPropertyBlock _block;
        private static bool _hidName;

        [HarmonyPatch(typeof(LightManager), "Awake")]
        private static class LightManagerAwakePatch
        {
            private static void Postfix(LightManager __instance)
            {
                _lights = __instance;
                Renderers.Clear();
                foreach (var r in Object.FindObjectsOfType<MeshRenderer>())
                {
                    var mat = r.sharedMaterial;
                    var mesh = r.GetComponent<MeshFilter>()?.sharedMesh;
                    if (mat != null && mat.name == "Billboard" && mesh != null && mesh.name == "BillboardMesh") Renderers.Add(r);
                }
                if (Renderers.Count == 0) Plugin.Log.LogWarning("Shop sign: billboard renderer not found; ShopSignImage has no effect");
                _hidName = false;
                CaptureVanillaText(__instance.m_BillboardText);
                Apply();
            }
        }

        internal static void Init()
        {
            Plugin.ShopSignImage.SettingChanged += (_, __) => Apply();
            Plugin.ShopSignCrop.SettingChanged += (_, __) => Apply();
            Plugin.ShowShopNameOnSign.SettingChanged += (_, __) => Apply();
            Plugin.ShopSignFont.SettingChanged += (_, __) => Apply();
            Plugin.ShopSignTextColor.SettingChanged += (_, __) => Apply();
            Plugin.ShopSignTextSize.SettingChanged += (_, __) => Apply();
            Plugin.ShopSignOutline.SettingChanged += (_, __) => Apply();
            Plugin.ShopSignOutlineColor.SettingChanged += (_, __) => Apply();
        }

        /// <summary>Applies the shop sign settings to the current shop scene (no-op before it loads).</summary>
        internal static void Apply()
        {
            if (_lights == null) return;
            var text = _lights.m_BillboardText;
            bool show = Plugin.ShowShopNameOnSign.Value;
            if (text != null && (!show || _hidName))
            {
                text.gameObject.SetActive(show);
                _hidName = !show;
            }
            if (text != null) ApplyText(text);

            var tex = TextureFor(ImagePath(Plugin.ShopSignImage.Value), Plugin.ShopSignCrop.Value?.Trim() ?? "");
            foreach (var r in Renderers)
            {
                if (r == null) continue;
                if (tex == null) { r.SetPropertyBlock(null); continue; }
                _block ??= new MaterialPropertyBlock();
                _block.Clear();
                var mat = r.sharedMaterial;
                foreach (var prop in mat.GetTexturePropertyNames())
                    if (mat.GetTexture(prop) == mat.mainTexture) _block.SetTexture(prop, tex); // _MainTex and _EmissionMap
                r.SetPropertyBlock(_block);
            }
        }

        /// <summary>The setting holds a file name in &lt;plugin&gt;\ShopSign (TCG Studio copies the chosen image there) or a full path.</summary>
        private static string ImagePath(string value)
        {
            value = value?.Trim().Trim('"');
            if (string.IsNullOrEmpty(value)) return null;
            return Path.IsPathRooted(value) ? value : Path.Combine(Plugin.PluginDir, Folder, value);
        }

        private static Texture2D TextureFor(string path, string crop)
        {
            string key = path + "|" + crop;
            if (key == _textureKey) return _texture;
            if (_texture != null) Object.Destroy(_texture);
            _texture = null;
            _textureKey = key;
            if (path == null) return null;
            if (!File.Exists(path))
            {
                Plugin.Log.LogWarning($"Shop sign image not found: {path}");
                return null;
            }
            var vanilla = Renderers.Count > 0 && Renderers[0] != null ? Renderers[0].sharedMaterial.mainTexture : null;
            if (vanilla == null) return null;
            try { _texture = Compose(path, crop, vanilla); }
            catch (System.Exception e) { Plugin.Log.LogWarning($"Shop sign image {path} failed: {e.Message}"); }
            if (_texture != null) Plugin.Log.LogInfo($"Shop sign: using {path}");
            return _texture;
        }

        /// <summary>Writes the cropped banner's two halves into a copy of the vanilla texture.</summary>
        private static Texture2D Compose(string path, string cropSetting, Texture vanilla)
        {
            var src = new Texture2D(2, 2, TextureFormat.RGBA32, false);
            if (!src.LoadImage(File.ReadAllBytes(path))) { Object.Destroy(src); throw new IOException("not a PNG/JPG image"); }
            var baseTex = TextureReadback.Read(vanilla, new RectInt(0, 0, vanilla.width, vanilla.height));
            if (baseTex == null) { Object.Destroy(src); return null; }

            var crop = ParseCrop(cropSetting) ?? CentreCrop((float)src.width / src.height);
            int w = baseTex.width, h = baseTex.height;
            var px = baseTex.GetPixels32();
            Paint(px, w, h, src, crop, LeftHalfUv, 0f);
            Paint(px, w, h, src, crop, RightHalfUv, 0.5f);

            var dst = new Texture2D(w, h, TextureFormat.RGBA32, true) { name = "TCGCC_ShopSign", wrapMode = TextureWrapMode.Clamp };
            dst.SetPixels32(px);
            dst.Apply(updateMipmaps: true, makeNoLongerReadable: true);
            dst.hideFlags = HideFlags.DontUnloadUnusedAsset;
            Object.Destroy(src);
            Object.Destroy(baseTex);
            return dst;
        }

        /// <summary>The largest FaceAspect area from the middle of an image of aspect <paramref name="have"/> (UV).</summary>
        private static Rect CentreCrop(float have) => have > FaceAspect
            ? new Rect((1 - FaceAspect / have) / 2, 0, FaceAspect / have, 1)
            : new Rect(0, (1 - have / FaceAspect) / 2, 1, have / FaceAspect);

        /// <summary>ShopSignCrop "x,y,w,h" (fractions, top-left origin; Studio's crop box keeps it at FaceAspect) → UV rect
        /// (bottom-up). Null when empty or invalid.</summary>
        private static Rect? ParseCrop(string s)
        {
            if (string.IsNullOrEmpty(s)) return null;
            var parts = s.Split(',');
            if (parts.Length != 4) return Invalid(s);
            var f = new float[4];
            for (int i = 0; i < 4; i++)
                if (!float.TryParse(parts[i].Trim(), System.Globalization.NumberStyles.Float, System.Globalization.CultureInfo.InvariantCulture, out f[i]))
                    return Invalid(s);
            float x = Mathf.Clamp01(f[0]), y = Mathf.Clamp01(f[1]), w = Mathf.Clamp(f[2], 0, 1 - x), h = Mathf.Clamp(f[3], 0, 1 - y);
            if (w < 0.01f || h < 0.01f) return Invalid(s);
            return new Rect(x, 1 - y - h, w, h);
        }

        private static Rect? Invalid(string s)
        {
            Plugin.Log.LogWarning($"ShopSignCrop \"{s}\" isn't x,y,width,height (fractions 0-1); using the middle of the image");
            return null;
        }

        private static void Paint(Color32[] px, int w, int h, Texture2D src, Rect crop, Rect uv, float half)
        {
            float x0 = uv.xMin * w, x1 = uv.xMax * w, y0 = uv.yMin * h, y1 = uv.yMax * h;
            int ix0 = Mathf.Max(0, Mathf.FloorToInt(x0) - Bleed), ix1 = Mathf.Min(w, Mathf.CeilToInt(x1) + Bleed);
            int iy0 = Mathf.Max(0, Mathf.FloorToInt(y0) - Bleed), iy1 = Mathf.Min(h, Mathf.CeilToInt(y1) + Bleed);
            for (int y = iy0; y < iy1; y++)
            {
                float fy = Mathf.Clamp01((y + 0.5f - y0) / (y1 - y0));
                float v = crop.y + crop.height * fy;
                for (int x = ix0; x < ix1; x++)
                {
                    float fx = Mathf.Clamp01((x + 0.5f - x0) / (x1 - x0));
                    float u = crop.x + crop.width * (half + fx * 0.5f);
                    Color c = src.GetPixelBilinear(u, v);
                    Color b = px[y * w + x];
                    px[y * w + x] = Color.Lerp(b, new Color(c.r, c.g, c.b, 1f), c.a); // transparent parts keep the vanilla art
                }
            }
        }

        // ---------------------------------------------------------------- the shop name text
        // [Visuals - Shop sign text]: font, colour, size and outline of LightManager.m_BillboardText (TextMeshProUGUI, Fredoka One SDF,
        // gold with a yellow→white vertex gradient, auto size 18–75 in a 671.82 × 130 rect). LightManager.Awake stores its colour in
        // m_BillboardTextOriginalColor and EvaluateWorldUIBrightness sets color = that × brightness, so a custom colour goes there.

        private static TextMeshProUGUI _text; // the text the vanilla values below belong to
        private static TMP_FontAsset _vanillaFont;
        private static Material _vanillaMaterial;
        private static Color _vanillaColor;
        private static bool _vanillaGradient;
        private static float _vanillaSize, _vanillaSizeMin, _vanillaSizeMax;
        private static Vector2 _vanillaRect;
        private static TMP_FontAsset _font; // loaded from ShopSignFont
        private static string _fontKey;
        private static Material _outlineMaterial;
        private static readonly int OutlineWidthId = Shader.PropertyToID("_OutlineWidth");
        private static readonly int OutlineColorId = Shader.PropertyToID("_OutlineColor");
        private static readonly int GradientScaleId = Shader.PropertyToID("_GradientScale");

        private static void CaptureVanillaText(TextMeshProUGUI text)
        {
            _text = text;
            if (text == null) return;
            _vanillaFont = text.font;
            _vanillaMaterial = text.fontSharedMaterial;
            _vanillaColor = text.color; // what LightManager.Awake just stored as m_BillboardTextOriginalColor
            _vanillaGradient = text.enableVertexGradient;
            _vanillaSize = text.fontSize;
            _vanillaSizeMin = text.fontSizeMin;
            _vanillaSizeMax = text.fontSizeMax;
            _vanillaRect = text.rectTransform.sizeDelta;
        }

        private static void ApplyText(TextMeshProUGUI text)
        {
            if (text != _text || _vanillaFont == null) return;
            var font = FontFor(ImagePath(Plugin.ShopSignFont.Value)) ?? _vanillaFont;
            if (text.font != font) text.font = font;
            var baseMaterial = font == _vanillaFont ? _vanillaMaterial : font.material;

            // Outline: a copy of the font's material. The setting is a fraction of the font size; TMP's _OutlineWidth is a fraction of
            // the SDF range (±_GradientScale atlas pixels at the font's sampling size), half inside and half outside the letter edge.
            float outline = Plugin.ShopSignOutline.Value;
            if (_outlineMaterial != null) Object.Destroy(_outlineMaterial);
            _outlineMaterial = null;
            if (outline > 0f && baseMaterial.HasProperty(OutlineWidthId) && baseMaterial.HasProperty(GradientScaleId))
            {
                _outlineMaterial = new Material(baseMaterial) { name = baseMaterial.name + " (TCGCC outline)" };
                float range = 2f * baseMaterial.GetFloat(GradientScaleId) / font.faceInfo.pointSize; // SDF range in ems
                _outlineMaterial.SetFloat(OutlineWidthId, Mathf.Clamp01(outline / range));
                _outlineMaterial.SetColor(OutlineColorId, ParseColor(Plugin.ShopSignOutlineColor.Value, "ShopSignOutlineColor") ?? Color.black);
                _outlineMaterial.EnableKeyword("OUTLINE_ON");
                ShaderUtilities.UpdateShaderRatios(_outlineMaterial);
            }
            text.fontSharedMaterial = _outlineMaterial != null ? _outlineMaterial : baseMaterial;

            // Size: the auto-size range scales; the rect grows in height only so big letters aren't shrunk back (width = the sign's).
            float scale = Plugin.ShopSignTextSize.Value;
            text.fontSizeMin = _vanillaSizeMin * scale;
            text.fontSizeMax = _vanillaSizeMax * scale;
            text.fontSize = _vanillaSize * scale;
            text.rectTransform.sizeDelta = new Vector2(_vanillaRect.x, _vanillaRect.y * Mathf.Max(1f, scale));

            // Colour: replaces the gold and its gradient; LightManager keeps dimming it at night.
            var custom = ParseColor(Plugin.ShopSignTextColor.Value, "ShopSignTextColor");
            var color = custom ?? _vanillaColor;
            text.enableVertexGradient = custom == null && _vanillaGradient;
            if (_lights != null)
            {
                var t = Traverse.Create(_lights);
                t.Field("m_BillboardTextOriginalColor").SetValue(color);
                var lit = color * t.Field<float>("m_GlobalBrightness").Value; // as EvaluateWorldUIBrightness does
                lit.a = 1f;
                text.color = lit;
            }
            text.SetAllDirty();
        }

        /// <summary>A TMP font asset built from the font file (dynamic atlas), cached per path; null = use the game's font.</summary>
        private static TMP_FontAsset FontFor(string path)
        {
            if (path == _fontKey) return _font;
            var old = _font;
            _font = null;
            _fontKey = path;
            if (old != null && _text != null && _text.font == old) _text.font = _vanillaFont;
            DestroyFont(old);
            if (path == null) return null;
            if (!File.Exists(path))
            {
                Plugin.Log.LogWarning($"Shop sign font not found: {path}");
                return null;
            }
            Font source = null;
            try
            {
                source = new Font(path); // FreeType opens the file
                // Padding 18 at 90 pt leaves room for ShopSignOutline up to ~0.4 of the font size.
                var asset = TMP_FontAsset.CreateFontAsset(source, 90, 18, GlyphRenderMode.SDFAA, 1024, 1024, AtlasPopulationMode.Dynamic, true);
                if (asset == null || !asset.TryAddCharacters("Aa0", out _))
                {
                    Plugin.Log.LogWarning($"Shop sign font {path} couldn't be loaded (not a TrueType/OpenType font?); using the game's font");
                    if (asset != null) DestroyFont(asset);
                    else Object.Destroy(source);
                    return null;
                }
                asset.name = "TCGCC_ShopSign_" + Path.GetFileNameWithoutExtension(path);
                asset.hideFlags = HideFlags.DontUnloadUnusedAsset;
                if (_vanillaFont != null) asset.fallbackFontAssetTable = new List<TMP_FontAsset> { _vanillaFont }; // letters the font lacks
                _font = asset;
                Plugin.Log.LogInfo($"Shop sign: font {path}");
            }
            catch (System.Exception e)
            {
                Plugin.Log.LogWarning($"Shop sign font {path} failed: {e.Message}; using the game's font");
                if (source != null) Object.Destroy(source);
            }
            return _font;
        }

        private static void DestroyFont(TMP_FontAsset asset)
        {
            if (asset == null) return;
            if (asset.atlasTextures != null)
                foreach (var tex in asset.atlasTextures)
                    if (tex != null) Object.Destroy(tex);
            if (asset.material != null) Object.Destroy(asset.material);
            if (asset.sourceFontFile != null) Object.Destroy(asset.sourceFontFile);
            Object.Destroy(asset);
        }

        private static Color? ParseColor(string value, string setting)
        {
            value = value?.Trim();
            if (string.IsNullOrEmpty(value)) return null;
            if (!value.StartsWith("#")) value = "#" + value;
            if (ColorUtility.TryParseHtmlString(value, out var c)) return c;
            Plugin.Log.LogWarning($"{setting} \"{value}\" isn't a #RRGGBB colour; ignored");
            return null;
        }
    }
}
