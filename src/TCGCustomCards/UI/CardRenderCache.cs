using System.Collections;
using System.Collections.Generic;
using UnityEngine;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// Pictures of cards drawn by the game's own card renderer (borders, foil, our holo shader) for IMGUI screens (MTG deck builder,
    /// MTG table dialogs/preview). A few pooled Card3dUIGroups ("slots") are parked far below the shop — still under the
    /// Card3dUISpawner, whose world-space Canvas draws the card faces — and a disabled camera photographs them into RenderTextures
    /// (LRU-cached).
    /// Two steps per picture: a card is set up (SetCardUI) at the end of one frame and photographed at the end of the next, after
    /// every LateUpdate has run for it — our holo layer (FullImageFoilGlow.LateUpdate) and the game's foil update a frame late, so
    /// photographing right after SetCardUI put the previous card's holo on the picture.
    /// Requests come from OnGUI (the caller shows flat art until the picture exists). The hover preview has its own slot and is
    /// re-photographed every frame so holos shimmer; a second live lane serves a double-faced card's back beside it (idle otherwise).
    /// </summary>
    internal class CardRenderCache : MonoBehaviour
    {
        private const int TileW = 300, TileH = 420, LiveW = 600, LiveH = 840;
        private const int MaxCached = 160, GridSlots = 4;
        /// <summary>Holo shader clock for still pictures (with the light sweep placed across the card by the shader): every tile
        /// gets the same good-looking frozen moment instead of a random one (between two sweeps the holo looked absent).</summary>
        private const float FrozenHoloTime = 2f;
        private static readonly Vector3 Stage = new Vector3(0f, -500f, 0f); // below the shop: nothing else is there
        private static readonly Color Background = new Color(0.07f, 0.07f, 0.09f, 1f);

        private sealed class Slot
        {
            public Card3dUIGroup Card;
            public Vector3 Pos;
            public string Key;       // card staged on it (null = free)
            public CardData Data;
            public int StagedFrame;
        }

        /// <summary>A live preview: its own slot, re-photographed every frame while it's asked for (0 = card, 1 = its back face).</summary>
        private sealed class LiveLane
        {
            public Slot Slot;
            public CardData Wanted;
            public int WantedFrame = -10;
            public RenderTexture Tex;
            public string Key;
        }

        private const int LiveLanes = 2;
        private static CardRenderCache _inst;
        private Camera _cam;
        private readonly List<Slot> _grid = new List<Slot>();
        private readonly LiveLane[] _lanes = { new LiveLane(), new LiveLane() };
        private readonly Dictionary<string, RenderTexture> _cache = new Dictionary<string, RenderTexture>();
        private readonly LinkedList<string> _lru = new LinkedList<string>();
        private readonly Dictionary<string, CardData> _pending = new Dictionary<string, CardData>();

        private bool _failed, _loggedFrame;

        public static string Key(CardData d) =>
            d == null ? null : $"{(int)d.expansionType}:{(int)d.monsterType}:{(int)d.borderType}:{(d.isFoil ? 1 : 0)}:{(d.isDestiny ? 1 : 0)}:{d.cardGrade}" +
                               (Runtime.Mtg.MtgCardFaces.IsBack(d) ? ":back" : "");

        private static CardRenderCache Inst
        {
            get
            {
                if (_inst != null) return _inst;
                var go = new GameObject("TCGCC_CardRenderCache");
                DontDestroyOnLoad(go);
                _inst = go.AddComponent<CardRenderCache>();
                return _inst;
            }
        }

        /// <summary>The in-game picture of a card, or null (not ready yet — it's queued; show flat art meanwhile).</summary>
        public static Texture Get(CardData data)
        {
            if (data == null) return null;
            var c = Inst;
            if (c._failed) return null;
            string key = Key(data);
            if (c._cache.TryGetValue(key, out var rt) && rt != null && rt.IsCreated())
            {
                c._lru.Remove(key);
                c._lru.AddFirst(key);
                return rt;
            }
            if (!c.IsStaged(key)) c._pending[key] = data;
            return null;
        }

        /// <summary>Like <see cref="Get"/> but at preview size and re-photographed every frame (animated foil).</summary>
        public static Texture GetLive(CardData data, int lane = 0)
        {
            if (data == null) return null;
            var c = Inst;
            if (c._failed) return null;
            var l = c._lanes[Mathf.Clamp(lane, 0, LiveLanes - 1)];
            l.Wanted = data;
            l.WantedFrame = Time.frameCount;
            if (l.Key == Key(data) && l.Tex != null && l.Tex.IsCreated()) return l.Tex;
            return Get(data); // the tile picture meanwhile
        }

        private bool IsStaged(string key)
        {
            foreach (var s in _grid) if (s.Key == key) return true;
            return false;
        }

        private void OnEnable() => StartCoroutine(EndOfFrameLoop());

        private IEnumerator EndOfFrameLoop()
        {
            var wait = new WaitForEndOfFrame();
            while (true)
            {
                yield return wait;
                try { Step(); }
                catch (System.Exception e) { Plugin.Log.LogWarning($"Card pictures: {e.Message}"); }
            }
        }

        /// <summary>End of frame: photograph slots staged in an earlier frame, then stage the next requests.</summary>
        private void Step()
        {
            int frame = Time.frameCount;
            bool anyLive = false;
            foreach (var l in _lanes) anyLive |= frame - l.WantedFrame <= 1;
            if (_pending.Count == 0 && !anyLive && _grid.TrueForAll(s => s.Key == null)) return;
            if (!EnsureRig()) return;

            // 1) photograph what was staged before this frame
            foreach (var s in _grid)
            {
                if (s.Key == null || s.StagedFrame >= frame) continue;
                var rt = new RenderTexture(TileW, TileH, 16, RenderTextureFormat.ARGB32) { name = "TCGCC_card_" + s.Key };
                rt.Create();
                if (Photograph(s, rt, frozen: true)) Store(s.Key, rt);
                else { rt.Release(); Destroy(rt); }
                s.Key = null;
                s.Data = null;
            }
            for (int i = 0; i < _lanes.Length; i++)
            {
                var l = _lanes[i];
                if (frame - l.WantedFrame > 1 || l.Slot.Key == null || l.Slot.StagedFrame >= frame || l.Slot.Key != Key(l.Wanted)) continue;
                if (l.Tex == null)
                {
                    l.Tex = new RenderTexture(LiveW, LiveH, 16, RenderTextureFormat.ARGB32) { name = "TCGCC_card_live" + i };
                    l.Tex.Create();
                }
                if (Photograph(l.Slot, l.Tex, frozen: false)) l.Key = l.Slot.Key; // live: holo animates
            }

            // 2) stage the next cards (photographed at the end of the next frame)
            foreach (var s in _grid)
            {
                if (s.Key != null || _pending.Count == 0) continue;
                string key = null;
                CardData data = null;
                foreach (var kv in _pending) { key = kv.Key; data = kv.Value; break; }
                _pending.Remove(key);
                if (_cache.ContainsKey(key)) continue;
                StageCard(s, key, data);
            }
            foreach (var l in _lanes)
            {
                if (frame - l.WantedFrame > 1 || l.Slot.Key == Key(l.Wanted)) continue;
                l.Key = null;
                StageCard(l.Slot, Key(l.Wanted), l.Wanted);
            }
        }

        private void Store(string key, RenderTexture rt)
        {
            if (_cache.TryGetValue(key, out var old) && old != null) { old.Release(); Destroy(old); _lru.Remove(key); }
            _cache[key] = rt;
            _lru.AddFirst(key);
            while (_lru.Count > MaxCached)
            {
                string drop = _lru.Last.Value;
                _lru.RemoveLast();
                if (_cache.TryGetValue(drop, out var d) && d != null) { d.Release(); Destroy(d); }
                _cache.Remove(drop);
            }
        }

        private bool EnsureRig()
        {
            if (_cam == null)
            {
                var go = new GameObject("TCGCC_CardRenderCam");
                DontDestroyOnLoad(go);
                _cam = go.AddComponent<Camera>();
                _cam.enabled = false; // photographs by hand only
                _cam.orthographic = true;
                _cam.clearFlags = CameraClearFlags.SolidColor;
                // Opaque, like the screens behind the pictures: the card shaders write colour but not alpha.
                _cam.backgroundColor = Background;
                _cam.nearClipPlane = 0.01f;
                _cam.farClipPlane = 1f;   // only the slot in front of it (slots are 2 units apart)
                _cam.allowHDR = false;
                _cam.allowMSAA = false;
            }
            // Slots are pooled cards; a scene change destroys them (null) — take new ones then.
            bool broken = _grid.Count < GridSlots || _grid.Exists(s => s.Card == null) || System.Array.Exists(_lanes, l => l.Slot == null || l.Slot.Card == null);
            if (!broken) return true;
            var spawner = CSingleton<Card3dUISpawner>.Instance;
            if (spawner == null) return false; // not in the shop scene yet
            try
            {
                _grid.Clear();
                for (int i = 0; i < GridSlots; i++) _grid.Add(NewSlot(spawner, Stage + new Vector3(2f * i, 0f, 0f)));
                for (int i = 0; i < _lanes.Length; i++)
                {
                    _lanes[i].Slot = NewSlot(spawner, Stage + new Vector3(-2f * (i + 1), 0f, 0f));
                    _lanes[i].Key = null;
                }
                return true;
            }
            catch (System.Exception e)
            {
                Plugin.Log.LogWarning($"Card pictures unavailable (flat art used instead): {e.Message}");
                _failed = true;
                return false;
            }
        }

        private static Slot NewSlot(Card3dUISpawner spawner, Vector3 pos)
        {
            // Stays under the spawner: its world-space Canvas draws the card face (the card itself has no Canvas).
            var card = spawner.GetCardUI();
            card.transform.position = pos;
            card.transform.rotation = Quaternion.identity;
            card.m_IgnoreCulling = true; // Card3dUISpawner.Update would hide it for being far from the player
            card.SetAlwaysCulling(false);
            card.m_CardUIAnimGrp.gameObject.SetActive(true);
            card.m_ScaleGrp.localScale = Vector3.one;
            card.m_ScaleGrp.localPosition = Vector3.zero;
            card.m_ScaleGrp.localRotation = Quaternion.identity;
            return new Slot { Card = card, Pos = pos };
        }

        /// <summary>Sets a card up on a slot (same setup as PlayCardSet.GetNewCard3d); photographed next frame.</summary>
        private static void StageCard(Slot s, string key, CardData data)
        {
            s.Key = key;
            s.Data = data;
            s.StagedFrame = Time.frameCount;
            var card = s.Card;
            card.gameObject.SetActive(true);
            card.transform.position = s.Pos;
            var ui = card.m_CardUI;
            ui.SetFoilCullListVisibility(isActive: true);
            ui.ResetFarDistanceCull();
            ui.SetCardUI(data);
            ui.SetFoilMaterialListFromSettingData(isWorldView: false);
            ui.SetFoilBlendedMaterialListFromSettingData(isWorldView: false);
            ui.SetBrightness(1f); // shop lighting (night) would darken the picture
            card.EvaluateCardGrade(data);
        }

        /// <summary>Photographs a slot: camera framed on the card-back mesh (card-shaped, parallel to the face), from the face side.</summary>
        private bool Photograph(Slot s, RenderTexture rt, bool frozen)
        {
            var card = s.Card;
            var ui = card.m_CardUI;
            var rtf = (RectTransform)ui.transform;
            var back = card.m_CardBackMesh != null ? card.m_CardBackMesh.GetComponentInChildren<Renderer>(true) : null;
            if (back == null) return false;
            var b = back.bounds;
            Vector3 center = b.center;
            float w = Extent(b.size, rtf.right), h = Extent(b.size, rtf.up);
            if (h < 1e-4f || w < 1e-4f) return false;
            // Looking along the UI's +forward from in front of it: from the other side the face came out mirrored.
            var look = rtf.forward;
            _cam.transform.rotation = Quaternion.LookRotation(look, rtf.up);
            _cam.transform.position = center - look * 0.5f;
            _cam.aspect = w / h;
            _cam.orthographicSize = h / 2f * 1.02f;
            // The back mesh sits just in front of the face from this side: hide it for the shot.
            bool backWasActive = card.m_CardBackMesh.activeSelf;
            if (backWasActive) card.m_CardBackMesh.SetActive(false);
            Canvas.ForceUpdateCanvases();
            if (frozen)
            {
                Shader.SetGlobalFloat("_TCGCC_HoloFreeze", 1f); // shader globals: reach the UI mask copies of the holo material too
                Shader.SetGlobalFloat("_TCGCC_HoloTime", FrozenHoloTime);
            }
            _cam.targetTexture = rt;
            _cam.Render();
            _cam.targetTexture = null;
            if (frozen) Shader.SetGlobalFloat("_TCGCC_HoloFreeze", 0f); // back to live for everything else
            if (backWasActive) card.m_CardBackMesh.SetActive(true);
            if (!_loggedFrame)
            {
                _loggedFrame = true;
                Plugin.Log.LogInfo($"Card pictures: card {w:F3}×{h:F3}, {GridSlots} grid slots + {LiveLanes} preview slots");
            }
            return true;
        }

        /// <summary>Size of an axis-aligned box along a direction (exact when the card is aligned with the world axes).</summary>
        private static float Extent(Vector3 size, Vector3 dir) =>
            Mathf.Abs(dir.x) * size.x + Mathf.Abs(dir.y) * size.y + Mathf.Abs(dir.z) * size.z;
    }
}
