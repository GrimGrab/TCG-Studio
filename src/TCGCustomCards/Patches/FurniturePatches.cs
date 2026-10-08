using System.Collections.Generic;
using System.Linq;
using System.Reflection.Emit;
using HarmonyLib;
using TCGCustomCards.Runtime;
using TCGCustomCards.UI;
using UnityEngine;

namespace TCGCustomCards.Patches
{
    // Custom furniture (Runtime/FurnitureInjector): lookups, names, shop list, deco bonus.

    /// <summary>
    /// Vanilla loops over the object list's count but indexes the purchase list (InventoryBase.cs:71), which only works while every
    /// searched type is found early. Search the purchase list itself.
    /// </summary>
    [HarmonyPatch(typeof(InventoryBase), nameof(InventoryBase.GetFurniturePurchaseData), typeof(EObjectType))]
    internal static class FurniturePurchaseLookupPatch
    {
        private static bool Prefix(EObjectType objType, ref FurniturePurchaseData __result)
        {
            var list = CSingleton<InventoryBase>.Instance.m_ObjectData_SO?.m_FurniturePurchaseDataList;
            if (list == null) return true;
            __result = null;
            for (int i = 0; i < list.Count; i++)
                if (list[i] != null && list[i].objectType == objType) { __result = list[i]; break; }
            return false;
        }
    }

    /// <summary>Furniture names/descriptions are I2 terms (null for ours): serve the library's text.</summary>
    [HarmonyPatch(typeof(FurniturePurchaseData), nameof(FurniturePurchaseData.GetName))]
    internal static class FurnitureNamePatch
    {
        private static bool Prefix(FurniturePurchaseData __instance, ref string __result)
        {
            var f = Registry.GetFurniture(__instance.objectType);
            if (f == null) return true;
            __result = f.Def.Name;
            return false;
        }
    }

    [HarmonyPatch(typeof(FurniturePurchaseData), nameof(FurniturePurchaseData.GetDescription))]
    internal static class FurnitureDescriptionPatch
    {
        private static bool Prefix(FurniturePurchaseData __instance, ref string __result)
        {
            var f = Registry.GetFurniture(__instance.objectType);
            if (f == null || string.IsNullOrEmpty(f.Def.Description)) return true; // base piece's description term
            __result = f.Def.Description;
            return false;
        }
    }

    [HarmonyPatch(typeof(ObjectData), nameof(ObjectData.GetName))]
    internal static class FurnitureObjectNamePatch
    {
        private static bool Prefix(ObjectData __instance, ref string __result)
        {
            var f = Registry.GetFurniture(__instance.objectType);
            if (f == null) return true;
            __result = f.Def.Name;
            return false;
        }
    }

    /// <summary>
    /// The shop's panel pool is fixed in the scene and the vanilla Init indexes it by purchase entry (FurnitureShopUIScreen.cs:17-23):
    /// grow it first. Afterwards hide vanilla pieces in custom-only mode and end the scroll at the last shown panel.
    /// </summary>
    [HarmonyPatch(typeof(FurnitureShopUIScreen), "Init")]
    internal static class FurnitureShopListPatch
    {
        private static void Prefix(FurnitureShopUIScreen __instance)
        {
            int count = CSingleton<InventoryBase>.Instance.m_ObjectData_SO.m_FurniturePurchaseDataList.Count;
            UiPool.Grow(__instance.m_FurnitureShopPanelUIList, count, "Furniture shop");
        }

        private static void Postfix(FurnitureShopUIScreen __instance)
        {
            if (!VanillaFilter.HideFurniture) return;
            var list = CSingleton<InventoryBase>.Instance.m_ObjectData_SO.m_FurniturePurchaseDataList;
            var panels = __instance.m_FurnitureShopPanelUIList;
            FurnitureShopPanelUI last = null;
            for (int i = 0; i < list.Count && i < panels.Count; i++)
            {
                if (VanillaFilter.IsHiddenFurniture(list[i].objectType)) panels[i].SetActive(isActive: false);
                else last = panels[i];
            }
            // Same as GenericSliderScreen.Init with m_ScrollEndPosParent = the last shown panel.
            if (last != null && __instance.m_ScrollEndPos != null)
            {
                __instance.m_ScrollEndPos.transform.SetParent(last.transform, worldPositionStays: false);
                __instance.m_ScrollEndPos.transform.localPosition = new Vector3(0f, __instance.m_ScrollEndPosOffsetYAmount, 0f);
            }
        }
    }

    /// <summary>
    /// The till and card screens only follow their point's position and rotation (FollowObject): a custom counter's screen size (its
    /// point's scale: "screen", and "cardMachine" for the card screen on it) is applied to the spawned screen.
    /// </summary>
    [HarmonyPatch]
    internal static class FurnitureScreenScalePatch
    {
        private static IEnumerable<System.Reflection.MethodBase> TargetMethods()
        {
            yield return AccessTools.Method(typeof(WorldCanvasUIManager), nameof(WorldCanvasUIManager.SpawnCashCounterScreenUI));
            yield return AccessTools.Method(typeof(WorldCanvasUIManager), nameof(WorldCanvasUIManager.SpawnCreditCardScreenUI));
        }

        private static void Postfix(Transform followTarget, MonoBehaviour __result, System.Reflection.MethodBase __originalMethod)
        {
            var piece = followTarget != null ? followTarget.GetComponentInParent<InteractableObject>() : null;
            var f = piece != null ? Registry.GetFurniture(piece.m_ObjectType) : null;
            if (f?.Def.Points == null || __result == null) return;
            string role = __originalMethod.Name == nameof(WorldCanvasUIManager.SpawnCashCounterScreenUI) ? "screen" : "cardMachine";
            var pt = f.Def.Points.FirstOrDefault(p => p.Role == role);
            if (pt?.Scale is float sc && sc > 0 && sc != 1f) __result.transform.localScale *= sc;
        }
    }

    /// <summary>
    /// The shop's deco bonus counts placed furniture in a list of exactly 57 slots indexed by (int)objectType (CustomerManager.cs:256),
    /// which is out of range for our types. Size it to cover our block.
    /// </summary>
    [HarmonyPatch(typeof(CustomerManager), "EvaluateMaxCustomerCount")]
    internal static class FurnitureDecoBonusPatch
    {
        public static int ObjectSlots()
        {
            int max = (int)EObjectType.MAX;
            foreach (var f in Registry.Furniture) if (f.Prefab != null) max = Mathf.Max(max, (int)f.Object + 1);
            return max;
        }

        private static IEnumerable<CodeInstruction> Transpiler(IEnumerable<CodeInstruction> instructions)
        {
            bool done = false;
            foreach (var ins in instructions)
            {
                if (!done && (ins.opcode == OpCodes.Ldc_I4_S || ins.opcode == OpCodes.Ldc_I4) && System.Convert.ToInt32(ins.operand) == (int)EObjectType.MAX)
                {
                    done = true;
                    // Same instruction object: keeps its labels.
                    ins.opcode = OpCodes.Call;
                    ins.operand = AccessTools.Method(typeof(FurnitureDecoBonusPatch), nameof(ObjectSlots));
                }
                yield return ins;
            }
            if (!done) Plugin.Log.LogWarning("Deco bonus: object slot count not found — custom furniture may break the customer count");
        }
    }
}
