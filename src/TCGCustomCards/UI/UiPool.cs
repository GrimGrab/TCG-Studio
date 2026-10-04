using System.Collections.Generic;
using System.Linq;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.UI
{
    /// <summary>Grows the scene-authored UI pools of vanilla list screens (restock, check price, checkout, furniture shop, …).</summary>
    internal static class UiPool
    {
        /// <summary>
        /// Grows a scene-authored UI pool to <paramref name="want"/> entries so new entries continue the list's layout:
        /// pool split over row containers → clone whole rows; one parent with a layout group → clone into it;
        /// one hand-placed parent → place clones one row pitch below the entry a row above.
        /// </summary>
        public static void Grow<T>(List<T> pool, int want, string what) where T : Component
        {
            if (pool == null || pool.Count == 0 || pool.Count >= want) return;
            int before = pool.Count;
            var rows = pool.Select(p => p.transform.parent).Distinct().ToList();
            string mode;
            if (rows.Count > 1) { mode = $"{rows.Count} row containers"; GrowRows(pool, want, rows); }
            else if (rows[0].GetComponent<LayoutGroup>() != null) { mode = rows[0].GetComponent<LayoutGroup>().GetType().Name; GrowInto(pool, want); }
            else { mode = "hand-placed"; GrowPlaced(pool, want); }
            Plugin.Log.LogInfo($"{what}: pool grown {before} → {pool.Count} ({mode})");
        }

        private static T Clone<T>(T template, Transform parent, int index) where T : Component
        {
            var clone = Object.Instantiate(template.gameObject, parent, false).GetComponent<T>();
            clone.name = $"{template.name}_TCGCC_{index}";
            return clone;
        }

        private static void GrowInto<T>(List<T> pool, int want) where T : Component
        {
            var template = pool[pool.Count - 1];
            while (pool.Count < want) pool.Add(Clone(template, template.transform.parent, pool.Count));
        }

        private static void GrowPlaced<T>(List<T> pool, int want) where T : Component
        {
            // Columns = entries sharing the first entry's row; pitch = first entry of row 2 minus first entry of row 1.
            var first = pool[0].transform.localPosition;
            int cols = pool.TakeWhile(p => Mathf.Abs(p.transform.localPosition.y - first.y) < 1f).Count();
            if (cols >= pool.Count) cols = 1;
            Vector3 pitch = pool.Count > cols ? pool[cols].transform.localPosition - first : new Vector3(0f, -100f, 0f);
            var template = pool[pool.Count - 1];
            while (pool.Count < want)
            {
                var clone = Clone(template, template.transform.parent, pool.Count);
                clone.transform.localPosition = pool[pool.Count - cols].transform.localPosition + pitch;
                pool.Add(clone);
            }
        }

        private static void GrowRows<T>(List<T> pool, int want, List<Transform> rows) where T : Component
        {
            // Template = the last full row (the last one may be partly used).
            int full = rows.Max(r => pool.Count(p => p.transform.parent == r));
            var template = rows.Last(r => pool.Count(p => p.transform.parent == r) == full);
            var container = template.parent;
            bool laidOut = container != null && container.GetComponent<LayoutGroup>() != null;
            Vector3 pitch = rows[rows.Count - 1].localPosition - rows[rows.Count - 2].localPosition;
            var last = rows[rows.Count - 1];
            while (pool.Count < want)
            {
                var row = Object.Instantiate(template.gameObject, container, false).transform;
                row.name = $"{template.name}_TCGCC_{pool.Count}";
                row.SetSiblingIndex(last.GetSiblingIndex() + 1);
                if (!laidOut) row.localPosition = last.localPosition + pitch;
                foreach (var entry in row.GetComponentsInChildren<T>(true))
                {
                    if (entry.transform.parent != row) continue;
                    entry.name = $"{entry.name}_TCGCC_{pool.Count}";
                    pool.Add(entry);
                }
                last = row;
            }
        }
    }
}
