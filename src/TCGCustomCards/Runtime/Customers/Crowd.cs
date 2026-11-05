using System.Collections.Generic;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// The shop reacting to a moment (now: a pack hit). Size = price / PackHitPrice: everyone in range turns to look, says
    /// something more often the bigger it is (always for a huge hit), and on big hits a few free customers walk over to
    /// gawk (mod state <see cref="GawkState"/>) before going back to shopping. Queued or busy customers only turn and talk.
    /// No saved state.
    /// </summary>
    internal class Crowd : IPersonaModule
    {
        public string Id => "crowd";
        public bool Enabled => Cast.Enabled;

        public void Init() => CustomerStates.Register(new GawkState());

        /// <summary>
        /// Own reactions that replace the crowd's for a watcher: (watcher, their persona, who pulled, their persona, card,
        /// price) → true = handled (no congratulations, no gawking). Rivalries: jealousy instead.
        /// </summary>
        public static readonly List<System.Func<Customer, Persona, Customer, Persona, CardData, float, bool>> Overrides =
            new List<System.Func<Customer, Persona, Customer, Persona, CardData, float, bool>>();

        private static bool Overridden(Customer watcher, Persona wp, Customer who, Persona p, CardData card, float price)
        {
            foreach (var f in Overrides)
            {
                try { if (f(watcher, wp, who, p, card, price)) return true; }
                catch (System.Exception e) { Plugin.Log.LogError($"Customers: crowd override failed: {e.Message}"); }
            }
            return false;
        }

        // In a game of their own, or on their way out: they don't turn back for someone else's pull.
        private static bool Busy(Customer c) =>
            c.m_CurrentState == ECustomerState.PlayingAtTable || c.m_CurrentState == ECustomerState.EndingPlayTableGame ||
            c.m_CurrentState == ECustomerState.ExitingShop || CustomerStates.IsIn(c, CustomerStates.LeaveId);

        /// <summary>Reactions to <paramref name="who"/> pulling <paramref name="card"/> worth <paramref name="price"/>.</summary>
        public static void React(Customer who, Persona p, CardData card, float price)
        {
            if (!Cast.Enabled || who == null) return;
            float size = price / Mathf.Max(0.01f, Plugin.CustomerPackHitPrice.Value);
            bool huge = size >= 10f, big = size >= 3f;
            float scale = huge ? 2f : big ? 1.5f : 1f; // word travels further the better the pull
            float radius = Plugin.CustomerCrowdRange.Value * scale;
            int reactors = Mathf.CeilToInt(Plugin.CustomerCrowdReactors.Value * scale);
            int gawkers = huge ? 6 : big ? 4 : 2;
            var tokens = new Dialogue.Tokens { Card = CardInfo.Name(card), Other = p.name, Price = price };
            string situation = huge ? "nearbyHuge" : "nearbyHit";
            var near = new List<KeyValuePair<Customer, Persona>>();
            foreach (var v in Cast.Visitors())
                if (v.Key != who && !Busy(v.Key) && Vector3.Distance(v.Key.transform.position, who.transform.position) <= radius) near.Add(v);
            // Someone with hype ("crowd") there, the pull's owner included, brings more people over.
            float crowd = Traits.Effect(p, "crowd");
            foreach (var v in near) crowd = Mathf.Max(crowd, Traits.Effect(v.Value, "crowd"));
            reactors += Mathf.RoundToInt(crowd);
            gawkers += Mathf.RoundToInt(crowd);
            // The closest walk over first.
            near.Sort((a, b) => Vector3.Distance(a.Key.transform.position, who.transform.position)
                .CompareTo(Vector3.Distance(b.Key.transform.position, who.transform.position)));
            foreach (var v in near)
            {
                if (Overridden(v.Key, v.Value, who, p, card, price)) // friends and rivals have their own reason: not one of the places
                {
                    CustomerLife.Glance(v.Key, who.transform.position, Random.Range(2.5f, 4.5f));
                    continue;
                }
                if (reactors <= 0) continue;
                // Someone walking past either comes over to look or carries on: no reacting on the move.
                bool canCome = gawkers > 0 && CustomerLife.IsFree(v.Key);
                if (!canCome && v.Key.m_Anim != null && v.Key.m_Anim.GetFloat("MoveSpeed") > 0.05f) continue;
                reactors--;
                CustomerLife.Glance(v.Key, who.transform.position, Random.Range(2.5f, 4.5f));
                if (canCome)
                {
                    gawkers--;
                    CustomerStates.Goto(v.Key, GawkState.StateId, new Gawk { At = who, Tokens = tokens, Situation = situation });
                    continue;
                }
                float chance = huge ? 1f : big ? 0.6f : 0.25f;
                if (v.Value.traits.Contains("hype")) chance = Mathf.Max(chance, 0.8f);
                Dialogue.Say(v.Key, situation, tokens, chance, Random.Range(0.4f, 1.6f));
            }
            if (big) News.Add("pull", p.name, card: tokens.Card, price: price);
        }

        private class Gawk
        {
            public Customer At;
            public Dialogue.Tokens Tokens;
            public string Situation;
            public bool Spoke;
            public float Until;
        }

        /// <summary>Walks up to the lucky customer, looks, says something, then goes back to shopping.</summary>
        private class GawkState : ModState
        {
            public const string StateId = "crowd.gawk";
            public override string Id => StateId;

            public override void Enter(Customer c, StateContext ctx)
            {
                var g = (Gawk)ctx.Data;
                // A spot on the gawker's side of them, about an arm's length away.
                var from = g.At.transform.position;
                var dir = c.transform.position - from;
                dir.y = 0f;
                dir = dir.sqrMagnitude < 0.01f ? Vector3.forward : dir.normalized;
                ctx.WalkTo(from + dir * 1.3f);
            }

            public override void Tick(Customer c, StateContext ctx)
            {
                var g = (Gawk)ctx.Data;
                bool gone = g.At == null || !g.At.m_IsActive;
                if (gone || ctx.Elapsed > 20f || (g.Spoke && Time.time >= g.Until))
                {
                    CustomerStates.ToVanilla(c, CustomerLife.Shop);
                    return;
                }
                if (!ctx.HasArrived) return;
                CustomerLife.FaceTowards(c, g.At.transform.position);
                if (g.Spoke) return;
                g.Spoke = true;
                g.Until = Time.time + Random.Range(3.5f, 6f);
                CustomerLife.Stand(c);
                Dialogue.Say(c, "gawk", g.Tokens, 0.9f);
            }
        }

        public void Load(JToken state) { }
        public JToken Save(ICollection<string> keep) => null;
        public string Summary(Persona p) => null;
    }
}
