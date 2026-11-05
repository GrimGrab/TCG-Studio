using System.Collections.Generic;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] Shoplifting: customers with the Thief trait (effect "steal") may skip the till
    /// and walk out with unpaid items. They are marked in red and announced; look at them and left click to catch
    /// them: they go back and pay, upset. A thief who reaches the exit takes the stock with them (vanilla removes a leaving
    /// customer's bag) and never counts as sold (CustomerLife.WalkOutWithoutPaying); the day's report shows it as a loss
    /// ("Stolen goods"). Being caught never bans them. Saved: times caught / times got away per persona, and today's stolen value.
    /// </summary>
    internal class Shoplifting : IPersonaModule
    {
        public class Row
        {
            public int c; // times caught
            public int e; // times got away
        }


        /// <summary>What thieves got away with since the last end-of-day report (value at your prices, items).</summary>
        private static float _stolenToday;
        private static int _stolenItemsToday;

        public string Id => "shoplifting";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerShoplifting.Value;

        private static readonly PersonaTable<Row> Table = new PersonaTable<Row>();
        /// <summary>Per visit: decided (one try per visit), walking out with <see cref="Value"/> unpaid, or caught and paying.</summary>
        private class Theft
        {
            public bool Decided, Stealing, Caught;
            public float Value;
        }

        /// <summary>Thieves walking out now → value of what they carry.</summary>
        public static IEnumerable<KeyValuePair<Customer, float>> Thieves
        {
            get
            {
                foreach (var v in Cast.Visitors())
                {
                    var t = Cast.PeekVisitData<Theft>(v.Key);
                    if (t != null && t.Stealing) yield return new KeyValuePair<Customer, float>(v.Key, t.Value);
                }
            }
        }

        /// <summary>You caught a thief: (customer, persona).</summary>
        public static event System.Action<Customer, Persona> CaughtStealing;

        /// <summary>A customer just started walking out without paying.</summary>
        public static event System.Action<Customer> StartedStealing;

        public static int Caught(Persona p) => Table.Get(p, create: false)?.c ?? 0;
        public static int GotAway(Persona p) => Table.Get(p, create: false)?.e ?? 0;

        public static bool IsStealing(Customer c) => Cast.PeekVisitData<Theft>(c)?.Stealing == true;

        public static void Catch(Customer c)
        {
            var theft = Cast.PeekVisitData<Theft>(c);
            if (theft == null || !theft.Stealing) return;
            float value = theft.Value;
            theft.Stealing = false;
            theft.Caught = true;
            var p = Cast.Of(c);
            if (p != null)
            {
                Table.Get(p).c++;
                Loyalty.Change(p, Loyalty.CaughtStealing, "caught stealing");
                Dialogue.Say(c, "caught", null, 1f);
                Plugin.Log.LogInfo($"Customers: you caught {p.name} stealing ${value:0.00} of stock");
                News.Add("caught", p.name, price: value);
                CaughtStealing?.Invoke(c, p);
            }
            foreach (var v in Cast.Visitors())
            {
                if (v.Key == c || Vector3.Distance(v.Key.transform.position, c.transform.position) > 8f) continue;
                Dialogue.Say(v.Key, "cheerCatch", new Dialogue.Tokens { Other = p?.name }, 0.5f, Random.Range(0.8f, 2f));
            }
            CustomerStates.ToVanilla(c, CustomerLife.SendToPay);
        }

        public void Init()
        {
            UI.CustomerDebugPanel.Section("Theft", (c, p, rows) =>
            {
                if (!On) return;
                rows.Add(("steal chance", $"{Mathf.Max(0f, Traits.Effect(p, "steal")) * 100f:0}%"));
                var t = Cast.PeekVisitData<Theft>(c);
                if (t != null)
                    rows.Add(("this visit", t.Stealing ? $"<color=#FF6A5A>walking out with ${t.Value:0.00}</color>" : t.Caught ? "caught, paying" : t.Decided ? "decided not to" : "-"));
            });
            CustomerLife.HeadingToPay += c =>
            {
                var theft = On ? Cast.VisitData<Theft>(c) : null;
                if (theft == null || theft.Caught || theft.Decided) return;
                theft.Decided = true;
                var p = Cast.Of(c);
                if (p == null || CustomerLife.IsTournamentEntrant(c) || Random.value >= Traits.Effect(p, "steal")) return;
                float value = 0f;
                foreach (var item in c.m_ItemInBagList) if (item != null) value += item.GetCurrentPrice();
                foreach (var card in c.m_CardInBagList) if (card != null) value += card.GetCurrentPrice();
                if (value <= 0f) return;
                theft.Stealing = true;
                theft.Value = value;
                Plugin.Log.LogInfo($"Customers: {p.name} is trying to walk out with ${value:0.00} of unpaid stock");
                Dialogue.Say(c, "sneak", null, 0.6f, 1f);
                CustomerLife.WalkOutWithoutPaying(c);
                StartedStealing?.Invoke(c);
            };
            CustomerLife.CheckedOut += c =>
            {
                var theft = Cast.PeekVisitData<Theft>(c);
                if (theft == null || !theft.Caught) return;
                theft.Caught = false;
                CustomerStates.Leave(c);
            };
            Cast.VisitEnded += (c, p, paid) =>
            {
                var theft = Cast.PeekVisitData<Theft>(c);
                if (theft != null && theft.Stealing)
                {
                    float value = theft.Value;
                    Table.Get(p).e++;
                    _stolenToday += value;
                    _stolenItemsToday += c.m_ItemInBagList.Count + c.m_CardInBagList.Count;
                    Notice = $"{p.name} got away with ${value:0.00} of your stock!";
                    NoticeUntil = Time.time + 6f;
                    Plugin.Log.LogInfo($"Customers: {p.name} got away with ${value:0.00} of stock");
                    News.Add("theft", p.name, price: value);
                }
            };
            DayReport.AddRow("Stolen goods", () => -_stolenToday, () => On || _stolenToday > 0f);
            DayReport.Closed += () =>
            {
                _stolenToday = 0f;
                _stolenItemsToday = 0;
            };
        }

        /// <summary>Message shown for a few seconds after a thief got away (drawn by the nameplates).</summary>
        public static string Notice;
        public static float NoticeUntil;

        public void Load(JToken state)
        {
            Table.Load(state?["rows"]);
            _stolenToday = state?["today"]?.Value<float>("v") ?? 0f;
            _stolenItemsToday = state?["today"]?.Value<int>("n") ?? 0;
        }

        public JToken Save(ICollection<string> keep)
        {
            var rows = Table.Save(keep);
            if (rows == null && _stolenToday <= 0f) return null;
            var state = new JObject();
            if (rows != null) state["rows"] = rows;
            if (_stolenToday > 0f) state["today"] = new JObject { ["v"] = System.Math.Round(_stolenToday, 2), ["n"] = _stolenItemsToday };
            return state;
        }

        public string Summary(Persona p)
        {
            var row = Table.Get(p, create: false);
            if (row == null) return null;
            var parts = new List<string>();
            if (row.c > 0) parts.Add($"caught stealing {row.c}x");
            if (row.e > 0) parts.Add($"got away {row.e}x");
            return parts.Count == 0 ? null : string.Join(", ", parts);
        }
    }
}
