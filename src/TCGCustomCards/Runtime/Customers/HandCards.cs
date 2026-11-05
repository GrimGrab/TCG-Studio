using System.Collections;
using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// The cards in a customer's hand while they rip packs: the game's table-play fan (Customer.m_GameCardFanOut, a
    /// CardFanRandomizer over 16 card meshes) reads a 4×3 atlas, T_3dCardModel1–6: the back top-left, Tetramon faces in the
    /// other 11 cells (measured from the game files 2026-10-27, the same layout in all six). We draw an atlas of our own on
    /// the GPU, once per pack: the pack's card back in the back cell and the pack's cards in the face cells, and give it to
    /// the fan's renderers through a property block (the game's materials stay as they are; the fan randomizes them again the
    /// next time it is shown, at a play table). One render texture per ripping customer, back in a small pool when the rip ends: making a new one is what
    /// the graphics driver charges for.
    /// </summary>
    internal static class HandCards
    {
        // Kept on the customer's fan: pooled with them, and there with or without Personalities.
        private class Hand : MonoBehaviour
        {
            public RenderTexture Atlas;
            public List<Renderer> Renderers;
            public bool Hidden;
            public Vector3 FanScale, SingleScale;

            private void OnDestroy()
            {
                if (Atlas != null) { Atlas.Release(); Destroy(Atlas); }
            }
        }

        private static Hand HandOf(Customer c)
        {
            var fan = c != null ? c.m_GameCardFanOut : null;
            if (fan == null) return null;
            var hand = fan.GetComponent<Hand>();
            return hand != null ? hand : fan.AddComponent<Hand>();
        }

        /// <summary>The cards out of sight (still active: drawing needs them) until <see cref="RevealCards"/>.</summary>
        public static void HideCards(Customer c)
        {
            var hand = HandOf(c);
            if (hand == null || hand.Hidden) return;
            hand.Hidden = true;
            hand.FanScale = c.m_GameCardFanOut.transform.localScale;
            hand.SingleScale = c.m_GameCardSingle.transform.localScale;
            c.m_GameCardFanOut.transform.localScale = Vector3.zero;
            c.m_GameCardSingle.transform.localScale = Vector3.zero;
        }

        public static void RevealCards(Customer c)
        {
            var hand = HandOf(c);
            if (hand == null || !hand.Hidden) return;
            hand.Hidden = false;
            c.m_GameCardFanOut.transform.localScale = hand.FanScale;
            c.m_GameCardSingle.transform.localScale = hand.SingleScale;
        }

        /// <summary>The fan's own size, also while <see cref="HideCards"/> holds it at nothing.</summary>
        public static Vector3 FanScale(Customer c)
        {
            var hand = HandOf(c);
            return hand != null && hand.Hidden ? hand.FanScale : c.m_GameCardFanOut.transform.localScale;
        }

        private const int Size = 1024;
        private const float WaitForPictures = 1.5f; // seconds; cards not drawn by then keep the game's picture
        // Cells in atlas pixels, top-down; the back is the first. (Static fields initialise in order: the edges come first.)
        private static readonly int[] ColX = { 13, 263, 512, 762 }, ColX1 = { 255, 501, 751, 1000 };
        private static readonly int[] RowY = { 6, 345, 685 }, RowY1 = { 337, 676, 1016 };
        private static readonly RectInt[] Cells = BuildCells();
        // The cards' thin sides read this strip at the bottom of the back's cell (measured from the fan's meshes).
        private static readonly RectInt EdgeStrip = new RectInt(13, 324, 242, 13);

        private static Material _draw;
        private static MaterialPropertyBlock _block;
        private static bool _noShaderLogged;

        static HandCards()
        {
            Cast.VisitEnded += (c, p, paid) => Clear(c);
        }

        private static RectInt[] BuildCells()
        {
            var cells = new RectInt[12];
            for (int row = 0, i = 0; row < 3; row++)
                for (int col = 0; col < 4; col++, i++)
                    cells[i] = new RectInt(ColX[col], RowY[row], ColX1[col] - ColX[col], RowY1[row] - RowY[row]);
            return cells;
        }

        /// <summary>
        /// Puts <paramref name="cards"/> (a pack's contents) in the customer's hand. Run inside a state's coroutine: it waits
        /// up to <see cref="WaitForPictures"/> for the card pictures. The fan must already be shown.
        /// </summary>
        public static IEnumerator Show(Customer c, List<CardData> cards, float waitForPictures = WaitForPictures, bool fillMissing = false, Component alsoOn = null)
        {
            if (c == null || cards == null || cards.Count == 0) yield break;
            var fan = c.m_GameCardFanOut != null ? c.m_GameCardFanOut.GetComponentInChildren<CardFanRandomizer>(true) : null;
            if (fan == null || fan.m_MeshRendererList == null || fan.m_MeshRendererList.Count == 0) yield break;
            var hand = HandOf(c);
            if (hand == null) yield break;

            // Pictures by the game's own card renderer (borders, foil); they take a couple of frames each.
            var faces = new Texture[Cells.Length - 1];
            float until = Time.time + waitForPictures;
            while (true)
            {
                bool all = true;
                for (int i = 0; i < faces.Length; i++)
                    if (faces[i] == null && (faces[i] = UI.CardRenderCache.Get(cards[i % cards.Count])) == null) all = false;
                if (all || Time.time >= until) break;
                yield return null;
            }
            if (c == null || c.m_GameCardFanOut == null || !c.m_GameCardFanOut.activeInHierarchy) yield break; // the rip ended meanwhile
            if (fillMissing)
            {
                // Cards not drawn in time repeat one that was, rather than show the game's picture.
                var any = faces.FirstOrDefault(f => f != null);
                if (any == null) yield break;
                for (int i = 0; i < faces.Length; i++) if (faces[i] == null) faces[i] = any;
            }
            // Looks only: a failure keeps the game's cards in their hand and the rip goes on.
            try { Apply(c, fan, hand, cards[0], faces, alsoOn); }
            catch (System.Exception e)
            {
                if (!_failLogged) Plugin.Log.LogWarning($"Customers: couldn't put the pack's cards in a customer's hand: {e}");
                _failLogged = true;
            }
        }

        private static bool _failLogged;

        private static void Apply(Customer c, CardFanRandomizer fan, Hand hand, CardData first, Texture[] faces, Component alsoOn)
        {
            using var measured = Debug.SpikeWatch.Measure("hand cards");
            Debug.SpikeWatch.Mark("painting hand cards");
            var vanilla = MaterialTex.Main(fan.m_MeshRendererList[0].sharedMaterial);
            if (vanilla == null || !Draw(hand, vanilla, Back(first), faces)) return;
            hand.Renderers = new List<Renderer>();
            _block ??= new MaterialPropertyBlock();
            // Every card under the fan: the game's list can leave some out, and those would still show its own pictures behind ours.
            var renderers = fan.GetComponentsInChildren<MeshRenderer>(true).ToList();
            if (c.m_GameCardSingle != null) renderers.AddRange(c.m_GameCardSingle.GetComponentsInChildren<MeshRenderer>(true)); // the card in their other hand
            // Their cards on the table; the playmat, deck box and comic there aren't cards.
            if (alsoOn != null) renderers.AddRange(alsoOn.GetComponentsInChildren<MeshRenderer>(true).Where(r => r.name.StartsWith("Card")));
            foreach (var r in renderers)
            {
                _block.Clear();
                foreach (var prop in TextureProps(r.sharedMaterial)) _block.SetTexture(prop, hand.Atlas);
                r.SetPropertyBlock(_block);
                hand.Renderers.Add(r);
            }
            if (Plugin.CustomerDebug.Value)
                Plugin.Log.LogInfo($"Hand cards: {renderers.Count} card(s) painted{(alsoOn != null ? " including the table's" : "")}, back {(Back(first).tex != null ? Back(first).tex.name : "the game's")}");
        }

        /// <summary>The game's pictures back in the customer's hand and the atlas released.</summary>
        public static void Clear(Customer c)
        {
            var hand = c != null && c.m_GameCardFanOut != null ? c.m_GameCardFanOut.GetComponent<Hand>() : null;
            if (hand == null) return;
            if (hand.Renderers != null)
                foreach (var r in hand.Renderers)
                    if (r != null) r.SetPropertyBlock(null);
            hand.Renderers = null;
            if (hand.Atlas != null)
            {
                if (Pool.Count < MaxPooled) Pool.Push(hand.Atlas);
                else { hand.Atlas.Release(); Object.Destroy(hand.Atlas); }
            }
            hand.Atlas = null;
        }

        private const int MaxPooled = 8;
        private static readonly Stack<RenderTexture> Pool = new Stack<RenderTexture>();

        private static RenderTexture NewAtlas()
        {
            while (Pool.Count > 0)
            {
                var pooled = Pool.Pop();
                if (pooled != null && pooled.IsCreated()) return pooled;
            }
            Debug.SpikeWatch.Mark("making a new hand texture");
            var atlas = new RenderTexture(Size, Size, 0, RenderTextureFormat.ARGB32)
                { name = "TCGCC_HandCards", useMipMap = true, autoGenerateMips = true, wrapMode = TextureWrapMode.Clamp, hideFlags = HideFlags.DontUnloadUnusedAsset };
            atlas.Create();
            return atlas;
        }

        // Back sprites (the game's and ImageCache.GetCardBack's) are squares with the card ≈86.7% of the height, centred; the
        // card's width follows from the cell's shape.
        private const float BackCardHeight = 0.867f;

        /// <summary>
        /// The back of the pack's cards and the part of its texture (UV) that is the card: a custom set's own image as it is
        /// (set's → setup's), else the square back sprite (the frame's / the expansion's) cropped to the card in its middle.
        /// </summary>
        private static (Texture tex, Rect uv) Back(CardData card)
        {
            Sprite sprite = null;
            if (Registry.IsCustom(card.expansionType) && Registry.Get(card.expansionType) is CustomSet set)
            {
                var own = set.CardBackPath != null ? ImageCache.Get(set.CardBackPath) : null;
                if (own != null && own.texture != null && Mathf.Abs(own.texture.width - own.texture.height) > 2)
                    return (own.texture, SpriteUv(own));
                sprite = set.CardBack;
            }
            else
            {
                var list = CSingleton<InventoryBase>.Instance?.m_MonsterData_SO?.m_CardBackImageList;
                int i = (int)card.expansionType;
                sprite = list != null && i >= 0 && i < list.Count ? list[i] : null;
            }
            if (sprite == null || sprite.texture == null) return (null, default);
            var r = SpriteUv(sprite);
            var cell = Cells[0];
            float h = r.height * BackCardHeight;
            float w = h * ((float)cell.width / cell.height) * (sprite.textureRect.height / sprite.textureRect.width) * (r.width / r.height);
            return (sprite.texture, new Rect(r.center.x - w / 2, r.center.y - h / 2, w, h));
        }

        private static Rect SpriteUv(Sprite s)
        {
            var r = s.textureRect;
            float tw = s.texture.width, th = s.texture.height;
            return new Rect(r.x / tw, r.y / th, r.width / tw, r.height / th);
        }

        /// <summary>The material's texture slots that hold one of the game's hand-card atlases.</summary>
        private static List<string> TextureProps(Material mat)
        {
            var props = new List<string>();
            if (mat != null)
                foreach (var name in mat.GetTexturePropertyNames())
                    if (mat.GetTexture(name) is Texture t && t != null && t.name.StartsWith("T_3dCardModel")) props.Add(name);
            if (props.Count == 0) props.Add("_BaseMap");
            return props;
        }

        /// <summary>The game's atlas as the base (cells we can't fill keep its picture), the back, then the faces.</summary>
        private static bool Draw(Hand hand, Texture vanilla, (Texture tex, Rect uv) back, Texture[] faces)
        {
            if (_draw == null)
            {
                var shader = Shader.Find("UI/Default") ?? Shader.Find("Sprites/Default");
                if (shader == null)
                {
                    if (!_noShaderLogged) Plugin.Log.LogWarning("Customers: no shader to draw the cards in customers' hands");
                    _noShaderLogged = true;
                    return false;
                }
                _draw = new Material(shader) { hideFlags = HideFlags.HideAndDontSave };
            }
            if (hand.Atlas == null) hand.Atlas = NewAtlas();
            var before = RenderTexture.active;
            Graphics.Blit(vanilla, hand.Atlas);
            RenderTexture.active = hand.Atlas;
            GL.PushMatrix();
            GL.LoadOrtho(); // 0..1, bottom-left = UV (0,0)
            if (back.tex != null) Quad(back.tex, Cells[0], back.uv);
            Quad(Texture2D.blackTexture, EdgeStrip, new Rect(0, 0, 1, 1));
            for (int i = 0; i < faces.Length; i++)
                if (faces[i] != null) Quad(faces[i], Cells[i + 1], new Rect(0, 0, 1, 1));
            GL.PopMatrix();
            RenderTexture.active = before;
            return true;
        }

        private static void Quad(Texture tex, RectInt cellTopDown, Rect src)
        {
            _draw.mainTexture = tex;
            _draw.SetPass(0);
            float x0 = cellTopDown.xMin / (float)Size, x1 = cellTopDown.xMax / (float)Size;
            float y0 = 1f - cellTopDown.yMax / (float)Size, y1 = 1f - cellTopDown.yMin / (float)Size;
            GL.Begin(GL.QUADS);
            GL.Color(Color.white);
            GL.TexCoord2(src.xMin, src.yMin); GL.Vertex3(x0, y0, 0);
            GL.TexCoord2(src.xMin, src.yMax); GL.Vertex3(x0, y1, 0);
            GL.TexCoord2(src.xMax, src.yMax); GL.Vertex3(x1, y1, 0);
            GL.TexCoord2(src.xMax, src.yMin); GL.Vertex3(x1, y0, 0);
            GL.End();
        }
    }
}
