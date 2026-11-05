using System;
using System.Collections;
using System.Collections.Generic;
using HarmonyLib;
using UnityEngine;

namespace TCGCustomCards.Hooks
{
    /// <summary>
    /// A state of our own in a customer's state machine. Register it once (<see cref="CustomerStates.Register"/>); move
    /// customers with <see cref="CustomerStates.Goto"/>. While a customer is in it, vanilla makes no decisions for them
    /// (Customer.Update only acts on its own states) but still walks them along the path a state starts with
    /// <see cref="StateContext.WalkTo"/>.
    /// </summary>
    internal abstract class ModState
    {
        public abstract string Id { get; }

        /// <summary>The customer entered the state. <paramref name="ctx"/> is fresh per entry.</summary>
        public virtual void Enter(Customer c, StateContext ctx) { }

        /// <summary>Every frame while in the state (after vanilla's Update).</summary>
        public virtual void Tick(Customer c, StateContext ctx) { }

        /// <summary>
        /// The walk started by <see cref="StateContext.WalkTo"/> ended (arrived, or no path). Runs inside vanilla's path
        /// follower, which reads the path right after: set a flag here and stand / hand back / move on in <see cref="Tick"/>
        /// (Goto another state and DeactivateCustomer are fine; ResetPath and ToVanilla are not).
        /// </summary>
        public virtual void Arrived(Customer c, StateContext ctx) { }

        /// <summary>The customer leaves the state (another state, back to vanilla, visit over). Undo what it showed.</summary>
        public virtual void Exit(Customer c, StateContext ctx) { }
    }

    /// <summary>Per customer, per entry into a state: its data, timer and helpers.</summary>
    internal class StateContext
    {
        public readonly Customer Customer;
        public readonly ModState State;
        public readonly object Data;
        public readonly float Since = Time.time;
        internal Coroutine Routine;
        internal Vector3? Spot;     // reserved by CustomerStates.FreeSpot
        internal Transform Target;  // the walk target handed to vanilla's path follower

        /// <summary>The last <see cref="WalkTo"/> ended (set before <see cref="ModState.Arrived"/>); act on it in Tick.</summary>
        public bool HasArrived { get; internal set; }

        internal StateContext(Customer c, ModState state, object data)
        {
            Customer = c;
            State = state;
            Data = data;
        }

        public float Elapsed => Time.time - Since;

        /// <summary>Walks to <paramref name="spot"/> with vanilla's own pathing and movement; <see cref="ModState.Arrived"/> follows.</summary>
        public void WalkTo(Vector3 spot)
        {
            HasArrived = false;
            if (Target == null) Target = new GameObject("TCGCC_CustomerTarget").transform;
            Target.position = spot;
            CustomerLife.WalkTo(Customer, Target);
        }

        /// <summary>
        /// Runs a coroutine for this state; it stops when the customer leaves the state. An exception in it (or in a coroutine
        /// it yields) is logged and the customer walks out, instead of standing in the state for ever.
        /// </summary>
        public void Run(IEnumerator routine)
        {
            HooksRunner.Stop(Routine);
            Routine = HooksRunner.Run(Guarded(routine));
        }

        private IEnumerator Guarded(IEnumerator routine)
        {
            var stack = new Stack<IEnumerator>();
            stack.Push(routine);
            while (stack.Count > 0)
            {
                object step;
                try
                {
                    if (!stack.Peek().MoveNext()) { stack.Pop(); continue; }
                    step = stack.Peek().Current;
                }
                catch (Exception e)
                {
                    Plugin.Log.LogError($"CustomerStates: {State?.Id} coroutine failed, the customer leaves: {e}");
                    if (Customer != null && State?.Id != CustomerStates.LeaveId) CustomerStates.Goto(Customer, CustomerStates.LeaveId);
                    yield break;
                }
                if (step is IEnumerator nested) stack.Push(nested);
                else yield return step;
            }
        }

        internal void Dispose()
        {
            HooksRunner.Stop(Routine);
            Routine = null;
            if (Target != null) UnityEngine.Object.Destroy(Target.gameObject);
            Target = null;
        }
    }

    /// <summary>
    /// Single owner of our states in the customer state machine (docs/hooks.md "CustomerStates"). Every ModState gets an
    /// ECustomerState value from 1000 up: vanilla's Update skips all its decisions for such a value but keeps its path
    /// follower, its save writes any unknown state as Idle (GetCustomerSaveData :4077) and day end deactivates everyone
    /// (CustomerManager :348). Back to vanilla only through <see cref="ToVanilla"/>, which calls one of vanilla's own entry
    /// methods with the path reset (an old path's end would otherwise fire OnReachedPathEnd in the new state).
    /// Built in: <see cref="Leave"/> (walk out, end of visit).
    /// </summary>
    internal static class CustomerStates
    {
        /// <summary>The built-in state that walks a customer out and ends the visit (see <see cref="Leave"/>).</summary>
        public const string LeaveId = "leave";

        private const int FirstValue = 1000;
        private static readonly List<ModState> States = new List<ModState>();
        private static readonly Dictionary<string, ModState> ById = new Dictionary<string, ModState>();
        private static readonly Dictionary<Customer, StateContext> Current = new Dictionary<Customer, StateContext>();

        static CustomerStates()
        {
            Register(new LeaveState());
            CustomerLife.Activated += Drop;
            CustomerLife.Leaving += Drop;
            CustomerLife.Tick.Add(c =>
            {
                if (Current.Count > 0 && Current.TryGetValue(c, out var ctx)) Safe(ctx.State, "tick", () => ctx.State.Tick(c, ctx));
            });
            // Loading a save rebuilds the shop scene: its customers are gone, so are their states (routines stop, no Exit).
            UnityEngine.SceneManagement.SceneManager.sceneLoaded += (scene, mode) =>
            {
                foreach (var ctx in Current.Values) ctx.Dispose();
                Current.Clear();
            };
        }

        public static void Register(ModState state)
        {
            if (ById.ContainsKey(state.Id)) return;
            ById[state.Id] = state;
            States.Add(state);
        }

        /// <summary>The customer's mod state, or null when vanilla runs them.</summary>
        public static StateContext Of(Customer c) => c != null && Current.TryGetValue(c, out var ctx) ? ctx : null;

        public static bool IsIn(Customer c, string id) => Of(c)?.State.Id == id;

        /// <summary>Moves the customer into a mod state (leaving their current one, mod or vanilla). False = unknown state.</summary>
        public static bool Goto(Customer c, string id, object data = null)
        {
            if (c == null || !ById.TryGetValue(id, out var state)) return false;
            ExitCurrent(c);
            var ctx = new StateContext(c, state, data);
            Current[c] = ctx;
            SetValue(c, (ECustomerState)(FirstValue + States.IndexOf(state)));
            Safe(state, "enter", () => state.Enter(c, ctx));
            return true;
        }

        /// <summary>
        /// Hands the customer back to vanilla through one of its entry methods (<see cref="CustomerLife.SendOut"/>,
        /// <see cref="CustomerLife.SendToPay"/>, <see cref="CustomerLife.Shop"/>…); the path is reset first.
        /// </summary>
        public static void ToVanilla(Customer c, Action<Customer> entry)
        {
            ExitCurrent(c);
            SetValue(c, ECustomerState.Idle);
            CustomerLife.ResetPath(c);
            entry(c);
        }

        /// <summary>
        /// Sends the customer out for good: vanilla's ExitShop once for its bookkeeping (review, customer count, sales tally;
        /// it runs only once per visit), then our <c>leave</c> state walks them to an exit and ends the visit. Unpaid items
        /// must be dealt with first (ExitShop sends anyone with unpaid items to pay). False = they can't leave now.
        /// </summary>
        public static bool Leave(Customer c)
        {
            if (c == null) return false;
            if (IsIn(c, LeaveState.StateId)) return true;
            ExitCurrent(c);
            SetValue(c, ECustomerState.Idle);
            CustomerLife.ResetPath(c);
            if (!CustomerLife.SendOut(c)) return false;
            return Goto(c, LeaveState.StateId);
        }

        private static readonly AccessTools.FieldRef<Customer, ECustomerState> StateField =
            AccessTools.FieldRefAccess<Customer, ECustomerState>("m_CurrentState");

        private static void SetValue(Customer c, ECustomerState value) => StateField(c) = value;

        private static void ExitCurrent(Customer c)
        {
            if (!Current.TryGetValue(c, out var ctx)) return;
            Current.Remove(c);
            Safe(ctx.State, "exit", () => ctx.State.Exit(c, ctx));
            ctx.Dispose();
        }

        /// <summary>Visit over (or a new visitor in the pooled object): the state ends, vanilla resets the customer itself.</summary>
        private static void Drop(Customer c)
        {
            if (c is null) return;
            ExitCurrent(c);
        }

        private static void Safe(ModState state, string what, Action action)
        {
            Debug.HangWatch.Step(state.Id);
            try { action(); }
            catch (Exception e) { Plugin.Log.LogError($"Customers: state '{state.Id}' {what} failed: {e}"); }
        }

        // ---- Helpers for states

        private const float SpotSpacing = 1.5f; // metres between two reserved spots

        /// <summary>
        /// A free place for the customer's current state from vanilla's in-shop wander points (CustomerManager
        /// .m_ShopLocationPointList, used when a customer can't find what they want): at least <paramref name="tillClearance"/>
        /// metres from every till and away from spots other states hold. Reserved until the state ends. Null = none free.
        /// </summary>
        public static Vector3? FreeSpot(Customer c, float tillClearance)
        {
            var ctx = Of(c);
            var points = CSingleton<CustomerManager>.Instance?.m_ShopLocationPointList;
            if (ctx == null || points == null) return null;
            var tills = CSingleton<ShelfManager>.Instance?.m_CashierCounterList;
            var place = RandomPlace(c, points, tills, tillClearance);
            if (!place.HasValue)
            {
                // Fall back on the game's own points.
                var free = new List<Vector3>();
                foreach (var t in points)
                    if (t != null && !NearTill(t.position, tills, tillClearance) && !Reserved(t.position)) free.Add(t.position);
                if (free.Count == 0) return null;
                place = free[UnityEngine.Random.Range(0, free.Count)];
            }
            ctx.Spot = place;
            if (Plugin.CustomerDebug.Value) Plugin.Log.LogInfo($"Customer spot: {Cast_Name(c)} goes to ({place.Value.x:0.0}, {place.Value.z:0.0})");
            return ctx.Spot;
        }

        private static string Cast_Name(Customer c) => Runtime.Customers.Cast.Of(c)?.name ?? "someone";

        /// <summary>
        /// A random walkable place within the area the game's location points cover, clear of tills and other held spots and reachable from
        /// where the customer stands; null when several tries find none.
        /// </summary>
        private static Vector3? RandomPlace(Customer c, List<Transform> points, List<InteractableCashierCounter> tills, float tillClearance)
        {
            var graph = AstarPath.active;
            if (graph == null) return null;
            var area = new Bounds();
            bool any = false;
            foreach (var t in points)
            {
                if (t == null) continue;
                if (any) area.Encapsulate(t.position);
                else { area = new Bounds(t.position, Vector3.zero); any = true; }
            }
            if (!any) return null;
            var from = graph.GetNearest(c.transform.position, Pathfinding.NNConstraint.Default).node;
            for (int i = 0; i < 16; i++)
            {
                var wish = new Vector3(UnityEngine.Random.Range(area.min.x, area.max.x), area.center.y, UnityEngine.Random.Range(area.min.z, area.max.z));
                var near = graph.GetNearest(wish, Pathfinding.NNConstraint.Default);
                if (near.node == null || Vector3.Distance(near.position, wish) > 1.5f) continue; // inside a wall or shelf
                Vector3 spot = near.position;
                if (NearTill(spot, tills, tillClearance) || Reserved(spot)) continue;
                if (from != null && !Pathfinding.PathUtilities.IsPathPossible(from, near.node)) continue;
                return spot;
            }
            return null;
        }

        private static bool NearTill(Vector3 pos, List<InteractableCashierCounter> tills, float distance)
        {
            if (tills == null) return false;
            foreach (var till in tills)
                if (till != null && till.gameObject.activeInHierarchy && Vector3.Distance(pos, till.transform.position) < distance) return true;
            return false;
        }

        private static bool Reserved(Vector3 pos)
        {
            foreach (var ctx in Current.Values)
                if (ctx.Spot.HasValue && Vector3.Distance(pos, ctx.Spot.Value) < SpotSpacing) return true;
            return false;
        }

        // ---- Vanilla hook: path end for customers in our states (their Tick comes from CustomerLife.Tick)

        [HarmonyPatch(typeof(Customer), "OnReachedPathEnd")]
        private static class ArrivedPatch
        {
            [HarmonyPriority(Priority.First)]
            private static bool Prefix(Customer __instance)
            {
                if (!Current.TryGetValue(__instance, out var ctx)) return true;
                ctx.HasArrived = true;
                Safe(ctx.State, "arrived", () => ctx.State.Arrived(__instance, ctx));
                return false;
            }
        }

        /// <summary>
        /// While a customer is in our state, only a path to the state's own walk target is taken. A path vanilla asked for just
        /// before (to a shelf…) still arrives after we stopped them, and vanilla's OnPathComplete (:3071) would walk them along
        /// it while the state keeps facing them elsewhere — a chat listener wandering off backwards (2026-10-26).
        /// </summary>
        [HarmonyPatch(typeof(Customer), nameof(Customer.OnPathComplete))]
        private static class PathPatch
        {
            [HarmonyPriority(Priority.First)]
            private static bool Prefix(Customer __instance, Pathfinding.Path p)
            {
                if (Current.Count == 0 || !Current.TryGetValue(__instance, out var ctx)) return true;
                // A failed path ends the walk (OnPathComplete → OnReachedPathEnd → the state's Arrived): let it through.
                if (p == null || p.error) return true;
                var target = ctx.Target;
                var points = p?.vectorPath;
                if (target != null && points != null && points.Count > 0 && Vector3.Distance(points[points.Count - 1], target.position) < 1.5f)
                    return true; // ours
                return false; // stray: dropped (the path pool reclaims an unclaimed path)
            }
        }

        // ---- Built-in: leave

        /// <summary>Walks to a random exit point and ends the visit there (or after a timeout, if the way is blocked).</summary>
        private class LeaveState : ModState
        {
            public const string StateId = LeaveId;
            private const float Timeout = 40f;

            public override string Id => StateId;

            public override void Enter(Customer c, StateContext ctx)
            {
                var exit = CustomerManager.GetRandomExitPoint();
                if (exit == null) c.DeactivateCustomer();
                else ctx.WalkTo(exit.position);
            }

            public override void Tick(Customer c, StateContext ctx)
            {
                if (ctx.Elapsed > Timeout) c.DeactivateCustomer();
            }

            public override void Arrived(Customer c, StateContext ctx) => c.DeactivateCustomer();
        }
    }
}
