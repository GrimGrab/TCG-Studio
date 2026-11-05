using System;
using System.Collections.Generic;
using HarmonyLib;
using UnityEngine;
using static TCGCustomCards.Core.SafeEvents;

namespace TCGCustomCards.Hooks
{
    /// <summary>
    /// Single owner of the patches on the game's employees (Worker): events and per-frame ticks for what they do, and a timed pause. Vanilla always runs.
    /// A pause sets m_IsPausingAction, which the worker's own state machine (Worker.cs:1729) and a till it mans (InteractableCashierCounter.cs:130) both wait
    /// on; the game uses it while the player talks to them, so a pause never ends during that.
    /// </summary>
    internal static class WorkerLife
    {
        /// <summary>Their state changed: (worker, before, now). SetState is too small to patch (Mono inlines it), so this comes from a look each frame.</summary>
        public static event Action<Worker, EWorkerState, EWorkerState> StateChanged;
        /// <summary>A different customer has reached their till: (worker, customer).</summary>
        public static event Action<Worker, Customer> NewCustomer;
        /// <summary>They started a day (not when restored from a save).</summary>
        public static event Action<Worker> Arrived;
        public static event Action<Worker> GoingHome;
        public static event Action<Worker> GotBonus;
        public static event Action<Worker> Fired;
        /// <summary>About once a second per active worker.</summary>
        public static event Action<Worker> SlowTick;
        /// <summary>Their speeds were just worked out: change them here.</summary>
        public static event Action<Worker> StatsSet;

        private class State
        {
            public EWorkerState Last = EWorkerState.None;
            public Customer Serving;
            public bool Met; // the customer has reached the till
            public float PauseUntil, NextSlow;
            public bool Paused;
        }

        private static readonly Dictionary<Worker, State> States = new Dictionary<Worker, State>();
        private static readonly AccessTools.FieldRef<Worker, InteractableCashierCounter> Counter =
            Core.Compat.Field<Worker, InteractableCashierCounter>(typeof(WorkerLife), "m_CurrentCashierCounter");

        private static State Of(Worker w)
        {
            if (!States.TryGetValue(w, out var s)) States[w] = s = new State { NextSlow = Time.time + UnityEngine.Random.value };
            return s;
        }

        /// <summary>The customer at the till they man, or null.</summary>
        public static Customer Serving(Worker w)
        {
            var till = Counter(w);
            return till != null && w.m_CurrentState == EWorkerState.ManningCounter ? till.m_CurrentCustomer : null;
        }

        private static readonly AccessTools.FieldRef<Worker, EWorkerTask> Task = Core.Compat.Field<Worker, EWorkerTask>(typeof(WorkerLife), "m_WorkerTask");

        /// <summary>What they've been told to do.</summary>
        public static EWorkerTask TaskOf(Worker w) => Task(w);

        /// <summary>The till they man, or null.</summary>
        public static InteractableCashierCounter Till(Worker w) => w.m_CurrentState == EWorkerState.ManningCounter ? Counter(w) : null;

        /// <summary>The worker serving this customer, or null.</summary>
        public static Worker ServedBy(Customer c)
        {
            foreach (var kv in States)
                if (kv.Key != null && kv.Key.m_IsActive && kv.Value.Serving == c) return kv.Key;
            return null;
        }

        public static IEnumerable<Worker> Active()
        {
            foreach (var kv in States)
                if (kv.Key != null && kv.Key.m_IsActive) yield return kv.Key;
        }

        public static bool IsPaused(Worker w) => States.TryGetValue(w, out var s) && s.Paused;

        /// <summary>They stop what they're doing for a while (a till they man waits too). False when they're paused already, or the player is talking to them.</summary>
        public static bool Pause(Worker w, float seconds)
        {
            if (w == null || !w.m_IsActive || w.m_IsPausingAction) return false;
            var s = Of(w);
            s.Paused = true;
            s.PauseUntil = Time.time + seconds;
            w.m_IsPausingAction = true;
            return true;
        }

        private static void EndPause(Worker w, State s)
        {
            s.Paused = false;
            w.m_IsPausingAction = false;
        }

        [HarmonyPatch(typeof(Worker), nameof(Worker.Update))]
        private static class TickPatch
        {
            private static void Postfix(Worker __instance)
            {
                if (!__instance.m_IsActive) return;
                var s = Of(__instance);
                float now = Time.time;
                if (s.Paused && now >= s.PauseUntil)
                {
                    // Their screen open: the game clears the flag itself when it closes.
                    var player = Core.GameScene.Existing<InteractionPlayerController>();
                    if (player == null || !player.IsInUIMode()) EndPause(__instance, s);
                }
                var state = __instance.m_CurrentState;
                if (state != s.Last)
                {
                    var before = s.Last;
                    s.Last = state;
                    Raise(StateChanged, __instance, before, state);
                }
                var customer = Serving(__instance);
                if (customer != s.Serving)
                {
                    s.Serving = customer;
                    s.Met = false;
                }
                if (customer != null && !s.Met && CustomerLife.IsAtTill(customer))
                {
                    s.Met = true;
                    Raise(NewCustomer, __instance, customer);
                }
                if (now >= s.NextSlow)
                {
                    s.NextSlow = now + 1f;
                    Raise(SlowTick, __instance);
                }
            }
        }

        [HarmonyPatch(typeof(Worker), nameof(Worker.ActivateWorker))]
        private static class ActivatePatch
        {
            private static void Postfix(Worker __instance, bool resetTask)
            {
                var s = Of(__instance);
                s.Paused = false;
                s.Serving = null;
                s.Met = false;
                s.Last = EWorkerState.None;
                if (!resetTask) Raise(Arrived, __instance);
            }
        }

        [HarmonyPatch(typeof(Worker), nameof(Worker.DeactivateWorker))]
        private static class DeactivatePatch
        {
            private static void Postfix(Worker __instance)
            {
                if (States.TryGetValue(__instance, out var s) && s.Paused) EndPause(__instance, s);
            }
        }

        [HarmonyPatch(typeof(Worker), nameof(Worker.OnDayEnded))]
        private static class GoHomePatch
        {
            private static void Postfix(Worker __instance) => Raise(GoingHome, __instance);
        }

        [HarmonyPatch(typeof(Worker), nameof(Worker.GiveSalaryBonus))]
        private static class BonusPatch
        {
            private static void Postfix(Worker __instance) => Raise(GotBonus, __instance);
        }

        [HarmonyPatch(typeof(Worker), nameof(Worker.FireWorker))]
        private static class FirePatch
        {
            private static void Prefix(Worker __instance) => Raise(Fired, __instance);
        }

        [HarmonyPatch]
        private static class StatsPatch
        {
            private static IEnumerable<System.Reflection.MethodBase> TargetMethods()
            {
                yield return AccessTools.Method(typeof(Worker), "EvaluateWorkerAttribute");
                yield return AccessTools.Method(typeof(Worker), "EvaluateTaskLevelStat");
            }

            private static void Postfix(Worker __instance) => Raise(StatsSet, __instance);
        }

        // A load rebuilds the scene: its workers are gone.
        static WorkerLife()
        {
            UnityEngine.SceneManagement.SceneManager.sceneLoaded += (scene, mode) => States.Clear();
        }
    }
}
