using System.Collections.Generic;
using System.Linq;
using System.Reflection;
using System.Reflection.Emit;
using HarmonyLib;
using TCGCustomCards.Runtime;
using UnityEngine;

namespace TCGCustomCards.Patches
{
    /// <summary>
    /// Packs with more (or fewer) than 7 cards. Vanilla's opening sequence ends the card-by-card reveal at a literal 7
    /// (<c>m_CurrentOpenedCardIndex &gt;= 7</c>, CardOpeningSequence.cs:384/:419) and bounds the "next card value" look-ahead
    /// at a literal 6 (:332/:403); everything else loops over <c>m_Card3dUIList.Count</c>. The transpiler turns those
    /// literals into the slot count of the pack being opened (7 for vanilla packs, so vanilla behaves exactly as before).
    /// </summary>
    internal static class PackSizePatches
    {
        /// <summary>False when the transpiler didn't find vanilla's literals (game update): packs then stay at 7 cards.</summary>
        internal static bool RevealPatched { get; private set; }

        /// <summary>Cards in the pack being revealed = card slots in use.</summary>
        public static int Cards(CardOpeningSequence s) => s.m_Card3dUIList.Count;

        public static int LastIndex(CardOpeningSequence s) => s.m_Card3dUIList.Count - 1;

        [HarmonyPatch(typeof(CardOpeningSequence), "Update")]
        internal static class RevealCountPatch
        {
            private static readonly FieldInfo OpenedIndex = AccessTools.Field(typeof(CardOpeningSequence), "m_CurrentOpenedCardIndex");

            private static IEnumerable<CodeInstruction> Transpiler(IEnumerable<CodeInstruction> instructions)
            {
                var code = instructions.ToList();
                // Only literals compared with m_CurrentOpenedCardIndex ("m_StateIndex = 7" etc. stay as they are).
                var hits = new List<int>();
                for (int i = 1; i < code.Count; i++)
                    if (code[i - 1].LoadsField(OpenedIndex) && (code[i].opcode == OpCodes.Ldc_I4_7 || code[i].opcode == OpCodes.Ldc_I4_6))
                        hits.Add(i);
                int sevens = hits.Count(i => code[i].opcode == OpCodes.Ldc_I4_7), sixes = hits.Count - sevens;
                RevealPatched = sevens == 2 && sixes == 2;
                if (!RevealPatched)
                {
                    Plugin.Log.LogError($"Pack opening: vanilla card-count literals not found (7×{sevens}, 6×{sixes}, expected 2 each) - game updated? Packs stay at 7 cards.");
                    return code; // nothing changed: vanilla as it is
                }
                for (int k = hits.Count - 1; k >= 0; k--) // back to front: inserts don't shift the remaining hits
                {
                    int i = hits[k];
                    bool seven = code[i].opcode == OpCodes.Ldc_I4_7;
                    code[i].opcode = OpCodes.Ldarg_0; // keeps the instruction's labels
                    code[i].operand = null;
                    code.Insert(i + 1, new CodeInstruction(OpCodes.Call, AccessTools.Method(typeof(PackSizePatches), seven ? nameof(Cards) : nameof(LastIndex))));
                }
                Plugin.Log.LogInfo("Pack opening: card count follows the pack (reveal patched)");
                return code;
            }
        }

        /// <summary>Vanilla's card pools hold 10 CardData; our roller fills one per card, so bigger packs need bigger pools.</summary>
        internal static void EnsurePool(List<CardData> pool, int cards)
        {
            while (pool.Count < cards) pool.Add(new CardData());
        }

        // ------------------------------------------------------------------ card slots per opening

        /// <summary>Most cards a pack may have (set.json cardsPerPack); the final reveal fits up to 3 rows of 8.</summary>
        internal const int MaxCards = 24;

        // Measured (docs/runtime-facts.md "Pack opening"): each slot is CardOpeningGrp/Card3dUIGroup (i) with the anim root
        // AnimGrp as its child; slots stack 0.001 apart in local z; the 7 final-reveal positions are one row under
        // ShowAllCardPosList (x -0.1..0.11 step 0.035, z 0.044, rotated 350° about y). The final-reveal clip moves neither
        // the root's position nor its scale.
        private static List<Card3dUIGroup> _ui;              // every slot: vanilla's 7 first, then our clones
        private static List<Animation> _anim;
        private static List<Transform> _vanillaPos;          // vanilla's 7 final-reveal positions
        private static readonly Dictionary<int, (List<Transform> pos, float scale)> Layouts = new Dictionary<int, (List<Transform>, float)>();
        private static float _finalScale = 1f;               // card scale in the final reveal of the current pack

        /// <summary>Cards in a pack of this type: its set.json cardsPerPack for custom packs, 7 for vanilla ones.</summary>
        internal static int CardsIn(ECollectionPackType type)
        {
            if (!RevealPatched || !Registry.IsCustom(type)) return 7;
            var pack = Registry.Get(type);
            return pack == null ? 7 : Mathf.Clamp(pack.Def.CardsPerPack, 1, MaxCards);
        }

        /// <summary>
        /// Before each opening: the sequence's three lists hold exactly the slots this pack needs (every vanilla loop then runs
        /// over the right number of cards), with the final-reveal layout for that count. A 7-card pack gets vanilla's own
        /// lists back, untouched.
        /// </summary>
        [HarmonyPatch(typeof(CardOpeningSequence), nameof(CardOpeningSequence.OpenScreen))]
        internal static class SlotsPerPackPatch
        {
            private static void Prefix(CardOpeningSequence __instance, ECollectionPackType collectionPackType)
            {
                try
                {
                    Use(__instance, CardsIn(collectionPackType));
                }
                catch (System.Exception e)
                {
                    Plugin.Log.LogError($"Pack opening: couldn't set up card slots: {e}");
                    Use(__instance, 7);
                }
            }
        }

        /// <summary>Final reveal: cards of a big pack shrink to fit (vanilla 7 keep scale 1). Set while state 7 lays them out.</summary>
        [HarmonyPatch(typeof(CardOpeningSequence), "Update")]
        internal static class FinalScalePatch
        {
            private static void Prefix(CardOpeningSequence __instance)
            {
                if (__instance.m_StateIndex != 7 || Mathf.Approximately(_finalScale, 1f)) return;
                foreach (var g in __instance.m_Card3dUIList)
                    if (g != null && g.m_ScaleGrp != null) g.m_ScaleGrp.localScale = Vector3.one * _finalScale;
            }
        }

        private static CardOpeningSequence _owner;            // the opening the cached slots/layouts belong to

        private static void Use(CardOpeningSequence s, int cards)
        {
            // Loading a save reloads the shop scene: a new opening with new card slots, the old ones destroyed. Everything
            // cached here (slots, clones, layout positions) belonged to the old scene - start over from the new one.
            if (_ui != null && (_owner != s || _ui.Any(g => g == null) || _vanillaPos.Any(t => t == null)))
            {
                _ui = null;
                _anim = null;
                _vanillaPos = null;
                Layouts.Clear();
                Plugin.Log.LogInfo("Pack opening: new scene, card slots set up again");
            }
            _owner = s;
            if (_ui == null)
            {
                // First opening: remember vanilla's slots and positions before anything changes.
                _ui = new List<Card3dUIGroup>(s.m_Card3dUIList);
                _anim = new List<Animation>(s.m_CardAnimList);
                _vanillaPos = new List<Transform>(s.m_ShowAllCardPosList);
                Layouts[_ui.Count] = (_vanillaPos, 1f);
            }
            EnsureSlots(cards);
            // Every slot back to full size (the reveal shows each card big; the final layout scales them in state 7).
            foreach (var g in _ui)
                if (g != null && g.m_ScaleGrp != null) g.m_ScaleGrp.localScale = Vector3.one;
            for (int i = cards; i < _ui.Count; i++) _ui[i].gameObject.SetActive(false); // slots this pack doesn't use
            var layout = LayoutFor(cards);
            _finalScale = layout.scale;
            s.m_Card3dUIList = _ui.Take(cards).ToList();
            s.m_CardAnimList = _anim.Take(cards).ToList();
            s.m_ShowAllCardPosList = layout.pos;
        }

        /// <summary>Clones the last vanilla slot until there are enough (clones keep stacking 0.001 apart like vanilla's).</summary>
        private static void EnsureSlots(int cards)
        {
            if (_ui.Count >= cards) return;
            var src = _ui[_ui.Count - 1];
            var prev = _ui[_ui.Count - 2];
            var step = src.transform.localPosition - prev.transform.localPosition;
            string animPath = RelativePath(src.transform, _anim[_ui.Count - 1].transform);
            while (_ui.Count < cards)
            {
                var last = _ui[_ui.Count - 1];
                bool wasActive = src.gameObject.activeSelf;
                src.gameObject.SetActive(false); // clone inactive: no Awake/OnEnable before it's cleaned up
                var go = Object.Instantiate(src.gameObject, src.transform.parent);
                src.gameObject.SetActive(wasActive);
                go.name = $"Card3dUIGroup ({_ui.Count}) TCGCC";
                go.transform.localPosition = last.transform.localPosition + step;
                go.transform.localRotation = last.transform.localRotation;
                go.transform.localScale = last.transform.localScale;
                // Our card-face add-ons attach to each CardUI on its first SetCardUI: drop copies so they attach to the clone.
                foreach (var c in go.GetComponentsInChildren<FullImageFoilGlow>(true)) Object.DestroyImmediate(c);
                foreach (var c in go.GetComponentsInChildren<FullImageGradeLabel>(true)) Object.DestroyImmediate(c);
                foreach (var t in go.GetComponentsInChildren<Transform>(true).Where(t => t != null && t.name.StartsWith("TCGCC_")).ToList())
                    if (t != null) Object.DestroyImmediate(t.gameObject);
                var g = go.GetComponent<Card3dUIGroup>();
                var anim = (animPath.Length == 0 ? go.transform : go.transform.Find(animPath))?.GetComponent<Animation>();
                if (g == null || anim == null) throw new System.InvalidOperationException($"cloned slot has no Card3dUIGroup/Animation ('{animPath}')");
                // CardUI's link to its group is private (not copied by Instantiate) and set in Card3dUIGroup.Awake, which a clone
                // only runs when first shown, after the opening already set its card up. Without it the card back (and graded
                // case parts) were skipped on cloned slots. Link it now, like Awake does.
                if (g.m_CardUI != null) g.m_CardUI.InitCard3dUIGroup(g);
                _ui.Add(g);
                _anim.Add(anim);
            }
            Plugin.Log.LogInfo($"Pack opening: {_ui.Count} card slots (cloned {_ui.Count - 7})");
        }

        // Measured final reveal (16:9, 4K): vanilla's 7 cards make one overlapping row across nearly the whole screen width,
        // each card about 65% of the screen height, centred. A grid keeps to that area: rows stack along the camera's up,
        // cards keep vanilla's rotation and overlap ratio, and shrink so the grid is no wider than vanilla's row and no
        // taller than GridHeight card heights.
        private const int MaxPerRow = 8;
        private const float GridHeight = 1.3f;   // grid height limit in vanilla card heights (vanilla row = 1)
        private const float RowGap = 1.04f;      // row pitch in card heights

        /// <summary>Final-reveal positions + card scale for this many cards (cached per count; 7 = vanilla's own).</summary>
        private static (List<Transform> pos, float scale) LayoutFor(int cards)
        {
            if (Layouts.TryGetValue(cards, out var cached)) return cached;
            var p = _vanillaPos[0].parent;
            var first = _vanillaPos[0].localPosition;
            var lastV = _vanillaPos[_vanillaPos.Count - 1].localPosition;
            var centre = _vanillaPos.Aggregate(Vector3.zero, (a, t) => a + t.localPosition) / _vanillaPos.Count;
            var step = (lastV - first) / (_vanillaPos.Count - 1);             // vanilla's spacing along the row
            var (cardW, cardH) = CardSize(p, step.magnitude);
            // "Up" on screen, in the positions' parent space (the opening group follows the camera, so this is fixed).
            var cam = CSingleton<InteractionPlayerController>.Instance?.m_Cam;
            var up = cam != null ? p.InverseTransformDirection(cam.transform.up).normalized : Vector3.up;

            int rows = Mathf.CeilToInt(cards / (float)MaxPerRow);
            int cols = Mathf.CeilToInt(cards / (float)rows);
            float vanillaWidth = (_vanillaPos.Count - 1) * step.magnitude + cardW;
            float scale = Mathf.Min(1f,
                vanillaWidth / ((cols - 1) * step.magnitude + cardW),           // no wider than vanilla's row
                GridHeight / (rows * RowGap));                                  // fits the screen's height
            var list = new List<Transform>(cards);
            for (int i = 0; i < cards; i++)
            {
                int row = i / cols, col = i % cols;
                int inRow = Mathf.Min(cols, cards - row * cols);                 // last row may be shorter: centred
                var pos = centre
                          + step * scale * (col - (inRow - 1) / 2f)
                          + up * (cardH * RowGap * scale * ((rows - 1) / 2f - row));
                var t = new GameObject($"TCGCC_ShowAll_{cards}_{i}").transform;
                t.SetParent(p, false);
                t.localPosition = pos;
                t.localRotation = _vanillaPos[Mathf.Min(col, _vanillaPos.Count - 1)].localRotation;
                list.Add(t);
            }
            Plugin.Log.LogInfo($"Pack opening: final layout for {cards} cards = {rows} row(s) × {cols}, scale {scale:0.###} (card {cardW:0.####}×{cardH:0.####})");
            return Layouts[cards] = (list, scale);
        }

        /// <summary>A card's width and height in the positions' parent space, from a slot's back mesh (fallback: from the spacing).</summary>
        private static (float w, float h) CardSize(Transform space, float spacing)
        {
            try
            {
                var back = _ui[0].m_CardBackMesh;
                var mf = back != null ? back.GetComponent<MeshFilter>() : null;
                if (mf != null && mf.sharedMesh != null)
                {
                    var size = mf.sharedMesh.bounds.size;
                    // mesh axes → world (lossy scale) → parent space; the card lies in the mesh's two largest axes
                    var world = Vector3.Scale(size, back.transform.lossyScale);
                    var dims = new[] { Mathf.Abs(world.x), Mathf.Abs(world.y), Mathf.Abs(world.z) }.OrderByDescending(v => v).ToArray();
                    float toLocal = space.InverseTransformVector(Vector3.right * 1f).magnitude;
                    float h = dims[0] * toLocal, w = dims[1] * toLocal;
                    if (h > 0.001f && w > 0.001f && h < spacing * 10f) return (w, h);
                }
            }
            catch (System.Exception e)
            {
                Plugin.Log.LogWarning($"Pack opening: card size not measured ({e.Message}), using the spacing");
            }
            return (spacing * 1.55f, spacing * 2.2f); // measured: card ~0.055 × 0.077 at 0.035 spacing
        }

        private static string RelativePath(Transform root, Transform t)
        {
            var parts = new List<string>();
            for (var p = t; p != null && p != root; p = p.parent) parts.Insert(0, p.name);
            return string.Join("/", parts);
        }
    }
}
