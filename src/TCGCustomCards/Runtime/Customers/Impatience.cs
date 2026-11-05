using System.Collections.Generic;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] Impatience: every customer gives up after PatienceSeconds × their traits' "patience" (Impatient
    /// ×0.25) waiting at the till: in the queue, at the counter before anything is scanned (the wait counts from joining the
    /// queue), or to trade cards with the player (not while the player is in a trade screen). A new kind of wait starts the clock again. Halfway they grumble; at the end they leave the queue or the till and the shop, leaving their things behind
    /// (<see cref="CustomerLife.Abandon"/>: back on their shelves, else a box on the floor / your collection). Saved: times
    /// they gave up, per persona.
    /// </summary>
    internal class Impatience : IPersonaModule
    {
        public class Row
        {
            public int g; // times they gave up waiting
        }

        public string Id => "impatience";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerImpatience.Value;

        private static readonly PersonaTable<Row> Table = new PersonaTable<Row>();

        /// <summary>Per visit: the wait at the till under way (Active), since when, grumbled yet.</summary>
        private class Wait
        {
            public bool Active;
            public float Since;
            public bool Grumbled;
            public bool Trading; // waiting to trade, not to pay
            public float LastTick;
        }

        private static void StopWaiting(Customer c)
        {
            var w = Cast.PeekVisitData<Wait>(c);
            if (w != null) w.Active = false;
        }

        private static readonly HarmonyLib.AccessTools.FieldRef<Customer, float> VanillaTimer =
            Core.Compat.Field<Customer, float>(typeof(Impatience), "m_Timer");
        private static readonly HarmonyLib.AccessTools.FieldRef<Customer, bool> TradedAlready =
            Core.Compat.Field<Customer, bool>(typeof(Impatience), "m_HasTradedCard");

        public void Init()
        {
            UI.CustomerDebugPanel.Section("Patience", DebugRows);
            // On each customer's own update, a few times a second by the clock (patience is counted in seconds).
            CustomerLife.SlowTick.Add(c =>
            {
                if (!On) { StopWaiting(c); return; }
                var p = Cast.Of(c);
                if (p != null) Check(c, p);
            });
        }

        private static void Check(Customer c, Persona p)
        {
            float patience = Patience(p);
            bool atTill = CustomerLife.IsWaitingAtTill(c), trading = c.m_CurrentState == ECustomerState.WaitingToTradeCard;
            if (patience <= 0f || !(atTill || trading || CustomerLife.IsWaitingInQueue(c)))
            {
                StopWaiting(c);
                return;
            }
            var w = Cast.VisitData<Wait>(c);
            if (w == null) return;
            float now = Time.time;
            string where = trading ? "to trade" : atTill ? "at the till" : "in the queue";
            if (!w.Active || w.Trading != trading)
            {
                w.Active = true;
                w.Trading = trading;
                w.Since = now;
                w.LastTick = now;
                w.Grumbled = false;
                if (Plugin.CustomerDebug.Value) Plugin.Log.LogInfo($"Customers: {p.name} is waiting {where}, patience {patience:0}s");
            }
            if (trading)
            {
                // Vanilla sends them back to shopping after 60 s without a word (not counting a player in a trade screen): this clock decides instead.
                if (CSingleton<CustomerManager>.Instance.m_IsPlayerTrading) w.Since += now - w.LastTick;
                VanillaTimer(c) = 0f;
            }
            w.LastTick = now;
            float waited = now - w.Since;
            if (!w.Grumbled && waited >= patience / 2f)
            {
                w.Grumbled = true;
                Dialogue.Say(c, trading ? "tradeTooSlow" : "tooSlow", null, 0.9f);
            }
            if (waited < patience) return;
            w.Active = false;
            if (trading)
            {
                // As vanilla's own give-up; ExitShop (through Leave) frees their place at the till.
                c.m_ExclaimationMesh.SetActive(false);
                c.m_InteractCollider.SetActive(false);
                TradedAlready(c) = true;
            }
            else if (atTill) CustomerLife.LeaveTill(c);
            else CustomerLife.LeaveQueue(c);
            int left = CustomerLife.Abandon(c);
            CustomerLife.DropPlans(c);
            CustomerStates.Leave(c);
            Table.Get(p).g++;
            Loyalty.Change(p, Loyalty.GaveUpWaiting, "waited too long at the till");
            Dialogue.Say(c, trading ? "tradeGaveUp" : "gaveUp", null, 1f);
            Plugin.Log.LogInfo($"Customers: {p.name} gave up waiting {where} after {waited:0}s and left" +
                (left > 0 ? $"; {left} thing(s) didn't fit back on their shelf (items boxed on the floor, cards to your collection)" : ""));
        }

        /// <summary>Seconds this customer waits at the till: the PatienceSeconds setting × their traits (Impatient ×0.25).</summary>
        private static float Patience(Persona p) => Plugin.CustomerPatienceSeconds.Value * Traits.Effect(p, "patience");

        /// <summary>How much of their patience is left, 0 to 1 (it only runs out while they wait at the till or to trade); null when nobody leaves for waiting.</summary>
        public static float? PatienceLeft(Customer c, Persona p)
        {
            if (!On || c == null || p == null) return null;
            var w = Cast.PeekVisitData<Wait>(c);
            return w == null || !w.Active ? 1f : Mathf.Clamp01(1f - (Time.time - w.Since) / Patience(p));
        }

        private static void DebugRows(Customer c, Persona p, List<(string, string)> rows)
        {
            if (!On) { rows.Add(("impatience", "off: nobody leaves for waiting")); return; }
            float patience = Patience(p);
            rows.Add(("patience", $"{patience:0} s ({Plugin.CustomerPatienceSeconds.Value:0} s × traits {Traits.Effect(p, "patience"):0.##})"));
            var w = Cast.PeekVisitData<Wait>(c);
            if (w == null || !w.Active) { rows.Add(("waiting", "no")); return; }
            if (w.Trading) rows.Add(("waiting", CSingleton<CustomerManager>.Instance.m_IsPlayerTrading ? "to trade (paused: you're trading)" : "to trade"));
            rows.Add(("waited", UI.CustomerDebugPanel.Bar(Time.time - w.Since, patience)));
            rows.Add(("grumbled", w.Grumbled ? "yes" : $"at {patience / 2f:0} s"));
        }

        public void Load(JToken state)
        {
            Table.Load(state);
        }

        public JToken Save(ICollection<string> keep) => Table.Save(keep);

        public string Summary(Persona p)
        {
            int g = Table.Get(p, create: false)?.g ?? 0;
            return g == 0 ? null : $"gave up waiting at the till {g}x";
        }
    }
}
