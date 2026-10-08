using System.Collections.Generic;
using System.Reflection.Emit;
using HarmonyLib;
using TCGCustomCards.Core;
using TCGCustomCards.Runtime;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Patches
{
    // Custom decorations (Runtime/DecorationInjector): lookups by EDecoObject, names, deco bonus, hiding vanilla in the deco screens.

    /// <summary>Vanilla indexes m_DecoDataList by (int)EDecoObject (InventoryBase.cs:61); ours live outside it.</summary>
    [HarmonyPatch(typeof(InventoryBase), nameof(InventoryBase.GetSpawnDecoObjectPrefab))]
    internal static class DecorationPrefabLookupPatch
    {
        private static bool Prefix(EDecoObject objType, ref InteractableObject __result)
        {
            var d = Registry.GetDecoration(objType);
            if (d == null) return true;
            __result = d.Prefab;
            return false;
        }
    }

    /// <summary>Same for m_DecoPurchaseDataList (InventoryBase.cs:83). A decoration of ours that is gone gets a blank entry instead of a crash.</summary>
    [HarmonyPatch(typeof(InventoryBase), nameof(InventoryBase.GetItemDecoPurchaseData))]
    internal static class DecorationPurchaseLookupPatch
    {
        internal static readonly DecoPurchaseData Missing = new DecoPurchaseData
        {
            name = "Missing decoration", mainNameText = "", replaceNameXXXText = "", replaceNameYYYText = "", price = 0f
        };

        private static bool Prefix(EDecoObject decoObject, ref DecoPurchaseData __result)
        {
            var d = Registry.GetDecoration(decoObject);
            if (d != null) { __result = d.Purchase ?? Missing; return false; }
            if (!Registry.InDecoBlock((int)decoObject)) return true;
            __result = Missing;
            return false;
        }
    }

    /// <summary>Decoration names are I2 terms (null for ours): serve the library's name.</summary>
    [HarmonyPatch]
    internal static class DecorationNamePatch
    {
        private static IEnumerable<System.Reflection.MethodBase> TargetMethods()
        {
            yield return AccessTools.Method(typeof(DecoPurchaseData), nameof(DecoPurchaseData.GetName));
            yield return AccessTools.Method(typeof(DecoData), nameof(DecoData.GetName));
            yield return AccessTools.Method(typeof(ShopDecoData), nameof(ShopDecoData.GetName));
        }

        private static bool Prefix(object __instance, ref string __result)
        {
            var d = DecorationInjector.Of(__instance);
            if (d == null)
            {
                if (!ReferenceEquals(__instance, DecorationPurchaseLookupPatch.Missing)) return true;
                __result = DecorationPurchaseLookupPatch.Missing.name;
                return false;
            }
            __result = d.Def.Name;
            return false;
        }
    }

    /// <summary>
    /// The shop's deco bonus counts placed decorations in a list of exactly 184 slots indexed by (int)EDecoObject
    /// (CustomerManager.cs:239), out of range for ours. Size it to cover our block.
    /// </summary>
    [HarmonyPatch(typeof(CustomerManager), "EvaluateMaxCustomerCount")]
    internal static class DecorationDecoBonusPatch
    {
        public static int DecoSlots()
        {
            int max = (int)EDecoObject.MAX;
            foreach (var d in Registry.Decorations) if (d.Prefab != null) max = Mathf.Max(max, (int)d.Object + 1);
            return max;
        }

        private static IEnumerable<CodeInstruction> Transpiler(IEnumerable<CodeInstruction> instructions)
        {
            bool done = false;
            foreach (var ins in instructions)
            {
                if (!done && (ins.opcode == OpCodes.Ldc_I4_S || ins.opcode == OpCodes.Ldc_I4) && System.Convert.ToInt32(ins.operand) == (int)EDecoObject.MAX)
                {
                    done = true;
                    ins.opcode = OpCodes.Call;
                    ins.operand = AccessTools.Method(typeof(DecorationDecoBonusPatch), nameof(DecoSlots));
                }
                yield return ins;
            }
            if (!done) Plugin.Log.LogWarning("Deco bonus: decoration slot count not found — custom decorations may break the customer count");
        }
    }

    /// <summary>
    /// Hiding vanilla decorations ([Content - Decorations] ShowVanilla… off). Both deco screens page through the shop lists by index
    /// (8 per page) and the panels keep that index (it is the surface's identity), so the lists can't be filtered in place. While a
    /// toggle is off, the page is built from the visible indices instead (same calls as vanilla, ShopBuyDecoUIScreen.cs /
    /// PlaceDecoUIScreen.cs EvaluatePanelUIPage). Buy app: hidden vanilla entries leave. Decorate screen: owned / bought ones stay.
    /// The default look (index 0) and the equipped look always stay.
    /// </summary>
    [HarmonyPatch]
    internal static class DecorationScreenFilterPatch
    {
        private const int PerPage = 8; // hardcoded in both screens

        private static IEnumerable<System.Reflection.MethodBase> TargetMethods()
        {
            yield return AccessTools.Method(typeof(ShopBuyDecoUIScreen), "EvaluatePanelUIPage");
            yield return AccessTools.Method(typeof(PlaceDecoUIScreen), "EvaluatePanelUIPage");
        }

        private static bool Prefix(object __instance)
        {
            if (!VanillaFilter.HidePosters && !VanillaFilter.HideDecoObjects && !VanillaFilter.HideSurfaces) return true;
            var t = Traverse.Create(__instance);
            bool buy = __instance is ShopBuyDecoUIScreen;
            bool surfaces = t.Field<bool>("m_IsShopWallFloorCeiling").Value;
            int category = t.Field<int>("m_CategoryIndex").Value;
            var panels = t.Field<List<ShopDecoPanelUI>>("m_ShopDecoPanelUIList").Value;
            var shopList = t.Field<List<ShopDecoData>>("m_CurrentShopDecoDataList").Value;
            var itemList = t.Field<List<EDecoObject>>("m_CurrentItemDecoList").Value;
            int equipped = t.Field<int>("m_CurrentEquippedShopDecoIndex").Value;
            int equippedB = t.Field<int>("m_CurrentEquippedShopDecoIndexB").Value;

            var visible = new List<int>();
            if (surfaces)
            {
                var kind = category == 0 ? DecorationKind.Wall : category == 1 ? DecorationKind.Floor : DecorationKind.Ceiling;
                for (int i = 0; i < shopList.Count; i++)
                    if (!VanillaFilter.HideSurfaces || i == 0 || i == equipped || i == equippedB || Registry.GetSurface(kind, i) != null
                        || (!buy && Unlocked(category, i)))
                        visible.Add(i);
            }
            else
            {
                bool hide = category == 3 ? VanillaFilter.HidePosters : VanillaFilter.HideDecoObjects;
                for (int i = 0; i < itemList.Count; i++)
                {
                    var deco = itemList[i];
                    if (!hide || Registry.GetDecoration(deco) != null || (!buy && Owned(deco))) visible.Add(i);
                }
            }

            int page = t.Field<int>("m_PageIndex").Value;
            int maxPage = Mathf.Max(0, (visible.Count - 1) / PerPage);
            if (page > maxPage) page = maxPage;
            t.Field<int>("m_PageIndex").Value = page;
            t.Field<int>("m_PageMaxIndex").Value = maxPage;
            t.Field<TextMeshProUGUI>("m_PageText").Value.text = page + 1 + " / " + (maxPage + 1);
            t.Field<Button>("m_PreviousPageBtn").Value.interactable = page > 0;
            t.Field<Button>("m_NextPageBtn").Value.interactable = page < maxPage;

            for (int i = 0; i < panels.Count; i++)
            {
                int k = page * PerPage + i;
                var panel = panels[i];
                if (i >= PerPage || k >= visible.Count) { panel.SetActive(isActive: false); continue; }
                int idx = visible[k];
                if (surfaces)
                {
                    panel.InitShopWallFloorCeiling(shopList[idx], idx);
                    if (buy)
                    {
                        panel.ShowPurchaseDecoButtonUI();
                        panel.EvaluateShopDecoBoughtUIState(category, idx);
                    }
                    else
                    {
                        panel.EvaluateShopDecoUnlockedState(category, idx);
                        panel.EvaluateShopDecoEquippedUIState(equipped, isShopLotB: false);
                        panel.EvaluateShopDecoEquippedUIState(equippedB, isShopLotB: true);
                    }
                }
                else
                {
                    var deco = itemList[idx];
                    panel.InitDecoObject(InventoryBase.GetItemDecoPurchaseData(deco), deco, idx);
                    if (buy) panel.ShowPurchaseDecoButtonUI();
                    panel.EvaluateOwnedDecoItemCount();
                }
                panel.SetActive(isActive: true);
            }
            return false;
        }

        private static bool Unlocked(int category, int index) =>
            category == 0 ? CPlayerData.IsDecoWallUnlocked(index) : category == 1 ? CPlayerData.IsDecoFloorUnlocked(index) : CPlayerData.IsDecoCeilingUnlocked(index);

        private static bool Owned(EDecoObject deco)
        {
            var list = CPlayerData.m_DecorationInventoryList;
            return list != null && (int)deco >= 0 && (int)deco < list.Count && list[(int)deco] > 0;
        }
    }
}
