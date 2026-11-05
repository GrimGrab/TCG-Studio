using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// A card shown above a customer's head — a pulled card, a traded card — turned to the player, with its value over it.
    /// One per customer; it follows them (CustomerLife.Tick), goes away after its time, when replaced or when the visit ends.
    /// The card is the game's pooled 3D card, set up like the binder's card taken out into the world.
    /// </summary>
    internal static class CardBubble
    {
        private class Shown
        {
            public Card3dUIGroup Card;
            public float Until;
        }

        private const float CardScale = 4.5f;        // × a card's normal size
        private const float CardHeight = 0.09f;      // metres: a card's height at its normal size (≈ 9 cm)
        private const float CardBottom = 1.95f;      // metres above their feet: clear of the head
        private const float CardAboveFeet = CardBottom + CardHeight * CardScale / 2f;    // the card's centre
        private const float LabelAboveFeet = CardBottom + CardHeight * CardScale + 0.08f; // the value, just over its top

        static CardBubble()
        {
            CustomerLife.Tick.Add(Follow);
            Cast.VisitEnded += (c, p, paid) => Hide(c);
        }

        /// <summary>Shows <paramref name="card"/> above the customer for <paramref name="seconds"/>, its value (or <paramref name="label"/>) over it.</summary>
        public static void Show(Customer c, CardData card, float seconds, string label = null)
        {
            Hide(c);
            var shown = Cast.VisitData<Shown>(c);
            var spawner = CSingleton<Card3dUISpawner>.Instance;
            if (shown == null || spawner == null || card == null) return;
            var g = spawner.GetCardUI();
            // The binder's card taken out into the world, step for step (CollectionBinderFlipAnimCtrl :204-215): no culling
            // (the spawner hides the art past 2 m, Card3dUISpawner.Update :65, leaving the plain card base) and the foil
            // materials for world view (the UI ones draw a solid rectangle over a foil card in the world).
            g.m_IgnoreCulling = true;
            g.m_CardUIAnimGrp.gameObject.SetActive(true);
            g.SetSimplifyCardDistanceCull(isCull: false);
            g.m_CardUI.SetFoilCullListVisibility(isActive: true);
            g.m_CardUI.ResetFarDistanceCull();
            g.m_CardUI.SetFoilMaterialList(spawner.m_FoilMaterialWorldView);
            g.m_CardUI.SetFoilBlendedMaterialList(spawner.m_FoilBlendedMaterialWorldView);
            g.m_CardUI.SetCardUI(card);
            g.m_CardUI.SetFoilMaterialListFromSettingData(isWorldView: true);
            g.m_CardUI.SetFoilBlendedMaterialListFromSettingData(isWorldView: true);
            // Sized like the game sizes a shown card (ElectronicCardListener: SetLocalScale on the scale group; DisableCard
            // resets it). The root keeps the prefab's scale and stays with the spawner: a root scale or a customer parent
            // made it a huge plane somewhere in the shop.
            g.SetLocalScale(Vector3.one * CardScale);
            shown.Card = g;
            shown.Until = Time.time + seconds;
            Place(c, g);
            float price = CardInfo.Price(card);
            Dialogue.Popup(c, label ?? $"${price:0.00}", LabelAboveFeet, seconds, label == null ? ValueColour(price) : (Color?)null);
        }

        public static void Hide(Customer c)
        {
            var shown = Cast.PeekVisitData<Shown>(c);
            var g = shown?.Card;
            if (g == null) return;
            shown.Card = null;
            g.DisableCard(); // resets culling, scale and parent for the pool
        }

        /// <summary>The colour of a card value: white, then green → gold → red as it gets bigger next to a hit (PackHitPrice).</summary>
        public static Color ValueColour(float price)
        {
            float hit = Mathf.Max(0.01f, Plugin.CustomerPackHitPrice.Value);
            return price >= hit * 10f ? new Color(1f, 0.35f, 0.35f) : price >= hit * 3f ? new Color(1f, 0.82f, 0.3f)
                : price >= hit ? new Color(0.49f, 0.99f, 0.49f) : Color.white;
        }

        /// <summary>A card value with a colour tag, for IMGUI text (the debug panel); the in-world popup can't show tags.</summary>
        public static string Value(float price) => $"<color=#{ColorUtility.ToHtmlStringRGB(ValueColour(price))}>${price:0.00}</color>";

        private static void Follow(Customer c)
        {
            var shown = Cast.PeekVisitData<Shown>(c);
            if (shown?.Card == null) return;
            if (Time.time >= shown.Until) Hide(c);
            else Place(c, shown.Card);
        }

        private static void Place(Customer c, Card3dUIGroup g)
        {
            g.transform.position = c.transform.position + Vector3.up * CardAboveFeet;
            var cam = Core.GameScene.Existing<InteractionPlayerController>()?.m_Cam;
            if (cam != null) g.transform.rotation = Quaternion.LookRotation(g.transform.position - cam.transform.position);
        }
    }
}
