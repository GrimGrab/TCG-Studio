using System.Collections.Generic;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Per-set replacement for CPlayerData's per-expansion lists. Sized cards × 12 (6 borders × foil),
    /// so there is no 2100-slot cap. Slot layout matches vanilla: pos*12 + border (+6 if foil).
    /// Lists are the exact types vanilla code expects, so they can be handed back from patched getters.
    /// </summary>
    internal class CardStore
    {
        public const int SlotsPerCard = 12;
        public const int BordersPerCard = 6;
        public const int MaxHistory = 30;

        public readonly int SlotCount;
        public readonly List<int> Counts;
        public readonly List<bool> Collected;
        public readonly List<float> PriceSet;
        public readonly List<FloatList> GradedPriceSet;
        public readonly List<MarketPrice> Market;

        public CardStore(int cardCount)
        {
            SlotCount = cardCount * SlotsPerCard;
            Counts = new List<int>(SlotCount);
            Collected = new List<bool>(SlotCount);
            PriceSet = new List<float>(SlotCount);
            GradedPriceSet = new List<FloatList>(SlotCount);
            Market = new List<MarketPrice>(SlotCount);
            for (int i = 0; i < SlotCount; i++)
            {
                Counts.Add(0);
                Collected.Add(false);
                PriceSet.Add(0f);
                GradedPriceSet.Add(NewGradedList());
                Market.Add(new MarketPrice { pastPricePercentChangeList = new List<float>() });
            }
        }

        /// <summary>Clears player state (counts, set prices, drift). Generated base prices are kept.</summary>
        public void ResetPlayerState()
        {
            for (int i = 0; i < SlotCount; i++)
            {
                Counts[i] = 0;
                Collected[i] = false;
                PriceSet[i] = 0f;
                GradedPriceSet[i] = NewGradedList();
                Market[i].pricePercentChangeList = 0f;
                Market[i].pastPricePercentChangeList = new List<float>();
            }
        }

        public bool InRange(int slot) => slot >= 0 && slot < SlotCount;

        public static int Slot(int cardPos, ECardBorderType border, bool foil) =>
            cardPos * SlotsPerCard + (int)border + (foil ? BordersPerCard : 0);

        public static int CardPos(int slot) => slot / SlotsPerCard;
        public static ECardBorderType Border(int slot) => (ECardBorderType)(slot % BordersPerCard);
        public static bool Foil(int slot) => slot % SlotsPerCard >= BordersPerCard;

        private static FloatList NewGradedList()
        {
            var l = new FloatList { floatDataList = new List<float>(10) };
            for (int g = 0; g < 10; g++) l.floatDataList.Add(0f);
            return l;
        }
    }
}
