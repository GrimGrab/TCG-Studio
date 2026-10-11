using UnityEngine;
using UnityEngine.InputSystem;
using UnityEngine.InputSystem.Controls;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// Controller buttons the game leaves free while browsing the binder (measured from the game's default gamepad map:
    /// A/B/X/Y, shoulders, triggers, d-pad, left stick and Start are all used there; the right stick moves the camera).
    /// </summary>
    internal enum BinderPadButton { None, LeftStickClick, RightStickClick, SelectButton }

    /// <summary>
    /// Raw gamepad buttons through the game's own InputManager device (its button reader is private). Only read while the
    /// game says a controller is in use, so a pad lying on the desk never acts during mouse play.
    /// </summary>
    internal static class PadInput
    {
        public static bool Active
        {
            get
            {
                var im = CSingleton<InputManager>.Instance;
                return im != null && im.m_IsControllerActive && im.m_CurrentGamepad != null;
            }
        }

        public static bool IsPS => CSingleton<InputManager>.Instance?.m_IsPSController ?? false;

        private static Gamepad Pad => Active ? CSingleton<InputManager>.Instance.m_CurrentGamepad : null;

        public static ButtonControl Control(EGamepadControlBtn b)
        {
            var g = Pad;
            if (g == null) return null;
            switch (b)
            {
                case EGamepadControlBtn.X: return g.buttonSouth;
                case EGamepadControlBtn.O: return g.buttonEast;
                case EGamepadControlBtn.Square: return g.buttonWest;
                case EGamepadControlBtn.Triangle: return g.buttonNorth;
                case EGamepadControlBtn.L1: return g.leftShoulder;
                case EGamepadControlBtn.R1: return g.rightShoulder;
                case EGamepadControlBtn.L2: return g.leftTrigger;
                case EGamepadControlBtn.R2: return g.rightTrigger;
                case EGamepadControlBtn.L3: return g.leftStickButton;
                case EGamepadControlBtn.R3: return g.rightStickButton;
                case EGamepadControlBtn.Start: return g.startButton;
                case EGamepadControlBtn.Select: return g.selectButton;
                default: return null;
            }
        }

        public static bool Down(EGamepadControlBtn b) => Control(b)?.wasPressedThisFrame ?? false;

        public static EGamepadControlBtn ToGame(BinderPadButton b)
        {
            switch (b)
            {
                case BinderPadButton.LeftStickClick: return EGamepadControlBtn.L3;
                case BinderPadButton.RightStickClick: return EGamepadControlBtn.R3;
                case BinderPadButton.SelectButton: return EGamepadControlBtn.Select;
                default: return EGamepadControlBtn.None;
            }
        }

        public static bool Down(BinderPadButton b) => b != BinderPadButton.None && Down(ToGame(b));

        /// <summary>
        /// D-pad or left stick as one of four directions (zero = none). The stick needs 0.5 to start a direction and keeps
        /// <paramref name="held"/> down to 0.3, like the game's own down/up thresholds.
        /// </summary>
        public static Vector2Int Direction(Vector2Int held)
        {
            var g = Pad;
            if (g == null) return Vector2Int.zero;
            if (g.dpad.up.isPressed) return Vector2Int.up;
            if (g.dpad.down.isPressed) return Vector2Int.down;
            if (g.dpad.left.isPressed) return Vector2Int.left;
            if (g.dpad.right.isPressed) return Vector2Int.right;
            var v = g.leftStick.ReadValue();
            if (held != Vector2Int.zero && Vector2.Dot(v, held) >= 0.3f) return held;
            if (Mathf.Abs(v.x) >= Mathf.Abs(v.y))
                return Mathf.Abs(v.x) >= 0.5f ? new Vector2Int(v.x > 0f ? 1 : -1, 0) : Vector2Int.zero;
            return Mathf.Abs(v.y) >= 0.5f ? new Vector2Int(0, v.y > 0f ? 1 : -1) : Vector2Int.zero;
        }

        /// <summary>A face button is held or was let go this frame (its release must not reach the binder).</summary>
        public static bool FaceButtonBusy()
        {
            var g = Pad;
            if (g == null) return false;
            foreach (var c in new[] { g.buttonSouth, g.buttonEast, g.buttonWest, g.buttonNorth })
                if (c.isPressed || c.wasReleasedThisFrame) return true;
            return false;
        }

        /// <summary>The game's own glyph for a button (Xbox or PlayStation set, as the game picked).</summary>
        public static Sprite Glyph(EGamepadControlBtn b)
        {
            var so = CSingleton<CGameManager>.Instance?.m_TextSO;
            if (so == null || b == EGamepadControlBtn.None) return null;
            var list = IsPS ? so.m_PSCtrlBtnSpriteList : so.m_XBoxCtrlBtnSpriteList;
            int i = (int)b;
            return list != null && i >= 0 && i < list.Count ? list[i] : null;
        }
    }
}
