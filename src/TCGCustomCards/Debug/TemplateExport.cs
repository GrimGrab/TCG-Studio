using System.IO;
using UnityEngine;

namespace TCGCustomCards.Debug
{
    /// <summary>
    /// Exports vanilla pack/box textures, icons, card backs and (v5) deck box / playmat templates to &lt;plugin&gt;\templates\ as PNG. Textures show the UV layout
    /// custom pack art must follow; icons keep the sprite's full rect (including transparent padding) so custom icons
    /// made from them display with the same proportions. Re-exports when <see cref="Version"/> changes.
    /// </summary>
    internal static class TemplateExport
    {
        private const int Version = 10;

        public static void RunIfMissing()
        {
            string dir = Path.Combine(Plugin.PluginDir, "templates");
            string marker = Path.Combine(dir, "templates.version");
            if (File.Exists(marker) && File.ReadAllText(marker).Trim() == Version.ToString()) return;
            Directory.CreateDirectory(dir);

            var so = CSingleton<InventoryBase>.Instance.m_StockItemData_SO;
            int n = 0;
            for (int i = 0; i < (int)EItemType.Max && i < so.m_ItemDataList.Count; i++)
            {
                var type = (EItemType)i;
                string name = type.ToString();
                if (!name.Contains("CardPack") && !name.Contains("CardBox") && !name.EndsWith("Pack")) continue;
                var mesh = so.m_ItemMeshDataList[i];
                if (mesh?.material != null && mesh.material.mainTexture != null)
                    n += SaveTexture(mesh.material.mainTexture, Path.Combine(dir, $"{name}_texture.png"));
                var icon = so.m_ItemDataList[i].icon;
                if (icon != null) n += SaveSprite(icon, Path.Combine(dir, $"{name}_icon.png"));
            }
            n += SaveUvLayout(so.m_ItemMeshDataList[(int)EItemType.BasicCardPack], Path.Combine(dir, "BasicCardPack_uv.png"));
            n += SaveUvLayout(so.m_ItemMeshDataList[(int)EItemType.BasicCardBox], Path.Combine(dir, "BasicCardBox_uv.png"));

            var backs = CSingleton<InventoryBase>.Instance.m_MonsterData_SO.m_CardBackImageList;
            for (int i = 0; i < backs.Count; i++)
                if (backs[i] != null) n += SaveSprite(backs[i], Path.Combine(dir, $"CardBack_{(ECardExpansionType)i}.png"));

            // Border sprites per frame template (normal-border style data[0]): show the rim each grade draws around the art.
            var uiSettings = CSingleton<InventoryBase>.Instance.m_MonsterData_SO.m_CardUISettingList;
            for (int i = 0; i < uiSettings.Count && i < (int)ECardExpansionType.MAX; i++)
            {
                var data = uiSettings[i]?.cardUISettingDataList;
                if (data == null || data.Count == 0 || data[0] == null) continue;
                var exp = (ECardExpansionType)i;
                for (int b = 0; b < data[0].cardBorderImageList.Count; b++)
                    if (data[0].cardBorderImageList[b] != null)
                        n += SaveSprite(data[0].cardBorderImageList[b], Path.Combine(dir, $"Border_{exp}_{(ECardBorderType)b}.png"));
                if (data[0].cardBorderMask != null) n += SaveSprite(data[0].cardBorderMask, Path.Combine(dir, $"BorderMask_{exp}.png"));
            }

            // v5: deck boxes / playmats (textures, icons, meshes, license rows) for TCG Studio's accessory editor.
            try { n += AccessoryTemplateExport.Run(dir); }
            catch (System.Exception e) { Plugin.Log.LogWarning($"Accessory template export failed: {e}"); }
            // v10: card pack / box meshes for TCG Studio's pack editor.
            try { n += PackTemplateExport.Run(dir); }
            catch (System.Exception e) { Plugin.Log.LogWarning($"Pack template export failed: {e}"); }

            File.WriteAllText(marker, Version.ToString());
            Plugin.Log.LogInfo($"Exported {n} vanilla template images to {dir}");
        }

        internal static int SaveTexture(Texture tex, string path)
        {
            var img = Read(tex, new RectInt(0, 0, tex.width, tex.height));
            if (img == null) return 0;
            File.WriteAllBytes(path, img.EncodeToPNG());
            Object.Destroy(img);
            return 1;
        }

        /// <summary>
        /// Draws the mesh's UV triangles (red) over its dimmed texture, showing exactly which texture areas the model uses.
        /// Needs a CPU-readable mesh; logs and skips otherwise.
        /// </summary>
        private static int SaveUvLayout(ItemMeshData md, string path)
        {
            var mesh = md?.mesh;
            if (mesh == null || md.material == null || md.material.mainTexture == null) return 0;
            if (!mesh.isReadable)
            {
                Plugin.Log.LogInfo($"UV layout: mesh '{mesh.name}' is not readable; skipped {Path.GetFileName(path)}");
                return 0;
            }
            var tex = md.material.mainTexture;
            var img = Read(tex, new RectInt(0, 0, tex.width, tex.height));
            if (img == null) return 0;
            var px = img.GetPixels32();
            for (int i = 0; i < px.Length; i++) px[i] = new Color32((byte)(px[i].r / 3), (byte)(px[i].g / 3), (byte)(px[i].b / 3), 255);
            int w = img.width, h = img.height;
            var uv = mesh.uv;
            var tris = mesh.triangles;
            var red = new Color32(255, 40, 40, 255);
            void Line(Vector2 a, Vector2 b)
            {
                int x0 = Mathf.RoundToInt(a.x * (w - 1)), y0 = Mathf.RoundToInt(a.y * (h - 1));
                int x1 = Mathf.RoundToInt(b.x * (w - 1)), y1 = Mathf.RoundToInt(b.y * (h - 1));
                int steps = Mathf.Max(Mathf.Abs(x1 - x0), Mathf.Abs(y1 - y0), 1);
                for (int s = 0; s <= steps; s++)
                {
                    int x = x0 + (x1 - x0) * s / steps, y = y0 + (y1 - y0) * s / steps;
                    // UVs can tile outside 0..1; wrap them like the default Repeat mode.
                    x = ((x % w) + w) % w; y = ((y % h) + h) % h;
                    px[y * w + x] = red;
                }
            }
            for (int t = 0; t + 2 < tris.Length; t += 3)
            {
                Vector2 a = uv[tris[t]], b = uv[tris[t + 1]], c = uv[tris[t + 2]];
                Line(a, b); Line(b, c); Line(c, a);
            }
            img.SetPixels32(px);
            img.Apply();
            File.WriteAllBytes(path, img.EncodeToPNG());
            Object.Destroy(img);
            Plugin.Log.LogInfo($"UV layout: {mesh.name} ({tris.Length / 3} triangles) → {Path.GetFileName(path)}");
            return 1;
        }

        /// <summary>Writes the sprite at its full rect size: trimmed pixels placed at textureRectOffset, transparent elsewhere.</summary>
        internal static int SaveSprite(Sprite sprite, string path)
        {
            try
            {
                Rect full = sprite.rect, packed = sprite.textureRect;
                Vector2 offset = sprite.textureRectOffset;
                var src = Read(sprite.texture, new RectInt((int)packed.x, (int)packed.y, (int)packed.width, (int)packed.height));
                if (src == null) return 0;
                int w = Mathf.RoundToInt(full.width), h = Mathf.RoundToInt(full.height);
                var outTex = new Texture2D(w, h, TextureFormat.RGBA32, false);
                var clear = new Color32[w * h];
                outTex.SetPixels32(clear);
                outTex.SetPixels(Mathf.RoundToInt(offset.x), Mathf.RoundToInt(offset.y), src.width, src.height, src.GetPixels());
                outTex.Apply();
                File.WriteAllBytes(path, outTex.EncodeToPNG());
                Plugin.Log.LogInfo($"Template {Path.GetFileName(path)}: rect {full.width}x{full.height}, pixels {packed.width}x{packed.height} at {offset}, " +
                                   $"ppu {sprite.pixelsPerUnit}, texture {sprite.texture.width}x{sprite.texture.height}");
                Object.Destroy(src);
                Object.Destroy(outTex);
                return 1;
            }
            catch (System.Exception e)
            {
                Plugin.Log.LogWarning($"Template export failed for {path}: {e.Message}");
                return 0;
            }
        }

        /// <summary>GPU read-back of a (non-readable) texture region.</summary>
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
