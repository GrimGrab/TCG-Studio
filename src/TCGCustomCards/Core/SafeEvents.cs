using System;

namespace TCGCustomCards.Core
{
    /// <summary>Raises an event so one failing handler neither stops the others nor reaches the game's code that led here.</summary>
    internal static class SafeEvents
    {
        private static void Each<TDelegate>(TDelegate handlers, Action<TDelegate> call) where TDelegate : Delegate
        {
            if (handlers == null) return;
            foreach (TDelegate h in handlers.GetInvocationList())
            {
                try { call(h); }
                catch (Exception e) { Plugin.Log.LogError($"Event handler failed: {e}"); }
            }
        }

        public static void Raise<T>(Action<T> handlers, T a) => Each(handlers, h => h(a));
        public static void Raise<T1, T2>(Action<T1, T2> handlers, T1 a, T2 b) => Each(handlers, h => h(a, b));
        public static void Raise<T1, T2, T3>(Action<T1, T2, T3> handlers, T1 a, T2 b, T3 c) => Each(handlers, h => h(a, b, c));
    }
}
