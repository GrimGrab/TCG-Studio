using System.Collections.Generic;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] Browsing: customers with the trait effect "browse" (Tire-kickers), when deciding what to do next (CustomerChoices), may wander along
    /// 2–4 shelves — the spots where the game makes customers stand (ShelfCompartment / InteractableCardCompartment
    /// m_CustomerStandLoc) — look at the stock for a few seconds, sometimes pick something up and put it back, then go back
    /// to shopping. No saved state.
    /// </summary>
    internal class Browsing : IPersonaModule
    {
        public string Id => "browsing";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerBrowsing.Value;

        private const float BrowseWeight = 0.35f; // × "browse", against vanilla's 1 at each decision
        private const int MaxPerVisit = 2;

        private class Visit
        {
            public int Browses;
            public float NextTry;
        }

        private class Walk
        {
            public List<(Vector3 stand, Vector3 look)> Stops;
            public int At = -1;
            public float LookUntil;
            public bool Looking;
            public float StopStarted;
        }

        public void Init()
        {
            CustomerStates.Register(new BrowseState());
            CustomerChoices.Register("browse", Weight, TryStart);
            UI.CustomerDebugPanel.Section("Browsing", (c, p, rows) =>
            {
                if (!On || Traits.Effect(p, "browse") <= 0f) return;
                var w = CustomerStates.Of(c)?.State.Id == BrowseState.StateId ? (Walk)CustomerStates.Of(c).Data : null;
                rows.Add(("now", w == null ? "not browsing" : $"shelf {w.At + 1} of {w.Stops.Count}" + (w.Looking ? ", looking" : ", walking")));
                rows.Add(("this visit", $"{Cast.PeekVisitData<Visit>(c)?.Browses ?? 0} of {MaxPerVisit} browses"));
            });
        }

        private static float Weight(Customer c)
        {
            var p = On ? Cast.Of(c) : null;
            float browse = p == null ? 0f : Traits.Effect(p, "browse");
            var v = Cast.PeekVisitData<Visit>(c);
            if (browse <= 0f || (v != null && (v.Browses >= MaxPerVisit || Time.time < v.NextTry))) return 0f;
            return BrowseWeight * browse;
        }

        private static bool TryStart(Customer c)
        {
            var v = Cast.VisitData<Visit>(c);
            if (v == null) return false;
            v.NextTry = Time.time + 30f;
            var stops = PickStops(c, Random.Range(2, 5));
            if (stops.Count == 0) return false;
            v.Browses++;
            return CustomerStates.Goto(c, BrowseState.StateId, new Walk { Stops = stops });
        }

        /// <summary>Random shelf spots (item and card shelves), nearest-ish first so the walk makes sense.</summary>
        private static List<(Vector3, Vector3)> PickStops(Customer c, int n)
        {
            var all = new List<(Vector3 stand, Vector3 look)>();
            foreach (var shelf in ShelfManager.GetShelfList() ?? new List<Shelf>())
            {
                if (shelf == null || !shelf.gameObject.activeInHierarchy) continue;
                foreach (var comp in shelf.GetItemCompartmentList() ?? new List<ShelfCompartment>())
                    if (comp != null && comp.m_CustomerStandLoc != null) all.Add((comp.m_CustomerStandLoc.position, comp.transform.position));
            }
            foreach (var shelf in ShelfManager.GetCardShelfList() ?? new List<CardShelf>())
            {
                if (shelf == null || !shelf.gameObject.activeInHierarchy) continue;
                foreach (var comp in shelf.GetCardCompartmentList() ?? new List<InteractableCardCompartment>())
                    if (comp != null && comp.m_CustomerStandLoc != null) all.Add((comp.m_CustomerStandLoc.position, comp.transform.position));
            }
            var picked = new List<(Vector3, Vector3)>();
            var from = c.transform.position;
            for (int i = 0; i < n && all.Count > 0; i++)
            {
                // Of three random candidates, the closest to where they are: wandering, not zig-zagging.
                int best = Random.Range(0, all.Count);
                for (int k = 0; k < 2; k++)
                {
                    int j = Random.Range(0, all.Count);
                    if (Vector3.Distance(from, all[j].stand) < Vector3.Distance(from, all[best].stand)) best = j;
                }
                picked.Add(all[best]);
                from = all[best].stand;
                all.RemoveAt(best);
            }
            return picked;
        }

        private class BrowseState : ModState
        {
            public const string StateId = "browsing.browse";
            public override string Id => StateId;

            public override void Enter(Customer c, StateContext ctx) => Next(c, ctx);

            public override void Tick(Customer c, StateContext ctx)
            {
                var w = (Walk)ctx.Data;
                if (!w.Looking)
                {
                    // At the shelf (or stuck on the way for 12 s): look at it.
                    if (!ctx.HasArrived && Time.time - w.StopStarted < 12f) return;
                    w.Looking = true;
                    w.LookUntil = Time.time + Random.Range(3f, 6f);
                    CustomerLife.Stand(c);
                    CustomerLife.FaceTowards(c, w.Stops[w.At].look);
                    if (Random.value < 0.4f) c.m_Anim.SetTrigger("GrabItem"); // picks something up… and puts it back
                    Dialogue.Say(c, "browseLook", null, 0.25f);
                    return;
                }
                CustomerLife.FaceTowards(c, w.Stops[w.At].look);
                if (Time.time >= w.LookUntil) Next(c, ctx);
            }

            private static void Next(Customer c, StateContext ctx)
            {
                var w = (Walk)ctx.Data;
                w.At++;
                if (w.At >= w.Stops.Count)
                {
                    CustomerStates.ToVanilla(c, CustomerLife.Shop);
                    return;
                }
                w.Looking = false;
                w.StopStarted = Time.time;
                ctx.WalkTo(w.Stops[w.At].stand);
            }
        }

        public void Load(JToken state) { }
        public JToken Save(ICollection<string> keep) => null;
        public string Summary(Persona p) => null;
    }
}
