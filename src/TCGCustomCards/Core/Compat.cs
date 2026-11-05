using System;
using System.Collections.Generic;
using System.Linq;
using System.Reflection;
using HarmonyLib;

namespace TCGCustomCards.Core
{
    /// <summary>
    /// Single owner of "does the game still match what the mod relies on". After a game update, a mismatch switches off only the
    /// feature that needs it (one "Game compatibility:" error line, listed by TCG Studio's Debug log page) instead of breaking
    /// the game or other features:
    /// <list type="bullet">
    /// <item><see cref="Patch"/>: every patch class on its own — a game method that moved or vanished skips that class only
    /// (Harmony's PatchAll stops at the first failure, leaving the rest unpatched).</item>
    /// <item><see cref="Field{T,F}"/>: a private game field. With an owner class, when it's gone (reported) the patches of that class's
    /// family aren't applied (Patch reads the fields first) and other code using it reads an empty value. With a feature name: null.</item>
    /// <item><see cref="Report"/> / <see cref="Ok"/>: features check their own assumptions and stay off when one failed.</item>
    /// </list>
    /// </summary>
    internal static class Compat
    {
        private static readonly Dictionary<string, string> Problems = new Dictionary<string, string>(); // feature → first problem

        /// <summary>False once <paramref name="feature"/> reported a mismatch with the game.</summary>
        public static bool Ok(string feature) => !Problems.ContainsKey(feature);

        /// <summary>Switches <paramref name="feature"/> off for this game start (logged once).</summary>
        public static void Report(string feature, string problem)
        {
            if (Problems.ContainsKey(feature)) return;
            Problems[feature] = problem;
            Plugin.Log.LogError($"Game compatibility: {feature} is off — {problem}"); // TCG Studio's Debug log page lists these
        }

        /// <summary>Applies every [HarmonyPatch] class of <paramref name="assembly"/> on its own; a class whose target is gone is reported.</summary>
        public static void Patch(Harmony harmony, Assembly assembly)
        {
            int ok = 0;
            var types = AccessTools.GetTypesFromAssembly(assembly); // every type, like PatchAll (no-op for non-patch classes)
            foreach (var type in types) ReadFieldRefs(type); // first: a class patched below may share a family with a later one
            foreach (var type in types)
            {
                if (!Ok(FeatureOf(type))) continue; // a field it relies on is gone (reported)
                try
                {
                    var patched = harmony.CreateClassProcessor(type).Patch();
                    if (patched != null && patched.Count > 0) ok++;
                }
                catch (Exception e)
                {
                    Report(FeatureOf(type), $"patch {type.FullName} doesn't fit this game version: {(e.InnerException ?? e).Message}");
                }
            }
            Plugin.Log.LogInfo($"Game compatibility: {ok} patch class(es) applied, {Problems.Count} feature(s) off");
        }

        /// <summary>Runs the static initialisers of a class holding field references, so a field that is gone is known before patching.</summary>
        private static void ReadFieldRefs(Type type)
        {
            try
            {
                if (type.ContainsGenericParameters) return;
                const BindingFlags all = BindingFlags.Static | BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.DeclaredOnly;
                if (type.GetFields(all).Any(f => f.FieldType.IsGenericType && f.FieldType.GetGenericTypeDefinition() == typeof(AccessTools.FieldRef<,>)))
                    System.Runtime.CompilerServices.RuntimeHelpers.RunClassConstructor(type.TypeHandle);
            }
            catch (Exception e)
            {
                Report(FeatureOf(type), $"{type.FullName} couldn't start: {(e.InnerException ?? e).Message}");
            }
        }

        /// <summary>A private field of a game class for <paramref name="owner"/>'s family. Never null: when it's gone, that family's patches
        /// aren't applied and the reference reads an empty value and ignores writes.</summary>
        public static AccessTools.FieldRef<T, F> Field<T, F>(Type owner, string name) where T : class =>
            Field<T, F>(FeatureOf(owner), name) ?? Missing<T, F>.Ref;

        /// <summary>What a reference to a field that's gone points at: emptied again on every use.</summary>
        private static class Missing<T, F> where T : class
        {
            private static F _slot;
            internal static readonly AccessTools.FieldRef<T, F> Ref = Empty;

            private static ref F Empty(T _)
            {
                _slot = default;
                return ref _slot;
            }
        }

        /// <summary>A private field of a game class, or null (reported against <paramref name="feature"/>) when the game no longer has it.</summary>
        public static AccessTools.FieldRef<T, F> Field<T, F>(string feature, string name) where T : class
        {
            var f = AccessTools.Field(typeof(T), name);
            if (f == null || !typeof(F).IsAssignableFrom(f.FieldType))
            {
                Report(feature, $"{typeof(T).Name}.{name} ({typeof(F).Name}) is gone");
                return null;
            }
            return AccessTools.FieldRefAccess<T, F>(f);
        }

        /// <summary>A private instance method of a game class without arguments, or null (reported) when it's gone.</summary>
        public static Action<T> Method<T>(string feature, string name) where T : class
        {
            var m = AccessTools.Method(typeof(T), name, Type.EmptyTypes);
            if (m == null)
            {
                Report(feature, $"{typeof(T).Name}.{name}() is gone");
                return null;
            }
            return AccessTools.MethodDelegate<Action<T>>(m);
        }

        /// <summary>Feature name of a patch class: its outermost class without "Patch(es)" (e.g. PackSizePatches → "PackSize").</summary>
        private static string FeatureOf(Type t)
        {
            while (t.DeclaringType != null) t = t.DeclaringType;
            var n = t.Name;
            foreach (var s in new[] { "Patches", "Patch" })
                if (n.EndsWith(s) && n.Length > s.Length) return n.Substring(0, n.Length - s.Length);
            return n;
        }
    }
}
