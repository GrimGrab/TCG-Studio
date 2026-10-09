using System.Collections.Generic;
using System.Linq;
using HarmonyLib;
using TCGCustomCards.Runtime;

namespace TCGCustomCards.Patches
{
    /// <summary>
    /// Per-set card versions (set.json "variants"): versions a set doesn't have are never made up by the game and are left out
    /// of the binder and Check Price unless the player owns one (cards bought before the set changed stay visible).
    /// Packs: <see cref="PackRoller"/>. Save slots stay 12 per card (shelves, decks and graded cards refer to them).
    /// </summary>
    internal static class VariantPatches
    {
        /// <summary>The custom set of an expansion when it has some versions switched off; null otherwise (vanilla too).</summary>
        internal static CustomSet Limited(ECardExpansionType exp)
        {
            if (!Registry.IsCustom(exp)) return null;
            var set = Registry.Get(exp);
            return set != null && !set.Def.AllVariants ? set : null;
        }

        /// <summary>Whether this save index of the expansion is a version the set has.</summary>
        internal static bool Allowed(CustomSet set, int saveIndex) =>
            set.Def.VariantMask[saveIndex % CardStore.SlotsPerCard];

        /// <summary>A slot is listed when the set has that version or the player owns one (bought before it was switched off).</summary>
        internal static bool Listed(CustomSet set, ECardExpansionType exp, int saveIndex) =>
            Allowed(set, saveIndex) || CPlayerData.GetCardAmountByIndex(saveIndex, exp, false) > 0;

        // ------------------------------------------------------------------ binder

        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, List<int>> SortedRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, List<int>>("m_SortedIndexList");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, ECardExpansionType> ExpRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, ECardExpansionType>("m_ExpansionType");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, bool> GradedRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, bool>("m_IsGradedCardAlbum");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, int> MaxIndexRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, int>("m_MaxIndex");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, int> IndexRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, int>("m_Index");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, CollectionBinderUI> UiRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, CollectionBinderUI>("m_CollectionBinderUI");

        private static readonly AccessTools.FieldRef<CollectionBinderUI, int> MaxCollectRef =
            AccessTools.FieldRefAccess<CollectionBinderUI, int>("m_MaxCardCollectionCount");

        private static CustomSet BinderSet(CollectionBinderFlipAnimCtrl c) => GradedRef(c) ? null : Limited(ExpRef(c));

        /// <summary>Every sort fills all 12 slots per card: drop the versions the set doesn't have (unless owned), so cards pack together.</summary>
        [HarmonyPatch]
        internal static class BinderSortPatch
        {
            private static readonly HashSet<string> Logged = new HashSet<string>();

            private static IEnumerable<System.Reflection.MethodBase> TargetMethods() =>
                new[] { "SortByDefault", "SortByCardAmount", "SortByPriceAmount", "SortByCardType", "SortByCardRarity",
                        "SortByDuplicatePriceAmount", "SortByTotalValueAmount" }
                    .Select(n => AccessTools.Method(typeof(CollectionBinderFlipAnimCtrl), n))
                    .Where(m => m != null);

            private static void Postfix(CollectionBinderFlipAnimCtrl __instance, System.Reflection.MethodBase __originalMethod)
            {
                var set = BinderSet(__instance);
                if (set == null) return;
                var exp = ExpRef(__instance);
                var list = SortedRef(__instance);
                int before = list.Count;
                list.RemoveAll(i => !Listed(set, exp, i));
                if (Logged.Add(set.Def.Id)) // once per set and session
                    Plugin.Log.LogInfo($"Card versions: binder '{set.Def.Id}' ({__originalMethod.Name}) lists {list.Count} of {before} slots " +
                                       $"({set.Def.VariantMask.Count(v => v)} versions per card)");
            }
        }

        /// <summary>Pages follow the filtered list (vanilla counts 12 slots per card when the binder opens).</summary>
        [HarmonyPatch(typeof(CollectionBinderFlipAnimCtrl), "UpdateBinderAllCardUI")]
        internal static class BinderPagesPatch
        {
            private static void Prefix(CollectionBinderFlipAnimCtrl __instance)
            {
                var set = BinderSet(__instance);
                if (set == null) return;
                var ui0 = UiRef(__instance);
                if (ui0 != null)
                {
                    // "x / N": N counts only the versions the set has. Set here (not by patching SetMaxCardCollectCount: the
                    // runtime inlines that one-line setter, so a patch on it never ran) and the text written again.
                    int total = set.Shown.Count * set.Def.VariantMask.Count(v => v);
                    if (MaxCollectRef(ui0) != total)
                    {
                        MaxCollectRef(ui0) = total;
                        var exp = ExpRef(__instance);
                        ui0.SetCardCollected(CPlayerData.GetCardCollectedAmount(exp, isDimensionCard: false), exp);
                    }
                }
                int pages = UnityEngine.Mathf.Max(1, UnityEngine.Mathf.CeilToInt(SortedRef(__instance).Count / 12f));
                if (MaxIndexRef(__instance) == pages) return;
                MaxIndexRef(__instance) = pages;
                var ui = UiRef(__instance);
                ui?.SetMaxPage(pages);
                if (IndexRef(__instance) > pages)
                {
                    IndexRef(__instance) = pages;
                    ui?.SetCurrentPage(pages);
                }
            }
        }

        // ------------------------------------------------------------------ Check Price

        /// <summary>
        /// Check Price lists every save index (12 per card). For a set with versions switched off, the page is filled again
        /// from the listed slots only, with the page count, page text and scroll end to match.
        /// </summary>
        [HarmonyPatch(typeof(CheckPriceScreen), "EvaluateCardPanelUI")]
        internal static class CheckPricePatch
        {
            private static readonly AccessTools.FieldRef<CheckPriceScreen, ECardExpansionType> Exp =
                AccessTools.FieldRefAccess<CheckPriceScreen, ECardExpansionType>("m_CurrentExpansionType");
            private static readonly AccessTools.FieldRef<CheckPriceScreen, int> Page =
                AccessTools.FieldRefAccess<CheckPriceScreen, int>("m_CardPageIndex");
            private static readonly AccessTools.FieldRef<CheckPriceScreen, int> PageMax =
                AccessTools.FieldRefAccess<CheckPriceScreen, int>("m_CardPageMaxIndex");

            private static void Postfix(CheckPriceScreen __instance)
            {
                var exp = Exp(__instance);
                var set = Limited(exp);
                if (set == null) return;
                var listed = new List<int>();
                for (int i = 0; i < set.Shown.Count * CardStore.SlotsPerCard; i++)
                    if (Listed(set, exp, i)) listed.Add(i);

                int per = __instance.m_MaxCardUICountPerPage;
                int max = UnityEngine.Mathf.Max(0, UnityEngine.Mathf.CeilToInt(listed.Count / (float)per) - 1);
                PageMax(__instance) = max;
                if (Page(__instance) > max) Page(__instance) = max;
                int start = Page(__instance) * per;
                var panels = __instance.m_CheckPricePanelUIList;
                CheckPricePanelUI last = null;
                for (int j = 0; j < panels.Count; j++)
                {
                    int k = start + j;
                    if (j >= per || k >= listed.Count) { panels[j].SetActive(isActive: false); continue; }
                    panels[j].InitCard(__instance, listed[k], exp, isDestiny: false, cardGrade: 0);
                    panels[j].SetActive(isActive: true);
                    last = panels[j];
                }
                if (last != null && __instance.m_ScrollEndParent != null)
                {
                    // Same as vanilla: the scroll ends one panel height below the last panel shown.
                    var end = __instance.m_ScrollEndParent.transform;
                    end.parent = last.transform;
                    var pos = last.transform.position;
                    pos.y += __instance.m_CardScrollOffsetPosEnd.position.y - __instance.m_CardScrollOffsetPosStart.position.y;
                    end.position = pos;
                }
                __instance.m_PageText.text = Page(__instance) + 1 + " / " + (max + 1);
            }
        }

        // ------------------------------------------------------------------ trade customers

        /// <summary>
        /// Trade customers pick a random save index for the card they bring (and for the card they ask for when the player has
        /// nothing that fits): every version equally likely. The made-up card is fitted to the set's versions right where
        /// it's made, so its price, the matching card and the screen all follow. A version the player owns is left as is.
        /// </summary>
        [HarmonyPatch(typeof(CustomerTradeCardScreen), nameof(CustomerTradeCardScreen.SetCustomer))]
        internal static class TradeCardPatch
        {
            private static readonly System.Reflection.MethodInfo GetCard =
                AccessTools.Method(typeof(CPlayerData), nameof(CPlayerData.GetCardData), new[] { typeof(int), typeof(ECardExpansionType), typeof(bool) });
            private static readonly System.Reflection.MethodInfo GetGraded =
                AccessTools.Method(typeof(CPlayerData), nameof(CPlayerData.GetGradedCardData));

            private static IEnumerable<CodeInstruction> Transpiler(IEnumerable<CodeInstruction> instructions)
            {
                int n = 0;
                foreach (var ci in instructions)
                {
                    if (ci.Calls(GetCard)) { ci.operand = AccessTools.Method(typeof(TradeCardPatch), nameof(Card)); n++; }
                    else if (ci.Calls(GetGraded)) { ci.operand = AccessTools.Method(typeof(TradeCardPatch), nameof(Graded)); n++; }
                    yield return ci;
                }
                if (n == 0) Plugin.Log.LogWarning("Card versions: trade customer card calls not found - trades may offer switched-off versions");
            }

            public static CardData Card(int index, ECardExpansionType exp, bool isDestiny) =>
                Fit(CPlayerData.GetCardData(index, exp, isDestiny), owned: true);

            public static CardData Graded(CompactCardDataAmount data) => Fit(CPlayerData.GetGradedCardData(data), owned: false);

            private static CardData Fit(CardData card, bool owned)
            {
                var set = card == null ? null : Limited(card.expansionType);
                if (set == null || set.Def.Allows(card.borderType, card.isFoil)) return card;
                if (owned && CPlayerData.GetCardAmount(card) > 0) return card; // the player has this one: a fair trade target
                var (border, foil) = PackRoller.FitVariant(set.Def, card.borderType, card.isFoil);
                card.borderType = border;
                card.isFoil = foil;
                return card;
            }
        }
    }
}
