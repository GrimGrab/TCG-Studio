using System;

namespace TCGCustomCards.Core
{
    /// <summary>
    /// Which vanilla furniture belongs to which <see cref="FurnitureType"/>, and what each type can hold. The type is decided by the
    /// piece's component (checked on the prefab when injecting); these names are only the defaults and the allowed bases.
    /// </summary>
    internal static class FurnitureKinds
    {
        /// <summary>Game component that makes a piece this type (checked on the base prefab).</summary>
        public static Type ComponentOf(FurnitureType t)
        {
            switch (t)
            {
                case FurnitureType.Shelf: return typeof(Shelf);
                case FurnitureType.CardShelf: return typeof(CardShelf);
                case FurnitureType.PlayTable: return typeof(InteractablePlayTable);
                case FurnitureType.BulkDonationBox: return typeof(InteractableBulkDonationBox);
                case FurnitureType.TrashBin: return typeof(InteractableTrashBin);
                case FurnitureType.EmptyBoxStorage: return typeof(InteractableEmptyBoxStorage);
                case FurnitureType.CardStorageShelf: return typeof(InteractableCardStorageShelf);
                case FurnitureType.AutoPackOpener: return typeof(InteractableAutoPackOpener);
                case FurnitureType.AutoCleanser: return typeof(InteractableAutoCleanser);
                case FurnitureType.Workbench: return typeof(InteractableWorkbench);
                default: return typeof(InteractableCashierCounter);
            }
        }

        /// <summary>Vanilla piece used when a def names no base.</summary>
        public static string DefaultBase(FurnitureType t)
        {
            switch (t)
            {
                case FurnitureType.Shelf: return "Shelf";
                case FurnitureType.CardShelf: return "CardShelf";
                case FurnitureType.PlayTable: return "PlayTable";
                case FurnitureType.BulkDonationBox: return "BulkDonationBox";
                case FurnitureType.TrashBin: return "Trashbin";
                case FurnitureType.EmptyBoxStorage: return "EmptyBoxStorage";
                case FurnitureType.CardStorageShelf: return "CardStorageShelf";
                case FurnitureType.AutoPackOpener: return "AutoPackOpener1";
                case FurnitureType.AutoCleanser: return "AutoCleanser1";
                case FurnitureType.Workbench: return "Workbench";
                default: return "CashCounter";
            }
        }

        /// <summary>
        /// The piece's own component must be exactly this type's component (a TournamentPrizeShelf is also a CardShelf but is
        /// saved in a different list).
        /// </summary>
        public static bool Matches(FurnitureType t, InteractableObject prefab)
        {
            if (prefab == null) return false;
            return prefab.GetType() == ComponentOf(t);
        }

        /// <summary>A position role: the game field that holds it (a Transform or a List of them) and whether its count may change.</summary>
        public struct PointRole
        {
            public string Role, Field;
            public bool Resizable;
            public PointRole(string role, string field, bool resizable = false) { Role = role; Field = field; Resizable = resizable; }
        }

        /// <summary>Position roles per type (fields right after InteractableObject's in each class; TCG Studio reads the same ones).</summary>
        public static PointRole[] PointRoles(FurnitureType t)
        {
            switch (t)
            {
                case FurnitureType.PlayTable:
                    return new[] { new PointRole("sit", "m_SitLocList"), new PointRole("stand", "m_StandLocList"), new PointRole("standB", "m_StandLocBList") };
                case FurnitureType.CashCounter:
                    return new[] { new PointRole("cashier", "m_LockPlayerPos"), new PointRole("queue", "m_QueueStartPos"),
                        new PointRole("placeItems", "m_CustomerPlaceItemPos"), new PointRole("trade", "m_TradeCardStandLocList", true) };
                case FurnitureType.AutoPackOpener:
                case FurnitureType.AutoCleanser: return new[] { new PointRole("worker", "m_StandLoc") };
                case FurnitureType.Workbench: return new[] { new PointRole("player", "m_LockPlayerPos") };
                case FurnitureType.BulkDonationBox:
                case FurnitureType.CardStorageShelf:
                case FurnitureType.EmptyBoxStorage: return new[] { new PointRole("customer", "m_CustomerStandLocList", true) };
                default: return new PointRole[0];
            }
        }

        public static bool HasItemSpots(FurnitureType t) => t == FurnitureType.Shelf;
        public static bool HasCardSpots(FurnitureType t) => t == FurnitureType.CardShelf;

        /// <summary>Kept in the shop even when vanilla furniture is hidden, unless a custom piece of the type exists.</summary>
        public static bool IsEssential(FurnitureType t) =>
            t == FurnitureType.CashCounter || t == FurnitureType.Workbench || t == FurnitureType.TrashBin || t == FurnitureType.EmptyBoxStorage;
    }
}
