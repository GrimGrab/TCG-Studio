using HarmonyLib;
using TCGCustomCards.Binder;
using UnityEngine;

namespace TCGCustomCards.Patches
{
    /// <summary>
    /// Enhanced binder ([Binder] EnhancedBinder), one-slot-per-card layout: the book, page flips, close-up, taking cards and
    /// the card pickers (deck edit, bulk donation, grading) stay vanilla; only what fills the pages changes.
    /// Pages come from <see cref="BinderIndex"/> (one entry per card, sorted once) instead of m_SortedIndexList (12 save
    /// slots per card). Every picker only reads each slot's CardData, so a slot may show any version of the card.
    /// Graded album and the Classic layout: vanilla.
    /// </summary>
    internal static class EnhancedBinderPatches
    {
        private const int PerPage = 12;

        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, ECardExpansionType> ExpRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, ECardExpansionType>("m_ExpansionType");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, bool> GradedRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, bool>("m_IsGradedCardAlbum");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, int> IndexRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, int>("m_Index");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, int> MaxIndexRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, int>("m_MaxIndex");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, bool> CanUpdateSortRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, bool>("m_CanUpdateSort");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, bool> BookOpenRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, bool>("m_IsBookOpen");
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, bool> CanFlipRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, bool>("m_CanFlip");
        private static readonly AccessTools.FieldRef<CollectionBinderUI, int> MaxCollectRef =
            AccessTools.FieldRefAccess<CollectionBinderUI, int>("m_MaxCardCollectionCount");
        private static readonly System.Action<CollectionBinderFlipAnimCtrl> SyncColliders =
            AccessTools.MethodDelegate<System.Action<CollectionBinderFlipAnimCtrl>>(
                AccessTools.Method(typeof(CollectionBinderFlipAnimCtrl), "UpdateCurrentInteractableCard3dList"));
        private static readonly System.Action<CollectionBinderFlipAnimCtrl, bool> SortUpdated =
            AccessTools.MethodDelegate<System.Action<CollectionBinderFlipAnimCtrl, bool>>(
                AccessTools.Method(typeof(CollectionBinderFlipAnimCtrl), "OnSortingMethodUpdated"));

        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, System.Collections.Generic.List<int>> SortedRef =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, System.Collections.Generic.List<int>>("m_SortedIndexList");
        private static readonly System.Action<CollectionBinderFlipAnimCtrl, int, int> FillGraded =
            AccessTools.MethodDelegate<System.Action<CollectionBinderFlipAnimCtrl, int, int>>(
                AccessTools.Method(typeof(CollectionBinderFlipAnimCtrl), "UpdateBinderGradedCardUI"));

        internal static ECardExpansionType Expansion(CollectionBinderFlipAnimCtrl ctrl) => ExpRef(ctrl);
        internal static bool IsGraded(CollectionBinderFlipAnimCtrl ctrl) => GradedRef(ctrl);

        /// <summary>The panel's Apply without a set switch: rebuild the list and refill the pages in place (no close/reopen).</summary>
        internal static void Rebuild(CollectionBinderFlipAnimCtrl ctrl)
        {
            if (ctrl == null || !(Active(ctrl) || GradedActive(ctrl))) return;
            SortUpdated(ctrl, true);
            if (CanFlipRef(ctrl)) SyncColliders(ctrl);
        }

        /// <summary>The graded album is sorted/filtered by the panel (setting on; any layout).</summary>
        internal static bool GradedActive(CollectionBinderFlipAnimCtrl ctrl) => Plugin.EnhancedBinder.Value && GradedRef(ctrl);

        /// <summary>
        /// Graded album: our filtered, sorted slab order goes into m_SortedIndexList and the game's own graded page fill
        /// draws the slabs from it.
        /// </summary>
        private static void SortGraded(CollectionBinderFlipAnimCtrl ctrl, bool backToFirstPage)
        {
            var pages = ctrl.m_BinderPageGrpList;
            for (int i = 0; i < pages.Count; i++) pages[i].SetGradedCardMode(true);
            if (backToFirstPage) IndexRef(ctrl) = 1;
            CanUpdateSortRef(ctrl) = false;
            var list = BinderIndex.GradedList(BinderIndex.GradedQuery, out var st);
            var sorted = SortedRef(ctrl);
            sorted.Clear();
            sorted.AddRange(list);
            int max = Mathf.Max(1, Mathf.CeilToInt(list.Count / (float)PerPage));
            MaxIndexRef(ctrl) = max;
            if (IndexRef(ctrl) > max) IndexRef(ctrl) = max;
            var ui = ctrl.m_CollectionBinderUI;
            if (ui != null)
            {
                ui.SetMaxPage(max);
                ui.SetCurrentPage(IndexRef(ctrl));
                ui.m_CardCollectedText.text = st.Matching == st.Cards ? st.Cards.ToString() : st.Matching + " / " + st.Cards;
                ui.SetTotalValue(st.Value);
            }
            int page = IndexRef(ctrl);
            FillGraded(ctrl, 0, page);
            FillGraded(ctrl, 1, page + 1);
            FillGraded(ctrl, 2, page - 1);
        }

        /// <summary>The one-slot-per-card layout is filling this binder (not graded, setting on).</summary>
        internal static bool Active(CollectionBinderFlipAnimCtrl ctrl) =>
            Plugin.EnhancedBinder.Value && Plugin.BinderLayout.Value == BinderLayout.OnePerCard && !GradedRef(ctrl);

        // ------------------------------------------------------------------ list + pages

        /// <summary>Sort (or the binder opening): build our list once and fill the three pages.</summary>
        [HarmonyPatch(typeof(CollectionBinderFlipAnimCtrl), "OnSortingMethodUpdated")]
        private static class SortPatch
        {
            private static bool Prefix(CollectionBinderFlipAnimCtrl __instance, bool backToFirstPage)
            {
                if (GradedActive(__instance))
                {
                    SortGraded(__instance, backToFirstPage);
                    return false;
                }
                if (!Active(__instance)) return true;
                var pages = __instance.m_BinderPageGrpList;
                for (int i = 0; i < pages.Count; i++) pages[i].SetGradedCardMode(false);
                if (backToFirstPage) IndexRef(__instance) = 1;
                CanUpdateSortRef(__instance) = false;

                var exp = ExpRef(__instance);
                BinderIndex.Build(exp);
                var ui = __instance.m_CollectionBinderUI;
                if (ui != null)
                {
                    // "x / N" in cards, not versions. Field set directly: Mono inlines the one-line setter, so patching it never runs.
                    MaxCollectRef(ui) = BinderIndex.Stats.Cards;
                    ui.SetCardCollected(BinderIndex.Stats.Owned, exp);
                    ui.SetTotalValue(BinderIndex.Stats.Value); // across every set shown
                }
                Fill(__instance, 0, IndexRef(__instance)); // may clamp the page to the new page count
                int page = IndexRef(__instance);
                Fill(__instance, 1, page + 1);
                Fill(__instance, 2, page - 1);
                ui?.SetCurrentPage(page);
                return false;
            }
        }

        /// <summary>Page flips (and the ±10 jumps) fill pages through here.</summary>
        [HarmonyPatch(typeof(CollectionBinderFlipAnimCtrl), "UpdateBinderAllCardUI")]
        private static class PagePatch
        {
            [HarmonyPriority(Priority.High)]
            private static bool Prefix(CollectionBinderFlipAnimCtrl __instance, int binderIndex, int pageIndex)
            {
                if (Active(__instance))
                {
                    Fill(__instance, binderIndex, pageIndex);
                    return false;
                }
                HidePips(__instance.m_BinderPageGrpList[binderIndex]);
                return true;
            }
        }

        [HarmonyPatch(typeof(CollectionBinderFlipAnimCtrl), "UpdateBinderGradedCardUI")]
        private static class GradedPagePatch
        {
            private static void Prefix(CollectionBinderFlipAnimCtrl __instance, int binderIndex) =>
                HidePips(__instance.m_BinderPageGrpList[binderIndex]);
        }

        private static void HidePips(BinderPageGrp grp)
        {
            foreach (var slot in grp.m_CardList) VersionPips.Hide(slot);
        }

        private static void Fill(CollectionBinderFlipAnimCtrl ctrl, int binderIndex, int pageIndex)
        {
            int pages = Mathf.Max(1, Mathf.CeilToInt(BinderIndex.Entries.Count / (float)PerPage));
            if (MaxIndexRef(ctrl) != pages)
            {
                MaxIndexRef(ctrl) = pages;
                if (IndexRef(ctrl) > pages) IndexRef(ctrl) = pages;
                ctrl.m_CollectionBinderUI?.SetMaxPage(pages);
                ctrl.m_CollectionBinderUI?.SetCurrentPage(IndexRef(ctrl));
            }
            var grp = ctrl.m_BinderPageGrpList[binderIndex];
            for (int j = 0; j < grp.m_CardList.Count; j++) FillSlot(ctrl, grp, j, pageIndex);
        }

        private static void FillSlot(CollectionBinderFlipAnimCtrl ctrl, BinderPageGrp grp, int j, int pageIndex)
        {
            // Our own sorts and filters: vanilla's sort type would only change the count text (Duplicates shows count-1).
            const ECollectionSortingType plain = ECollectionSortingType.Default;
            var slot = grp.m_CardList[j];
            int k = (pageIndex - 1) * PerPage + j;
            var e = pageIndex >= 1 && k < BinderIndex.Entries.Count ? BinderIndex.Entries[k] : null;
            int v = e == null ? -1 : BinderIndex.Shown(e);
            if (v < 0)
            {
                grp.SetSingleCard(j, null, 0, plain);
                VersionPips.Hide(slot);
                return;
            }
            grp.SetSingleCard(j, e.Card(v), e.Count(v), plain); // count text: the shown version, vanilla style
            VersionPips.Show(slot, e, v);
        }

        /// <summary>The entry behind a slot of the open page (0..11), or null.</summary>
        private static BinderEntry EntryAt(CollectionBinderFlipAnimCtrl ctrl, int j)
        {
            int k = (IndexRef(ctrl) - 1) * PerPage + j;
            return j >= 0 && k >= 0 && k < BinderIndex.Entries.Count ? BinderIndex.Entries[k] : null;
        }

        // ------------------------------------------------------------------ panel

        /// <summary>Sort Album opens our panel inside the vanilla sort screen (graded album / Classic: vanilla content).</summary>
        [HarmonyPatch(typeof(CollectionBinderUI), nameof(CollectionBinderUI.OpenSortAlbumScreen))]
        private static class PanelOpenPatch
        {
            private static void Postfix(CollectionBinderUI __instance, bool isGradedCardAlbum) =>
                UI.BinderPanel.OnSortScreenOpened(__instance, isGradedCardAlbum);
        }

        /// <summary>Esc while the panel is open closes the panel (or leaves its search box) instead of opening the pause menu.</summary>
        [HarmonyPatch(typeof(PauseScreen), nameof(PauseScreen.OpenScreen))]
        private static class EscapePatch
        {
            private static bool Prefix() => !UI.BinderPanel.HandleEscape();
        }

        /// <summary>While typing in the panel's search box (or right after a panel click) the binder's keys and clicks do nothing.</summary>
        [HarmonyPatch(typeof(InteractionPlayerController), "RaycastViewCardAlbumState")]
        private static class TypingPatch
        {
            private static bool Prefix() => !UI.BinderPanel.BlockBinderInput;
        }

        // ------------------------------------------------------------------ taking a card, switching versions

        /// <summary>
        /// After a card is taken (or picked for deck edit / donation / grading) the slot shows what's left: another owned
        /// version instead of an empty slot when the shown one ran out, and the counts and squares update.
        /// </summary>
        [HarmonyPatch(typeof(CollectionBinderFlipAnimCtrl), nameof(CollectionBinderFlipAnimCtrl.OnRightMouseButtonUp))]
        private static class TakePatch
        {
            private static void Postfix(CollectionBinderFlipAnimCtrl __instance)
            {
                if (!BookOpenRef(__instance) || !CanFlipRef(__instance)) return;
                if (GradedActive(__instance))
                {
                    // A slab left the list: later slabs moved up one place, so the order is rebuilt (same page).
                    SortGraded(__instance, backToFirstPage: false);
                    SyncColliders(__instance);
                    return;
                }
                if (!Active(__instance)) return;
                Fill(__instance, 0, IndexRef(__instance));
                SyncColliders(__instance); // a slot emptied by vanilla may show another version now
            }
        }

        // ------------------------------------------------------------------ close-up: number, set, favourite

        /// <summary>The close-up's rarity line, before our additions (rebuilt when the favourite mark changes).</summary>
        private static string _closeUpBase;

        /// <summary>Close-up info: card number and set after the rarity line, plus "Favourite".</summary>
        [HarmonyPatch(typeof(CollectionBinderFlipAnimCtrl), "EnterViewUpCloseState")]
        private static class CloseUpPatch
        {
            private static void Postfix(CollectionBinderFlipAnimCtrl __instance)
            {
                _closeUpBase = null;
                if (!Plugin.EnhancedBinder.Value || GradedRef(__instance) || !__instance.m_IsHoldingCardCloseUp) return;
                var ui = __instance.m_CollectionBinderUI;
                var card = __instance.m_CurrentViewInteractableCard3d?.m_Card3dUI?.m_CardUI?.GetCardData();
                if (ui == null || ui.m_CardFullRarityNameText == null || card == null) return;
                _closeUpBase = ui.m_CardFullRarityNameText.text;
                ShowCloseUpInfo(ui, card);
            }
        }

        private static void ShowCloseUpInfo(CollectionBinderUI ui, CardData card)
        {
            if (_closeUpBase == null) return;
            var info = CardInfo.Of(card.monsterType);
            string set = Runtime.Registry.IsCustom(card.expansionType)
                ? Runtime.Registry.Get(card.expansionType)?.Def.Name
                : I2.Loc.LocalizationManager.GetTranslation(card.expansionType.ToString());
            string text = _closeUpBase;
            if (!string.IsNullOrEmpty(info.Number)) text += "  ·  #" + info.Number;
            if (!string.IsNullOrEmpty(set)) text += "  ·  " + set;
            if (Favourites.Has(card)) text += "  ·  Favourite";
            ui.m_CardFullRarityNameText.text = text;
        }

        private static bool _refillAfterCloseUp;

        /// <summary>The close-up card is back on its page: show a favourite mark changed meanwhile.</summary>
        [HarmonyPatch(typeof(CollectionBinderFlipAnimCtrl), nameof(CollectionBinderFlipAnimCtrl.OnCardFinishLerpHide))]
        private static class CloseUpEndPatch
        {
            private static void Postfix(CollectionBinderFlipAnimCtrl __instance)
            {
                if (!_refillAfterCloseUp) return;
                _refillAfterCloseUp = false;
                if (Active(__instance) && BookOpenRef(__instance)) Fill(__instance, 0, IndexRef(__instance));
            }
        }

        /// <summary>Favourite key on the card under the mouse (or held up close); Home / End jump to the first / last page.</summary>
        [HarmonyPatch(typeof(CollectionBinderFlipAnimCtrl), "Update")]
        private static class KeysPatch
        {
            private static void Postfix(CollectionBinderFlipAnimCtrl __instance)
            {
                if (!Plugin.EnhancedBinder.Value || GradedRef(__instance) || !BookOpenRef(__instance) || UI.BinderPanel.Typing) return;
                if (__instance.m_IsHoldingCardCloseUp)
                {
                    if (__instance.m_IsExitingCardCloseUp || !Plugin.BinderFavouriteKey.Value.IsDown()) return;
                    var card = __instance.m_CurrentViewInteractableCard3d?.m_Card3dUI?.m_CardUI?.GetCardData();
                    if (card == null) return;
                    bool fav = Favourites.Toggle(card);
                    SoundManager.GenericPop(1f, fav ? 1.3f : 0.9f);
                    UI.Toast.Show(fav ? "Added to favourites" : "Removed from favourites");
                    if (__instance.m_CollectionBinderUI != null) ShowCloseUpInfo(__instance.m_CollectionBinderUI, card);
                    _refillAfterCloseUp = true; // the mark on the slot behind; refilling now would show that slot again
                    return;
                }
                if (!Active(__instance) || !CanFlipRef(__instance)) return;
                if (__instance.m_CollectionBinderUI != null && __instance.m_CollectionBinderUI.IsSortAlbumScreenOpen()) return;
                var hovered = __instance.m_CurrentRaycastedInteractableCard3d;
                if (hovered != null && Plugin.BinderFavouriteKey.Value.IsDown())
                {
                    var card = hovered.m_Card3dUI?.m_CardUI?.GetCardData();
                    int j = __instance.m_InteractableCard3dList.IndexOf(hovered);
                    if (card == null || j < 0) return;
                    bool fav = Favourites.Toggle(card);
                    SoundManager.GenericPop(1f, fav ? 1.3f : 0.9f);
                    UI.Toast.Show(fav ? "Added to favourites" : "Removed from favourites");
                    FillSlot(__instance, __instance.m_BinderPageGrpList[0], j, IndexRef(__instance));
                    return;
                }
                if (Input.GetKeyDown(KeyCode.Home)) JumpTo(__instance, 1);
                else if (Input.GetKeyDown(KeyCode.End)) JumpTo(__instance, MaxIndexRef(__instance));
            }
        }

        private static void JumpTo(CollectionBinderFlipAnimCtrl ctrl, int page)
        {
            page = Mathf.Clamp(page, 1, Mathf.Max(1, MaxIndexRef(ctrl)));
            if (page == IndexRef(ctrl)) return;
            IndexRef(ctrl) = page;
            Fill(ctrl, 0, page);
            Fill(ctrl, 1, page + 1);
            Fill(ctrl, 2, page - 1);
            ctrl.m_CollectionBinderUI?.SetCurrentPage(page);
            SyncColliders(ctrl);
            SoundManager.PlayAudio("SFX_AlbumFlip", 0.6f);
        }

        /// <summary>Mouse wheel over a card: next / previous owned version in that slot.</summary>
        [HarmonyPatch(typeof(CollectionBinderFlipAnimCtrl), "Update")]
        private static class WheelPatch
        {
            private static void Postfix(CollectionBinderFlipAnimCtrl __instance)
            {
                float wheel = Input.mouseScrollDelta.y;
                if (wheel == 0f || !Active(__instance)) return;
                if (!BookOpenRef(__instance) || !CanFlipRef(__instance) || __instance.m_IsHoldingCardCloseUp) return;
                if (__instance.m_CollectionBinderUI != null && __instance.m_CollectionBinderUI.IsSortAlbumScreenOpen()) return;
                var hovered = __instance.m_CurrentRaycastedInteractableCard3d;
                if (hovered == null) return;
                int j = __instance.m_InteractableCard3dList.IndexOf(hovered);
                var e = EntryAt(__instance, j);
                if (e == null || !BinderIndex.Switch(e, wheel > 0f ? -1 : 1)) return;
                FillSlot(__instance, __instance.m_BinderPageGrpList[0], j, IndexRef(__instance));
                __instance.OnCardRaycasted(hovered); // price popup of the version now shown
                SoundManager.GenericPop(1f, 1.2f);
            }
        }
    }
}
