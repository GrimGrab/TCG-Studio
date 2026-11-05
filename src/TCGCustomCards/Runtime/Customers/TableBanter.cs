using System.Collections.Generic;
using HarmonyLib;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] TableBanter: two customers playing each other at a table talk now and then; the other one sometimes answers.
    /// The clock is per player, so the talkative one speaks more. Not at a table the player is playing at: the game has their attention.
    /// </summary>
    internal class TableBanter : IPersonaModule
    {
        private const float MinGap = 25f, MaxGap = 70f; // seconds between someone's lines
        private const float ReplyChance = 0.6f;

        private static readonly AccessTools.FieldRef<Customer, InteractablePlayTable> TableOf =
            Core.Compat.Field<Customer, InteractablePlayTable>(typeof(TableBanter), "m_CurrentPlayTable");

        private class Clock
        {
            public float Next;
        }

        public string Id => "tableBanter";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerTableBanter.Value;

        public void Init()
        {
            CustomerLife.SlowTick.Add(Tick);
        }

        private static void Tick(Customer c)
        {
            if (!On || c.m_CurrentState != ECustomerState.PlayingAtTable) return;
            var table = TableOf(c);
            if (table == null || !table.GetHasStartPlay() || table.GetHasStartPlayerPlayCard()) return;
            var clock = Cast.VisitData<Clock>(c);
            if (clock == null) return;
            float now = Time.time;
            if (clock.Next <= 0f) clock.Next = now + Random.Range(MinGap / 2f, MaxGap);
            if (now < clock.Next) return;
            clock.Next = now + Random.Range(MinGap, MaxGap);
            var p = Cast.Of(c);
            if (p == null || Random.value >= Traits.Effect(p, "talk") * Plugin.CustomerChattiness.Value) return;

            Customer other = null;
            foreach (var seated in table.m_OccupiedCustomer)
                if (seated != null && seated != c) other = seated;
            var op = other != null ? Cast.Of(other) : null;
            Dialogue.Say(c, "tableBanter", new Dialogue.Tokens { Other = op?.name ?? "friend" }, 1f);
            if (op == null) return;
            // The other one doesn't start talking over this.
            var theirs = Cast.VisitData<Clock>(other);
            if (theirs != null) theirs.Next = Mathf.Max(theirs.Next, now + MinGap);
            if (Random.value < ReplyChance) Dialogue.Say(other, "tableReply", new Dialogue.Tokens { Other = p.name }, 1f, Dialogue.Gap);
        }

        public void Load(JToken state) { }
        public JToken Save(ICollection<string> keep) => null;
        public string Summary(Persona p) => null;
    }
}
