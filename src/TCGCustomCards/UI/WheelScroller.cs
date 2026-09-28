using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.UI
{
    /// <summary>Scrolls a ScrollRect from the raw mouse wheel while it is active (modal popups on screens without EventSystem scroll).</summary>
    internal class WheelScroller : MonoBehaviour
    {
        private ScrollRect _scroll;
        private float _step;

        /// <summary>True while any WheelScroller is visible; used to stop the screen underneath from also scrolling.</summary>
        public static int ActiveCount;

        public void Init(ScrollRect scroll, float rowStep)
        {
            _scroll = scroll;
            _step = rowStep;
        }

        private void OnEnable() => ActiveCount++;
        private void OnDisable() => ActiveCount = Mathf.Max(0, ActiveCount - 1);

        private void Update()
        {
            if (_scroll == null) return;
            float wheel = Input.mouseScrollDelta.y;
            if (Mathf.Abs(wheel) < 0.01f) return;
            var content = _scroll.content;
            float max = Mathf.Max(0f, content.rect.height - _scroll.viewport.rect.height);
            float y = Mathf.Clamp(content.anchoredPosition.y - wheel * _step * 0.5f, 0f, max);
            content.anchoredPosition = new Vector2(content.anchoredPosition.x, y);
        }
    }
}
