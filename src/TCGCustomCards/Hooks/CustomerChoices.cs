using System;
using System.Collections.Generic;
using HarmonyLib;
using UnityEngine;

namespace TCGCustomCards.Hooks
{
    /// <summary>
    /// Single owner of a customer's "what next" (docs/hooks.md "CustomerChoices"). Vanilla's DetermineShopAction (:676) is
    /// the moment a customer between tasks picks what to do (buy, cards, bulk box, table, pay, leave — random rolls). Features
    /// register a <see cref="Register">choice</see>: a weight for this customer now and a start (a mod state, or a vanilla
    /// entry such as CustomerLife.GoPlayTable). At each decision the weights compete with vanilla's own (<see cref="VanillaWeight"/>);
    /// a choice that wins and starts replaces vanilla's roll for that one decision, otherwise vanilla decides as always.
    /// Activities so begin between tasks, never mid-walk, and traits tilt the odds through the weights.
    /// </summary>
    internal static class CustomerChoices
    {
        public const float VanillaWeight = 1f;

        private class Choice
        {
            public string Id;
            public Func<Customer, float> Weight;
            public Func<Customer, bool> Start;
        }

        private static readonly List<Choice> Choices = new List<Choice>();

        /// <summary>
        /// A new choice. <paramref name="weight"/>: how much this customer wants it now (0 = not at all; against vanilla's 1);
        /// keep it cheap. <paramref name="start"/>: begin it (Goto a mod state or a vanilla entry); false = can't now (no
        /// partner…), and vanilla decides. Caps (once a visit, …) belong in the weight.
        /// </summary>
        public static void Register(string id, Func<Customer, float> weight, Func<Customer, bool> start) =>
            Choices.Add(new Choice { Id = id, Weight = weight, Start = start });

        /// <summary>Each choice's weight for this customer now (debug panel).</summary>
        public static IEnumerable<(string id, float weight)> Weights(Customer c)
        {
            foreach (var ch in Choices) yield return (ch.Id, SafeWeight(ch, c));
        }

        private static float SafeWeight(Choice ch, Customer c)
        {
            try { return Mathf.Max(0f, ch.Weight(c)); }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Customers: choice '{ch.Id}' weight failed: {e.Message}");
                return 0f;
            }
        }

        /// <summary>Customers vanilla runs normally (not a tournament entrant, not paid and on their way out, not in a mod state).</summary>
        private static bool Eligible(Customer c) =>
            c.m_IsActive && !CustomerLife.IsTournamentEntrant(c) && !CustomerLife.HasCheckedOut(c) && !CustomerLife.IsPaused(c) &&
            CustomerStates.Of(c) == null && !EndOfDayReportScreen.IsActive() && !_inPathFollower;

        /// <summary>
        /// Vanilla's path follower is deciding (a walk ended, and that led to DetermineShopAction). It reads the path right after, and an activity starting
        /// here would drop it: a NullReferenceException in Customer.Update. Vanilla's own roll this once; they choose again next time.
        /// </summary>
        private static bool _inPathFollower;

        [HarmonyPatch]
        private static class PathFollowerPatch
        {
            private static IEnumerable<System.Reflection.MethodBase> TargetMethods()
            {
                yield return AccessTools.Method(typeof(Customer), "OnReachedPathEnd");
                yield return AccessTools.Method(typeof(Customer), "WaypointEndUpdate");
            }

            private static void Prefix() => _inPathFollower = true;

            private static Exception Finalizer(Exception __exception)
            {
                _inPathFollower = false;
                return __exception;
            }
        }

        [HarmonyPatch(typeof(Customer), "DetermineShopAction")]
        private static class DecidePatch
        {
            [HarmonyPriority(Priority.Low)]
            private static bool Prefix(Customer __instance, bool __runOriginal)
            {
                if (!__runOriginal || Choices.Count == 0 || !Eligible(__instance)) return true;
                Debug.HangWatch.Step("customer chooses what next");
                var weights = new float[Choices.Count];
                float total = VanillaWeight;
                for (int i = 0; i < Choices.Count; i++) total += weights[i] = SafeWeight(Choices[i], __instance);
                float roll = UnityEngine.Random.value * total;
                for (int i = 0; i < Choices.Count; i++)
                {
                    if ((roll -= weights[i]) >= 0f) continue;
                    try { return !Choices[i].Start(__instance); }
                    catch (Exception e)
                    {
                        Plugin.Log.LogError($"Customers: choice '{Choices[i].Id}' failed to start: {e}");
                        return true;
                    }
                }
                return true; // vanilla's own roll
            }
        }
    }
}
