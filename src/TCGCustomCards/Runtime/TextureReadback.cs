using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Copies a (non-readable) game texture into a readable one through the GPU, e.g. to compose custom card backs on top of
    /// the vanilla back texture.
    /// </summary>
    internal static class TextureReadback
    {
        /// <summary>GPU read-back of a texture region (null on failure).</summary>
        internal static Texture2D Read(Texture tex, RectInt r)
        {
            try
            {
                var rt = RenderTexture.GetTemporary(tex.width, tex.height, 0, RenderTextureFormat.ARGB32, RenderTextureReadWrite.sRGB);
                Graphics.Blit(tex, rt);
                var prev = RenderTexture.active;
                RenderTexture.active = rt;
                var outTex = new Texture2D(r.width, r.height, TextureFormat.RGBA32, false);
                outTex.ReadPixels(new Rect(r.x, r.y, r.width, r.height), 0, 0);
                outTex.Apply();
                RenderTexture.active = prev;
                RenderTexture.ReleaseTemporary(rt);
                return outTex;
            }
            catch (System.Exception e)
            {
                Plugin.Log.LogWarning($"Texture read-back failed for {tex.name}: {e.Message}");
                return null;
            }
        }
    }
}
