using System.Collections.Generic;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// The shop's running log of notable moments — big pulls, thefts, kicks, trades, rip-offs — that customers bring up in
    /// chats, today's or from days ago ({when}: "today", "yesterday", "3 days ago", "last week", "a while back"). Saved: the
    /// last <see cref="Max"/> items, names as text (a walk-in who left is still "that guy who…").
    /// </summary>
    internal class News : IPersonaModule
    {
        [JsonObject(ItemNullValueHandling = NullValueHandling.Ignore)]
        public class Item
        {
            public string k;   // kind: "pull", "theft", "caught", "kick", "trade", "ripoff"
            public string w;   // who (persona name)
            public string o;   // the other one (trades, rip-offs)
            public string c;   // card (the one that matters: the pull, what the victim gave away)
            public string c2;  // second card (what they got for it)
            public float p;    // price of c
            public int d;      // day (CPlayerData.m_CurrentDay)
        }

        private const int Max = 40;
        private static List<Item> _items = new List<Item>();

        public string Id => "news";
        public bool Enabled => Cast.Enabled;

        public void Init() { }

        public static void Add(string kind, string who, string other = null, string card = null, float price = 0f, string card2 = null)
        {
            _items.Add(new Item { k = kind, w = who, o = other, c = card, c2 = card2, p = (float)System.Math.Round(price, 2), d = CPlayerData.m_CurrentDay });
            if (_items.Count > Max) _items.RemoveAt(0);
        }

        /// <summary>
        /// Something to talk about that isn't about <paramref name="speaker"/> or <paramref name="listener"/> themselves (a
        /// rip-off of the listener is fair game: "remember when…"), recent ones far more likely. Null when there's nothing.
        /// </summary>
        public static Item Pick(string speaker, string listener)
        {
            var options = _items.FindAll(i => i.w != speaker && i.w != listener);
            if (options.Count == 0) return null;
            int today = CPlayerData.m_CurrentDay;
            float total = 0f;
            foreach (var i in options) total += Weight(today - i.d);
            float roll = Random.value * total;
            foreach (var i in options)
                if ((roll -= Weight(today - i.d)) < 0f) return i;
            return options[options.Count - 1];
        }

        private static float Weight(int daysAgo) => 1f / (1f + Mathf.Max(0, daysAgo));

        /// <summary>How customers say when it happened.</summary>
        public static string When(Item i)
        {
            int ago = CPlayerData.m_CurrentDay - i.d;
            return ago <= 0 ? "today" : ago == 1 ? "yesterday" : ago < 7 ? $"{ago} days ago" : ago < 14 ? "last week" : "a while back";
        }

        public static int Count => _items.Count;

        public static IEnumerable<Item> Items => _items;

        /// <summary>A line for the whiteboard, or null for a kind it doesn't show.</summary>
        public static string Describe(Item i)
        {
            string what;
            switch (i.k)
            {
                case "pull": what = $"{i.w} pulled {i.c} (${i.p:0})"; break;
                case "theft": what = $"{i.w} got away with ${i.p:0} of stock"; break;
                case "caught": what = $"{i.w} was caught stealing"; break;
                case "kick": what = $"{i.w} was kicked out"; break;
                case "trade": what = $"{i.w} traded {i.c2} to {i.o} for {i.c}"; break;
                case "ripoff": what = $"{i.w} ripped off {i.o}"; break;
                case "friends": what = $"{i.w} and {i.o} became friends"; break;
                case "duel": what = $"{i.w} beat {i.o} in a grudge match"; break;
                case "rivalry": what = $"{i.w} now sees {i.o} as a rival"; break;
                case "nick":
                case "nickBad": what = $"{i.w} became {i.c}"; break;
                case "nickTaken": what = $"{i.w} took {i.c} from {i.o}"; break;
                default: return null;
            }
            return $"Day {i.d}: {what}";
        }

        public void Load(JToken state) => _items = state?.ToObject<List<Item>>() ?? new List<Item>();

        public JToken Save(ICollection<string> keep) => _items.Count == 0 ? null : JToken.FromObject(_items,
            new JsonSerializer { NullValueHandling = NullValueHandling.Ignore, DefaultValueHandling = DefaultValueHandling.Ignore });

        public string Summary(Persona p) => null;
    }
}
