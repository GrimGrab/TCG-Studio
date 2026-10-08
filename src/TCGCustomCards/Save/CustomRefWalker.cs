using System;
using System.Collections;
using System.Collections.Generic;
using System.Reflection;
using TCGCustomCards.Runtime;
using UnityEngine;

namespace TCGCustomCards.Save
{
    /// <summary>
    /// Reflection walker over the game's [Serializable] save objects. Finds, neutralizes or remaps every reference to custom
    /// content: ECardExpansionType / EMonsterType / EItemType / EObjectType / EDecoObject values, CardData and CompactCardDataAmount (whose cardSaveIndex
    /// depends on card order), and the int deck box / playmat item of DeckCompactCardDataList. Types that cannot hold such references are skipped (cached), so big price lists cost nothing.
    /// </summary>
    internal static class CustomRefWalker
    {
        public enum Mode { Detect, Neutralize, Remap }

        /// <summary>Old→new int translation for Remap mode. Returns false when the target no longer exists.</summary>
        public interface IRemapper
        {
            bool Expansion(int oldValue, out int newValue);
            bool Monster(int oldValue, out int newValue);
            bool Item(int oldValue, out int newValue);
            /// <summary>Was this EItemType int a custom item when the data was saved?</summary>
            bool WasCustomItem(int oldValue);
            bool CardSlot(int oldExpansion, int oldSlot, out int newSlot);
            bool Furniture(int oldValue, out int newValue);
            bool Deco(int oldValue, out int newValue);
        }

        private static readonly Dictionary<Type, bool> MayHoldCache = new Dictionary<Type, bool>();
        private static readonly Dictionary<Type, FieldInfo[]> FieldCache = new Dictionary<Type, FieldInfo[]>();

        public static bool IsCustomExpansion(int v) => v >= Registry.ExpansionBase;
        public static bool IsCustomMonster(int v) => v >= Registry.MonsterBase - 1;
        public static bool IsCustomItem(int v) => Registry.IsCustomItem((EItemType)v);
        /// <summary>Custom furniture: anything in our furniture block (other loaders use 200000+, vanilla 0–56).</summary>
        public static bool IsCustomFurniture(int v) => Registry.InFurnitureBlock(v);
        /// <summary>Custom placeable decoration: anything in our EDecoObject block (vanilla 1–183).</summary>
        public static bool IsCustomDeco(int v) => Registry.InDecoBlock(v);

        public static bool IsCustomCard(CardData c) => c != null && (IsCustomExpansion((int)c.expansionType) || IsCustomMonster((int)c.monsterType));

        // Orphans: custom ints that no installed set/card/pack claims (set removed, pre-M4 saves, …).
        public static bool IsOrphanExpansion(int v) => v >= Registry.ExpansionBase && Registry.Get((ECardExpansionType)v) == null;
        public static bool IsOrphanMonster(int v) => v >= Registry.MonsterBase - 1 && !Registry.TryGetCard((EMonsterType)v, out _, out _);
        public static bool IsOrphanItem(int v) => v >= (int)EItemType.Max && !Registry.IsCustomItem((EItemType)v);
        public static bool IsOrphanFurniture(int v) => Registry.InFurnitureBlock(v) && !Registry.IsCustomFurniture((EObjectType)v);
        public static bool IsOrphanDeco(int v) => Registry.InDecoBlock(v) && !Registry.IsCustomDecoration((EDecoObject)v);

        /// <summary>A save entry for a placed piece (it has an EObjectType objectType field) that is one of our furniture pieces.</summary>
        public static bool IsCustomFurnitureEntry(object entry, bool orphansOnly = false)
        {
            if (entry == null) return false;
            var f = entry.GetType().GetField("objectType");
            if (f == null || f.FieldType != typeof(EObjectType)) return false;
            int v = (int)(EObjectType)f.GetValue(entry);
            return orphansOnly ? IsOrphanFurniture(v) : IsCustomFurniture(v);
        }

        /// <summary>
        /// Detect: true if the graph references custom content. Neutralize: strips it (in place). Remap: false if something could not be remapped.
        /// With <paramref name="orphansOnly"/>, only references to content that is no longer installed count as custom.
        /// </summary>
        public static bool Walk(object obj, Mode mode, IRemapper remap = null, bool orphansOnly = false)
        {
            var state = new State { Mode = mode, Remap = remap, OrphansOnly = orphansOnly };
            Visit(obj, state, 0);
            return mode == Mode.Remap ? !state.Failed : state.Found;
        }

        private class State
        {
            public Mode Mode;
            public IRemapper Remap;
            public bool OrphansOnly;
            public bool Found;
            public bool Failed;

            public bool Expansion(int v) => OrphansOnly ? IsOrphanExpansion(v) : IsCustomExpansion(v);
            public bool Monster(int v) => OrphansOnly ? IsOrphanMonster(v) : IsCustomMonster(v);
            public bool Item(int v) => OrphansOnly ? IsOrphanItem(v) : Mode == Mode.Remap ? Remap.WasCustomItem(v) : IsCustomItem(v);
            public bool Furniture(int v) => OrphansOnly ? IsOrphanFurniture(v) : IsCustomFurniture(v);
            public bool Deco(int v) => OrphansOnly ? IsOrphanDeco(v) : IsCustomDeco(v);
            public bool Card(CardData c) => c != null && (Expansion((int)c.expansionType) || Monster((int)c.monsterType));
        }

        public static bool MayHold(Type t)
        {
            if (t == null) return false;
            if (MayHoldCache.TryGetValue(t, out bool v)) return v;
            MayHoldCache[t] = false; // cycle guard
            bool result;
            if (t == typeof(ECardExpansionType) || t == typeof(EMonsterType) || t == typeof(EItemType) || t == typeof(EObjectType) || t == typeof(EDecoObject) || t == typeof(CardData) || t == typeof(CompactCardDataAmount)
                || t == typeof(DeckCompactCardDataList))
                result = true;
            else if (t.IsPrimitive || t.IsEnum || t == typeof(string) || typeof(UnityEngine.Object).IsAssignableFrom(t))
                result = false;
            else if (t.IsArray)
                result = MayHold(t.GetElementType());
            else if (t.IsGenericType && typeof(IList).IsAssignableFrom(t))
                result = MayHold(t.GetGenericArguments()[0]);
            else
            {
                result = false;
                foreach (var f in Fields(t)) if (MayHold(f.FieldType)) { result = true; break; }
            }
            MayHoldCache[t] = result;
            return result;
        }

        public static FieldInfo[] Fields(Type t)
        {
            if (FieldCache.TryGetValue(t, out var fs)) return fs;
            var list = new List<FieldInfo>();
            foreach (var f in t.GetFields(BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic))
                if (!f.IsNotSerialized && (f.IsPublic || f.GetCustomAttribute<SerializeField>() != null)) list.Add(f);
            return FieldCache[t] = list.ToArray();
        }

        private static void Visit(object obj, State s, int depth)
        {
            if (obj == null || depth > 32) return;
            var type = obj.GetType();
            if (!MayHold(type)) return;

            if (obj is IList list)
            {
                VisitList(list, type.IsArray ? type.GetElementType() : type.GetGenericArguments()[0], s, depth);
                return;
            }

            if (obj is CompactCardDataAmount compact && IsCustomExpansion((int)compact.expansionType) && s.Mode == Mode.Remap)
            {
                if (s.Remap.CardSlot((int)compact.expansionType, compact.cardSaveIndex, out int slot)) compact.cardSaveIndex = slot;
                else s.Failed = true;
            }

            if (obj is DeckCompactCardDataList deck)
            {
                deck.deckBoxIndex = DeckItem(deck.deckBoxIndex, s);
                deck.playmatIndex = DeckItem(deck.playmatIndex, s);
            }

            foreach (var f in Fields(type))
            {
                var ft = f.FieldType;
                if (!MayHold(ft)) continue;
                if (ft.IsEnum)
                {
                    int v = Convert.ToInt32(f.GetValue(obj));
                    if (!TryEnum(ft, v, s, out int nv)) continue;
                    f.SetValue(obj, Enum.ToObject(ft, nv));
                    // An item slot that was cleared should not keep a count.
                    if (s.Mode == Mode.Neutralize && ft == typeof(EItemType))
                    {
                        var amount = type.GetField("amount");
                        if (amount != null && amount.FieldType == typeof(int)) amount.SetValue(obj, 0);
                    }
                }
                else Visit(f.GetValue(obj), s, depth + 1);
            }
        }

        private static void VisitList(IList list, Type elem, State s, int depth)
        {
            if (elem.IsEnum)
            {
                for (int i = 0; i < list.Count; i++)
                    if (TryEnum(elem, Convert.ToInt32(list[i]), s, out int nv)) list[i] = Enum.ToObject(elem, nv);
                return;
            }
            List<int> remove = null;
            for (int i = 0; i < list.Count; i++)
            {
                var item = list[i];
                if (item == null) continue;
                if (s.Mode == Mode.Neutralize)
                {
                    if (item is CardData cd && s.Card(cd)) { list[i] = BlankCard(); s.Found = true; continue; }
                    if (item is CompactCardDataAmount cc && s.Expansion((int)cc.expansionType)) { (remove ??= new List<int>()).Add(i); s.Found = true; continue; }
                }
                Visit(item, s, depth + 1);
            }
            if (remove != null && !list.IsFixedSize)
                for (int k = remove.Count - 1; k >= 0; k--) list.RemoveAt(remove[k]);
        }

        /// <summary>Handles one enum value. Returns true if the value must be replaced by <paramref name="newValue"/>.</summary>
        private static bool TryEnum(Type t, int v, State s, out int newValue)
        {
            newValue = v;
            bool custom = t == typeof(ECardExpansionType) ? s.Expansion(v)
                        : t == typeof(EMonsterType) ? s.Monster(v)
                        : t == typeof(EObjectType) ? s.Furniture(v)
                        : t == typeof(EDecoObject) ? s.Deco(v)
                        : t == typeof(EItemType) && s.Item(v);
            if (!custom) return false;
            s.Found = true;
            switch (s.Mode)
            {
                case Mode.Detect:
                    return false;
                case Mode.Neutralize:
                    newValue = t == typeof(ECardExpansionType) ? (int)ECardExpansionType.Tetramon
                             : t == typeof(EMonsterType) ? (int)EMonsterType.None
                             : t == typeof(EObjectType) ? (int)EObjectType.None
                             : t == typeof(EDecoObject) ? (int)EDecoObject.None
                             : (int)EItemType.None;
                    return true;
                default:
                    bool ok = t == typeof(ECardExpansionType) ? s.Remap.Expansion(v, out newValue)
                            : t == typeof(EMonsterType) ? s.Remap.Monster(v, out newValue)
                            : t == typeof(EObjectType) ? s.Remap.Furniture(v, out newValue)
                            : t == typeof(EDecoObject) ? s.Remap.Deco(v, out newValue)
                            : s.Remap.Item(v, out newValue);
                    if (!ok) s.Failed = true;
                    return ok;
            }
        }

        /// <summary>
        /// A deck's deck box / playmat is an EItemType stored as a plain int (0 = none). Custom ones are cleared to 0 in the vanilla save
        /// and remapped on restore; one that can't be remapped (accessory deleted) becomes "none" rather than failing the whole deck.
        /// </summary>
        private static int DeckItem(int v, State s)
        {
            if (v <= 0 || !s.Item(v)) return v;
            s.Found = true;
            switch (s.Mode)
            {
                case Mode.Detect: return v;
                case Mode.Neutralize: return 0;
                default: return s.Remap.Item(v, out int nv) ? nv : 0;
            }
        }

        public static CardData BlankCard() => new CardData { monsterType = EMonsterType.None, expansionType = ECardExpansionType.Tetramon };
    }
}
