using System.Collections.Generic;
using HarmonyLib;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// [Visuals - Game look] OpenSignImage / ClosedSignImage: the open/closed sign at the door (InteractableOpenCloseSign). Both of
    /// its models (Model_Open, Model_Close) use material MAT_OpenCloseSign with one 512² texture T_OpenCloseSign that holds the green
    /// OPEN board above the red CLOSED board; each model shows one board on its front and the other on its back. A picture is painted
    /// inside a board's black frame (the frame and screws stay) in a copy of that texture, set per renderer.
    /// </summary>
    internal static class DoorSign
    {
        // Measured from the texture (game files, 2026-10-10; Studio gameextract.DoorBoards/DoorScrews have the same values): the
        // coloured area inside each board's frame (+1 px; the mask keeps the frame) as x, y, w, h fractions with a top-left origin, and the screw heads (centre as
        // fractions of the board, radius as a fraction of the texture width).
        private static readonly Rect OpenBoard = new Rect(42f / 512, 25f / 512, 328f / 512, 206f / 512);
        private static readonly Rect ClosedBoard = new Rect(42f / 512, 277f / 512, 328f / 512, 206f / 512);
        private static readonly Vector3[] Screws = { new Vector3(0.0412f, 0.0583f, 7f / 512), new Vector3(0.9558f, 0.0583f, 7f / 512) };
        /// <summary>Width ÷ height of a board (328 × 206 texture pixels; the mesh maps it without stretching).</summary>
        internal const float BoardAspect = 328f / 206f;

        private static readonly List<InteractableOpenCloseSign> Signs = new List<InteractableOpenCloseSign>();
        private static readonly PictureCache<Texture2D> Texture = new PictureCache<Texture2D>();
        private static Texture _vanilla;

        [HarmonyPatch(typeof(InteractableOpenCloseSign), "OnEnable")]
        private static class OnEnablePatch
        {
            private static void Postfix(InteractableOpenCloseSign __instance)
            {
                Signs.RemoveAll(s => s == null); // destroyed with the previous shop scene
                if (!Signs.Contains(__instance)) Signs.Add(__instance);
                Apply();
            }
        }

        internal static void Init()
        {
            Plugin.OpenSignImage.SettingChanged += (_, __) => Apply();
            Plugin.OpenSignCrop.SettingChanged += (_, __) => Apply();
            Plugin.ClosedSignImage.SettingChanged += (_, __) => Apply();
            Plugin.ClosedSignCrop.SettingChanged += (_, __) => Apply();
        }

        internal static void Apply()
        {
            foreach (var sign in Signs)
            {
                if (sign == null) continue;
                foreach (var r in Renderers(sign))
                {
                    _vanilla ??= MaterialTex.Main(r.sharedMaterial);
                    ShopPicture.SetTexture(r, TextureFor());
                }
            }
        }

        /// <summary>The sign's renderers that draw the board texture (not the highlight outline).</summary>
        private static IEnumerable<Renderer> Renderers(InteractableOpenCloseSign sign)
        {
            foreach (var model in new[] { sign.m_OpenShopMesh, sign.m_CloseShopMesh })
            {
                if (model == null) continue;
                foreach (var r in model.GetComponentsInChildren<MeshRenderer>(true))
                    if (MaterialTex.Main(r.sharedMaterial) is Texture t && t != null && t.name == "T_OpenCloseSign")
                        yield return r;
            }
        }

        private static Texture2D TextureFor()
        {
            string open = ShopPicture.ImagePath(Plugin.OpenSignImage.Value), closed = ShopPicture.ImagePath(Plugin.ClosedSignImage.Value);
            string openCrop = Plugin.OpenSignCrop.Value, closedCrop = Plugin.ClosedSignCrop.Value;
            return Texture.Get(string.Join("|", open, openCrop, closed, closedCrop), () =>
            {
                if (_vanilla == null || (open == null && closed == null)) return null;
                var openSrc = open == null ? null : ShopPicture.Load(open, "Open sign");
                var closedSrc = closed == null ? null : ShopPicture.Load(closed, "Closed sign");
                if (openSrc == null && closedSrc == null) return null;
                var tex = ShopPicture.Compose(_vanilla, "TCGCC_OpenCloseSign", (px, w, h) =>
                {
                    var vanilla = (Color32[])px.Clone(); // the mask reads the frame from the untouched pixels
                    Board(px, vanilla, w, h, openSrc, openCrop, "OpenSignCrop", OpenBoard);
                    Board(px, vanilla, w, h, closedSrc, closedCrop, "ClosedSignCrop", ClosedBoard);
                });
                if (openSrc != null) Object.Destroy(openSrc);
                if (closedSrc != null) Object.Destroy(closedSrc);
                if (tex != null) Plugin.Log.LogInfo($"Open/closed sign: using {open ?? "vanilla"} / {closed ?? "vanilla"}");
                return tex;
            });
        }

        private static void Board(Color32[] px, Color32[] vanilla, int w, int h, Texture2D src, string crop, string setting, Rect board)
        {
            if (src == null) return;
            var dst = new Rect(board.x, 1 - board.y - board.height, board.width, board.height); // UV is bottom-up
            ShopPicture.Paint(px, w, h, src, ShopPicture.Crop(crop, setting, src, BoardAspect), dst, (x, y) =>
            {
                // Keep the dark frame (its rounded inner corners reach into the board) and the screw heads.
                var c = vanilla[y * w + x];
                float luma = (0.3f * c.r + 0.59f * c.g + 0.11f * c.b) / 255f;
                float keep = Mathf.Clamp01((luma - 0.1f) / 0.15f);
                foreach (var s in Screws)
                {
                    float dx = x + 0.5f - (board.x + s.x * board.width) * w, dy = (h - y - 0.5f) - (board.y + s.y * board.height) * h;
                    keep = Mathf.Min(keep, Mathf.Clamp01(Mathf.Sqrt(dx * dx + dy * dy) - s.z * w + 1f));
                }
                return keep;
            });
        }
    }
}
