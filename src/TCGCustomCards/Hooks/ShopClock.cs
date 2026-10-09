using System.Collections.Generic;
using HarmonyLib;

namespace TCGCustomCards.Hooks
{
    /// <summary>
    /// Single owner of the patch that stops the shop's day clock (LightManager.Update: time of day, sun, skybox, light timers).
    /// Vanilla never pauses it (menus and the phone let time run). While any reason holds it, LightManager.Update is skipped;
    /// customers keep moving. See docs/hooks.md.
    /// </summary>
    internal static class ShopClock
    {
        private static readonly HashSet<string> Reasons = new HashSet<string>();

        public static bool Held => Reasons.Count > 0;

        public static void Hold(string reason)
        {
            if (Reasons.Add(reason)) Plugin.Log.LogInfo($"Shop clock held ({reason})");
        }

        public static void Release(string reason)
        {
            if (Reasons.Remove(reason)) Plugin.Log.LogInfo($"Shop clock released ({reason})");
        }

        [HarmonyPatch(typeof(LightManager), "Update")]
        private static class ClockPatch
        {
            private static bool Prefix() => Reasons.Count == 0;
        }
    }
}
