using System;
using System.Collections.Generic;
using System.Linq;
using Newtonsoft.Json;
using TCGCustomCards.Core;
using TCGCustomCards.Save;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>MTG decks as saved in the side-car (stable ids, never runtime ints).</summary>
    internal sealed class MtgDeckSave
    {
        [JsonProperty("name")] public string Name = "Deck";
        [JsonProperty("cards")] public List<MtgDeckCardSave> Cards = new List<MtgDeckCardSave>();
        /// <summary>Free basic lands by name (Plains, Island, …).</summary>
        [JsonProperty("basics")] public Dictionary<string, int> Basics = new Dictionary<string, int>();
    }

    internal sealed class MtgDeckCardSave
    {
        [JsonProperty("set")] public string Set;
        [JsonProperty("card")] public string Card;
        /// <summary>Border + foil, same key as the rest of the side-car ("Base", "Gold_foil", …).</summary>
        [JsonProperty("variant")] public string Variant;
        [JsonProperty("count")] public int Count;
    }

    /// <summary>
    /// The player's MTG decks (option [MTG] MtgDeckBuilder). Cards in a deck are taken out of the collection
    /// (CPlayerData.ReduceCard) and given back when removed or when the deck is deleted, like the vanilla builder.
    /// Entries of sets that aren't installed stay in the save untouched (not playable, not returned).
    /// </summary>
    internal static class MtgDeckStore
    {
        internal sealed class Entry
        {
            public MtgDeckCardSave Save;
            public CustomSet Set;      // null = set not installed
            public int Pos = -1;
            public ECardBorderType Border;
            public bool Foil;
            public bool Live => Set != null && Pos >= 0;
            public CardData Data => new CardData
            {
                expansionType = Set.Expansion, monsterType = Set.Monsters[Pos].MonsterType, borderType = Border, isFoil = Foil,
            };
        }

        internal sealed class Deck
        {
            public MtgDeckSave Save;
            public readonly List<Entry> Entries = new List<Entry>();
            public string Name { get => Save.Name; set => Save.Name = value; }
            public int Basics(string name) => Save.Basics.TryGetValue(name, out int n) ? n : 0;
            public int Total => Entries.Where(e => e.Live).Sum(e => e.Save.Count) + Save.Basics.Values.Sum();
            public List<string> Problems => MtgDeckRules.Problems(Total);
            public bool Valid => Problems.Count == 0;
        }

        public static readonly List<Deck> Decks = new List<Deck>();
        public static int Active = -1;
        public static Deck ActiveDeck => Active >= 0 && Active < Decks.Count ? Decks[Active] : null;

        // ------------------------------------------------------------------ side-car

        public static void Load(List<MtgDeckSave> saved, int active)
        {
            Decks.Clear();
            foreach (var s in saved ?? new List<MtgDeckSave>())
            {
                if (s == null) continue;
                s.Cards = s.Cards ?? new List<MtgDeckCardSave>();
                s.Basics = s.Basics ?? new Dictionary<string, int>();
                var d = new Deck { Save = s };
                foreach (var c in s.Cards) d.Entries.Add(Resolve(c));
                Decks.Add(d);
            }
            Active = active >= 0 && active < Decks.Count && Decks[active].Valid ? active : -1;
        }

        public static List<MtgDeckSave> Export() => Decks.Select(d => d.Save).ToList();

        private static Entry Resolve(MtgDeckCardSave c)
        {
            var e = new Entry { Save = c };
            var set = c.Set == null ? null : Registry.Get(c.Set);
            if (set != null && c.Card != null && set.PosByCardId.TryGetValue(c.Card, out int pos) &&
                SideCar.TryParseVariant(c.Variant ?? "Base", out var border, out bool foil))
            {
                e.Set = set;
                e.Pos = pos;
                e.Border = border;
                e.Foil = foil;
            }
            return e;
        }

        // ------------------------------------------------------------------ decks

        public static Deck Create()
        {
            int n = 1;
            while (Decks.Any(d => d.Name == $"MTG Deck {n}")) n++;
            var deck = new Deck { Save = new MtgDeckSave { Name = $"MTG Deck {n}" } };
            Decks.Add(deck);
            return deck;
        }

        /// <summary>Deletes a deck, returning its cards to the collection.</summary>
        public static void Delete(Deck deck)
        {
            foreach (var e in deck.Entries.Where(e => e.Live && e.Save.Count > 0))
                CPlayerData.AddCard(e.Data, e.Save.Count);
            int i = Decks.IndexOf(deck);
            Decks.Remove(deck);
            if (Active == i) Active = -1;
            else if (Active > i) Active--;
        }

        public static bool SetActive(Deck deck)
        {
            if (deck == null || !deck.Valid) return false;
            Active = Decks.IndexOf(deck);
            return true;
        }

        // ------------------------------------------------------------------ cards

        /// <summary>Copies of an MTG name in the deck (all printings/variants together).</summary>
        public static int CopiesOf(Deck deck, string mtgName) =>
            deck.Entries.Where(e => e.Live && string.Equals(e.Set.Card(e.Pos).Mtg?.Name, mtgName, StringComparison.OrdinalIgnoreCase))
                .Sum(e => e.Save.Count);

        /// <summary>Owned copies (collection) of a card, all variants.</summary>
        public static int Owned(CustomSet set, int pos)
        {
            int n = 0;
            for (int s = 0; s < CardStore.SlotsPerCard; s++) n += set.Store.Counts[pos * CardStore.SlotsPerCard + s];
            return n;
        }

        /// <summary>
        /// Takes one owned copy of a card out of the collection into the deck (the cheapest variant: Base non-foil first, foils
        /// last). Returns why it couldn't (null = added).
        /// </summary>
        public static string AddCard(Deck deck, CustomSet set, int pos, MtgCardInfo info)
        {
            string why = MtgDeckRules.CanAddCopy(info, CopiesOf(deck, info.Name));
            if (why != null) return why;
            for (int f = 0; f < 2; f++)
                for (int b = 0; b < CardStore.BordersPerCard; b++)
                {
                    var border = (ECardBorderType)b;
                    bool foil = f == 1;
                    if (set.Store.Counts[CardStore.Slot(pos, border, foil)] <= 0) continue;
                    var data = new CardData { expansionType = set.Expansion, monsterType = set.Monsters[pos].MonsterType, borderType = border, isFoil = foil };
                    CPlayerData.ReduceCard(data, 1);
                    string variant = PriceModel.VariantKey(border, foil);
                    var entry = deck.Entries.FirstOrDefault(e => e.Live && e.Set == set && e.Pos == pos && e.Border == border && e.Foil == foil);
                    if (entry == null)
                    {
                        var save = new MtgDeckCardSave { Set = set.Def.Id, Card = set.Card(pos).Id, Variant = variant, Count = 0 };
                        deck.Save.Cards.Add(save);
                        entry = new Entry { Save = save, Set = set, Pos = pos, Border = border, Foil = foil };
                        deck.Entries.Add(entry);
                    }
                    entry.Save.Count++;
                    return null;
                }
            return "You don't have any more copies of this card";
        }

        /// <summary>Owned copies of one version (border + foil) of a card.</summary>
        public static int OwnedVariant(CustomSet set, int pos, ECardBorderType border, bool foil) =>
            set.Store.Counts[CardStore.Slot(pos, border, foil)];

        /// <summary>Takes one copy of a specific version out of the collection into the deck. Returns why it couldn't (null = added).</summary>
        public static string AddVariant(Deck deck, CustomSet set, int pos, ECardBorderType border, bool foil, MtgCardInfo info)
        {
            string why = MtgDeckRules.CanAddCopy(info, CopiesOf(deck, info.Name));
            if (why != null) return why;
            if (OwnedVariant(set, pos, border, foil) <= 0) return "You don't have any more copies of this version";
            var data = new CardData { expansionType = set.Expansion, monsterType = set.Monsters[pos].MonsterType, borderType = border, isFoil = foil };
            CPlayerData.ReduceCard(data, 1);
            var entry = deck.Entries.FirstOrDefault(e => e.Live && e.Set == set && e.Pos == pos && e.Border == border && e.Foil == foil);
            if (entry == null)
            {
                var save = new MtgDeckCardSave { Set = set.Def.Id, Card = set.Card(pos).Id, Variant = PriceModel.VariantKey(border, foil), Count = 0 };
                deck.Save.Cards.Add(save);
                entry = new Entry { Save = save, Set = set, Pos = pos, Border = border, Foil = foil };
                deck.Entries.Add(entry);
            }
            entry.Save.Count++;
            return null;
        }

        /// <summary>Puts one copy back into the collection.</summary>
        public static void RemoveOne(Deck deck, Entry entry)
        {
            if (!entry.Live || entry.Save.Count <= 0) return;
            CPlayerData.AddCard(entry.Data, 1);
            entry.Save.Count--;
            if (entry.Save.Count == 0)
            {
                deck.Entries.Remove(entry);
                deck.Save.Cards.Remove(entry.Save);
            }
            if (deck == ActiveDeck && !deck.Valid) Active = -1;
        }

        public static void ChangeBasic(Deck deck, string name, int delta)
        {
            int n = Math.Max(0, deck.Basics(name) + delta);
            if (n == 0) deck.Save.Basics.Remove(name);
            else deck.Save.Basics[name] = n;
            if (deck == ActiveDeck && !deck.Valid) Active = -1;
        }

        /// <summary>Builder info for a custom card (MTG data + fallbacks for sets installed before rarity/cmc existed).</summary>
        public static MtgCardInfo Info(CustomSet set, int pos)
        {
            var def = set.Card(pos);
            var m = def.Mtg;
            if (m == null) return null;
            return new MtgCardInfo
            {
                Name = m.Name,
                TypeLine = m.TypeLine ?? "",
                ManaCost = m.ManaCost ?? "",
                Colors = m.Colors ?? new List<string>(),
                Rarity = !string.IsNullOrEmpty(m.Rarity) ? m.Rarity : FallbackRarity(set.Rarity(pos).Tier),
                Cmc = m.Cmc > 0 ? m.Cmc : MtgDeckRules.CmcFromCost(m.ManaCost),
                Text = def.Description ?? "",
                SetId = set.Def.Id,
                SetName = set.Def.Name,
            };
        }

        private static string FallbackRarity(ERarity r)
        {
            switch (r)
            {
                case ERarity.Common: return "common";
                case ERarity.Rare: return "uncommon";
                case ERarity.Epic: return "rare";
                default: return "mythic";
            }
        }
    }
}
