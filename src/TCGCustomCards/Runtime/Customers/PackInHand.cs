using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// The pack a customer is about to open, in their left hand: a copy of how the bought item is drawn (its mesh, materials and the
    /// property block that picks its art), upright and tilted towards them. It follows the hand instead of hanging from it, so its pose
    /// doesn't depend on how the animation turns the wrist.
    /// </summary>
    [DefaultExecutionOrder(100)] // after the animations have moved the bones
    internal class PackInHand : MonoBehaviour
    {
        private const float Height = 0.16f; // metres, the pack's longest side
        private const float TiltBack = 15f;

        private Customer _customer;
        private Transform _hand;
        private Quaternion _turn;
        private Vector3 _centre;

        public static PackInHand Show(Customer c, Item source)
        {
            var filter = source != null ? source.m_MeshFilter : null;
            var mesh = filter != null ? filter.sharedMesh : null;
            var hand = FindBone(c.m_Anim.transform, "hand_l");
            if (mesh == null || hand == null || source.m_Mesh == null) return null;

            var go = new GameObject("TCGCC Pack In Hand");
            go.AddComponent<MeshFilter>().sharedMesh = mesh;
            var renderer = go.AddComponent<MeshRenderer>();
            renderer.sharedMaterials = source.m_Mesh.sharedMaterials;
            var block = new MaterialPropertyBlock();
            source.m_Mesh.GetPropertyBlock(block);
            renderer.SetPropertyBlock(block);

            var size = mesh.bounds.size;
            int thin = size.x <= size.y && size.x <= size.z ? 0 : size.y <= size.z ? 1 : 2;
            int tall = thin != 0 && size.x >= size.y && size.x >= size.z ? 0 : thin != 1 && size.y >= size.z ? 1 : 2;
            float scale = Height / Mathf.Max(size[tall], 0.001f);
            go.transform.localScale = Vector3.one * scale;

            var p = go.AddComponent<PackInHand>();
            p._customer = c;
            p._hand = hand;
            // The mesh's thin side towards the customer's front, its long side up.
            p._turn = Quaternion.Inverse(Quaternion.LookRotation(Axis(thin), Axis(tall)));
            p._centre = mesh.bounds.center * scale;
            p.LateUpdate();
            return p;
        }

        public static void Hide(ref PackInHand p)
        {
            if (p != null) Destroy(p.gameObject);
            p = null;
        }

        private static Vector3 Axis(int i) => i == 0 ? Vector3.right : i == 1 ? Vector3.up : Vector3.forward;

        private static Transform FindBone(Transform t, string name)
        {
            if (t.name == name) return t;
            for (int i = 0; i < t.childCount; i++)
            {
                var found = FindBone(t.GetChild(i), name);
                if (found != null) return found;
            }
            return null;
        }

        private void LateUpdate()
        {
            if (_customer == null || _hand == null || !_customer.gameObject.activeInHierarchy) { Destroy(gameObject); return; }
            var body = _customer.transform;
            var rotation = body.rotation * Quaternion.Euler(-TiltBack, 0f, 0f) * _turn;
            transform.SetPositionAndRotation(_hand.position + body.forward * 0.04f + body.up * 0.06f - rotation * _centre, rotation);
        }
    }
}
