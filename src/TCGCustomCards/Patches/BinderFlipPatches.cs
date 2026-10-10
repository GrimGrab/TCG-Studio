using HarmonyLib;
using TCGCustomCards.Runtime;
using TCGCustomCards.Runtime.Mtg;
using UnityEngine;

namespace TCGCustomCards.Patches
{
    /// <summary>
    /// Turning double-faced cards over (cards with a back-face picture): F (or a controller's Y) turns the card 180° so its
    /// back panel shows, and CardBackMesh draws the back-face picture on that panel. Only the visual card turns: its card
    /// data, collection, prices and saves don't change.
    /// - Binder close-up: the turn is added on top of the binder's own mouse tilt (it sets the card's target rotation every
    ///   frame; this postfix rotates that target), and the card name follows the side shown. Each new close-up starts on the front.
    /// - Holding cards: the front card in your hand turns (InteractableCard3d's own rotation lerp) and turns back by itself
    ///   when it leaves your hand, so nothing is ever placed turned around.
    /// </summary>
    internal static class CardFlip
    {
        private static readonly Quaternion Turned = Quaternion.Euler(0f, 180f, 0f);
        private static readonly AccessTools.FieldRef<InteractableCard3d, Quaternion> TargetRot =
            AccessTools.FieldRefAccess<InteractableCard3d, Quaternion>("m_CardTargetLerpRot");
        private static readonly AccessTools.FieldRef<InteractableCard3d, bool> LerpingRot =
            AccessTools.FieldRefAccess<InteractableCard3d, bool>("m_IsLerpingCardRot");

        /// <summary>Has a back-face picture (double-faced card).</summary>
        private static bool CanTurn(CardData d) => MtgCardFaces.BackFaceOf(d) != null;

        private static bool Pressed() => Input.GetKeyDown(KeyCode.F) || Input.GetKeyDown(KeyCode.JoystickButton3);

        // ------------------------------------------------------------------ binder close-up

        private static string _binderKey;
        private static InteractableCard3d _binderCard;
        private static bool _binderTurned;
        private static bool _binderHint;

        [HarmonyPatch(typeof(CollectionBinderFlipAnimCtrl), "Update")]
        private static class BinderPatch
        {
            private static void Postfix(CollectionBinderFlipAnimCtrl __instance)
            {
                var shown = __instance.m_CurrentSpawnedInteractableCard3d;
                var view = __instance.m_CurrentViewInteractableCard3d;
                bool closeUp = __instance.m_IsHoldingCardCloseUp && !__instance.m_IsExitingCardCloseUp && shown != null &&
                               shown.gameObject.activeInHierarchy && view != null;
                CardData front = closeUp ? view.m_Card3dUI?.m_CardUI?.GetCardData() : null;
                if (front == null) { _binderKey = null; _binderCard = null; _binderTurned = false; _binderHint = false; return; }

                string key = UI.CardRenderCache.Key(front);
                if (key != _binderKey || shown != _binderCard) // another card opened: start on its front
                {
                    _binderKey = key;
                    _binderCard = shown;
                    _binderTurned = false;
                }
                _binderHint = CanTurn(front);
                if (!_binderHint) return;
                Hooks.HooksRunner.Ensure();
                if (Pressed())
                {
                    _binderTurned = !_binderTurned;
                    SoundManager.GenericPop();
                    SetName(__instance, front, _binderTurned);
                }
                // The binder set this frame's target (mouse tilt) in Update before us; turn it around when showing the back.
                if (_binderTurned) shown.SetTargetRotation(TargetRot(shown) * Turned);
            }
        }

        private static void SetName(CollectionBinderFlipAnimCtrl ctrl, CardData front, bool back)
        {
            var nameText = ctrl.m_CollectionBinderUI != null ? ctrl.m_CollectionBinderUI.m_CardNameText : null;
            if (nameText == null || !Registry.IsCustom(front.expansionType) || !(Registry.Get(front.expansionType) is CustomSet set) ||
                !set.PosByMonster.TryGetValue(front.monsterType, out int pos)) return;
            nameText.text = back ? set.Card(pos).Mtg?.BackName ?? set.Card(pos).Name : InventoryBase.GetMonsterData(front.monsterType).GetName();
        }

        // ------------------------------------------------------------------ cards in your hand

        private static InteractableCard3d _handTurned;
        private static InteractableCard3d _handReturning; // turning back to the front (animated), handed back to the game once straight
        private static bool _handHint;

        /// <summary>Runs every frame while you hold cards (vanilla's hold-card raycast step).</summary>
        [HarmonyPatch(typeof(InteractionPlayerController), "RaycastHoldCardState")]
        private static class HandPatch
        {
            private static void Postfix()
            {
                var held = InteractionPlayerController.GetCurrentHoldCard();
                var data = held != null ? held.m_Card3dUI?.m_CardUI?.GetCardData() : null;
                _handHint = data != null && CanTurn(data);
                if (!_handHint) return;
                Hooks.HooksRunner.Ensure();
                if (!Pressed()) return;
                if (_handTurned == held)
                {
                    held.SetTargetRotation(Quaternion.identity); // animated back to the front, like turning it over
                    _handTurned = null;
                    _handReturning = held;
                }
                else
                {
                    if (_handTurned != null) TurnBack(_handTurned);
                    if (_handReturning != null && _handReturning != held) TurnBack(_handReturning);
                    held.SetTargetRotation(Turned);
                    _handTurned = held;
                    if (_handReturning == held) _handReturning = null;
                }
                SoundManager.GenericPop();
            }
        }

        private static void TurnBack(InteractableCard3d card)
        {
            if (card != null && card.m_Card3dUI != null)
            {
                card.m_Card3dUI.m_ScaleGrp.localRotation = Quaternion.identity;
                TargetRot(card) = Quaternion.identity;
                LerpingRot(card) = false; // leave its rotation to the game again
            }
            if (_handTurned == card) _handTurned = null;
            if (_handReturning == card) _handReturning = null;
        }

        /// <summary>Every frame (HooksRunner): a turned card that left your hand turns back; hints off outside their views.</summary>
        internal static void Tick()
        {
            var front = InteractionPlayerController.GetCurrentHoldCard();
            // A turned card that left the front of your hand is straightened at once (it may be going onto a rack).
            if (_handTurned != null && front != _handTurned) TurnBack(_handTurned);
            if (_handReturning != null)
            {
                if (front != _handReturning || _handReturning.m_Card3dUI == null) TurnBack(_handReturning);
                else if (Quaternion.Angle(_handReturning.m_Card3dUI.m_ScaleGrp.localRotation, Quaternion.identity) < 0.5f) TurnBack(_handReturning); // home: snap the last bit, game takes over
            }
            if (InteractionPlayerController.GetCurrentHoldCard() == null) _handHint = false;
        }

        /// <summary>"F: turn the card over" (HooksRunner.OnGUI).</summary>
        internal static void DrawHint()
        {
            bool binder = _binderHint && _binderCard != null;
            if (!binder && !_handHint) return;
            var style = new GUIStyle(GUI.skin.label) { alignment = TextAnchor.MiddleCenter, fontSize = Mathf.RoundToInt(Screen.height / 42f), richText = true };
            const string text = "<b>F</b>: turn the card over";
            var r = new Rect(0, Screen.height * 0.04f, Screen.width, Screen.height / 20f); // top centre: never over the card
            style.normal.textColor = Color.black;
            GUI.Label(new Rect(r.x + 2, r.y + 2, r.width, r.height), text, style);
            style.normal.textColor = Color.white;
            GUI.Label(r, text, style);
        }
    }
}
