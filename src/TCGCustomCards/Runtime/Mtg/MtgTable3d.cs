using System.Collections.Generic;
using System.Linq;
using Newtonsoft.Json.Linq;
using UnityEngine;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>
    /// P2 board: the MTG game as real 3D cards on the play table, rendered by the game's own card renderer (Card3dUISpawner +
    /// InteractableCard3d, like PlayCardSet.GetNewCard3d) and moved with its LerpToTransform. Layout is derived from each side's
    /// vanilla PlayCardSet lane lines, measured from the table midline between the two players (Arena-style): an open combat zone
    /// in the middle (attackers and their blockers slide into it), creatures in the front row, and a back row with lands on the
    /// left and other permanents (artifacts, unattached enchantments, planeswalkers) on the right; my hand at the bottom of the
    /// view. Row depths shrink (<see cref="_fit"/>) when the camera can't show them all. The stack top pops up on the centre show
    /// spot. Clicks/hover come from each card's projected screen rect (<see cref="Visible"/>, drawn by MtgTableUI).
    /// </summary>
    internal class MtgTable3d : MonoBehaviour
    {
        internal sealed class Entry
        {
            public string Key;
            public InteractableCard3d Card;
            public Transform Anchor;
            public JObject Json;       // null for face-down placeholders
            public bool Seen;
            public Rect ScreenRect;    // GUI space (y down), pixels
            public bool OnScreen;
            public readonly Vector2[] Quad = new Vector2[4]; // the card face's corners on screen (GUI space, y down), in order
            public bool HasQuad;
            public float Depth;        // distance from the camera (smaller = in front)
            public bool Covered;       // mostly hidden behind a nearer card: no overlay labels/frames
            public bool Unknown;       // no matching in-game card (token etc.): shown as a stand-in, labelled by the overlay
            public CardData Data;      // the face its CardUI shows (null = the stand-in back card)
        }

        public static bool IsUnknown(Entry e) => e.Unknown;

        private sealed class Side
        {
            public PlayCardSet Set;
            public Vector3 Center, AxisX, ToOwner;
            public float Spacing;
            public Vector3 Combat, Front, Back; // row centres, set each Sync by ComputeRows
            public Transform LaneRef, HandRef, DeckRef, DiscardRef, CenterRef;
            public List<Transform> HandSlots = new List<Transform>();
        }

        private static MtgTable3d _inst;
        private PlayTableGame _game;
        private MtgCardFaces _faces;
        private Side _me, _opp;
        private readonly Dictionary<string, Entry> _cards = new Dictionary<string, Entry>();
        private JObject _lastState;
        private Transform _root;
        private CardData _backData; // any card of the opponent's sets, for card backs
        private float _relayoutAt;
        private readonly Dictionary<int, (Vector3 pos, Side side, float scale)> _hosts = new Dictionary<int, (Vector3, Side, float)>();
        private readonly HashSet<int> _attachedIds = new HashSet<int>();
        private readonly Dictionary<int, int> _blockerOf = new Dictionary<int, int>(); // blocker id -> attacker id
        private readonly Dictionary<int, int> _attachCount = new Dictionary<int, int>();
        private readonly HashSet<int> _inCombat = new HashSet<int>(); // cards placed in the combat zone this Sync
        private const float AttachLift = 0.0015f;
        // Row depths from the table midline toward each owner, in lane spacings (lane lines are ~1.6 spacings apart).
        private const float CombatDepth = 0.65f, FrontDepth = 2.0f, BackDepth = 3.25f, HandGap = 1.15f;
        private const float FrontWidth = 5.2f, BackWidth = 5.8f, BackGap = 0.5f;
        private float _fit = 1f;        // row-depth factor so everything stays on screen (0.7..1)
        private float _cardScale = 1f;  // board card scale that goes with it (0.85..1)
        private float _fitLogged = -1f;
        private const float PopupSeconds = 1.5f;
        private int _stackTopId = -1;
        private float _stackTopSince;
        private bool _rectsLogged;

        public static bool Active => _inst != null && _inst.enabled && _inst._game != null;
        public static IEnumerable<Entry> Visible => Active ? _inst._cards.Values.Where(e => e.OnScreen) : Enumerable.Empty<Entry>();

        public static void Begin(PlayTableGame game, MtgCardFaces faces)
        {
            MtgSession.EnsureRunner();
            var go = GameObject.Find("TCGCC_Mtg");
            _inst = go.GetComponent<MtgTable3d>() ?? go.AddComponent<MtgTable3d>();
            _inst.Init(game, faces);
        }

        public static void End()
        {
            if (_inst == null) return;
            _inst.Clear();
            _inst._game = null;
            _inst.enabled = false;
        }

        private void Init(PlayTableGame game, MtgCardFaces faces)
        {
            Clear();
            _game = game;
            _faces = faces;
            _lastState = null;
            _backData = faces.Any();
            _rectsLogged = false;
            _fitLogged = -1f;
            if (_root == null) _root = new GameObject("TCGCC_MtgAnchors").transform;
            _me = Measure(game.m_PlayCardSetPlayer);
            _opp = Measure(game.m_PlayCardSetEnemy);
            enabled = true;

            // Vanilla top-down table view + playmats/deck boxes (PlayTableGame.DelayStart); DelayExit undoes it.
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            ipc.SetCameraWorldPositionTarget(game.m_CamLoc);
            ipc.StartAimLookAt(game.m_CamDownTarget, 8f);
            ipc.m_IsPlayingTopDownGameMode = true;
            ipc.m_CameraFOVController.StartLerpToFOV(40f);
            game.m_Grp.SetActive(true);
            game.m_PlayCardSetPlayer.EvaluatePlaymatDeckboxMesh();
            game.m_PlayCardSetEnemy.EvaluatePlaymatDeckboxMesh();
            Plugin.Log.LogInfo($"MTG table: lanes me {_me.Center} axis {_me.AxisX} spacing {_me.Spacing:F3} toOwner {_me.ToOwner}; " +
                               $"opp {_opp.Center} spacing {_opp.Spacing:F3}; hand slots {_me.HandSlots.Count}");
        }

        private static Side Measure(PlayCardSet set)
        {
            var s = new Side { Set = set };
            var lanes = set.m_ElementAreaPosList.Where(l => l?.transformList != null && l.transformList.Count > 0).Select(l => l.transformList[0]).ToList();
            s.LaneRef = lanes[0];
            s.Center = lanes.Aggregate(Vector3.zero, (a, t) => a + t.position) / lanes.Count;
            s.AxisX = (lanes[lanes.Count - 1].position - lanes[0].position);
            s.AxisX.y = 0;
            s.Spacing = s.AxisX.magnitude / Mathf.Max(1, lanes.Count - 1);
            s.AxisX.Normalize();
            s.HandSlots = set.m_HoldCardPosList.Where(t => t != null).ToList();
            var hand = s.HandSlots.Count > 0 ? s.HandSlots.Aggregate(Vector3.zero, (a, t) => a + t.position) / s.HandSlots.Count : s.Center;
            s.ToOwner = Vector3.Cross(Vector3.up, s.AxisX).normalized;
            var toHand = hand - s.Center;
            toHand.y = 0;
            if (Vector3.Dot(s.ToOwner, toHand) < 0) s.ToOwner = -s.ToOwner;
            s.HandRef = s.HandSlots.Count > 0 ? s.HandSlots[0] : s.LaneRef;
            s.DeckRef = set.m_DeckPosList.Count > 0 ? set.m_DeckPosList[set.m_DeckPosList.Count - 1] : s.LaneRef;
            s.DiscardRef = set.m_DiscardPosList.Count > 0 ? set.m_DiscardPosList[0] : s.LaneRef;
            s.CenterRef = set.m_CenterAreaShowPos != null ? set.m_CenterAreaShowPos : s.LaneRef;
            return s;
        }

        private void Clear()
        {
            foreach (var e in _cards.Values) Remove(e);
            _cards.Clear();
        }

        // ------------------------------------------------------------------ sync with Forge's state

        private void LateUpdate()
        {
            if (_game == null) return;
            var st = MtgSession.State;
            if (st != null && (!ReferenceEquals(st, _lastState) || Time.unscaledTime >= _relayoutAt))
            {
                _lastState = st;
                _relayoutAt = Time.unscaledTime + 0.5f; // hand rows follow the camera, which lerps into the top-down view
                Sync(st);
            }
            UpdateScreenRects();
        }

        private void Sync(JObject st)
        {
            foreach (var e in _cards.Values) e.Seen = false;
            int meId = (int?)st["me"] ?? -1;
            var players = (st["players"] as JArray)?.OfType<JObject>().ToList() ?? new List<JObject>();
            var me = players.FirstOrDefault(p => (int?)p["id"] == meId);
            var opp = players.FirstOrDefault(p => (int?)p["id"] != meId);

            // Cards attached to another card on the battlefield (Auras, Equipment) are laid out under their host afterwards.
            _hosts.Clear();
            _attachedIds.Clear();
            var allBf = players.SelectMany(p => (p["battlefield"] as JArray)?.OfType<JObject>() ?? Enumerable.Empty<JObject>()).ToList();
            var bfIds = new HashSet<int>(allBf.Select(c => (int?)c["id"] ?? -1));
            _attachCount.Clear();
            foreach (var c in allBf)
                if (c["attachedTo"] != null && c["attachedTo"].Type == JTokenType.Integer && bfIds.Contains((int)c["attachedTo"]))
                {
                    _attachedIds.Add((int)c["id"]);
                    _attachCount.TryGetValue((int)c["attachedTo"], out int n);
                    _attachCount[(int)c["attachedTo"]] = n + 1;
                }

            // Combat: blockers leave their row and stand across from the attacker they block (placed after the rows).
            _blockerOf.Clear();
            foreach (var cb in (st["combat"] as JArray)?.OfType<JObject>() ?? Enumerable.Empty<JObject>())
                foreach (var b in (cb["blockers"] as JArray)?.Where(t => t.Type == JTokenType.Integer) ?? Enumerable.Empty<JToken>())
                    _blockerOf[(int)b] = (int?)cb["attacker"] ?? -1;

            _inCombat.Clear();
            ComputeRows();
            if (me != null) LayoutPlayer(me, _me, meId, mine: true);
            if (opp != null) LayoutPlayer(opp, _opp, meId, mine: false);
            LayoutBlockers(players, meId);
            LayoutAttachments(allBf, meId);

            // Stack: a newly cast spell pops up big on the centre show spot for PopupSeconds, then lies at the side of the table
            // between the players (so it doesn't block responding with instants). The HUD lists the whole stack.
            var top = (st["stack"] as JArray)?.OfType<JObject>().FirstOrDefault()?["card"] as JObject;
            if (top != null)
            {
                int topId = (int?)top["id"] ?? -1;
                if (topId != _stackTopId) { _stackTopId = topId; _stackTopSince = Time.unscaledTime; }
                if (Time.unscaledTime - _stackTopSince < PopupSeconds)
                {
                    Place(top, meId, _me, _me.CenterRef, _me.CenterRef.position + Vector3.up * 0.02f, 1.15f, tapped: false, faceDown: false);
                    _relayoutAt = Mathf.Min(_relayoutAt, _stackTopSince + PopupSeconds); // shrink right on time
                }
                else
                {
                    var mid = (_me.Center + _opp.Center) / 2f;
                    var side = mid + _me.AxisX * _me.Spacing * 3.4f + Vector3.up * 0.003f;
                    Place(top, meId, _me, _me.LaneRef, side, 0.85f, tapped: false, faceDown: false);
                }
            }
            else _stackTopId = -1;

            foreach (var gone in _cards.Values.Where(e => !e.Seen).ToList())
            {
                Remove(gone);
                _cards.Remove(gone.Key);
            }
        }

        private void LayoutPlayer(JObject p, Side side, int meId, bool mine)
        {
            var bf = (p["battlefield"] as JArray)?.OfType<JObject>().Where(c => !_attachedIds.Contains((int?)c["id"] ?? -1)).ToList() ?? new List<JObject>();
            var creatures = bf.Where(c => B(c, "creature") && !_blockerOf.ContainsKey((int?)c["id"] ?? -1)).ToList();
            var lands = bf.Where(c => B(c, "land") && !B(c, "creature")).ToList();
            var support = bf.Where(c => !B(c, "land") && !B(c, "creature")).ToList(); // artifacts, enchantments, planeswalkers…
            Row(creatures, side, meId, side.Front, side.Spacing * FrontWidth);
            BackRow(lands, support, side, meId);

            // My hand: a row on the table at my edge of the camera view. (Vanilla's hand slots follow its on-screen hand buttons,
            // which we don't use.) The opponent's hand and both libraries aren't put on the table — counts are in the HUD.
            if (mine)
            {
                var handCenter = HandRowCenter(side, mine);
                var hand = (p["hand"] as JArray)?.OfType<JObject>().ToList() ?? new List<JObject>();
                HandRow(hand.Count, side, handCenter, (i, pos, scale) => Place(hand[i], meId, side, side.LaneRef, pos, scale, tapped: false, faceDown: false));
            }

            // The graveyard isn't put on the table either (face-up cards there read as "in play"); it's in the HUD's viewer.
        }

        private void Row(List<JObject> cards, Side side, int meId, Vector3 center, float width)
        {
            if (cards.Count == 0) return;
            float pitch = Mathf.Min(side.Spacing * _cardScale, width / cards.Count);
            float scale = Mathf.Clamp(pitch / side.Spacing, 0.45f, 1f);
            float start = -(cards.Count - 1) * pitch / 2f;
            // The owner's left is -AxisX for the far side: keep reading order left→right from the camera for both.
            for (int i = 0; i < cards.Count; i++)
            {
                var c = cards[i];
                float x = start + i * pitch;
                if (B(c, "attacking"))
                {
                    // Slides forward into the combat zone, keeping its slot along the row (and at a readable size).
                    PlaceBoard(c, meId, side, side.Combat + side.AxisX * x, Mathf.Max(scale, 0.8f));
                    _inCombat.Add((int?)c["id"] ?? -1);
                }
                else PlaceBoard(c, meId, side, center + side.AxisX * x, scale);
            }
        }

        /// <summary>
        /// Back row: lands packed from the camera-left end, other permanents from the camera-right end with a gap between
        /// (Arena-style), so a first artifact doesn't shuffle the lands. When crowded the width is shared by card count.
        /// </summary>
        private void BackRow(List<JObject> lands, List<JObject> support, Side side, int meId)
        {
            int nl = lands.Count, ns = support.Count;
            if (nl + ns == 0) return;
            float s = side.Spacing, width = s * BackWidth, gap = s * BackGap, natural = s * _cardScale;
            float wl, ws;
            if (ns == 0) { wl = width; ws = 0f; }
            else if (nl == 0) { wl = 0f; ws = width; }
            else if ((nl + ns) * natural + gap <= width) { wl = nl * natural; ws = ns * natural; }
            else { wl = (width - gap) * nl / (nl + ns); ws = width - gap - wl; }
            PackFromEdge(lands, side, meId, side.Back - side.AxisX * (width / 2f), +1, wl);
            PackFromEdge(support, side, meId, side.Back + side.AxisX * (width / 2f), -1, ws);
        }

        /// <summary>Lays cards side by side inward from <paramref name="edge"/> (dir +1 = toward +AxisX), keeping their order
        /// left→right from the camera.</summary>
        private void PackFromEdge(List<JObject> cards, Side side, int meId, Vector3 edge, int dir, float width)
        {
            int n = cards.Count;
            if (n == 0) return;
            float pitch = Mathf.Min(side.Spacing * _cardScale, width / n);
            float scale = Mathf.Clamp(pitch / side.Spacing, 0.45f, 1f);
            for (int i = 0; i < n; i++)
            {
                int slot = dir > 0 ? i : n - 1 - i;
                PlaceBoard(cards[i], meId, side, edge + side.AxisX * (dir * (slot + 0.5f) * pitch), scale);
            }
        }

        /// <summary>Places a battlefield card and registers it as a possible host for attachments. A card with attachments is
        /// lifted so they can sit under it without sinking into the playmat.</summary>
        private void PlaceBoard(JObject c, int meId, Side side, Vector3 pos, float scale)
        {
            int id = (int?)c["id"] ?? -1;
            if (_attachCount.TryGetValue(id, out int under)) pos += Vector3.up * AttachLift * under;
            Place(c, meId, side, side.LaneRef, pos, scale, B(c, "tapped"), faceDown: false);
            _hosts[id] = (pos, side, scale);
        }

        /// <summary>
        /// Row centres from the midline between the players. If the camera shows less than the opponent's back row (top edge) or
        /// my hand would have to sit on my back row, the front/back depths shrink by up to 30% and board cards to 85%.
        /// </summary>
        private void ComputeRows()
        {
            var mid = (_me.Center + _opp.Center) / 2f;
            float s = _me.Spacing, f = 1f;
            var cam = GameCamera;
            if (cam != null)
            {
                var plane = new Plane(Vector3.up, mid);
                var top = cam.ViewportPointToRay(new Vector3(0.42f, 0.97f, 0f));
                if (plane.Raycast(top, out float d))
                {
                    float avail = Vector3.Dot(top.GetPoint(d) - mid, _opp.ToOwner);
                    if (avail > 0f) f = Mathf.Min(f, avail / ((BackDepth + 0.6f) * s));
                }
                var hand = cam.ViewportPointToRay(new Vector3(0.42f, Plugin.MtgHandHeight.Value, 0f));
                if (plane.Raycast(hand, out d))
                {
                    float avail = Vector3.Dot(hand.GetPoint(d) - mid, _me.ToOwner);
                    if (avail > 0f) f = Mathf.Min(f, avail / ((BackDepth + HandGap) * s));
                }
            }
            _fit = Mathf.Clamp(f, 0.7f, 1f);
            _cardScale = Mathf.Max(0.85f, _fit);
            foreach (var side in new[] { _me, _opp })
            {
                side.Combat = mid + side.ToOwner * CombatDepth * s;
                side.Front = mid + side.ToOwner * FrontDepth * s * _fit;
                side.Back = mid + side.ToOwner * BackDepth * s * _fit;
            }
            if (Mathf.Abs(_fit - _fitLogged) >= 0.05f)
            {
                _fitLogged = _fit;
                Plugin.Log.LogInfo($"MTG table: row fit {_fit:F2} (raw {f:F2}), card scale {_cardScale:F2}");
            }
        }

        /// <summary>
        /// Auras/Equipment under their host card (wherever the host is — a stolen creature takes its Aura across), each one
        /// peeking out further toward the far edge and a little lower, so the host stays on top and the strip is clickable.
        /// Repeats so things attached to attachments also find their host.
        /// </summary>
        /// <summary>
        /// Each blocker stands on its own side's creature line directly across from the attacker it blocks, stepped toward it
        /// (attackers already step forward), several blockers of one attacker side by side. Unmatched blockers stay in place.
        /// </summary>
        private void LayoutBlockers(List<JObject> players, int meId)
        {
            if (_blockerOf.Count == 0) return;
            var perAttacker = _blockerOf.GroupBy(kv => kv.Value).ToDictionary(g => g.Key, g => g.Select(kv => kv.Key).ToList());
            foreach (var p in players)
            {
                var side = (int?)p["id"] == meId ? _me : _opp;
                foreach (var c in (p["battlefield"] as JArray)?.OfType<JObject>() ?? Enumerable.Empty<JObject>())
                {
                    int id = (int?)c["id"] ?? -1;
                    if (!_blockerOf.TryGetValue(id, out int attacker)) continue;
                    Vector3 pos;
                    if (_hosts.TryGetValue(attacker, out var a))
                    {
                        var group = perAttacker[attacker];
                        int k = group.IndexOf(id);
                        float along = Vector3.Dot(a.pos - side.Combat, side.AxisX) + (k - (group.Count - 1) / 2f) * side.Spacing * 0.9f;
                        pos = side.Combat + side.AxisX * along;
                        _inCombat.Add(id);
                    }
                    else pos = side.Front; // attacker not on the table (shouldn't happen)
                    PlaceBoard(c, meId, side, pos, 1f);
                }
            }
        }

        private void LayoutAttachments(List<JObject> allBf, int meId)
        {
            var pending = allBf.Where(c => _attachedIds.Contains((int?)c["id"] ?? -1)).ToList();
            var perHost = new Dictionary<int, int>();
            for (int pass = 0; pass < 4 && pending.Count > 0; pass++)
            {
                foreach (var c in pending.ToList())
                {
                    int host = (int)c["attachedTo"];
                    if (!_hosts.TryGetValue(host, out var h)) continue;
                    perHost.TryGetValue(host, out int k);
                    perHost[host] = ++k;
                    // Each attachment one step lower than the (lifted) host — the last one back at table height, never below it.
                    // In the combat zone they peek out sideways so they don't cover the opposing card.
                    var step = _inCombat.Contains(host) ? h.side.AxisX * h.side.Spacing * 0.25f : -h.side.ToOwner * h.side.Spacing * 0.3f;
                    var pos = h.pos + step * k - Vector3.up * AttachLift * k;
                    Place(c, meId, h.side, h.side.LaneRef, pos, h.scale, B(c, "tapped"), faceDown: false);
                    _hosts[(int?)c["id"] ?? -1] = (pos, h.side, h.scale);
                    pending.Remove(c);
                }
            }
            foreach (var c in pending) // host not on the table (shouldn't happen): don't lose the card
                Place(c, meId, _me, _me.LaneRef, (_me.Center + _opp.Center) / 2f, 0.7f, B(c, "tapped"), faceDown: false);
        }

        /// <summary>
        /// Centre of a hand row: the table point on the owner's line (lane centre + ToOwner·t) that the camera shows ~13% from the
        /// bottom (mine) or top (opponent's) edge, never closer than HandGap behind the back row (then it sits at that minimum).
        /// </summary>
        private static Vector3 HandRowCenter(Side side, bool mine)
        {
            float min = Vector3.Dot(side.Back - side.Center, side.ToOwner) + side.Spacing * HandGap;
            float t = min;
            var cam = GameCamera;
            if (cam != null)
            {
                // [MTG] HandHeight = where the hand's centre sits on screen (0 = bottom edge; negative = partly below it).
                float vy = Plugin.MtgHandHeight.Value;
                var ray = cam.ViewportPointToRay(new Vector3(0.42f, mine ? vy : 1f - vy, 0f));
                var plane = new Plane(Vector3.up, side.Center);
                if (plane.Raycast(ray, out float d))
                {
                    var hit = ray.GetPoint(d);
                    float along = Vector3.Dot(hit - side.Center, side.ToOwner);
                    if (along > min && along < side.Spacing * 16f) t = along;
                }
            }
            return side.Center + side.ToOwner * t + Vector3.up * 0.002f;
        }

        /// <summary>Lays out n hand cards centred on a row (overlapping when there are many).</summary>
        private static void HandRow(int n, Side side, Vector3 center, System.Action<int, Vector3, float> place)
        {
            if (n <= 0) return;
            float width = side.Spacing * 6.5f;
            float pitch = Mathf.Min(side.Spacing * 0.95f, width / n);
            float start = -(n - 1) * pitch / 2f;
            for (int i = 0; i < n; i++)
                place(i, center + side.AxisX * (start + i * pitch) + Vector3.up * 0.0005f * i, 1f);
        }

        /// <summary>Returns the card to the game's pools. The card is parented under its anchor while lerping: detach it first so
        /// destroying the anchor doesn't take the pooled object with it.</summary>
        private static void Remove(Entry e)
        {
            if (e.Card != null)
            {
                e.Card.StopLerpToTransform();
                e.Card.transform.SetParent(null, true);
                e.Card.OnDestroyed();
            }
            if (e.Anchor != null) Destroy(e.Anchor.gameObject);
        }

        /// <summary>The player's camera. The game doesn't tag it MainCamera, so Camera.main is null here.</summary>
        private static Camera GameCamera
        {
            get
            {
                var ipc = CSingleton<InteractionPlayerController>.Instance;
                return ipc != null && ipc.m_Cam != null ? ipc.m_Cam : Camera.main;
            }
        }

        private static bool B(JToken t, string k) => t?[k] != null && t[k].Type == JTokenType.Boolean && (bool)t[k];

        // ------------------------------------------------------------------ card objects

        private void Place(JObject json, int meId, Side side, Transform reference, Vector3 pos, float scale, bool tapped, bool faceDown)
        {
            string key = "card:" + (int)json["id"];
            bool hidden = B(json, "faceDown"); // morph/manifest etc.: never give it a face, show the back
            var data = hidden ? null : _faces.For(json, meId);
            var e = Get(key, data ?? _backData, side);
            // A card object is made once per Forge card. When its face becomes known or changes (a face-down card turned up,
            // a card first seen without a match), re-skin it — otherwise it keeps the stand-in it was made with (a card from
            // the player's deck) and shows that face up.
            if (data != null && !SameCard(e.Data, data)) Reskin(e, data);
            e.Unknown = data == null && !hidden;
            e.Json = json;
            // No matching in-game card (tokens etc.): show a card back; the overlay prints its name and stats on it.
            Aim(e, reference, pos, scale, tapped, faceDown || hidden || e.Unknown, side);
        }

        private static bool SameCard(CardData a, CardData b) =>
            a != null && b != null && a.expansionType == b.expansionType && a.monsterType == b.monsterType &&
            a.borderType == b.borderType && a.isFoil == b.isFoil && a.isDestiny == b.isDestiny &&
            MtgCardFaces.IsBack(a) == MtgCardFaces.IsBack(b); // transformed: same copy, other face

        /// <summary>Shows <paramref name="data"/> on the entry's existing card (same calls as <see cref="NewCard3d"/>).</summary>
        private static void Reskin(Entry e, CardData data)
        {
            var ui = e.Card != null ? e.Card.m_Card3dUI : null;
            if (ui == null) return;
            ui.m_CardUI.SetCardUI(data);
            ui.m_CardUI.SetFoilMaterialListFromSettingData(isWorldView: false);
            ui.m_CardUI.SetFoilBlendedMaterialListFromSettingData(isWorldView: false);
            e.Data = data;
        }

        private Entry Get(string key, CardData data, Side side)
        {
            if (!_cards.TryGetValue(key, out var e))
            {
                e = new Entry { Key = key, Anchor = new GameObject("TCGCC_" + key).transform };
                e.Card = NewCard3d(data, side.DeckRef);
                e.Data = data == _backData ? null : data; // stand-in: re-skinned once the real face is known
                _cards[key] = e;
            }
            e.Seen = true;
            return e;
        }

        /// <summary>Moves the entry's anchor (copying the vanilla reference's frame/scale) and lerps the card to it when it changed.</summary>
        private static void Aim(Entry e, Transform reference, Vector3 pos, float scale, bool tapped, bool faceDown, Side side)
        {
            var a = e.Anchor;
            a.SetParent(reference.parent, false);
            var rot = reference.rotation;
            if (faceDown) rot = side.DeckRef.rotation;
            if (tapped) rot = Quaternion.AngleAxis(90f, Vector3.up) * rot;
            // The visible card follows only position/rotation; its size is its ScaleGrp (vanilla 1.6), so scale that.
            var targetScale = reference.localScale;
            e.Card.SetTargetLocalScale(Vector3.one * 1.6f * scale);
            bool changed = (a.position - pos).sqrMagnitude > 1e-8f || Quaternion.Angle(a.rotation, rot) > 0.5f || (a.localScale - targetScale).sqrMagnitude > 1e-8f;
            a.position = pos;
            a.rotation = rot;
            a.localScale = targetScale;
            if (changed || e.Card.transform.parent != a) e.Card.LerpToTransform(a, a, 7f);
        }

        /// <summary>Same setup as PlayCardSet.GetNewCard3d (private): a pooled card UI following a spawned InteractableCard3d.</summary>
        private static InteractableCard3d NewCard3d(CardData data, Transform spawnAt)
        {
            if (data == null) data = new CardData { monsterType = EMonsterType.PiggyA, borderType = ECardBorderType.Base, expansionType = ECardExpansionType.Tetramon };
            var ui = CSingleton<Card3dUISpawner>.Instance.GetCardUI();
            var card = ShelfManager.SpawnInteractableObject(EObjectType.Card3d).GetComponent<InteractableCard3d>();
            ui.m_IgnoreCulling = true;
            ui.m_CardUI.SetFoilCullListVisibility(isActive: true);
            ui.m_CardUI.ResetFarDistanceCull();
            ui.m_CardUIAnimGrp.gameObject.SetActive(value: true);
            ui.m_CardUI.SetCardUI(data);
            ui.m_CardUI.SetFoilMaterialListFromSettingData(isWorldView: false);
            ui.m_CardUI.SetFoilBlendedMaterialListFromSettingData(isWorldView: false);
            ui.transform.position = card.transform.position;
            ui.transform.rotation = card.transform.rotation;
            card.SetCardUIFollow(ui);
            card.SetEnableCollision(isEnable: false);
            card.transform.position = spawnAt.position;
            card.transform.rotation = spawnAt.rotation;
            card.SetTargetLocalScale(Vector3.one * 1.6f);
            card.SetTargetLocalPos(Vector3.zero);
            card.SetTargetRotation(Quaternion.identity);
            ui.m_IgnoreCulling = true;
            ui.SetVisibility(isVisible: true);
            ui.SetAlwaysCulling(alwaysCulling: false);
            return card;
        }

        // ------------------------------------------------------------------ screen rects for clicks/hover

        private readonly Vector3[] _corners = new Vector3[8];

        /// <summary>Screen rect of each card = its face's corners projected (the card UI's RectTransform; the back mesh as fallback).
        /// Whole-object renderer bounds are far too big (effects/foil layers) and made click areas overlap.</summary>
        private void UpdateScreenRects()
        {
            var cam = GameCamera;
            if (cam == null) return;
            foreach (var e in _cards.Values)
            {
                e.OnScreen = false;
                int n = FaceCorners(e);
                if (n == 0) continue;
                float xMin = float.MaxValue, yMin = float.MaxValue, xMax = float.MinValue, yMax = float.MinValue;
                bool behind = false;
                for (int i = 0; i < n; i++)
                {
                    var sp = cam.WorldToScreenPoint(_corners[i]);
                    if (sp.z < 0) { behind = true; break; }
                    xMin = Mathf.Min(xMin, sp.x); xMax = Mathf.Max(xMax, sp.x);
                    yMin = Mathf.Min(yMin, sp.y); yMax = Mathf.Max(yMax, sp.y);
                }
                if (behind) continue;
                e.ScreenRect = Rect.MinMaxRect(xMin, Screen.height - yMax, xMax, Screen.height - yMin);
                e.HasQuad = n == 4;
                if (e.HasQuad)
                    for (int i = 0; i < 4; i++)
                    {
                        var sp = cam.WorldToScreenPoint(_corners[i]);
                        e.Quad[i] = new Vector2(sp.x, Screen.height - sp.y);
                    }
                var mid = Vector3.zero;
                for (int i = 0; i < n; i++) mid += _corners[i];
                e.Depth = cam.WorldToScreenPoint(mid / n).z;
                e.OnScreen = e.ScreenRect.width > 2 && e.ScreenRect.height > 2 && e.ScreenRect.width < Screen.width * 0.95f;
            }
            // A card more than half hidden behind a nearer one (e.g. the big card being played) gets no overlay labels.
            // Attachments (Equipment, Auras) lie under their host by design (LayoutAttachments lifts the host), but their centre
            // can measure nearer to the tilted camera — they never count as covering the card they're attached to, or the
            // creature would lose its power/toughness tag and combat frames when equipped.
            var shown = _cards.Values.Where(c => c.OnScreen).ToList();
            foreach (var e in shown)
            {
                e.Covered = false;
                float area = e.ScreenRect.width * e.ScreenRect.height;
                foreach (var f in shown)
                {
                    if (f == e || f.Depth >= e.Depth - 0.001f || IsAttachedUnder(f, e)) continue;
                    if (Overlap(e.ScreenRect, f.ScreenRect) > area * 0.5f) { e.Covered = true; break; }
                }
            }
            if (!_rectsLogged && _cards.Count > 0)
            {
                _rectsLogged = true;
                var first = _cards.Values.FirstOrDefault(c => c.OnScreen);
                Plugin.Log.LogInfo($"MTG table: {_cards.Values.Count(c => c.OnScreen)}/{_cards.Count} cards have click areas" +
                                   (first != null ? $", e.g. {first.Key} at {first.ScreenRect}" : "") + $" (camera {cam.name})");
            }
        }

        /// <summary>True when <paramref name="f"/> is attached to <paramref name="host"/>, directly or through other attachments.</summary>
        private bool IsAttachedUnder(Entry f, Entry host)
        {
            int hostId = (int?)host.Json?["id"] ?? -1;
            if (hostId < 0) return false;
            var c = f.Json;
            for (int depth = 0; depth < 4 && c != null; depth++)
            {
                if (c["attachedTo"]?.Type != JTokenType.Integer) return false;
                int to = (int)c["attachedTo"];
                if (to == hostId) return true;
                c = _cards.Values.FirstOrDefault(x => (int?)x.Json?["id"] == to)?.Json; // attached to an attachment of the host?
            }
            return false;
        }

        private static string _rectSource; // which card part gave usable click rects (logged once)

        /// <summary>
        /// World corners of the card's face. Candidates (card background/border/front images, the CardUI rect, the back mesh) are
        /// tried in order and the first whose projected size is plausible (0.4%–45% of the screen) wins.
        /// </summary>
        private static float Overlap(Rect a, Rect b)
        {
            float w = Mathf.Min(a.xMax, b.xMax) - Mathf.Max(a.xMin, b.xMin);
            float h = Mathf.Min(a.yMax, b.yMax) - Mathf.Max(a.yMin, b.yMin);
            return w > 0 && h > 0 ? w * h : 0f;
        }

        /// <summary>
        /// World corners of the visible card, for tight frames and click areas. CardBG overflows its mask (CardBGMask = the
        /// visible card), which made frames look like loose zones around the card; so the candidates (mask, front frame, BG,
        /// the back mesh's own oriented bounds) are all measured and the smallest plausible one wins. Sizes are logged once.
        /// </summary>
        private int FaceCorners(Entry e)
        {
            var group = e.Card != null ? e.Card.m_Card3dUI : null;
            if (group == null) return 0;
            var cam = GameCamera;
            var ui = group.m_CardUI;
            var rects = new List<(string name, RectTransform rt)>();
            if (ui != null)
            {
                if (ui.m_CardBGImage != null && ui.m_CardBGImage.transform.parent is RectTransform mask) rects.Add(("CardBGMask", mask));
                if (ui.m_CardFrontImage != null) rects.Add(("CardFrontImage", ui.m_CardFrontImage.rectTransform));
                if (ui.m_CardBGImage != null) rects.Add(("CardBGImage", ui.m_CardBGImage.rectTransform));
            }
            string best = null;
            float bestArea = float.MaxValue;
            var four = new Vector3[4];
            var sizes = _sizesLogged ? null : new List<string>();
            foreach (var (name, rt) in rects)
            {
                rt.GetWorldCorners(four);
                Consider(name, four, cam, ref best, ref bestArea, sizes);
            }
            if (BackMeshCorners(group, four)) Consider("CardBackMesh", four, cam, ref best, ref bestArea, sizes);
            if (sizes != null && sizes.Count > 0)
            {
                _sizesLogged = true;
                Plugin.Log.LogInfo("MTG table: card outline candidates (world w×h) " + string.Join(", ", sizes) + $" → {best}");
            }
            if (best == null) return 0;
            LogSource(best);
            return 4;
        }

        private bool _sizesLogged;

        /// <summary>Keeps the smallest plausible candidate (projected screen area) in <see cref="_corners"/>.</summary>
        private void Consider(string name, Vector3[] four, Camera cam, ref string best, ref float bestArea, List<string> sizes)
        {
            sizes?.Add($"{name} {(four[3] - four[0]).magnitude:F3}×{(four[1] - four[0]).magnitude:F3}");
            if (cam == null || !Plausible(cam, four)) return;
            var p = new Vector2[4];
            for (int i = 0; i < 4; i++) p[i] = cam.WorldToScreenPoint(four[i]);
            float area = Mathf.Abs((p[2].x - p[0].x) * (p[3].y - p[1].y) - (p[3].x - p[1].x) * (p[2].y - p[0].y)) / 2f;
            if (area >= bestArea) return;
            bestArea = area;
            best = name;
            for (int i = 0; i < 4; i++) _corners[i] = four[i];
        }

        /// <summary>The card mesh's face as 4 corners in order, from its local bounds (oriented with the card, unlike renderer.bounds;
        /// mesh.bounds is available even though game meshes aren't CPU-readable). The thinnest axis is the card's thickness.</summary>
        private static bool BackMeshCorners(Card3dUIGroup group, Vector3[] four)
        {
            var mf = group.m_CardBackMesh != null ? group.m_CardBackMesh.GetComponentInChildren<MeshFilter>(true) : null;
            if (mf == null || mf.sharedMesh == null) return false;
            var b = mf.sharedMesh.bounds;
            var ext = b.extents;
            int thin = ext.x <= ext.y && ext.x <= ext.z ? 0 : ext.y <= ext.z ? 1 : 2;
            int u = thin == 0 ? 1 : 0, v = thin == 2 ? 1 : 2;
            var su = new[] { -1f, -1f, 1f, 1f };
            var sv = new[] { -1f, 1f, 1f, -1f };
            for (int i = 0; i < 4; i++)
            {
                var local = b.center;
                local[u] += ext[u] * su[i];
                local[v] += ext[v] * sv[i];
                four[i] = mf.transform.TransformPoint(local);
            }
            return true;
        }

        private static bool Plausible(Camera cam, Vector3[] corners)
        {
            float xMin = float.MaxValue, xMax = float.MinValue, yMin = float.MaxValue, yMax = float.MinValue;
            foreach (var c in corners)
            {
                var sp = cam.WorldToScreenPoint(c);
                if (sp.z < 0) return false;
                xMin = Mathf.Min(xMin, sp.x); xMax = Mathf.Max(xMax, sp.x);
                yMin = Mathf.Min(yMin, sp.y); yMax = Mathf.Max(yMax, sp.y);
            }
            float w = (xMax - xMin) / Screen.width, h = (yMax - yMin) / Screen.height;
            return w > 0.002f && h > 0.002f && w < 0.95f && h < 0.95f; // big is fine (the card being played pops up large)
        }

        private static void LogSource(string name)
        {
            if (_rectSource == name) return;
            _rectSource = name;
            Plugin.Log.LogInfo($"MTG table: card click areas from {name}");
        }
    }
}
