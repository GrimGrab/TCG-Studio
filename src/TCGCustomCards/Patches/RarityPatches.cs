using System.Collections.Generic;
using System.Linq;
using HarmonyLib;
using TCGCustomCards.Core;
using TCGCustomCards.Runtime;
using TCGCustomCards.UI;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Patches
{
    // Sets' own rarities (set.json "rarities"). Like Enhanced Prefab Loader, every rarity that isn't a vanilla name is a real ERarity
    // value of its own (Registry.RarityBase+, allocated per launch); the vanilla save never keeps one (SaveStripper). Vanilla code
    // that assumes 4 rarities is patched here: name, icon, binder rarity sort, the workbench / bulk donation rarity filter, fame.
    // Not patched (as in EPL): the daily rarity price events, which only match vanilla rarities.

    /// <summary>Card text, binder, Check Price, graded slabs, price graph and prize names (all via GetRarityName / GetFullCardTypeName).</summary>
    [HarmonyPatch(typeof(MonsterData), nameof(MonsterData.GetRarityName))]
    internal static class RarityNamePatch
    {
        private static bool Prefix(MonsterData __instance, ref string __result)
        {
            var r = Registry.RarityOf(__instance.MonsterType);
            if (r == null) return true;
            __result = r.Name;
            return false;
        }
    }

    /// <summary>
    /// The game's rarity icon for the rarity's place in the list (vanilla would show the Common icon for values past its list).
    /// Vanilla sets the sprite on every SetCardUI, so reused CardUIs need no reset.
    /// </summary>
    [HarmonyPatch(typeof(CardUI), nameof(CardUI.SetCardUI))]
    internal static class RarityIconPatch
    {
        private static readonly AccessTools.FieldRef<CardUI, CardUISettingData> SettingData = AccessTools.FieldRefAccess<CardUI, CardUISettingData>("m_CardUISettingData");

        private static void Postfix(CardUI __instance, CardData cardData)
        {
            if (__instance.m_RarityImage == null || !Registry.TryGetCard(cardData.monsterType, out var set, out int pos)) return;
            var data = SettingData(__instance);
            if (data != null) __instance.m_RarityImage.sprite = data.GetCardRaritySprite(set.Rarity(pos).Tier);
        }
    }

    /// <summary>
    /// Vanilla sorts by rarity with a loop over the 4 vanilla values, so cards of any other rarity vanish from the binder. Custom sets
    /// are sorted by their rarity list instead (same order inside a rarity, same compact-view handling). Graded albums compare values
    /// and need nothing.
    /// </summary>
    [HarmonyPatch(typeof(CollectionBinderFlipAnimCtrl), "SortByCardRarity")]
    internal static class BinderRaritySortPatch
    {
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, bool> Graded = AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, bool>("m_IsGradedCardAlbum");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, ECardExpansionType> Expansion = AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, ECardExpansionType>("m_ExpansionType");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, List<int>> Sorted = AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, List<int>>("m_SortedIndexList");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, List<int>> Temp = AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, List<int>>("m_SortTempList");

        private static bool Prefix(CollectionBinderFlipAnimCtrl __instance, bool isCompactView)
        {
            if (Graded(__instance)) return true;
            var exp = Expansion(__instance);
            var set = Registry.Get(exp);
            if (set == null) return true;
            var sorted = Sorted(__instance);
            var temp = Temp(__instance);
            sorted.Clear();
            temp.Clear();
            int per = CPlayerData.GetCardAmountPerMonsterType(exp);
            var order = Enumerable.Range(0, set.Shown.Count).OrderBy(p => set.Rarity(p).Rank); // stable: card order inside a rarity
            foreach (int pos in order)
                for (int n = 0; n < per; n++)
                {
                    int slot = pos * per + n;
                    if (isCompactView && CPlayerData.GetCardAmountByIndex(slot, exp, isDimensionCard: false) == 0) temp.Add(slot);
                    else sorted.Add(slot);
                }
            sorted.AddRange(temp);
            return false;
        }
    }

    /// <summary>Fame counts a custom rarity like the vanilla rarity for its place in the list (vanilla adds nothing for values it doesn't know).</summary>
    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.GetCardFameAmount))]
    internal static class RarityFamePatch
    {
        private static void Prefix(CardData cardData, ref MonsterData __state)
        {
            // Swap in the vanilla value for the duration of the call; restored in the postfix.
            var r = Registry.RarityOf(cardData.monsterType);
            if (r == null || !Registry.IsCustomRarity(r.Value)) return;
            __state = InventoryBase.GetMonsterData(cardData.monsterType);
            __state.Rarity = r.Tier;
        }

        private static void Postfix(CardData cardData, MonsterData __state)
        {
            if (__state != null) __state.Rarity = Registry.RarityOf(cardData.monsterType)?.Value ?? __state.Rarity;
        }
    }

    // ---- Rarity filter (workbench, bulk donation / quick fill) ----

    /// <summary>Which set the next rarity picker is for (set by the screens' change-rarity buttons).</summary>
    internal static class RarityFilterContext
    {
        public static CustomSet Set;

        /// <summary>A custom rarity limit that doesn't belong to the newly picked set goes back to "Any Rarity" (as in EPL).</summary>
        public static void ResetIfForeign(bool hasLimit, ERarity limit, ECardExpansionType expansion, System.Action reset)
        {
            if (!hasLimit || !Registry.IsCustomRarity(limit)) return;
            if (Registry.TryGetRarity(limit, out var owner, out _) && owner == Registry.Get(expansion)) return;
            reset();
        }
    }

    [HarmonyPatch(typeof(WorkbenchUIScreen), nameof(WorkbenchUIScreen.OnPressChangeRarityButton))]
    internal static class WorkbenchRarityButtonPatch
    {
        private static void Prefix(WorkbenchUIScreen __instance) =>
            RarityFilterContext.Set = Registry.Get(Traverse.Create(__instance).Field<ECardExpansionType>("m_CurrentCardExpansionType").Value);
    }

    [HarmonyPatch(typeof(BulkDonationBoxQuickFillScreen), nameof(BulkDonationBoxQuickFillScreen.OnPressChangeRarityButton))]
    internal static class QuickFillRarityButtonPatch
    {
        private static void Prefix(BulkDonationBoxQuickFillScreen __instance) =>
            RarityFilterContext.Set = Registry.Get(Traverse.Create(__instance).Field<ECardExpansionType>("m_CurrentCardExpansionType").Value);
    }

    [HarmonyPatch(typeof(WorkbenchUIScreen), "OnCardExpansionUpdated")]
    internal static class WorkbenchExpansionChangedPatch
    {
        private static void Postfix(WorkbenchUIScreen __instance)
        {
            var t = Traverse.Create(__instance);
            RarityFilterContext.ResetIfForeign(t.Field<bool>("m_HasRarityLimit").Value, t.Field<ERarity>("m_RarityLimit").Value,
                t.Field<ECardExpansionType>("m_CurrentCardExpansionType").Value,
                () => __instance.OnRarityLimitUpdated(new CEventPlayer_OnCardRaritySelectScreenUpdated(-1)));
        }
    }

    [HarmonyPatch(typeof(BulkDonationBoxQuickFillScreen), "OnCardExpansionUpdated")]
    internal static class QuickFillExpansionChangedPatch
    {
        private static void Postfix(BulkDonationBoxQuickFillScreen __instance)
        {
            var t = Traverse.Create(__instance);
            RarityFilterContext.ResetIfForeign(t.Field<bool>("m_HasRarityLimit").Value, t.Field<ERarity>("m_RarityLimit").Value,
                t.Field<ECardExpansionType>("m_CurrentCardExpansionType").Value,
                () => __instance.OnRarityLimitUpdated(new CEventPlayer_OnCardRaritySelectScreenUpdated(-1)));
        }
    }

    /// <summary>
    /// The rarity picker. Its buttons call OnPressButton(rarity) (-1 = Any Rarity) and highlight m_BtnHighlightList[rarity + 1].
    /// For a set with its own rarity list, the vanilla rarity buttons are hidden and cloned buttons (one per rarity of that set,
    /// in a scrollable grid) send that rarity's value. Other sets get the vanilla screen; a custom limit there shows as "Any".
    /// </summary>
    [HarmonyPatch(typeof(CardRaritySelectScreen), nameof(CardRaritySelectScreen.OpenScreen))]
    internal static class RarityPickerOpenPatch
    {
        private static readonly AccessTools.FieldRef<CardRaritySelectScreen, int> CurrentIndex = AccessTools.FieldRefAccess<CardRaritySelectScreen, int>("m_CurrentIndex");

        private static CardRaritySelectScreen _builtFor;
        private static ButtonGrid _grid;
        private static RectTransform _anyRow;
        /// <summary>Cloned buttons; a highlight outside its row keeps <c>offset</c> from it (rows move when the grid is laid out).</summary>
        private static readonly List<(RectTransform button, GameObject highlight, Vector3? offset)> Clones = new List<(RectTransform, GameObject, Vector3?)>();

        private static bool Prefix(ref ERarity initCardRarity)
        {
            var set = RarityFilterContext.Set;
            RarityFilterContext.Set = null;
            var screen = CSingleton<CardRaritySelectScreen>.Instance;
            if (screen == null) return true;
            int max = Registry.Sets.Select(s => s.Def.Rarities.Count).DefaultIfEmpty(0).Max();
            if (max > 0 && _builtFor != screen) Build(screen, max);
            foreach (var c in Clones) if (c.highlight != null) c.highlight.SetActive(false);

            bool own = set != null && _grid != null && Clones.Count >= set.Def.Rarities.Count;
            if (!own)
            {
                _grid?.Relayout(item => !item.name.StartsWith("TCGCC_"));
                if ((int)initCardRarity + 1 >= screen.m_BtnHighlightList.Count || (int)initCardRarity < -1) initCardRarity = ERarity.None;
                return true;
            }

            var rarities = set.Def.Rarities;
            for (int k = 0; k < Clones.Count; k++)
            {
                if (k >= rarities.Count) continue;
                int value = (int)rarities[k].Value;
                ButtonGrid.SetLabel(Clones[k].button, rarities[k].Name);
                ButtonGrid.SetOnClick(Clones[k].button, () => screen.OnPressButton(value));
            }
            var shown = new HashSet<RectTransform>(Clones.Take(rarities.Count).Select(c => c.button));
            _grid.Relayout(item => item == _anyRow || shown.Contains(item));
            foreach (var c in Clones)
                if (c.offset.HasValue && c.highlight != null && c.highlight.transform.parent == c.button.parent)
                    c.highlight.transform.localPosition = c.button.localPosition + c.offset.Value;

            var init = initCardRarity;
            int current = rarities.FindIndex(r => r.Value == init);
            CurrentIndex(screen) = current >= 0 ? (int)init + 1 : 0;
            foreach (var h in screen.m_BtnHighlightList) if (h != null) h.SetActive(false);
            if (current >= 0)
            {
                if (Clones[current].highlight != null) Clones[current].highlight.SetActive(true);
                _grid.ScrollTo(Clones[current].button);
            }
            else if (screen.m_BtnHighlightList.Count > 0 && screen.m_BtnHighlightList[0] != null) screen.m_BtnHighlightList[0].SetActive(true);

            SoundManager.GenericMenuOpen();
            screen.m_ScreenGrp.SetActive(true);
            ControllerScreenUIExtManager.OnOpenScreen(screen.m_ControllerScreenUIExtension);
            return false;
        }

        private static void Build(CardRaritySelectScreen screen, int count)
        {
            _builtFor = screen;
            _grid = null;
            _anyRow = null;
            Clones.Clear();
            var root = screen.m_ScreenGrp != null ? screen.m_ScreenGrp.transform : screen.transform;
            var byIndex = ExpansionPickerOpenPatch.FindButtons(root, nameof(CardRaritySelectScreen.OnPressButton));
            if (!byIndex.ContainsKey(-1) || byIndex.Count < 2)
            {
                Plugin.Log.LogWarning($"Rarity picker: buttons not found ({string.Join(",", byIndex.Keys)}); sets' own rarities can't be picked as a filter");
                return;
            }
            var rows = ExpansionPickerOpenPatch.ToRows(byIndex);
            _anyRow = rows[-1];
            var vanilla = rows.OrderBy(kv => kv.Key).Select(kv => kv.Value).ToList();

            // Highlight of the first rarity button (index 0 → m_BtnHighlightList[1]) is the template for the clones' highlights.
            var refRow = rows.ContainsKey(0) ? rows[0] : vanilla.Last();
            var refHighlight = screen.m_BtnHighlightList.Count > 1 ? screen.m_BtnHighlightList[1]?.transform as RectTransform : null;
            bool inside = refHighlight != null && refHighlight.IsChildOf(refRow);
            string path = inside ? ExpansionPickerOpenPatch.RelativePath(refRow, refHighlight) : null;
            var extra = new List<RectTransform>();
            if (refHighlight != null && !inside) extra.AddRange(screen.m_BtnHighlightList.Where(h => h != null).Select(h => (RectTransform)h.transform));

            _grid = ButtonGrid.Build(vanilla, count, (k, clone) =>
            {
                clone.name = $"TCGCC_RarityButton_{k}";
                GameObject highlight = null;
                Vector3? offset = null;
                if (inside) highlight = clone.Find(path)?.gameObject;
                else if (refHighlight != null)
                {
                    var h = (RectTransform)Object.Instantiate(refHighlight.gameObject, refHighlight.parent, false).transform;
                    h.name = $"TCGCC_RarityHighlight_{k}";
                    if (refHighlight.parent == clone.parent)
                    {
                        offset = refHighlight.localPosition - refRow.localPosition;
                        h.localPosition = clone.localPosition + offset.Value;
                    }
                    else h.position = clone.position + (refHighlight.position - refRow.position);
                    extra.Add(h);
                    highlight = h.gameObject;
                }
                if (highlight != null) highlight.SetActive(false);
                Clones.Add((clone, highlight, offset));
            }, extra, manualWheel: true);
            Plugin.Log.LogInfo($"Rarity picker: {byIndex.Count} vanilla buttons, {count} for sets' own rarities");
        }
    }
}
