using System.Collections.Generic;
using System.Linq;
using HarmonyLib;
using TCGCustomCards.Core;
using TCGCustomCards.Hooks;
using TMPro;
using UnityEngine.TextCore.LowLevel;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// The shop whiteboard: a built-in wall decoration (no Studio setup; placed and saved like any custom one) sold boxed in the furniture app.
    /// Text is a live TextMeshPro on the board, in a marker-like font, showing the one view picked for that board. Clicking it opens
    /// <see cref="UI.WhiteboardPanel"/>; regulars sometimes stop to read it.
    /// </summary>
    internal static class Whiteboard
    {
        public const string Id = "tcgcc-whiteboard";
        private const float Price = 150f;
        private const float Width = 1.4f, Height = 0.9f, Depth = 0.02f;
        private const float RegularReadWeight = 0.35f, WalkInReadWeight = 0.12f; // against vanilla's 1 a decision
        private const int ChangesOnBoard = 3, TopOnBoard = 5;
        private const float RefreshSeconds = 30f; // the numbers on a board catch up; its view only changes when the player changes it

        private static readonly List<Board> Placed = new List<Board>();
        private static Mesh _mesh;
        private static Texture2D _texture;
        private static Sprite _icon;
        private static TMP_FontAsset _font;

        private static bool On => Cast.Enabled && Plugin.CustomerNicknames.Value;

        /// <summary>The decoration's definition, or null while nicknames are off.</summary>
        public static DecorationDef Def() => !On ? null
            : new DecorationDef { Id = Id, Kind = DecorationKind.Poster, Tab = DecorationTab.Other, Name = "Shop Whiteboard", Price = Price, BuiltIn = true };

        /// <summary>What the furniture app sells: a box that becomes the decoration when opened (<see cref="OpenBoxPatch"/>). Null while nicknames are off.</summary>
        public static FurnitureDef BoxDef() => !On ? null
            : new FurnitureDef
            {
                Id = Id + "-box", Type = FurnitureType.TrashBin, BaseObject = EObjectType.Trashbin, Name = "Shop Whiteboard", Price = Price, Level = 0, BuiltIn = true,
                Description = "Goes on a wall. Shows your regulars' nicknames and stats; click it to see and manage all of them.",
            };

        /// <summary>A plain board: its back on the wall (z = 0), centred on y = 0, the picture on the front (+z).</summary>
        public static Mesh BoardMesh()
        {
            if (_mesh != null) return _mesh;
            float x = Width / 2f, y = Height / 2f, z = Depth;
            _mesh = new Mesh
            {
                name = "TCGCC_Whiteboard",
                hideFlags = HideFlags.DontUnloadUnusedAsset,
                vertices = new[]
                {
                    new Vector3(x, -y, z), new Vector3(-x, -y, z), new Vector3(-x, y, z), new Vector3(x, y, z), // front
                    new Vector3(x, y, z), new Vector3(-x, y, z), new Vector3(-x, y, 0f), new Vector3(x, y, 0f), // top
                    new Vector3(-x, -y, z), new Vector3(x, -y, z), new Vector3(x, -y, 0f), new Vector3(-x, -y, 0f), // bottom
                    new Vector3(x, -y, 0f), new Vector3(x, -y, z), new Vector3(x, y, z), new Vector3(x, y, 0f), // side
                    new Vector3(-x, -y, z), new Vector3(-x, -y, 0f), new Vector3(-x, y, 0f), new Vector3(-x, y, z), // side
                },
                uv = new[]
                {
                    new Vector2(0f, 0f), new Vector2(1f, 0f), new Vector2(1f, 1f), new Vector2(0f, 1f),
                    Vector2.zero, Vector2.zero, Vector2.zero, Vector2.zero, Vector2.zero, Vector2.zero, Vector2.zero, Vector2.zero,
                    Vector2.zero, Vector2.zero, Vector2.zero, Vector2.zero, Vector2.zero, Vector2.zero, Vector2.zero, Vector2.zero,
                },
                triangles = new[] { 0, 2, 1, 0, 3, 2, 4, 6, 5, 4, 7, 6, 8, 10, 9, 8, 11, 10, 12, 14, 13, 12, 15, 14, 16, 18, 17, 16, 19, 18 },
            };
            _mesh.RecalculateNormals();
            _mesh.RecalculateBounds();
            return _mesh;
        }

        /// <summary>White with a grey frame.</summary>
        public static Texture2D BoardTexture()
        {
            if (_texture != null) return _texture;
            const int w = 512, h = 330, frame = 10;
            _texture = new Texture2D(w, h, TextureFormat.RGBA32, false) { name = "TCGCC_Whiteboard", hideFlags = HideFlags.DontUnloadUnusedAsset };
            var px = new Color32[w * h];
            for (int y = 0; y < h; y++)
                for (int x = 0; x < w; x++)
                    px[y * w + x] = x < frame || y < frame || x >= w - frame || y >= h - frame ? new Color32(150, 152, 158, 255) : new Color32(245, 245, 240, 255);
            _texture.SetPixels32(px);
            _texture.Apply(false);
            return _texture;
        }

        /// <summary>The board on a clear square, for shop lists and the box's price tag.</summary>
        public static Sprite Icon()
        {
            if (_icon != null) return _icon;
            const int size = 256, pad = 8, top = 48, frame = 6;
            var tex = new Texture2D(size, size, TextureFormat.RGBA32, false) { name = "TCGCC_WhiteboardIcon", hideFlags = HideFlags.DontUnloadUnusedAsset };
            var px = new Color32[size * size];
            for (int y = 0; y < size; y++)
                for (int x = 0; x < size; x++)
                {
                    bool board = x >= pad && x < size - pad && y >= top && y < size - top;
                    bool edge = x < pad + frame || x >= size - pad - frame || y < top + frame || y >= size - top - frame;
                    px[y * size + x] = !board ? new Color32(0, 0, 0, 0) : edge ? new Color32(150, 152, 158, 255) : new Color32(245, 245, 240, 255);
                }
            tex.SetPixels32(px);
            tex.Apply(false);
            _icon = Sprite.Create(tex, new Rect(0f, 0f, size, size), new Vector2(0.5f, 0.5f));
            _icon.hideFlags = HideFlags.DontUnloadUnusedAsset;
            return _icon;
        }

        /// <summary>Every placed copy gets the behaviour; it makes its own text (see <see cref="Board.Awake"/>).</summary>
        public static void Dress(InteractableObject tpl)
        {
            tpl.gameObject.AddComponent<Board>();
            tpl.m_CanBoxUpObject = true; // the poster it's built on can't be
        }

        internal class Board : MonoBehaviour
        {
            private TextMeshPro _text;
            private int _view;
            private float _refreshAt;

            public View Showing => Views[_view];
            public string ViewTitle => Showing.Title;
            public string ViewId => Showing.Id;

            /// <summary>
            /// The text is made here, on the active copy, never on the inactive template: TextMeshPro's first Awake puts its default sizes over anything
            /// set before it, and those are as big as the board.
            /// </summary>
            private void Awake()
            {
                var go = new GameObject("TCGCC_WhiteboardText");
                go.transform.SetParent(transform, false);
                go.transform.localPosition = new Vector3(0f, 0f, Depth + 0.002f);
                go.transform.localRotation = Quaternion.Euler(0f, 180f, 0f); // text reads from the side it faces away from
                _text = go.AddComponent<TextMeshPro>();
                _text.rectTransform.sizeDelta = new Vector2(Width - 0.14f, Height - 0.12f);
                _text.enableAutoSizing = true;
                _text.fontSizeMin = 0.25f;
                _text.fontSizeMax = 0.8f;
                _text.alignment = TextAlignmentOptions.TopLeft;
                _text.overflowMode = TextOverflowModes.Truncate;
                _text.color = new Color(0.1f, 0.14f, 0.32f);
                _text.richText = true;
            }

            private void OnEnable()
            {
                Placed.Add(this);
                Nicknames.Changed += Write;
                Write();
            }

            private void OnDisable()
            {
                Placed.Remove(this);
                Nicknames.Changed -= Write;
            }

            private void Start() => ClaimView(this); // a loaded board has its saved position by now

            private void Update()
            {
                if (Time.time >= _refreshAt) Write();
            }

            public void ChangeView(int step)
            {
                _view = (_view + step + Views.Length) % Views.Length;
                Write();
            }

            public void SetView(string id)
            {
                int i = System.Array.FindIndex(Views, v => v.Id == id);
                if (i < 0 || i == _view) return;
                _view = i;
                Write();
            }

            private void Write()
            {
                if (_text == null) return;
                _refreshAt = Time.time + RefreshSeconds;
                var font = Font();
                if (font != null && _text.font != font) _text.font = font;
                var lines = new List<string> { $"<b>{Showing.Title.ToUpperInvariant()}</b>" };
                lines.AddRange(Showing.Lines());
                string text = string.Join("\n", lines);
                if (_text.text != text) _text.text = text;
            }
        }

        // ---- what a board can show

        /// <summary>A ranking of the regulars: a view on the boards and a list on the panel's Records tab.</summary>
        public class Record
        {
            public string Id, BoardTitle, PanelTitle, Line, Prefix;
            public System.Func<Persona, float> Value;

            public List<Persona> Top(int count) => Cast.AllRegulars.Where(p => Value(p) > 0f).OrderByDescending(Value).Take(count).ToList();
        }

        public static readonly Record[] Records =
        {
            new Record { Id = "spent", BoardTitle = "Best Customers", PanelTitle = "Biggest spenders", Line = "boardSpent", Prefix = "$", Value = p => p.spent },
            new Record { Id = "visits", BoardTitle = "Most Visits", PanelTitle = "Most visits", Line = "boardVisits", Prefix = "", Value = p => p.visits },
            new Record { Id = "packs", BoardTitle = "Most Packs Opened", PanelTitle = "Most packs opened", Line = "boardPacks", Prefix = "", Value = p => PackRipping.PacksOpened(p) },
            new Record { Id = "wins", BoardTitle = "Table Champions", PanelTitle = "Most table wins", Line = "boardWins", Prefix = "", Value = p => Rivalries.Wins(p) },
            new Record { Id = "pulls", BoardTitle = "Best Pulls", PanelTitle = "Best pull", Line = "boardPulls", Prefix = "$", Value = p => PackRipping.BestPull(p) },
            new Record { Id = "beatyou", BoardTitle = "Beat The Boss", PanelTitle = "Beat you the most", Line = "boardBoss", Prefix = "", Value = p => Rivalries.Record(p).beatYou },
        };

        /// <summary>One thing a board can show, and what someone reading it says.</summary>
        public class View
        {
            public string Id, Title;
            public System.Func<List<string>> Lines;
            /// <summary>The line's situation for this reader, with its tokens filled in; null = nothing to say about it ("boardEmpty").</summary>
            public System.Func<Persona, Dialogue.Tokens, string> React;
        }

        private static readonly View[] Views = new[]
            {
                new View { Id = "legends", Title = "Shop Legends", Lines = LegendLines, React = ReactLegends },
                new View { Id = "here", Title = "In The Shop", Lines = () => Listed(Cast.Visitors().Where(v => v.Value.regular).Select(v => v.Value.name).OrderBy(n => n).Take(TopOnBoard * 2)), React = ReactHere },
            }
            .Concat(Records.Select(r => new View { Id = r.Id, Title = r.BoardTitle, Lines = () => RecordLines(r), React = (p, t) => ReactRecord(r, p, t) }))
            .Concat(new[]
            {
                new View { Id = "rivals", Title = "Rivals", Lines = () => Listed(Rivals().Select(x => $"{x.name} vs {x.rival}").Take(TopOnBoard * 2)), React = ReactRivals },
                new View { Id = "gossip", Title = "Shop Gossip", Lines = () => Listed(News.Items.Reverse().Select(News.Describe).Where(s => s != null).Take(TopOnBoard)), React = ReactGossip },
                new View { Id = "shame", Title = "Wall Of Shame", Lines = () => Listed(Offenders().Take(TopOnBoard).Select(x => $"{x.p.name}: {x.total} offence(s)")), React = ReactShame },
            })
            .ToArray();

        /// <summary>Regulars with something on their record, the worst first.</summary>
        public static IEnumerable<(Persona p, int caught, int away, int kicks, int rips, int total)> Offenders() => Cast.AllRegulars
            .Select(p => (p, caught: Shoplifting.Caught(p), away: Shoplifting.GotAway(p), kicks: KickOut.Kicks(p), rips: Nicknames.RipOffs(p)))
            .Select(x => (x.p, x.caught, x.away, x.kicks, x.rips, total: x.caught + x.away + x.kicks + x.rips))
            .Where(x => x.total > 0).OrderByDescending(x => x.total);

        private static IEnumerable<(string name, string rival)> Rivals() =>
            Cast.AllRegulars.Select(p => (p.name, rival: Rivalries.RivalName(p))).Where(x => x.rival != null);

        private static List<string> Listed(IEnumerable<string> lines)
        {
            var list = lines.ToList();
            if (list.Count == 0) list.Add("Nobody yet...");
            return list;
        }

        private static List<string> LegendLines()
        {
            var lines = new List<string>();
            var holders = Nicknames.Holders();
            if (holders.Count == 0) lines.Add("Nobody yet...");
            foreach (var h in holders) lines.Add($"<b>{h.nick}</b>: {h.who}" + (h.number != null ? $" ({h.number})" : ""));
            var log = Nicknames.Log;
            if (log.Count > 0) lines.Add("");
            foreach (var c in log.Skip(Mathf.Max(0, log.Count - ChangesOnBoard))) lines.Add(c.t);
            return lines;
        }

        private static List<string> RecordLines(Record r)
        {
            int n = 0;
            return Listed(r.Top(TopOnBoard).Select(p => $"{++n}. {p.name}: {r.Prefix}{r.Value(p):0}"));
        }

        private static string ReactLegends(Persona p, Dialogue.Tokens t)
        {
            string mine = Nicknames.Of(p);
            if (mine != null) { t.Nick = mine; return "boardBrag"; }
            var holders = Nicknames.Holders();
            if (holders.Count == 0) return null;
            var h = holders[Random.Range(0, holders.Count)];
            t.Other = h.who;
            t.Nick = h.nick;
            return "boardRead";
        }

        private static string ReactHere(Persona p, Dialogue.Tokens t)
        {
            var others = Cast.Visitors().Where(v => v.Value.regular && v.Value != p).Select(v => v.Value.name).ToList();
            if (others.Count == 0) return null;
            t.Other = others[Random.Range(0, others.Count)];
            return "boardHere";
        }

        private static string ReactRecord(Record r, Persona p, Dialogue.Tokens t)
        {
            var top = r.Top(TopOnBoard);
            if (top.Contains(p)) return "boardTopMe";
            if (top.Count == 0) return null;
            t.Other = top[0].name;
            t.Price = r.Value(top[0]);
            return r.Line;
        }

        private static string ReactRivals(Persona p, Dialogue.Tokens t)
        {
            string mine = Rivalries.RivalName(p);
            if (mine != null && mine != "you") { t.Rival = mine; return "boardRivalMine"; }
            var pairs = Rivals().Where(x => x.rival != "you").ToList();
            if (pairs.Count == 0) return null;
            var pair = pairs[Random.Range(0, pairs.Count)];
            t.Other = pair.name;
            t.Rival = pair.rival;
            return "boardRivals";
        }

        private static string ReactGossip(Persona p, Dialogue.Tokens t)
        {
            var shown = News.Items.Reverse().Where(i => News.Describe(i) != null).Take(TopOnBoard).ToList();
            if (shown.Exists(i => i.w == p.name)) return "boardGossipMe";
            if (shown.Count == 0) return null;
            t.Other = shown[Random.Range(0, shown.Count)].w;
            return "boardGossip";
        }

        private static string ReactShame(Persona p, Dialogue.Tokens t)
        {
            var worst = Offenders().Take(TopOnBoard).ToList();
            if (worst.Exists(x => x.p == p)) return "boardShameMe";
            if (worst.Count == 0) return null;
            t.Other = worst[0].p.name;
            return "boardShame";
        }

        // ---- each board's view, saved by where it hangs (the game's saved decoration has nothing else of ours)

        public class SavedView
        {
            public float x, y, z;
            public string v;
        }

        private const float SameSpot = 0.1f;
        private const int MaxSavedViews = 32;
        private static List<SavedView> _savedViews = new List<SavedView>();

        private static void ClaimView(Board b)
        {
            var p = b.transform.position;
            int i = _savedViews.FindIndex(s => Vector3.Distance(new Vector3(s.x, s.y, s.z), p) < SameSpot);
            if (i < 0) return;
            b.SetView(_savedViews[i].v);
            _savedViews.RemoveAt(i);
        }

        /// <summary>From the nicknames module's load; boards already placed look now, later ones in their Start.</summary>
        public static void LoadViews(List<SavedView> views)
        {
            _savedViews = views ?? new List<SavedView>();
            foreach (var b in Placed.ToList()) ClaimView(b);
        }

        /// <summary>Boards not showing the default, plus saved ones no board has claimed yet. Null when there are none.</summary>
        public static List<SavedView> SaveViews()
        {
            var list = new List<SavedView>(_savedViews);
            foreach (var b in Placed)
            {
                if (b.ViewId == Views[0].Id) continue;
                var p = b.transform.position;
                list.Add(new SavedView { x = Mathf.Round(p.x * 100f) / 100f, y = Mathf.Round(p.y * 100f) / 100f, z = Mathf.Round(p.z * 100f) / 100f, v = b.ViewId });
            }
            if (list.Count > MaxSavedViews) list.RemoveRange(0, list.Count - MaxSavedViews);
            return list.Count == 0 ? null : list;
        }

        /// <summary>Marker-like fonts every Windows has; the game's own popup font if none of them loads, and for letters they lack.</summary>
        private static readonly string[] MarkerFonts = { "segoeprb.ttf", "segoepr.ttf", "Inkfree.ttf", "comicbd.ttf" };
        private static TMP_FontAsset _marker;
        private static bool _markerTried, _markerHasFallback;

        private static TMP_FontAsset Marker()
        {
            if (_markerTried) return _marker;
            _markerTried = true;
            foreach (var file in MarkerFonts)
            {
                string path = System.IO.Path.Combine(System.Environment.GetFolderPath(System.Environment.SpecialFolder.Fonts), file);
                if (!System.IO.File.Exists(path)) continue;
                try
                {
                    var source = new UnityEngine.Font(path) { hideFlags = HideFlags.DontUnloadUnusedAsset };
                    var asset = TMP_FontAsset.CreateFontAsset(source, 64, 6, GlyphRenderMode.SDFAA, 1024, 1024, AtlasPopulationMode.Dynamic, true);
                    if (asset == null || !asset.TryAddCharacters("Aa0", out _)) continue;
                    asset.hideFlags = HideFlags.DontUnloadUnusedAsset;
                    Plugin.Log.LogInfo($"Whiteboard: marker font {file}");
                    return _marker = asset;
                }
                catch (System.Exception e) { Plugin.Log.LogWarning($"Whiteboard: font {file} failed: {e.Message}"); }
            }
            Plugin.Log.LogWarning("Whiteboard: no marker font found, using the game's");
            return null;
        }

        private static TMP_FontAsset Font()
        {
            var popup = PopupFont();
            var marker = Marker();
            if (marker == null) return popup;
            if (popup != null && !_markerHasFallback)
            {
                _markerHasFallback = true;
                if (marker.fallbackFontAssetTable == null) marker.fallbackFontAssetTable = new List<TMP_FontAsset>();
                marker.fallbackFontAssetTable.Add(popup);
            }
            return marker;
        }

        private static TMP_FontAsset PopupFont()
        {
            if (_font != null) return _font;
            var spawner = GameScene.Existing<PricePopupSpawner>();
            if (spawner == null) return null;
            foreach (var p in spawner.m_PricePopupList)
                if (p != null && p.m_Text != null) return _font = p.m_Text.font;
            return null;
        }

        // ---- clicking it, and its box

        [HarmonyPatch(typeof(InteractableObject), nameof(InteractableObject.OnMouseButtonUp))]
        private static class ClickPatch
        {
            private static void Postfix(InteractableObject __instance)
            {
                var board = __instance.GetIsMovingObject() ? null : __instance.GetComponent<Board>();
                if (board != null) UI.WhiteboardPanel.Open(board);
            }
        }

        /// <summary>
        /// The box holds a hidden trash bin only because furniture needs a vanilla piece behind it: opening it drops the box and bin and hands the
        /// player the wall decoration to place, as the decorate screen does (ShelfManager.SpawnDecoObjectOnHand). Same clean-up as binning a box
        /// (InteractableTrashBin.DiscardBox); vanilla never takes a bin off its list, so we do.
        /// </summary>
        [HarmonyPatch(typeof(InteractablePackagingBox_Shelf), nameof(InteractablePackagingBox_Shelf.OnPressOpenBox))]
        private static class OpenBoxPatch
        {
            private static readonly AccessTools.FieldRef<InteractablePackagingBox_Shelf, InteractableObject> Boxed =
                Compat.Field<InteractablePackagingBox_Shelf, InteractableObject>(typeof(OpenBoxPatch), "m_BoxedObject");

            private static bool Prefix(InteractablePackagingBox_Shelf __instance)
            {
                var boxed = Boxed(__instance);
                if (boxed == null || Registry.GetFurniture(boxed.m_ObjectType)?.Def.BuiltIn != true) return true;
                var deco = Registry.Decorations.Find(d => d.Def.Id == Id);
                if (deco?.Prefab == null) return false;
                if (boxed is InteractableTrashBin bin) ShelfManager.RemoveTrashBin(bin);
                __instance.OnDestroyed();
                CSingleton<InteractionPlayerController>.Instance.OnExitHoldBoxMode();
                SoundManager.PlayAudio("SFX_BoxOpen", 0.5f);
                ShelfManager.SpawnDecoObjectOnHand(deco.Object);
                return false;
            }
        }

        /// <summary>Boxing a placed board: vanilla puts a decoration in the decoration inventory; this one goes back in its box, in the player's hands.</summary>
        [HarmonyPatch(typeof(InteractableObject), nameof(InteractableObject.BoxUpObject))]
        private static class BoxUpPatch
        {
            private static void Postfix(InteractableObject __instance)
            {
                if (__instance.GetComponent<Board>() == null) return;
                var box = Registry.Furniture.Find(f => f.Def.BuiltIn && f.Prefab != null);
                if (box == null) return; // stays in the decoration inventory
                CPlayerData.AddDecoItemToInventory(__instance.m_DecoObjectType, -1);
                var bin = ShelfManager.SpawnInteractableObject(box.Object);
                bin.Init();
                bin.BoxUpObject(holdBox: true);
            }
        }

        // ---- regulars reading it

        private class Visit
        {
            public bool Read;
            public float ReadUntil;
        }

        public static void Init()
        {
            CustomerStates.Register(new ReadState());
            CustomerChoices.Register("whiteboard", Weight, Start);
        }

        private static float Weight(Customer c)
        {
            var p = On && Placed.Count > 0 ? Cast.Of(c) : null;
            if (p == null || Cast.PeekVisitData<Visit>(c)?.Read == true) return 0f;
            foreach (var v in Cast.Visitors()) if (CustomerStates.IsIn(v.Key, ReadState.StateId)) return 0f; // one at a time
            return p.regular ? RegularReadWeight : WalkInReadWeight;
        }

        private static bool Start(Customer c)
        {
            var visit = Cast.VisitData<Visit>(c);
            if (visit == null || Placed.Count == 0) return false;
            visit.Read = true;
            if (Plugin.CustomerDebug.Value) Plugin.Log.LogInfo($"Whiteboard: {Cast.Of(c)?.name} goes to read it");
            return CustomerStates.Goto(c, ReadState.StateId, Placed[Random.Range(0, Placed.Count)]);
        }

        private class ReadState : ModState
        {
            public const string StateId = "whiteboard.read";
            private const float Seconds = 4f, Timeout = 45f; // from the far end of a big shop
            public override string Id => StateId;

            public override void Enter(Customer c, StateContext ctx)
            {
                var board = (Board)ctx.Data;
                var spot = board.transform.position + board.transform.forward * 1.3f;
                spot.y = c.transform.position.y; // on the floor: the board hangs on the wall, and a walk ends within reach of its target in 3D
                ctx.WalkTo(spot);
            }

            public override void Tick(Customer c, StateContext ctx)
            {
                var board = ctx.Data as Board;
                var visit = Cast.VisitData<Visit>(c);
                bool done = visit != null && visit.ReadUntil > 0f && Time.time >= visit.ReadUntil;
                if (board == null || ctx.Elapsed > Timeout || done)
                {
                    if (Plugin.CustomerDebug.Value && board != null && !ctx.HasArrived)
                        Plugin.Log.LogInfo($"Whiteboard: {Cast.Of(c)?.name} gave up after {ctx.Elapsed:0} s, {Vector3.Distance(c.transform.position, board.transform.position):0.0} m from the board");
                    CustomerStates.ToVanilla(c, CustomerLife.Shop);
                    return;
                }
                if (!ctx.HasArrived) return;
                CustomerLife.FaceTowards(c, board.transform.position);
                if (visit == null || visit.ReadUntil > 0f) return;
                visit.ReadUntil = Time.time + Seconds;
                if (Plugin.CustomerDebug.Value) Plugin.Log.LogInfo($"Whiteboard: {Cast.Of(c)?.name} arrived after {ctx.Elapsed:0} s, {Vector3.Distance(c.transform.position, board.transform.position):0.0} m from the board");
                CustomerLife.Stand(c);
                var tokens = new Dialogue.Tokens();
                Dialogue.Say(c, board.Showing.React(Cast.Of(c), tokens) ?? "boardEmpty", tokens, 1f);
            }

        }
    }
}
