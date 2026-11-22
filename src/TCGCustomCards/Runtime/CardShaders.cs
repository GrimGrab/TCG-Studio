using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Newtonsoft.Json;
using TCGCustomCards.Core;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Card shaders: the ones written in TCG Studio (accessory library "shaders") and the stock holo foils that come with the
    /// mod (one per border; what a foil version without a shader of its own is drawn with, [Foil] HoloFoil). One material per
    /// shader. Every shader is a copy of the template TCGCC/CardShader with its own pixel program, so they all take the same
    /// inputs: the card picture, value slots _P0.._P15 and pictures _Tex0.._Tex3, filled here from the shader's parameters,
    /// and the card's mask picture _Mask. A card with a mask or values of its own (set.json "shaderMask", "shaderParams")
    /// gets a copy of the shader's material with those filled in; every other card shares the shader's one.
    /// A player's shader comes from the bundle Studio compiled for it; the stock ones from bundles built into this DLL (the
    /// holo looks share one, as they share their code and differ in their values; one with code of its own has its own). A shader that can't be loaded is reported once and its cards are
    /// drawn normally. A setup can give stock shaders values of its own (library "stockValues"): they replace the stock values.
    /// </summary>
    internal static class CardShaders
    {
        public const string ShaderName = "TCGCC/CardShader";
        private const string StockPrefix = "tcgcc-holo-";
        private const int ValueSlots = 16, ImageSlots = 4;

        private static Dictionary<string, ShaderDef> _defs;
        private static Dictionary<string, ShaderDef> _stock;
        private static readonly string[] StockByBorder = new string[6];
        /// <summary>Stock bundles' shaders by bundle name; a null entry = tried and failed.</summary>
        private static readonly Dictionary<string, Shader> StockShaders = new Dictionary<string, Shader>();
        private const string HoloBundle = "tcgcc_stock";
        /// <summary>Materials by shader id; a null entry = tried and failed (don't try or report again).</summary>
        private static readonly Dictionary<string, Material> Materials = new Dictionary<string, Material>();
        /// <summary>Materials of cards with a mask or own values, by "shader|set|card" (as many as there are such cards).</summary>
        private static readonly Dictionary<string, Material> CardMaterials = new Dictionary<string, Material>();
        private static readonly HashSet<string> Reported = new HashSet<string>();

        public static bool IsStock(string id) => id != null && id.StartsWith(StockPrefix, StringComparison.Ordinal);

        /// <summary>The stock holo of a border's foil version (null when the stock list couldn't be read).</summary>
        public static string StockFor(ECardBorderType border)
        {
            LoadStock();
            int b = (int)border;
            return b >= 0 && b < StockByBorder.Length ? StockByBorder[b] : null;
        }

        /// <summary>A shader by id (the setup's or a stock one); null when unknown.</summary>
        private static ShaderDef Find(string id)
        {
            if (string.IsNullOrEmpty(id)) return null;
            ShaderDef def;
            if (IsStock(id))
            {
                LoadStock();
                _stock.TryGetValue(id, out def);
            }
            else
            {
                if (_defs == null) _defs = AccessoryLoader.LoadShaders(StudioPaths.Setup);
                _defs.TryGetValue(id, out def);
            }
            return def;
        }

        /// <summary>What a shader draws on: null = the picture, ShaderDef.SeeThrough or ShaderDef.Over.</summary>
        public static string Draws(string id)
        {
            string d = Find(id)?.Draws;
            return d == ShaderDef.SeeThrough || d == ShaderDef.Over ? d : null;
        }

        /// <summary>
        /// The material a card is drawn with by a shader: the shader's own, or the card's copy of it when the card has a mask
        /// or values of its own. Null when the shader is unknown or couldn't be loaded.
        /// </summary>
        public static Material Get(string id, string usedBy, CustomSet set, int pos)
        {
            var shared = Get(id, usedBy);
            if (shared == null || set == null || pos < 0) return shared;
            var card = set.Card(pos);
            if (!card.HasShaderData) return shared;
            var mask = string.IsNullOrEmpty(card.ShaderMask) ? null : ImageCache.Get(set.Def.Resolve(card.ShaderMask));
            return Own(id, usedBy, id + "|" + set.Def.Id + "|" + card.Id, card.Id, mask != null ? mask.texture : null, card.ShaderParams);
        }

        /// <summary>
        /// A copy of a shader's material with a mask picture and/or own parameter values (by name) filled in, made once per
        /// key: a card with its own data, a border's shader layer. Null when the shader is unknown or couldn't be loaded.
        /// </summary>
        public static Material Own(string id, string usedBy, string key, string name, Texture mask, Dictionary<string, List<float>> values)
        {
            var shared = Get(id, usedBy);
            if (shared == null) return null;
            if (mask == null && (values == null || values.Count == 0)) return shared;
            if (CardMaterials.TryGetValue(key, out var own) && own != null) return own;
            own = new Material(shared) { name = shared.name + "_" + name, hideFlags = HideFlags.DontUnloadUnusedAsset };
            if (mask != null) own.SetTexture("_Mask", mask);
            if (values != null)
            {
                int value = 0;
                foreach (var p in Find(id)?.Params ?? new List<ShaderParamDef>())
                {
                    if (p == null || p.Kind == ShaderParamDef.Image) continue;
                    if (value >= ValueSlots) break;
                    if (p.Name != null && values.TryGetValue(p.Name, out var v) && v != null)
                        own.SetVector("_P" + value, new Vector4(At(v, 0), At(v, 1), At(v, 2), At(v, 3)));
                    value++;
                }
            }
            return CardMaterials[key] = own;
        }

        /// <summary>The material of a shader, or null when the id is unknown or it couldn't be loaded.</summary>
        public static Material Get(string id, string usedBy)
        {
            if (string.IsNullOrEmpty(id)) return null;
            if (Materials.TryGetValue(id, out var m) && (m != null || Reported.Contains(id))) return m;
            m = null;
            var def = Find(id);
            if (def == null)
                Plugin.Log.LogWarning($"Card shader '{id}' (used by {usedBy}) is not in this setup's shaders, or was never compiled: those cards are drawn normally");
            else
                m = Build(def);
            if (m == null) Reported.Add(id);
            return Materials[id] = m;
        }

        private static void LoadStock()
        {
            if (_stock != null) return;
            _stock = new Dictionary<string, ShaderDef>();
            try
            {
                var own = AccessoryLoader.LoadStockValues(StudioPaths.Setup);
                using (var s = typeof(CardShaders).Assembly.GetManifestResourceStream("TCGCustomCards.stock-shaders.json"))
                using (var r = new StreamReader(s))
                    foreach (var def in JsonConvert.DeserializeObject<List<ShaderDef>>(r.ReadToEnd()))
                    {
                        if (own.TryGetValue(def.Id, out var values) && values != null) UseOwnValues(def, values);
                        _stock[def.Id] = def;
                        if (Enum.TryParse(def.Border, out ECardBorderType border) && (int)border < StockByBorder.Length)
                            StockByBorder[(int)border] = def.Id;
                    }
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Stock card shaders: the built-in list could not be read ({e.Message}): foil cards get no holo");
            }
        }

        /// <summary>
        /// The setup's own values for a stock shader (TCG Studio's Shaders page, library "stockValues") in place of the stock
        /// ones: same parameter, same number of values, never a picture.
        /// </summary>
        private static void UseOwnValues(ShaderDef def, Dictionary<string, List<float>> values)
        {
            int used = 0;
            foreach (var p in def.Params ?? new List<ShaderParamDef>())
            {
                if (p == null || p.Kind == ShaderParamDef.Image || p.Name == null || p.Value == null) continue;
                if (values.TryGetValue(p.Name, out var v) && v != null && v.Count == p.Value.Count)
                {
                    p.Value = new List<float>(v);
                    used++;
                }
            }
            if (used > 0) Plugin.Log.LogInfo($"Stock card shader {def.Label}: {used} value(s) of this setup's own");
        }

        /// <summary>The compiled shader of a stock bundle (built into this DLL as TCGCustomCards.stockbundle.&lt;name&gt;).</summary>
        private static Shader StockShader(string name)
        {
            if (string.IsNullOrEmpty(name)) name = HoloBundle;
            if (StockShaders.TryGetValue(name, out var cached)) return cached;
            Shader shader = null;
            try
            {
                using (var s = typeof(CardShaders).Assembly.GetManifestResourceStream("TCGCustomCards.stockbundle." + name))
                {
                    if (s == null) throw new Exception($"no built-in bundle {name}");
                    using (var m = new MemoryStream())
                    {
                        s.CopyTo(m);
                        var bundle = AssetBundle.LoadFromMemory(m.ToArray());
                        shader = bundle != null ? bundle.LoadAllAssets<Shader>().FirstOrDefault(x => x != null && x.name == ShaderName) : null;
                    }
                }
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Stock card shaders ({name}): {e.Message}");
            }
            Plugin.Log.LogInfo($"Stock card shaders ({name}): {(shader == null ? "could not be loaded: foil cards using them get no holo" : $"loaded (supported={shader.isSupported})")}");
            // Studio compiles every card shader for a game that draws in linear colour (it converts on the way in and out).
            if (StockShaders.Count == 0 && QualitySettings.activeColorSpace != ColorSpace.Linear)
                Plugin.Log.LogWarning("Card shaders: the game no longer draws in linear colour, so card shaders will look too dark until TCG Studio is updated for it");
            return StockShaders[name] = shader;
        }

        private static Material Build(ShaderDef def)
        {
            Shader shader;
            if (IsStock(def.Id))
            {
                shader = StockShader(def.Bundle);
                if (shader == null || !shader.isSupported) return null; // already logged
            }
            else
            {
                string path = StudioPaths.ItemFile(def.FolderPath, def.Bundle);
                if (path == null || !File.Exists(path))
                {
                    Plugin.Log.LogWarning($"Card shader {def.Label}: its compiled file is missing ({def.Bundle}): cards using it are drawn normally");
                    return null;
                }
                var bundle = AssetBundle.LoadFromFile(path);
                shader = bundle != null ? bundle.LoadAllAssets<Shader>().FirstOrDefault(s => s != null && s.name == ShaderName) : null;
                if (shader == null || !shader.isSupported)
                {
                    Plugin.Log.LogWarning($"Card shader {def.Label}: {(bundle == null ? "its compiled file could not be loaded" : shader == null ? "its compiled file holds no card shader" : "it is not supported on this GPU")}: cards using it are drawn normally");
                    return null;
                }
            }
            var mat = new Material(shader) { name = "TCGCC_CardShader_" + def.Id, hideFlags = HideFlags.DontUnloadUnusedAsset };
            int value = 0, image = 0;
            foreach (var p in def.Params ?? new List<ShaderParamDef>())
            {
                if (p == null) continue;
                if (p.Kind == ShaderParamDef.Image)
                {
                    if (image >= ImageSlots) continue;
                    var tex = ImageCache.GetTiling(StudioPaths.ItemFile(def.FolderPath, p.ImagePath), linear: false);
                    if (tex != null) mat.SetTexture("_Tex" + image, tex);
                    image++;
                }
                else if (value < ValueSlots)
                {
                    var v = p.Value ?? new List<float>();
                    // One declared property per slot (not an array): Unity's copies of the material for cards under a UI Mask
                    // only carry declared properties over.
                    mat.SetVector("_P" + value++, new Vector4(At(v, 0), At(v, 1), At(v, 2), At(v, 3)));
                }
            }
            if (!IsStock(def.Id)) Plugin.Log.LogInfo($"Card shader {def.Label} loaded ({value} value(s), {image} picture(s))");
            return mat;
        }

        private static float At(List<float> v, int i) => i < v.Count ? v[i] : 0f;
    }
}
