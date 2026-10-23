using UnityEngine;
using UnityEngine.Rendering;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Reading <see cref="Material.mainTexture"/> on a material whose shader has no main texture (the customers' clothes,
    /// glasses and watches: Shader_Apparel, Toony Colors) makes Unity log an error per material. Code that looks through
    /// renderers it doesn't own (a customer's carried bag, scenery, vanilla pieces) reads the texture through here.
    /// </summary>
    internal static class MaterialTex
    {
        private static readonly int MainTexId = Shader.PropertyToID("_MainTex");

        /// <summary>The material's main texture, or null when it has none (or no material).</summary>
        internal static Texture Main(Material m)
        {
            if (m == null || m.shader == null) return null;
            if (m.HasProperty(MainTexId)) return m.mainTexture;
            var s = m.shader;
            for (int i = 0, n = s.GetPropertyCount(); i < n; i++)
                if ((s.GetPropertyFlags(i) & ShaderPropertyFlags.MainTexture) != 0) return m.mainTexture;
            return null;
        }
    }
}
