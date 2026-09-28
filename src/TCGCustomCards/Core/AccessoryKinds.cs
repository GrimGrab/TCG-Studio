using System;

namespace TCGCustomCards.Core
{
    /// <summary>
    /// Which vanilla items belong to which accessory kind. Decided by item name, not category: the game's "Sleeve" category also
    /// holds the UP_* card supplies (other models, other shop tab) and collection books have no category at all.
    /// </summary>
    internal static class AccessoryKinds
    {
        public static AccessoryKind? KindOf(EItemType t)
        {
            if (t < 0 || t >= EItemType.Max) return null;
            string n = t.ToString();
            if (n.StartsWith("DeckBox", StringComparison.Ordinal)) return AccessoryKind.Deckbox;
            if (n.StartsWith("Playmat", StringComparison.Ordinal) || n.StartsWith("PlayMat", StringComparison.Ordinal)) return AccessoryKind.Playmat;
            if (n.StartsWith("CardSleeve_", StringComparison.Ordinal)) return AccessoryKind.Sleeve;
            if (n.StartsWith("D20DiceBox", StringComparison.Ordinal)) return AccessoryKind.Dice;
            if (n.StartsWith("Manga", StringComparison.Ordinal)) return AccessoryKind.Comic;
            if (n.StartsWith("BinderBook", StringComparison.Ordinal)) return AccessoryKind.Binder;
            // Battle decks: category TCG, sold on the booster-pack tab; nothing opens them.
            if (n.StartsWith("PreconDeck_", StringComparison.Ordinal)) return AccessoryKind.BattleDeck;
            return null;
        }

        /// <summary>Base model used when an accessory doesn't name one.</summary>
        public static string DefaultBase(AccessoryKind k)
        {
            switch (k)
            {
                case AccessoryKind.Deckbox: return "DeckBox1";
                case AccessoryKind.Playmat: return "Playmat1";
                case AccessoryKind.Sleeve: return "CardSleeve_Tetramon";
                case AccessoryKind.Dice: return "D20DiceBox";
                case AccessoryKind.Comic: return "Manga1";
                case AccessoryKind.BattleDeck: return "PreconDeck_Fire";
                default: return "BinderBook";
            }
        }
    }
}
