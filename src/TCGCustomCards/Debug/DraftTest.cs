using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Hooks;
using TCGCustomCards.Runtime;
using TCGCustomCards.Runtime.Mtg;
using TCGCustomCards.UI;
using UnityEngine.SceneManagement;

namespace TCGCustomCards.Debug
{
    /// <summary>
    /// [Debug] TestDraft: a booster draft without a tournament, to try the draft in game. Your packs = the MTG packs in your hands
    /// (or 3 of the first MTG pack when you hold none; nothing is used up); 7 Forge seats open the same kinds of packs. After
    /// building, the next right-click on a table with a waiting customer plays your deck against a random seat's deck.
    /// Your picks go into the collection.
    /// </summary>
    internal static class DraftTest
    {
        public static void Run()
        {
            if (SceneManager.GetActiveScene().name != "Start" || !GameInstance.m_FinishedSavefileLoading) return;
            if (MtgDraft.Running || MtgDraftUI.IsOpen) return;
            if (!MtgSession.CanPlayInGame) { Toast.Show("Test draft: MTG mode isn't installed (TCG Studio > Setup)"); return; }

            var held = PlayerItems.HeldTypes().Where(t => Registry.GetByItem(t) is CustomPack p && MtgEvents.IsMtgSet(p.Set)).ToList();
            if (held.Count == 0)
            {
                var pack = Registry.Packs.FirstOrDefault(p => MtgEvents.IsMtgSet(p.Set) && p.PackItem != EItemType.None);
                if (pack == null) { Toast.Show("Test draft: no MTG set with a pack installed"); return; }
                held = new List<EItemType> { pack.PackItem, pack.PackItem, pack.PackItem };
            }
            if (held.Count > 6) held = held.Take(6).ToList();
            const int seats = 8;
            var packs = held.Select(item => Enumerable.Range(0, seats).Select(_ => MtgDraft.Roll(item)).ToList()).ToList();
            var sets = held.Select(t => Registry.GetByItem(t).Set).Distinct().ToList();
            string err = MtgDraft.Start(packs, 0, MtgDraft.LandSetOf(sets), MtgDraftUI.DraftFinished);
            if (err != null) { Toast.Show("Test draft: " + err); return; }
            Plugin.Log.LogInfo($"Test draft: {held.Count} packs ({string.Join(", ", sets.Select(s => s.Def.Id))}) × {seats} seats");

            MtgDraftUI.Open((deck, copies) =>
            {
                var me = MtgDraft.Seats[MtgDraft.HumanSeat];
                foreach (var c in me.Picks) CPlayerData.AddCard(c.Data, 1); // a draft's picks are yours
                var others = Enumerable.Range(0, MtgDraft.Seats.Count).Where(i => i != MtgDraft.HumanSeat && MtgDraft.Seats[i].Deck != null).ToList();
                var opp = MtgDraft.Seats[others[UnityEngine.Random.Range(0, others.Count)]];
                MtgDraftGame.DevPending = new MtgSession.DeckPair
                {
                    Player = deck, Opponent = opp.Deck, Faces = new MtgCardFaces(copies, MtgDraft.DeckCopies(opp, opp.Deck)),
                    OpponentInfo = MtgDraft.DeckInfo(opp),
                };
                Toast.Show($"Draft deck ready ({deck.Total} cards) - right-click a table with a waiting customer to play it");
            }, why => Toast.Show(why == "aborted" || why == "left" ? "Draft left" : "Draft stopped: " + why));
        }
    }
}
