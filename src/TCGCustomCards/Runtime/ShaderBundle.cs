using System.IO;
using System.Linq;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// The mod's shader asset bundle <c>tcgcc_foil</c> next to the DLL (built by tools\build-shaders.ps1 with Unity 2021.3.38f1).
    /// Loaded once per session — Unity refuses to load the same bundle twice — and shared by every feature that needs a shader.
    /// </summary>
    internal static class ShaderBundle
    {
        public const string FileName = "tcgcc_foil";
        private static bool _loaded;
        private static Shader[] _shaders = new Shader[0];

        /// <summary>Shader by name, or null if the bundle or the shader is missing or unsupported (logged).</summary>
        public static Shader Get(string name)
        {
            if (!_loaded) Load();
            var s = _shaders.FirstOrDefault(x => x != null && x.name == name);
            if (s == null) { Plugin.Log.LogWarning($"Shader {name} not found in {FileName}"); return null; }
            if (!s.isSupported) { Plugin.Log.LogWarning($"Shader {name} is not supported on this GPU"); return null; }
            return s;
        }

        private static void Load()
        {
            _loaded = true;
            string path = Path.Combine(Plugin.PluginDir, FileName);
            if (!File.Exists(path)) { Plugin.Log.LogWarning($"{path} not found — custom shaders disabled"); return; }
            var bundle = AssetBundle.LoadFromFile(path);
            if (bundle == null) { Plugin.Log.LogWarning($"{FileName}: bundle failed to load — custom shaders disabled"); return; }
            _shaders = bundle.LoadAllAssets<Shader>();
            Plugin.Log.LogInfo($"{FileName}: loaded shaders {string.Join(", ", _shaders.Select(s => $"{s.name} (supported={s.isSupported})"))}");
        }
    }
}
