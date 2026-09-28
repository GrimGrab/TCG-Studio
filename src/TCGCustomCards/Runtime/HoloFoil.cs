using System.IO;
using System.Linq;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    public enum HoloPattern { None, Sparkle, Etched, Cosmos }
    public enum HoloMask { Flat, Bright, Dark }

    /// <summary>
    /// Our own holographic foil for full-image cards: shader TCGCC/HoloFoil from the asset bundle <c>tcgcc_foil</c> next to the DLL
    /// (built by tools\build-shaders.ps1 with Unity 2021.3.38f1). One material per grade (ECardBorderType Base..FullArt) from the
    /// presets below + [Foil] config; colour ramps are generated here. If the bundle/shader is missing the feature is off.
    /// </summary>
    internal static class HoloFoil
    {
        internal sealed class Preset
        {
            public string Name;
            public Color Color;
            public float HueSpread, SatMul, Strength, Tiling, Parallax, Bands, Sparkle, PatternScale, Edge;
            public HoloMask Mask;
            public HoloPattern Pattern;
        }

        /// <summary>Indexed by ECardBorderType: Base, FirstEdition, Silver, Gold, EX, FullArt. Colour/strength/pattern defaults =
        /// the user's in-game settings (2026-09-24).</summary>
        internal static readonly Preset[] Presets =
        {
            new Preset { Name = "Dark teal",         Color = new Color(0x37 / 255f, 0x67 / 255f, 0x61 / 255f), HueSpread = 0.00f, SatMul = 0.3f, Strength = 1.50f, Tiling = 1.2f, Parallax = 1.2f, Bands = 0.7f, Sparkle = 0.2f, PatternScale = 60, Edge = 0.10f, Mask = HoloMask.Flat,   Pattern = HoloPattern.None },
            new Preset { Name = "Cyan",          Color = new Color(0x00 / 255f, 0xF3 / 255f, 0xFF / 255f), HueSpread = 0.50f, SatMul = 0.9f, Strength = 1.50f, Tiling = 1.4f, Parallax = 1.6f, Bands = 0.5f, Sparkle = 0.4f, PatternScale = 70, Edge = 0.20f, Mask = HoloMask.Bright, Pattern = HoloPattern.None },
            new Preset { Name = "Brushed silver", Color = new Color(0xC7 / 255f, 0xD9 / 255f, 0xF7 / 255f, 0x9E / 255f), HueSpread = 0.04f, SatMul = 0.5f, Strength = 0.29f, Tiling = 1.0f, Parallax = 1.8f, Bands = 0.8f, Sparkle = 0.3f, PatternScale = 90, Edge = 0.25f, Mask = HoloMask.Flat,   Pattern = HoloPattern.Etched },
            new Preset { Name = "Gold leaf",      Color = new Color(0xFF / 255f, 0xD9 / 255f, 0x2E / 255f), HueSpread = 0.05f, SatMul = 1.0f, Strength = 0.31f, Tiling = 1.2f, Parallax = 1.6f, Bands = 0.7f, Sparkle = 0.5f, PatternScale = 80, Edge = 0.30f, Mask = HoloMask.Flat,   Pattern = HoloPattern.Etched },
            new Preset { Name = "Emerald",          Color = new Color(0x00 / 255f, 0xFF / 255f, 0x67 / 255f), HueSpread = 0.08f, SatMul = 1.0f, Strength = 1.50f, Tiling = 1.5f, Parallax = 1.8f, Bands = 0.6f, Sparkle = 0.5f, PatternScale = 70, Edge = 0.30f, Mask = HoloMask.Bright, Pattern = HoloPattern.None },
            new Preset { Name = "Violet",         Color = new Color(0x9E / 255f, 0x6A / 255f, 0xFF / 255f), HueSpread = 0.15f, SatMul = 1.0f, Strength = 0.30f, Tiling = 1.3f, Parallax = 1.6f, Bands = 0.5f, Sparkle = 0.8f, PatternScale = 50, Edge = 0.35f, Mask = HoloMask.Flat,   Pattern = HoloPattern.Sparkle },
        };

        private static bool _loaded;
        private static Shader _shader;
        private static readonly Material[] Materials = new Material[6];
        private static readonly Texture2D[] Ramps = new Texture2D[6];

        public static bool Available
        {
            get
            {
                if (!_loaded) Load();
                return _shader != null;
            }
        }

        private static void Load()
        {
            _loaded = true;
            _shader = ShaderBundle.Get("TCGCC/HoloFoil");
            Plugin.Log.LogInfo($"HoloFoil: shader {(_shader != null ? "loaded" : "unavailable — custom foil disabled")}");
        }

        /// <summary>Rebuild materials after a [Foil] setting changed.</summary>
        public static void Invalidate()
        {
            for (int i = 0; i < Materials.Length; i++)
            {
                if (Materials[i] != null) Object.Destroy(Materials[i]);
                if (Ramps[i] != null) Object.Destroy(Ramps[i]);
                Materials[i] = null;
                Ramps[i] = null;
            }
        }

        public static Material Get(ECardBorderType border)
        {
            int g = (int)border;
            if (g < 0 || g >= Materials.Length || !Available) return null;
            if (Plugin.HoloFoilStrength[g].Value <= 0f) return null;
            if (Materials[g] == null) Materials[g] = Build(g);
            return Materials[g];
        }

        private static Material Build(int g)
        {
            var p = Presets[g];
            Ramps[g] = Ramp(Plugin.HoloFoilColor[g].Value, p);
            var m = new Material(_shader) { name = $"TCGCC_HoloFoil_{(ECardBorderType)g}" };
            m.SetTexture("_Ramp", Ramps[g]);
            m.SetFloat("_Strength", Plugin.HoloFoilStrength[g].Value);
            m.SetFloat("_Tiling", p.Tiling);
            m.SetFloat("_Parallax", p.Parallax);
            m.SetFloat("_Bands", p.Bands);
            m.SetFloat("_ArtMask", (float)p.Mask);
            m.SetFloat("_Pattern", (float)Plugin.HoloFoilPattern[g].Value);
            m.SetFloat("_Sparkle", p.Sparkle);
            m.SetFloat("_PatternScale", p.PatternScale);
            m.SetFloat("_Edge", p.Edge);
            float motion = Mathf.Max(0f, Plugin.HoloFoilMotion.Value);
            m.SetFloat("_Speed", 0.06f * motion);
            m.SetFloat("_SweepSpeed", 0.15f * motion);
            m.SetFloat("_Sweep", 0.6f);
            m.SetFloat("_ViewAmount", Mathf.Clamp01(Plugin.HoloFoilViewReact.Value));
            return m;
        }

        /// <summary>256×1 looping colour ramp around the grade colour: hue wanders ±HueSpread, value/saturation pulse like foil.</summary>
        private static Texture2D Ramp(Color c, Preset p)
        {
            const int N = 256;
            Color.RGBToHSV(c, out float h, out float s, out float v);
            var px = new Color[N];
            for (int i = 0; i < N; i++)
            {
                float t = (float)i / N * Mathf.PI * 2f;
                float hue = Mathf.Repeat(h + p.HueSpread * Mathf.Sin(t), 1f);
                float sat = Mathf.Clamp01(s * p.SatMul * (0.75f + 0.25f * Mathf.Cos(t * 2f)));
                float val = Mathf.Clamp01(Mathf.Max(v, 0.35f) * (0.55f + 0.45f * (0.5f + 0.5f * Mathf.Sin(t * 2f + 1f))));
                px[i] = Color.HSVToRGB(hue, sat, val);
            }
            var tex = new Texture2D(N, 1, TextureFormat.RGBA32, false) { name = "TCGCC_HoloRamp", wrapMode = TextureWrapMode.Repeat, filterMode = FilterMode.Bilinear };
            tex.SetPixels(px);
            tex.Apply(false, true);
            tex.hideFlags = HideFlags.DontUnloadUnusedAsset;
            return tex;
        }
    }
}
