using System;
using System.Collections.Generic;
using System.Linq;
using HarmonyLib;
using UnityEngine;

namespace TCGCustomCards.Hooks
{
    /// <summary>
    /// Single owner of the patches on what the player carries (InteractionPlayerController.RaycastHoldItemState /
    /// EvaluateTakeItemFromShelf / EvaluateOpenCardPack): count and use up held items, and a right-click action on whatever
    /// the player looks at while holding card packs (vanilla: right-click takes an item from that shelf; the open-pack key
    /// opens the front pack). See docs/hooks.md.
    /// </summary>
    internal static class PlayerItems
    {
        /// <summary>
        /// Offered when the player looks at something (the raycast hit) while holding card packs: returns the hint text (e.g.
        /// "Add 4 packs to the draft stock") or null when nothing is offered there. Right-click runs <see cref="PackTargetRun"/>.
        /// </summary>
        public static Func<Transform, string> PackTargetLabel { get; set; }
        public static Action<Transform> PackTargetRun { get; set; }

        private static readonly AccessTools.FieldRef<InteractionPlayerController, List<Item>> HoldList =
            AccessTools.FieldRefAccess<InteractionPlayerController, List<Item>>("m_HoldItemList");
        private static readonly Action<InteractionPlayerController, Item> RemoveHoldItem =
            AccessTools.MethodDelegate<Action<InteractionPlayerController, Item>>(AccessTools.Method(typeof(InteractionPlayerController), "RemoveHoldItem"));

        private static List<Item> Held()
        {
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            return ipc == null ? new List<Item>() : HoldList(ipc) ?? new List<Item>();
        }

        public static int HeldCount(Func<EItemType, bool> match) => Held().Count(i => i != null && match(i.GetItemType()));

        /// <summary>Types of the held items, front first.</summary>
        public static List<EItemType> HeldTypes() => Held().Where(i => i != null).Select(i => i.GetItemType()).ToList();

        /// <summary>Uses up to <paramref name="n"/> matching held items for good (front first); returns their types.</summary>
        public static List<EItemType> ConsumeHeld(Func<EItemType, bool> match, int n)
        {
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            var taken = new List<EItemType>();
            if (ipc == null) return taken;
            var held = HoldList(ipc);
            var types = CPlayerData.m_HoldItemTypeList;
            foreach (var item in held.ToList())
            {
                if (taken.Count >= n) break;
                if (item == null || !match(item.GetItemType())) continue;
                // Vanilla's RemoveHoldItem drops the item and m_HoldItemTypeList[0] and resets hold mode when the hand is empty,
                // so move this item (and its type entry, the lists are parallel) to the front first.
                int i = held.IndexOf(item);
                held.RemoveAt(i);
                held.Insert(0, item);
                if (i < types.Count)
                {
                    var t = types[i];
                    types.RemoveAt(i);
                    types.Insert(0, t);
                }
                RemoveHoldItem(ipc, item);
                taken.Add(item.GetItemType());
                item.DisableItem();
            }
            return taken;
        }

        // ---- pack + table right-click

        private static Transform _target;
        private static string _label;
        private static int _frame = -10;

        private static bool Offered => _label != null && _target != null && Time.frameCount - _frame <= 1;

        [HarmonyPatch(typeof(InteractionPlayerController), "RaycastHoldItemState")]
        private static class LookPatch
        {
            private static void Postfix(InteractionPlayerController __instance)
            {
                _label = null;
                _target = null;
                var cam = __instance.m_Cam;
                if (PackTargetLabel == null || cam == null || !__instance.CanOpenPack()) return;
                int mask = LayerMask.GetMask("ShopModel", "ItemCompartment", "Physics", "Obstacles"); // vanilla's hold-mode mask
                if (!Physics.Raycast(new Ray(cam.transform.position, cam.transform.forward), out var hit, __instance.m_RayDistance, mask)) return;
                _label = PackTargetLabel(hit.transform);
                if (_label == null) return;
                _target = hit.transform;
                _frame = Time.frameCount;
                HooksRunner.Ensure();
            }
        }

        /// <summary>Runs the offered action instead of the vanilla one; false = nothing offered (vanilla runs).</summary>
        private static bool RunOffered()
        {
            if (!Offered || PackTargetRun == null) return false;
            var target = _target;
            _label = null;
            PackTargetRun(target);
            return true;
        }

        /// <summary>Right-click while holding items (vanilla: take an item from the shelf you look at).</summary>
        [HarmonyPatch(typeof(InteractionPlayerController), "EvaluateTakeItemFromShelf")]
        private static class RightClickPatch
        {
            private static bool Prefix() => !RunOffered();
        }

        /// <summary>The open-pack key while holding a pack (vanilla: open the front pack).</summary>
        [HarmonyPatch(typeof(InteractionPlayerController), nameof(InteractionPlayerController.EvaluateOpenCardPack))]
        private static class OpenPackKeyPatch
        {
            private static bool Prefix() => !RunOffered();
        }

        /// <summary>Hint under the crosshair while a pack-table action is offered.</summary>
        internal static void DrawHint()
        {
            if (!Offered) return;
            var style = new GUIStyle(GUI.skin.label) { alignment = TextAnchor.MiddleCenter, fontSize = Mathf.RoundToInt(Screen.height / 45f) };
            var text = "Right-click: " + _label;
            var r = new Rect(0, Screen.height * 0.58f, Screen.width, Screen.height / 20f);
            style.normal.textColor = Color.black;
            GUI.Label(new Rect(r.x + 2, r.y + 2, r.width, r.height), text, style);
            style.normal.textColor = Color.white;
            GUI.Label(r, text, style);
        }
    }

    /// <summary>Per-frame/OnGUI host for the hooks layer (created on first need, survives scene loads).</summary>
    internal class HooksRunner : MonoBehaviour
    {
        private static HooksRunner _instance;

        public static void Ensure()
        {
            if (_instance != null) return;
            var go = new GameObject("TCGCC_Hooks");
            DontDestroyOnLoad(go);
            _instance = go.AddComponent<HooksRunner>();
        }

        private void Update() => Patches.CardFlip.Tick();

        private void OnGUI()
        {
            PlayerItems.DrawHint();
            Patches.CardFlip.DrawHint();
        }
    }
}
