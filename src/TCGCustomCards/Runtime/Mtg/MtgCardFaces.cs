using System;
using System.Collections.Generic;
using System.Linq;
using Newtonsoft.Json.Linq;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>
    /// Which in-game card a Forge card is shown as. The player's cards are their actual copies from the selected deck (same
    /// border/foil), handed out by name the first time Forge shows a card and kept for that Forge card id all game. Everything
    /// else (free basic lands, the AI's deck, extra copies) is a plain Base/non-foil copy of the matching custom card.
    /// Tokens and cards not in any installed set return null (drawn as a card back with a label).
    /// </summary>
    internal sealed class MtgCardFaces
    {
        private readonly Dictionary<string, Queue<CardData>> _mine = new Dictionary<string, Queue<CardData>>(StringComparer.OrdinalIgnoreCase);
        private readonly Dictionary<int, CardData> _byForgeId = new Dictionary<int, CardData>();
        private static Dictionary<string, (CustomSet set, int pos)> _lookup;

        public MtgCardFaces(DeckCompactCardDataList deck)
        {
            if (deck?.compactCardDataAmountList == null) return;
            foreach (var c in deck.compactCardDataAmountList)
            {
                if (c == null || c.amount <= 0 || !Registry.IsCustom(c.expansionType)) continue;
                var set = Registry.Get(c.expansionType);
                int pos = CardStore.CardPos(c.cardSaveIndex);
                if (set == null || pos < 0 || pos >= set.Def.Cards.Count) continue;
                string name = set.Card(pos).Mtg?.Name;
                if (string.IsNullOrEmpty(name)) continue;
                if (!_mine.TryGetValue(name, out var q)) _mine[name] = q = new Queue<CardData>();
                for (int i = 0; i < c.amount; i++)
                    q.Enqueue(CPlayerData.GetCardData(c.cardSaveIndex, c.expansionType, c.isDestiny));
            }
        }

        /// <summary>The player's copies from an MTG deck-builder deck (their real border/foil).</summary>
        public MtgCardFaces(MtgDeckStore.Deck deck)
        {
            if (deck == null) return;
            foreach (var e in deck.Entries)
            {
                if (!e.Live || e.Save.Count <= 0) continue;
                string name = e.Set.Card(e.Pos).Mtg?.Name;
                if (string.IsNullOrEmpty(name)) continue;
                if (!_mine.TryGetValue(name, out var q)) _mine[name] = q = new Queue<CardData>();
                for (int i = 0; i < e.Save.Count; i++) q.Enqueue(e.Data);
            }
        }

        /// <summary>
        /// Exactly these copies (draft/sealed decks: the cards the player drafted, with their border/foil) and, optionally, the
        /// opponent's own drafted copies (their foils too) instead of plain Base copies.
        /// </summary>
        public MtgCardFaces(IEnumerable<CardData> copies, IEnumerable<CardData> opponentCopies = null)
        {
            Fill(_mine, copies);
            Fill(_theirs, opponentCopies);
        }

        private readonly Dictionary<string, Queue<CardData>> _theirs = new Dictionary<string, Queue<CardData>>(StringComparer.OrdinalIgnoreCase);

        private static void Fill(Dictionary<string, Queue<CardData>> into, IEnumerable<CardData> copies)
        {
            foreach (var c in copies ?? Enumerable.Empty<CardData>())
            {
                if (c == null || !Registry.IsCustom(c.expansionType)) continue;
                var set = Registry.Get(c.expansionType);
                if (set == null || !set.PosByMonster.TryGetValue(c.monsterType, out int pos)) continue;
                string name = set.Card(pos).Mtg?.Name;
                if (string.IsNullOrEmpty(name)) continue;
                if (!into.TryGetValue(name, out var q)) into[name] = q = new Queue<CardData>();
                q.Enqueue(c);
            }
        }

        /// <summary>Any card from the player's deck (its set's card back stands in for tokens), or null.</summary>
        public CardData Any() => _mine.Values.Where(q => q.Count > 0).Select(q => q.Peek()).FirstOrDefault();

        /// <summary>The in-game card for a Forge card (state JSON), or null when there's none.</summary>
        public CardData For(JObject card, int myPlayerId)
        {
            if (card == null) return null;
            int id = card["id"]?.Type == JTokenType.Integer ? (int)card["id"] : -1;
            if (_byForgeId.TryGetValue(id, out var known)) return known;

            string name = (string)card["oracleName"] ?? (string)card["name"];
            string set = (string)card["set"];
            int owner = card["owner"]?.Type == JTokenType.Integer ? (int)card["owner"] : -1;
            CardData data = null;
            if (owner == myPlayerId && name != null && _mine.TryGetValue(name, out var q) && q.Count > 0)
                data = q.Dequeue(); // one of the player's own copies
            else if (owner >= 0 && owner != myPlayerId && name != null && _theirs.TryGetValue(name, out var tq) && tq.Count > 0)
                data = tq.Dequeue(); // one of the customer's drafted copies
            if (data == null && Find(set, name, out var cs, out int pos))
                data = new CardData
                {
                    expansionType = cs.Expansion,
                    monsterType = cs.Monsters[pos].MonsterType,
                    borderType = ECardBorderType.Base,
                    isFoil = false,
                };
            if (id >= 0 && data != null) _byForgeId[id] = data;
            return data;
        }

        /// <summary>Custom card for a Forge set code + name (set code first, then any set).</summary>
        public static bool Find(string setCode, string name, out CustomSet set, out int pos)
        {
            if (_lookup == null)
            {
                _lookup = new Dictionary<string, (CustomSet, int)>(StringComparer.OrdinalIgnoreCase);
                foreach (var s in Registry.Sets)
                    for (int i = 0; i < s.Def.Cards.Count; i++)
                    {
                        var m = s.Def.Cards[i].Mtg;
                        if (m?.Name == null) continue;
                        string code = s.Def.Mtg?.SetCode ?? "";
                        if (!_lookup.ContainsKey(code + "|" + m.Name)) _lookup[code + "|" + m.Name] = (s, i);
                        if (!_lookup.ContainsKey("|" + m.Name)) _lookup["|" + m.Name] = (s, i);
                    }
            }
            set = null;
            pos = -1;
            if (string.IsNullOrEmpty(name)) return false;
            if (_lookup.TryGetValue((setCode ?? "") + "|" + name, out var hit) || _lookup.TryGetValue("|" + name, out hit))
            {
                set = hit.set;
                pos = hit.pos;
                return true;
            }
            return false;
        }
    }
}
