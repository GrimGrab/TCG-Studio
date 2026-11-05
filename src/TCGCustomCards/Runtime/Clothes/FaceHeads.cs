using UnityEngine;

namespace TCGCustomCards.Runtime.Clothes
{
    /// <summary>
    /// A regular's own face at every distance. Customers have LODs and the game only applies face sliders to LOD0, so from afar they'd
    /// get a stock head: someone with their own face stays on LOD0 for the visit. Customers are pooled, so the game picks again afterwards.
    /// </summary>
    internal static class FaceHeads
    {
        public static void Hold(Component c) => Force(c, 0);

        public static void Clear(Component c) => Force(c, -1);

        private static void Force(Component c, int lod)
        {
            foreach (var group in c.GetComponentsInChildren<LODGroup>(true))
                if (group.enabled && group.gameObject.activeInHierarchy) group.ForceLOD(lod); // Unity logs a warning for each disabled one
        }
    }
}
