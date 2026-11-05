using System.Collections.Generic;
using UnityEngine;

namespace TCGCustomCards.Hooks
{
    /// <summary>
    /// Keeps till lines honest: someone who left without the till knowing holds the first spot for ever, and everyone queues behind nobody
    /// and can't be served. Once a second each till's line is checked: anyone in it must be an active customer waiting to pay, once, and
    /// there must be a spot per person. What stays wrong for <see cref="GraceSeconds"/> (a paid customer stays listed for up to a second, by
    /// the game's own timer) is logged with what the customer was doing, and put right.
    /// </summary>
    internal static class TillGuard
    {
        private const float GraceSeconds = 3f;

        private static readonly Dictionary<Customer, float> WrongSince = new Dictionary<Customer, float>();
        private static readonly List<Customer> Seen = new List<Customer>();
        private static float _next;

        public static void Init() => CustomerLife.SlowTick.Add(_ => Tick());

        private static void Tick()
        {
            float now = Time.time;
            if (now < _next) return;
            _next = now + 1f;
            var tills = Core.GameScene.Existing<ShelfManager>()?.m_CashierCounterList;
            if (tills == null) return;
            foreach (var till in tills)
                if (till != null) Check(till, now);
            Forget(now);
        }

        private static void Check(InteractableCashierCounter till, float now)
        {
            var line = till.m_CustomerListInQueue;
            Seen.Clear();
            for (int i = line.Count - 1; i >= 0; i--)
            {
                var c = line[i];
                string why = Problem(c, line, i);
                if (why == null) continue;
                Seen.Add(c);
                if (c == null) continue;
                if (!WrongSince.TryGetValue(c, out float since)) WrongSince[c] = since = now;
                if (now - since < GraceSeconds) continue;
                Plugin.Log.LogWarning($"Till guard: removed {c.name} from place {i + 1} of a till line of {line.Count}: {why} " +
                                      $"(game state {c.m_CurrentState}, mod state {CustomerStates.Of(c)?.State.Id ?? "none"}, paid {CustomerLife.HasCheckedOut(c)})");
                WrongSince.Remove(c);
                CustomerLife.RemoveFromLine(till, i);
                return; // the rest next time: places have moved
            }
            if (line.RemoveAll(c => c == null) > 0) Plugin.Log.LogWarning("Till guard: removed a destroyed customer from a till line");

            var spots = till.m_ValidQueuePosList;
            if (Seen.Count == 0 && spots.Count > line.Count)
            {
                Plugin.Log.LogWarning($"Till guard: a till line had {spots.Count} spots for {line.Count} customer(s)");
                till.RemoveLastCustomerFromQueue();
            }

            var serving = till.m_CurrentCustomer;
            if (serving != null && !serving.m_IsActive)
            {
                Plugin.Log.LogWarning($"Till guard: a till was still serving {serving.name}, who has left");
                till.UpdateCashierCounterState(ECashierCounterState.Idle);
                till.UpdateCurrentCustomer(null);
            }
        }

        private static string Problem(Customer c, List<Customer> line, int place)
        {
            if (c == null) return "destroyed";
            if (!c.m_IsActive) return "no longer in the shop";
            if (line.IndexOf(c) != place) return "listed twice";
            if (CustomerStates.Of(c) != null) return "in a mod state";
            if (c.m_CurrentState != ECustomerState.QueuingToPay && c.m_CurrentState != ECustomerState.ReadyToPay) return "not waiting to pay";
            return null;
        }

        private static void Forget(float now)
        {
            if (WrongSince.Count == 0) return;
            var gone = new List<Customer>();
            foreach (var kv in WrongSince)
                if (kv.Key == null || !Seen.Contains(kv.Key) && now - kv.Value > GraceSeconds * 2f) gone.Add(kv.Key);
            foreach (var c in gone) WrongSince.Remove(c);
        }
    }
}
