using System;
using System.Collections.Generic;
using CC;
using HarmonyLib;
using UnityEngine;

namespace TCGCustomCards.Hooks
{
    /// <summary>
    /// Single owner of the patches on the "CC" character customizer (CC.CharacterCustomization) that dresses customers and
    /// workers. Vanilla always runs; these postfixes only tell what it just did. A character = a preset (Male3, Female0…)
    /// whose ApplyCharacterVars puts one apparel piece per slot on through setApparel, which instantiates the piece and gives
    /// its renderers their own material instances with the variant's colours and print, so a handler may restyle them for
    /// this one character.
    /// </summary>
    internal static class CustomerLooks
    {
        /// <summary>
        /// A piece was put on: (character, slot, piece, variant index, the piece's object). The object's materials are this
        /// character's own instances. Not raised for an empty slot.
        /// </summary>
        public static event Action<CharacterCustomization, int, scrObj_Apparel.Apparel, int, GameObject> PieceOn;

        /// <summary>The character's preset was applied (every slot dressed); handlers may change pieces with setApparel.</summary>
        public static event Action<CharacterCustomization> Dressed;

        /// <summary>
        /// A customer's portrait (tournament pairing screen, the mod's profile panel): (character model, female) to a sprite
        /// shown instead of the game's, or null for the game's. One handler.
        /// </summary>
        public static Func<int, bool, Sprite> Portrait;

        /// <summary>The preset a character starts from, instead of the name the game gave it; null for the game's. One handler.</summary>
        public static Func<CharacterCustomization, string> PresetName;

        /// <summary>Relies on: Initialize (CharacterCustomization.cs :89) loading the preset named by CharacterName (LoadFromJSON).</summary>
        [HarmonyPatch(typeof(CharacterCustomization), nameof(CharacterCustomization.Initialize))]
        private static class PresetPatch
        {
            private static void Prefix(CharacterCustomization __instance)
            {
                var name = PresetName?.Invoke(__instance);
                if (!string.IsNullOrEmpty(name)) __instance.CharacterName = name;
            }
        }

        private static readonly AccessTools.FieldRef<CharacterCustomization, List<GameObject>> ApparelObjects =
            AccessTools.FieldRefAccess<CharacterCustomization, List<GameObject>>("ApparelObjects");

        private static void Raise(Action a, string what)
        {
            try { a(); }
            catch (Exception e) { Plugin.Log.LogError($"CustomerLooks {what} handler failed: {e}"); }
        }

        /// <summary>
        /// Relies on: setApparel(selection, slot, materialSelection) (CharacterCustomization.cs :264) returning early for a
        /// selection outside ApparelTables[slot].Items, else storing the instantiated piece in ApparelObjects[slot] (none when
        /// the piece has no Mesh) and setting each renderer's materials[j] from Materials[materialSelection].
        /// </summary>
        [HarmonyPatch(typeof(CharacterCustomization), nameof(CharacterCustomization.setApparel))]
        private static class SetApparelPatch
        {
            private static void Postfix(CharacterCustomization __instance, int selection, int slot, int materialSelection)
            {
                var h = PieceOn;
                if (h == null || slot < 0 || slot >= __instance.ApparelTables.Count) return;
                var items = __instance.ApparelTables[slot].Items;
                if (selection < 0 || selection >= items.Count || items[selection].Mesh == null) return;
                var objs = ApparelObjects(__instance);
                var obj = objs != null && slot < objs.Count ? objs[slot] : null;
                if (obj == null) return;
                Raise(() => h(__instance, slot, items[selection], materialSelection, obj), "PieceOn");
            }
        }

        /// <summary>Relies on: CustomerManager.GetCustomerIconSprite (:641) indexing the icon lists by character model.</summary>
        [HarmonyPatch(typeof(CustomerManager), nameof(CustomerManager.GetCustomerIconSprite))]
        private static class PortraitPatch
        {
            private static void Postfix(int characterModelIndex, bool isFemale, ref Sprite __result)
            {
                var h = Portrait;
                if (h == null) return;
                try { __result = h(characterModelIndex, isFemale) ?? __result; }
                catch (Exception e) { Plugin.Log.LogError($"CustomerLooks Portrait handler failed: {e}"); }
            }
        }

        /// <summary>Relies on: ApplyCharacterVars (CharacterCustomization.cs :152) dressing every slot from the preset.</summary>
        [HarmonyPatch(typeof(CharacterCustomization), nameof(CharacterCustomization.ApplyCharacterVars))]
        private static class ApplyPatch
        {
            private static void Postfix(CharacterCustomization __instance)
            {
                var h = Dressed;
                if (h != null) Raise(() => h(__instance), "Dressed");
            }
        }
    }
}
