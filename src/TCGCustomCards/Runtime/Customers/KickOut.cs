using System.Collections.Generic;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers] KickOutKey: look at a customer and send them out (the leave state: they walk out, vanilla can't turn them
    /// around). Unpaid things are left behind (CustomerLife.Abandon); only someone you already started scanning pays first
    /// and leaves right after. A kicked customer is upset (mood −40);
    /// people nearby disapprove, unless the kicked one was stinky, then they cheer. Kicked 3 times = banned (never comes back).
    /// Saved: kick count per persona.
    /// </summary>
    internal class KickOut : IPersonaModule
    {
        public class Row
        {
            public int k; // times kicked out
        }

        public const int BanAfter = 3;

        public string Id => "kickOut";
        public bool Enabled => Cast.Enabled;

        private static readonly PersonaTable<Row> Table = new PersonaTable<Row>();
        /// <summary>Per visit: kicked out (no pack opening on the way), and still has to pay before leaving.</summary>
        private class Kicked
        {
            public bool LeaveAfterPaying;
        }

        public static int Kicks(Persona p) => Table.Get(p, create: false)?.k ?? 0;

        /// <summary>You kicked a customer out: (customer, persona).</summary>
        public static event System.Action<Customer, Persona> KickedOut;

        public static bool WasKicked(Customer c) => Cast.PeekVisitData<Kicked>(c) != null;

        /// <summary>Why this customer can't be kicked out right now, or null.</summary>
        public static string Refusal(Customer c)
        {
            if (CustomerLife.IsTournamentEntrant(c)) return "a tournament player";
            switch (c.m_CurrentState)
            {
                case ECustomerState.ExitingShop: return "already leaving";
                // A table seat or queue spot is booked in these: leaving would strand it.
                case ECustomerState.WantToPlayGame:
                case ECustomerState.WalkToPlayTable:
                case ECustomerState.QueueingPlayTable:
                case ECustomerState.PlayingAtTable:
                case ECustomerState.EndingPlayTableGame: return "at a play table";
                case ECustomerState.WantToTradeCard:
                case ECustomerState.WaitingToTradeCard: return "waiting to trade";
                case ECustomerState.TakingItemFromShelf:
                case ECustomerState.TakingItemFromCardShelf:
                case ECustomerState.TakingItemFromBulkDonationBox: return "grabbing something";
            }
            if (CustomerStates.IsIn(c, CustomerStates.LeaveId)) return "already leaving";
            if (Cast.PeekVisitData<Kicked>(c)?.LeaveAfterPaying == true) return "already on their way to pay";
            return null;
        }

        /// <summary>
        /// Kicks the customer out: their plans are dropped, they leave their unpaid things behind (<see cref="CustomerLife.Abandon"/>:
        /// back on their shelves, the rest in a box where they stand, cards to your collection; at the till they take them off
        /// the counter first) and leave, out of the till queue too. Only once you started scanning them up do they pay first
        /// and leave right after. Returns what happened, for the on-screen message; null = not kicked (see <see cref="Refusal"/>).
        /// </summary>
        /// <param name="onTheRecord">False: an employee saw them out. No kick on their record towards a ban, and no loyalty lost.</param>
        public static string Kick(Customer c, bool onTheRecord = true)
        {
            Debug.HangWatch.Step("kick out");
            var p = Cast.Of(c);
            if (p == null || Refusal(c) != null) return null;
            bool stinky = CustomerLife.IsStinky(c);
            if (onTheRecord) Table.Get(p).k++;
            var kicked = Cast.VisitData<Kicked>(c);
            if (onTheRecord) Loyalty.Change(p, Loyalty.KickedOut, "kicked out");
            Dialogue.Say(c, "kicked", null, 1f);
            foreach (var v in Cast.Visitors())
            {
                if (v.Key == c || Vector3.Distance(v.Key.transform.position, c.transform.position) > 8f) continue;
                Dialogue.Say(v.Key, stinky ? "cheerKick" : "booKick", new Dialogue.Tokens { Other = p.name }, 0.5f, Random.Range(0.8f, 2f));
            }
            CustomerLife.DropPlans(c);
            bool unpaid = !CustomerLife.HasCheckedOut(c) && (c.m_ItemInBagList.Count > 0 || c.m_CardInBagList.Count > 0);
            string how;
            bool ringingUp = c.m_CurrentState == ECustomerState.ReadyToPay && !CustomerLife.IsWaitingAtTill(c);
            if (unpaid && ringingUp)
            {
                // Already being scanned (or a worker runs the till): they pay, then leave.
                kicked.LeaveAfterPaying = true;
                how = "pays first (you already started ringing them up), then leaves";
            }
            else
            {
                int left = 0;
                if (unpaid)
                {
                    CustomerLife.LeaveTill(c); // nothing when they aren't at the till
                    left = CustomerLife.Abandon(c);
                }
                CustomerLife.LeaveQueue(c);
                CustomerStates.Leave(c);
                how = !unpaid ? "left" : left == 0 ? "put everything back and left" : "dropped their things and left";
            }
            Plugin.Log.LogInfo($"Customers: {(onTheRecord ? "you kicked" : "an employee threw")} {p.name} out ({Kicks(p)} time(s){(stinky ? ", stinky" : "")}): {how}");
            News.Add("kick", p.name);
            KickedOut?.Invoke(c, p);
            return how;
        }

        public void Init()
        {
            UI.CustomerDebugPanel.Section("Kick out", (c, p, rows) =>
            {
                var k = Cast.PeekVisitData<Kicked>(c);
                rows.Add(("now", k == null ? Refusal(c) is string why ? $"can't: {why}" : "can be kicked" : k.LeaveAfterPaying ? "kicked, pays first" : "kicked, leaving"));
                if (Kicks(p) > 0) rows.Add(("record", $"kicked {Kicks(p)}× (banned at {BanAfter})"));
            });
            // OnPayingDone picks the next thing to do (DetermineShopAction); a kicked customer leaves instead.
            CustomerLife.CheckedOut += c =>
            {
                var kicked = Cast.PeekVisitData<Kicked>(c);
                if (kicked == null || !kicked.LeaveAfterPaying) return;
                kicked.LeaveAfterPaying = false;
                CustomerStates.Leave(c);
            };
            Cast.VisitWeight.Add((p, w) => Kicks(p) >= BanAfter ? 0f : w);
        }

        public void Load(JToken state)
        {
            Table.Load(state);
        }

        public JToken Save(ICollection<string> keep) => Table.Save(keep);

        public string Summary(Persona p)
        {
            int k = Kicks(p);
            return k == 0 ? null : k >= BanAfter ? "BANNED (kicked out 3 times)" : $"kicked out {k} time(s)";
        }
    }
}
