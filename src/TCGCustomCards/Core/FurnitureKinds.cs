using System;
using System.Collections.Generic;
using UnityEngine;

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
                case FurnitureType.WarehouseShelf: return typeof(global::WarehouseShelf);
                case FurnitureType.TournamentPrizeShelf: return typeof(global::TournamentPrizeShelf);
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
                case FurnitureType.WarehouseShelf: return "WarehouseShelf";
                case FurnitureType.TournamentPrizeShelf: return "TournamentPrizeShelf";
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

        /// <summary>What a role's points are: where a person stands/sits, a spot the game uses (screen UI, where money lands…), or a
        /// working part with its own look (cash drawer, card machine, signs, the prize shelf's TV) that stays visible under an own model.</summary>
        public enum PointKind { Person, Spot, Part }

        /// <summary>
        /// A position role: the game field that holds it (a Transform, GameObject or component, or a List of Transforms) and whether its
        /// count may change. Parent: the role moves the parent of that object (the drawer's animation plays on its own transform).
        /// </summary>
        public struct PointRole
        {
            public string Role, Field;
            public bool Resizable, Parent;
            public PointKind Kind;
            public PointRole(string role, string field, bool resizable = false, PointKind kind = PointKind.Person, bool parent = false)
            { Role = role; Field = field; Resizable = resizable; Kind = kind; Parent = parent; }
        }

        /// <summary>The Transform a (non-list) role moves on a piece; null when the field is missing or unset.</summary>
        public static Transform RoleTransform(InteractableObject piece, PointRole role)
        {
            var field = piece.GetType().GetField(role.Field, System.Reflection.BindingFlags.Instance | System.Reflection.BindingFlags.Public | System.Reflection.BindingFlags.NonPublic);
            var v = field?.GetValue(piece);
            Transform t = v is Transform tr ? tr : v is GameObject go ? go.transform : v is Component c ? c.transform : null;
            return t != null && role.Parent ? t.parent : t;
        }

        /// <summary>Position roles per type (fields right after InteractableObject's in each class; TCG Studio reads the same ones).</summary>
        public static PointRole[] PointRoles(FurnitureType t)
        {
            switch (t)
            {
                case FurnitureType.PlayTable:
                    return new[] { new PointRole("sit", "m_SitLocList"), new PointRole("stand", "m_StandLocList"), new PointRole("standB", "m_StandLocBList") };
                case FurnitureType.CashCounter:
                    // Not the card screen: it sits on the card machine and moves with it.
                    return new[] { new PointRole("cashier", "m_LockPlayerPos"), new PointRole("queue", "m_QueueStartPos"),
                        new PointRole("placeItems", "m_CustomerPlaceItemPos", kind: PointKind.Spot), new PointRole("trade", "m_TradeCardStandLocList", true),
                        new PointRole("screen", "m_CounterScreenFollowLoc", kind: PointKind.Spot),
                        new PointRole("scanItem", "m_ScannedItemLerpPos", kind: PointKind.Spot),
                        new PointRole("money", "m_PlaceMoneyLocation", kind: PointKind.Spot),
                        new PointRole("coin", "m_PlaceCoinLocation", kind: PointKind.Spot),
                        new PointRole("drawer", "m_OpenCloseDrawerAnim", kind: PointKind.Part, parent: true),
                        new PointRole("cardMachine", "m_CreditCardMachineModel", kind: PointKind.Part),
                        new PointRole("cardPay", "m_CreditCardMachineTargetPos", kind: PointKind.Spot),
                        new PointRole("cardLook", "m_CreditCardPlayerLookRot", kind: PointKind.Spot),
                        new PointRole("bag", "m_ScanItemPlasticBag", kind: PointKind.Part),
                        new PointRole("closedSign", "m_CounterClosedMesh", kind: PointKind.Part),
                        new PointRole("tradeSign", "m_NoTradingNoticeMesh", kind: PointKind.Part) };
                case FurnitureType.AutoPackOpener:
                    return new[] { new PointRole("worker", "m_StandLoc"), new PointRole("screen", "m_UIPos", kind: PointKind.Spot),
                        new PointRole("packIn", "m_Pos", kind: PointKind.Spot), new PointRole("packInside", "m_PosInside", kind: PointKind.Spot) };
                case FurnitureType.AutoCleanser: return new[] { new PointRole("worker", "m_StandLoc") };
                case FurnitureType.Workbench: return new[] { new PointRole("player", "m_LockPlayerPos") };
                case FurnitureType.BulkDonationBox:
                case FurnitureType.CardStorageShelf:
                case FurnitureType.EmptyBoxStorage: return new[] { new PointRole("customer", "m_CustomerStandLocList", true) };
                // Winner points are picked by tournament placement: keep the base's count.
                case FurnitureType.TournamentPrizeShelf:
                    return new[] { new PointRole("customer", "m_CustomerStandLocList", true), new PointRole("winner", "m_WinnerStandLocList"),
                        new PointRole("screen", "m_ScreenMesh", kind: PointKind.Part) };
                default: return new PointRole[0];
            }
        }

        public static bool HasItemSpots(FurnitureType t) =>
            t == FurnitureType.Shelf || t == FurnitureType.WarehouseShelf || t == FurnitureType.TournamentPrizeShelf;
        public static bool HasCardSpots(FurnitureType t) => t == FurnitureType.CardShelf || t == FurnitureType.TournamentPrizeShelf;

        /// <summary>Groups whose children are the piece's item compartments (shop, warehouse and combi shelves; null = none).</summary>
        public static List<Transform> ItemGroups(InteractableObject piece)
        {
            switch (piece)
            {
                case global::Shelf s: return s.m_ShelfCompartmentGrpList;
                case global::WarehouseShelf w: return w.m_ShelfCompartmentGrpList;
                case CardItemCombiShelf c: return c.m_ShelfCompartmentGrpList;
                default: return null;
            }
        }

        /// <summary>Groups whose children are the piece's card compartments (card shelves incl. combi/prize shelves; null = none).</summary>
        public static List<Transform> CardGroups(InteractableObject piece) => (piece as global::CardShelf)?.m_CardShelfCompartmentGrpList;

        /// <summary>Kept in the shop even when vanilla furniture is hidden, unless a custom piece of the type exists.</summary>
        public static bool IsEssential(FurnitureType t) =>
            t == FurnitureType.CashCounter || t == FurnitureType.Workbench || t == FurnitureType.TrashBin || t == FurnitureType.EmptyBoxStorage;
    }
}
