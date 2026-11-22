using System;
using System.Collections.Generic;
using System.IO;
using Newtonsoft.Json;
using TCGCustomCards.Core;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Card borders (set.json "borders" slots + card "versionBorders" → accessory library "borders", or the ones that come with
    /// the mod): which border a card version is drawn with, its pictures and its shader layers' materials. The single owner of
    /// that choice; Patches/CardBorderView draws it. Full-image sets only ([Visuals] CustomBorders).
    /// </summary>
    internal static class CardBorders
    {
        private static Dictionary<string, BorderDef> _defs;
        private static Dictionary<string, BorderDef> _builtIn;
        private static readonly HashSet<string> Reported = new HashSet<string>();
        private static readonly Dictionary<string, Sprite> BuiltInSprites = new Dictionary<string, Sprite>();

        /// <summary>The ids of borders that come with the mod start with this (TCG Studio lists them under "Comes with the mod").</summary>
        public const string StockPrefix = "tcgcc-border-";

        /// <summary>The border a card's version is drawn with, or null for the game's rim.</summary>
        public static BorderDef For(CustomSet set, int pos, ECardBorderType border, bool foil)
        {
            if (set == null || pos < 0 || !Plugin.CustomBorders.Value || set.Def.RenderMode != Core.RenderMode.FullImage) return null;
            string id = set.Def.BorderOf(pos, (int)border + (foil ? CardStore.BordersPerCard : 0));
            if (string.IsNullOrEmpty(id)) return null;
            var def = Find(id);
            if (def == null && Reported.Add(id))
                Plugin.Log.LogWarning($"Card border '{id}' (used by {set.Def.Label}) is not in this setup's borders: those cards keep the game's border");
            return def;
        }

        public static BorderDef Find(string id)
        {
            if (string.IsNullOrEmpty(id)) return null;
            BorderDef def;
            if (id.StartsWith(StockPrefix, StringComparison.Ordinal))
            {
                LoadBuiltIn();
                _builtIn.TryGetValue(id, out def);
                return def;
            }
            if (_defs == null) _defs = AccessoryLoader.LoadBorders(StudioPaths.Setup);
            _defs.TryGetValue(id, out def);
            return def;
        }

        /// <summary>An image layer's picture, or null when missing.</summary>
        public static Sprite Picture(BorderDef def, BorderLayerDef layer)
        {
            if (def == null || layer == null || string.IsNullOrEmpty(layer.File)) return null;
            if (!def.BuiltIn) return ImageCache.Get(StudioPaths.ItemFile(def.FolderPath, layer.File));
            if (BuiltInSprites.TryGetValue(layer.File, out var s)) return s;
            s = null;
            try
            {
                using (var stream = typeof(CardBorders).Assembly.GetManifestResourceStream("TCGCustomCards.stockborder." + layer.File))
                using (var m = new MemoryStream())
                {
                    if (stream == null) throw new Exception("not built into this mod");
                    stream.CopyTo(m);
                    var tex = new Texture2D(2, 2, TextureFormat.RGBA32, true)
                    {
                        name = Path.GetFileNameWithoutExtension(layer.File), wrapMode = TextureWrapMode.Clamp,
                        filterMode = FilterMode.Trilinear, anisoLevel = 4
                    };
                    if (!tex.LoadImage(m.ToArray(), markNonReadable: true)) throw new Exception("could not be decoded");
                    s = Sprite.Create(tex, new Rect(0, 0, tex.width, tex.height), new Vector2(0.5f, 0.5f), 100f, 0, SpriteMeshType.FullRect);
                    s.name = tex.name;
                }
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Card border {def.Label}: picture {layer.File}: {e.Message}");
            }
            return BuiltInSprites[layer.File] = s;
        }

        /// <summary>
        /// The material of a shader layer: the shader's, or a copy with the layer's own values. No mask picture (cardMask = 1):
        /// the layer's shape comes from the clip to the picture under it. Null when the shader is unknown or can't be loaded.
        /// </summary>
        public static Material Material(BorderDef def, int index)
        {
            var layer = def.Layers[index];
            return CardShaders.Own(layer.Shader, $"card border {def.Label}", $"border|{def.Id}|{index}", $"{def.Id}_{index}", null, layer.Params);
        }

        private static void LoadBuiltIn()
        {
            if (_builtIn != null) return;
            _builtIn = new Dictionary<string, BorderDef>();
            try
            {
                using (var s = typeof(CardBorders).Assembly.GetManifestResourceStream("TCGCustomCards.stock-borders.json"))
                {
                    if (s == null) return; // this build has none
                    using (var r = new StreamReader(s))
                        foreach (var def in JsonConvert.DeserializeObject<List<BorderDef>>(r.ReadToEnd()))
                        {
                            if (def?.Id == null) continue;
                            def.BuiltIn = true;
                            if (def.Layers == null) def.Layers = new List<BorderLayerDef>();
                            _builtIn[def.Id] = def;
                        }
                }
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Card borders: the built-in list could not be read ({e.Message})");
            }
        }
    }
}
