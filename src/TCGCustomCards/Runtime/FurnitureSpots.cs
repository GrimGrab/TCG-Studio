using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Core;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Replaces a template piece's spots with the ones from its def. The game finds spots as the children of the shelf's compartment
    /// groups (Shelf/CardShelf.Init), so we clone the base's first compartment of each kind once per spot, move it into place, resize it,
    /// and remove the base's own compartments.
    ///
    /// Item spot geometry (ShelfCompartment.CalculatePositionList): items fill the box from m_StartLoc towards −right (width),
    /// +forward (depth) and ±up (height), with m_StartLoc's rotation. A def spot is the box's centre + rotation (= m_StartLoc's rotation)
    /// + size, so m_StartLoc sits at centre + right·w/2 − forward·d/2 ∓ up·h/2.
    /// Card spot geometry: the card sits at m_PutCardLocation; the def's pos/rot is that pose.
    /// </summary>
    internal static class FurnitureSpots
    {
        public static void Build(InteractableObject tpl, FurnitureDef def, ShelfData_ScriptableObject so, InventoryBase inv)
        {
            var itemSpots = def.Spots.Where(s => s.Kind == FurnitureSpotKind.Items).ToList();
            var cardSpots = def.Spots.Where(s => s.Kind == FurnitureSpotKind.Card).ToList();

            if (itemSpots.Count > 0)
            {
                var groups = FurnitureKinds.ItemGroups(tpl);
                int needed = PosSlotsNeeded(inv, itemSpots);
                Rebuild<ShelfCompartment>(tpl, def, groups, itemSpots, (c, s) => PlaceItemSpot(tpl.transform, c, s, needed));
            }
            if (cardSpots.Count > 0)
            {
                var groups = FurnitureKinds.CardGroups(tpl);
                Rebuild<InteractableCardCompartment>(tpl, def, groups, cardSpots, (c, s) => PlaceCardSpot(tpl.transform, c, s));
            }
        }

        private static void Rebuild<T>(InteractableObject tpl, FurnitureDef def, List<Transform> groups, List<FurnitureSpotDef> spots,
            System.Action<T, FurnitureSpotDef> place) where T : Component
        {
            if (groups == null || groups.Count == 0)
            {
                Plugin.Log.LogWarning($"Furniture '{def.Id}': base {def.BaseObject} has no {typeof(T).Name} groups; its own spots are kept");
                return;
            }
            var originals = new List<GameObject>();
            T sample = null;
            Transform parent = null;
            foreach (var g in groups)
            {
                if (g == null) continue;
                for (int i = 0; i < g.childCount; i++)
                {
                    var comp = g.GetChild(i).GetComponent<T>();
                    if (comp == null) continue;
                    originals.Add(comp.gameObject);
                    if (sample == null) { sample = comp; parent = g; }
                }
            }
            if (sample == null)
            {
                Plugin.Log.LogWarning($"Furniture '{def.Id}': base {def.BaseObject} has no {typeof(T).Name}; its own spots are kept");
                return;
            }
            // Clones first (from the untouched sample), then the base's compartments go. Detach before destroying so child order is clean.
            var clones = new List<T>();
            for (int i = 0; i < spots.Count; i++)
            {
                var go = Object.Instantiate(sample.gameObject, parent);
                go.name = $"{sample.gameObject.name}_spot{i + 1}";
                clones.Add(go.GetComponent<T>());
            }
            foreach (var o in originals)
            {
                o.transform.SetParent(null, false);
                Object.DestroyImmediate(o);
            }
            for (int i = 0; i < spots.Count; i++)
            {
                clones[i].transform.SetSiblingIndex(i);
                place(clones[i], spots[i]);
            }
        }

        private static void PlaceItemSpot(Transform piece, ShelfCompartment c, FurnitureSpotDef s, int posSlots)
        {
            if (c.m_StartLoc == null || c.m_EndWidthLoc == null || c.m_EndDepthLoc == null || c.m_EndHeightLoc == null)
            {
                Plugin.Log.LogWarning($"Spot '{c.name}': compartment is missing its box markers; left as on the base piece");
                return;
            }
            var rot = piece.rotation * Quaternion.Euler(s.Rot[0], s.Rot[1], s.Rot[2]);
            var centre = piece.TransformPoint(V(s.Pos));
            float w = s.Size[0], d = s.Size[1], h = s.Size[2];
            var right = rot * Vector3.right;
            var fwd = rot * Vector3.forward;
            var upDir = (c.m_HeightGoesUp ? 1f : -1f) * (rot * Vector3.up);
            var start = centre + right * (w * 0.5f) - fwd * (d * 0.5f) - upDir * (h * 0.5f);

            // The collider the player aims at, measured in the base spot's frame (centre of its box, m_StartLoc rotation) before moving.
            var colliders = c.GetComponents<BoxCollider>();
            var baseBounds = new Bounds[colliders.Length];
            float w0, d0;
            {
                var s0 = c.m_StartLoc;
                w0 = (c.m_EndWidthLoc.position - s0.position).magnitude;
                d0 = (c.m_EndDepthLoc.position - s0.position).magnitude;
                float h0 = (c.m_EndHeightLoc.position - s0.position).magnitude;
                var up0 = (c.m_HeightGoesUp ? 1f : -1f) * s0.up;
                var centre0 = s0.position - s0.right * (w0 * 0.5f) + s0.forward * (d0 * 0.5f) + up0 * (h0 * 0.5f);
                var frame0 = new GameObject("tcgcc_frame0").transform;
                frame0.SetPositionAndRotation(centre0, s0.rotation);
                for (int i = 0; i < colliders.Length; i++) baseBounds[i] = ColliderIn(colliders[i], frame0);
                Object.DestroyImmediate(frame0.gameObject);
            }

            MoveSoChildHasPose(c.transform, c.m_StartLoc, start, rot);
            c.m_EndWidthLoc.position = start - right * w;
            c.m_EndDepthLoc.position = start + fwd * d;
            c.m_EndHeightLoc.position = start + upDir * h;

            c.m_SizeX = s.Grid[0];
            c.m_SizeY = s.Grid[1];
            c.m_SizeZ = s.Grid[2];
            if (s.Boxes.HasValue) c.m_CanPutBox = s.Boxes.Value;
            EnsurePosSlots(c, posSlots);

            c.m_CustomerStandLoc = EnsureCustomer(piece, c.transform, c.m_CustomerStandLoc, s, centre);
            if (s.PriceTag != null && c.m_InteractablePriceTagList != null && c.m_InteractablePriceTagList.Count > 0 && c.m_InteractablePriceTagList[0] != null)
                c.m_InteractablePriceTagList[0].transform.position = piece.TransformPoint(V(s.PriceTag));
            if (c.m_GamepadQuickSelectAimLoc != null) c.m_GamepadQuickSelectAimLoc.position = centre;

            // The compartment's collider is what the player aims at to put items in. Keep the base collider's thickness and height
            // (vanilla shop spots have a flat box, height 0, but a collider with volume above it) and stretch it to the new width/depth.
            var frame = new GameObject("tcgcc_frame").transform;
            frame.SetPositionAndRotation(centre, rot);
            for (int i = 0; i < colliders.Length; i++)
            {
                var b = baseBounds[i];
                float sx = w0 > 1e-4f ? w / w0 : 1f, sz = d0 > 1e-4f ? d / d0 : 1f;
                var min = new Vector3(b.min.x * sx, b.min.y, b.min.z * sz);
                var max = new Vector3(b.max.x * sx, b.max.y, b.max.z * sz);
                // A taller spot than the base: the collider covers it too.
                min.y = Mathf.Min(min.y, -h * 0.5f);
                max.y = Mathf.Max(max.y, h * 0.5f);
                if (max.y - min.y < 0.02f) max.y = min.y + 0.1f; // never paper-thin
                var nb = new Bounds();
                nb.SetMinMax(min, max);
                FurnitureInjector.FitCollider(colliders[i], frame, nb);
            }
            Object.DestroyImmediate(frame.gameObject);
        }

        /// <summary>A box collider's extent in another frame (AABB of its 8 corners).</summary>
        private static Bounds ColliderIn(BoxCollider col, Transform frame)
        {
            var e = col.size * 0.5f;
            var min = new Vector3(float.MaxValue, float.MaxValue, float.MaxValue);
            var max = new Vector3(float.MinValue, float.MinValue, float.MinValue);
            for (int k = 0; k < 8; k++)
            {
                var local = col.center + new Vector3((k & 1) == 0 ? -e.x : e.x, (k & 2) == 0 ? -e.y : e.y, (k & 4) == 0 ? -e.z : e.z);
                var p = frame.InverseTransformPoint(col.transform.TransformPoint(local));
                min = Vector3.Min(min, p);
                max = Vector3.Max(max, p);
            }
            var b = new Bounds();
            b.SetMinMax(min, max);
            return b;
        }

        private static void PlaceCardSpot(Transform piece, InteractableCardCompartment c, FurnitureSpotDef s)
        {
            var rot = piece.rotation * Quaternion.Euler(s.Rot[0], s.Rot[1], s.Rot[2]);
            var pos = piece.TransformPoint(V(s.Pos));
            var anchor = c.m_PutCardLocation != null ? c.m_PutCardLocation : c.transform;
            MoveSoChildHasPose(c.transform, anchor, pos, rot);
            c.m_CustomerStandLoc = EnsureCustomer(piece, c.transform, c.m_CustomerStandLoc, s, pos);
            if (s.PriceTag != null && c.m_InteractablePriceTagList != null && c.m_InteractablePriceTagList.Count > 0 && c.m_InteractablePriceTagList[0] != null)
                c.m_InteractablePriceTagList[0].transform.position = piece.TransformPoint(V(s.PriceTag));
        }

        /// <summary>
        /// Every spot needs a customer point (customers stand there to take from it). The def's point wins; without one the clone keeps
        /// the base compartment's (moved along with the spot). A compartment without any gets one on the floor in front of the spot.
        /// </summary>
        private static Transform EnsureCustomer(Transform piece, Transform compartment, Transform stand, FurnitureSpotDef s, Vector3 spotWorld)
        {
            if (stand == null)
            {
                stand = new GameObject("CustomerStandLocation").transform;
                stand.SetParent(compartment, false);
                var front = piece.InverseTransformPoint(spotWorld);
                stand.position = piece.TransformPoint(front.x, 0f, front.z + 0.54f);
                Plugin.Log.LogWarning($"Spot '{compartment.name}': the base compartment has no customer point; one was added in front of it");
            }
            if (s.Customer != null) stand.position = piece.TransformPoint(s.Customer[0], 0f, s.Customer[1]);
            return stand;
        }

        /// <summary>Moves/rotates <paramref name="root"/> rigidly so that its descendant <paramref name="child"/> ends at the given world pose.</summary>
        private static void MoveSoChildHasPose(Transform root, Transform child, Vector3 pos, Quaternion rot)
        {
            if (child == root) { root.SetPositionAndRotation(pos, rot); return; }
            var relRot = Quaternion.Inverse(root.rotation) * child.rotation;
            root.rotation = rot * Quaternion.Inverse(relRot);
            root.position += pos - child.position;
        }

        /// <summary>
        /// The compartment has one slot transform per item it can hold (children of m_PosListGrp, collected in its Awake). Make sure there
        /// are enough for the biggest count any item can reach in this grid (vanilla can index past them otherwise).
        /// </summary>
        private static void EnsurePosSlots(ShelfCompartment c, int needed)
        {
            if (c.m_PosListGrp == null) return;
            var grp = c.m_PosListGrp;
            while (grp.childCount < needed)
            {
                var t = grp.childCount > 0 ? Object.Instantiate(grp.GetChild(0).gameObject, grp).transform : new GameObject("pos").transform;
                if (t.parent != grp) t.SetParent(grp, false);
                t.name = $"pos{grp.childCount - 1}";
            }
        }

        /// <summary>Largest item count CalculatePositionList can produce for any spot of this piece, over every item's dimensions.</summary>
        private static int PosSlotsNeeded(InventoryBase inv, List<FurnitureSpotDef> spots)
        {
            var dims = new List<Vector3> { Vector3.one, new Vector3(4, 4, 2), new Vector3(4, 4, 1) }; // empty compartment; boxes (ShelfCompartment :175/:181)
            var items = inv.m_StockItemData_SO?.m_ItemDataList;
            if (items != null) foreach (var it in items) if (it != null && it.itemDimension.x > 0 && it.itemDimension.y > 0 && it.itemDimension.z > 0) dims.Add(it.itemDimension);
            int best = 1;
            foreach (var s in spots)
                foreach (var dim in dims)
                {
                    float dx = Mathf.Min(dim.x, s.Grid[0]);
                    int n = Mathf.Max(1, Mathf.RoundToInt(s.Grid[0] / dx)) * Mathf.Max(1, Mathf.RoundToInt(s.Grid[1] / dim.y)) * Mathf.Max(1, Mathf.RoundToInt(s.Grid[2] / dim.z));
                    best = Mathf.Max(best, n);
                }
            return Mathf.Min(best, 4096);
        }

        private static Vector3 V(float[] a) => new Vector3(a[0], a[1], a[2]);
    }
}
