using System.Collections;
using System.Collections.Generic;
using System.Linq;
using HarmonyLib;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// The cards in customers' hands at a play table come from what the shop sells, and everyone at a table plays the same card game: the
    /// first to start picks a game (a set's <c>collection</c>, or the set itself without one), each player then holds cards of one of its
    /// sets. The game's own Tetramon cards stay one of the choices while [Content] ShowVanillaPacks is on. Works without Personalities.
    /// </summary>
    internal static class TableCards
    {
        private const int CardsInHand = 6;
        private const float WaitForPictures = 8f; // seconds; hands stay empty meanwhile rather than showing Tetramon
        private const string Vanilla = "";        // the game picked at a table: Tetramon

        private static readonly AccessTools.FieldRef<Customer, InteractablePlayTable> TableOf =
            Core.Compat.Field<Customer, InteractablePlayTable>(typeof(TableCards), "m_CurrentPlayTable");

        // Their own seat: on a loaded save they start playing before the table lists them in m_OccupiedCustomer.
        private static readonly AccessTools.FieldRef<Customer, int> SeatOf =
            Core.Compat.Field<Customer, int>(typeof(TableCards), "m_CurrentPlayTableSeatIndex");

        private static readonly Dictionary<InteractablePlayTable, string> GameAt = new Dictionary<InteractablePlayTable, string>();

        public static void Init()
        {
            CustomerLife.TableGameStarted += Started;
            CustomerLife.TableGameEnded += Ended;
        }

        private static string GameOf(CustomSet s) => string.IsNullOrWhiteSpace(s.Def.Collection) ? s.Def.Id : s.Def.Collection.Trim();

        /// <summary>The sets customers may play now: a tournament's, an MTG game's, else the ones the shop has licensed (all while none is).</summary>
        private static List<CustomSet> Pool(InteractablePlayTable table)
        {
            var eventSets = Mtg.MtgEvents.TournamentAiSets();
            if (eventSets != null) return eventSets;
            var sets = Registry.Sets.Where(s => s.Monsters.Count > 0).ToList();
            if (table != null && table.GetHasStartPlayerPlayCard() && Mtg.MtgSession.Playing)
            {
                var mtg = sets.Where(Mtg.MtgMode.HasMtg).ToList();
                if (mtg.Count > 0) return mtg;
            }
            var licensed = sets.Where(Mtg.MtgMode.IsLicensed).ToList();
            return licensed.Count > 0 ? licensed : sets;
        }

        private static void Started(Customer c)
        {
            using var measured = Debug.SpikeWatch.Measure("table cards");
            var table = TableOf(c);
            var pool = Pool(table);
            string game = Vanilla;
            if (table == null || !GameAt.TryGetValue(table, out game))
            {
                var games = pool.Select(GameOf).Distinct().ToList();
                if (Plugin.ShowVanillaPacks.Value && Mtg.MtgEvents.TournamentAiSets() == null) games.Add(Vanilla);
                game = games.Count > 0 ? games[Random.Range(0, games.Count)] : Vanilla;
                if (table != null)
                {
                    foreach (var gone in GameAt.Keys.Where(t => t == null).ToList()) GameAt.Remove(gone); // a loaded save rebuilt the scene
                    GameAt[table] = game;
                }
                if (Plugin.CustomerDebug.Value) Plugin.Log.LogInfo($"Table cards: this table plays {(game == Vanilla ? "the game's own cards" : game)}");
            }

            var sets = pool.Where(s => GameOf(s) == game).ToList();
            if (sets.Count == 0)
            {
                HandCards.Clear(c); // pooled customers may still hold the last person's cards
                return;
            }
            var set = sets[Random.Range(0, sets.Count)];
            var cards = new List<CardData>();
            for (int i = 0; i < CardsInHand; i++)
                cards.Add(new CardData { expansionType = set.Expansion, monsterType = set.Monsters[Random.Range(0, set.Monsters.Count)].MonsterType });
            HandCards.HideCards(c);
            int seat = SeatOf(c);
            c.StartCoroutine(Deal(c, cards, seat >= 0 && seat < (table?.m_TableGameItemSetList.Count ?? 0) ? table.m_TableGameItemSetList[seat] : null));
        }

        private static IEnumerator Deal(Customer c, List<CardData> cards, TableGameItemSet onTable)
        {
            yield return HandCards.Show(c, cards, WaitForPictures, fillMissing: true, alsoOn: onTable);
            HandCards.RevealCards(c);
        }

        private static void Ended(Customer c)
        {
            var table = TableOf(c);
            if (table != null) GameAt.Remove(table);
            HandCards.RevealCards(c);
            HandCards.Clear(c);
        }
    }
}
