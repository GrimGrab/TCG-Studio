using System.Collections.Generic;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// Reviews by named customers: the phone's store reviews carry the writer's name (Cast), and each customer keeps their last
    /// 3 reviews of your shop for their profile. Saved per persona: star level, day and the vanilla text reference (not the text).
    /// </summary>
    internal class Reviews : IPersonaModule
    {
        public class Rev
        {
            public int s;   // stars
            public int d;   // day
            public int t;   // ECustomerReviewType
            public int g;   // text good/bad level
            public int i;   // text index
            public int it;  // EItemType
        }

        public class Row
        {
            public List<Rev> r; // newest first, at most 3
        }

        public string Id => "reviews";
        public bool Enabled => Cast.Enabled;

        private static readonly PersonaTable<Row> Table = new PersonaTable<Row>();

        public void Init()
        {
            CustomerLife.ReviewAdded += (c, review) =>
            {
                var p = Cast.Enabled ? Cast.Of(c) : null;
                if (p == null) return;
                var row = Table.Get(p);
                if (row.r == null) row.r = new List<Rev>();
                row.r.Insert(0, new Rev
                {
                    s = review.starLevel, d = review.day, t = (int)review.customerReviewType, g = review.textSOGoodBadLevel,
                    i = review.textSOIndex, it = (int)review.itemType,
                });
                if (row.r.Count > 3) row.r.RemoveRange(3, row.r.Count - 3);
            };
        }

        /// <summary>The persona's reviews as shown on the phone: (stars, day, text), newest first.</summary>
        public static List<(int stars, int day, string text)> Of(Persona p)
        {
            var list = new List<(int, int, string)>();
            var row = Table.Get(p, create: false);
            var mgr = CSingleton<CustomerReviewManager>.Instance;
            if (row?.r == null || mgr == null) return list;
            foreach (var r in row.r)
            {
                var data = new CustomerReviewData
                {
                    starLevel = r.s, day = r.d, customerReviewType = (ECustomerReviewType)r.t, textSOGoodBadLevel = r.g,
                    textSOIndex = r.i, itemType = (EItemType)r.it, customerName = p.name,
                };
                string text;
                try { text = mgr.GetReviewTextString(data); }
                catch { text = ""; }
                list.Add((r.s, r.d, text));
            }
            return list;
        }

        public void Load(JToken state) => Table.Load(state);
        public JToken Save(ICollection<string> keep) => Table.Save(keep);

        public string Summary(Persona p)
        {
            var row = Table.Get(p, create: false);
            if (row?.r == null || row.r.Count == 0) return null;
            return $"last review {row.r[0].s}/5 (day {row.r[0].d})";
        }
    }
}
