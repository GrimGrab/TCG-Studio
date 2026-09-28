using System;
using System.Linq;
using TMPro;
using UnityEngine;
using UnityEngine.Events;
using UnityEngine.UI;
using Object = UnityEngine.Object;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// Two-choice dialog made from a clone of the game's "trash this box?" confirm screen (same look, UI mode, cursor).
    /// The confirm button runs choice A, the labelled cancel button choice B; any other cancel-type button just closes.
    /// </summary>
    internal static class ChoicePopup
    {
        private static ConfirmTrashScreen _screen;
        private static TextMeshProUGUI _message, _labelA, _labelB;
        private static Action _onA, _onB;

        private static bool _buildFailed;

        public static bool IsOpen => (_screen != null && _screen.IsScreenOpened()) || ImGuiChoice.IsOpen;

        public static void Show(string message, string a, Action onA, string b, Action onB)
        {
            if (IsOpen) return;
            if (_screen == null && !_buildFailed) _buildFailed = !Build();
            if (_screen == null)
            {
                ImGuiChoice.Show(message, a, onA, b, onB); // plain fallback if the game's screen changed
                return;
            }
            _message.text = message;
            _labelA.text = a;
            _labelB.text = b;
            _onA = onA;
            _onB = onB;
            _screen.OpenScreen();
        }

        private static void Choose(bool a)
        {
            var act = a ? _onA : _onB;
            _onA = _onB = null;
            if (a) SoundManager.GenericConfirm(); else SoundManager.GenericCancel();
            _screen.CloseScreen();
            try { act?.Invoke(); }
            catch (Exception e) { Plugin.Log.LogError($"ChoicePopup action failed: {e}"); }
        }

        private static void Close()
        {
            _onA = _onB = null;
            SoundManager.GenericCancel();
            _screen.CloseScreen();
        }

        private static bool Build()
        {
            var src = CSingleton<InteractionPlayerController>.Instance?.m_ConfirmTrashScreen;
            if (src == null)
            {
                Plugin.Log.LogWarning("ChoicePopup: no confirm screen to copy");
                return false;
            }
            var go = Object.Instantiate(src.gameObject, src.transform.parent, false);
            go.name = "TCGCC_ChoicePopup";
            _screen = go.GetComponent<ConfirmTrashScreen>();

            // Our texts must not be re-translated by I2 when the screen (re)enables.
            foreach (var loc in go.GetComponentsInChildren<I2.Loc.Localize>(true)) Object.Destroy(loc);

            Button confirm = null, cancel = null;
            foreach (var b in go.GetComponentsInChildren<Button>(true))
            {
                string method = PersistentMethod(b.onClick);
                for (int i = 0; i < b.onClick.GetPersistentEventCount(); i++)
                    b.onClick.SetPersistentListenerState(i, UnityEventCallState.Off);
                bool labelled = b.GetComponentInChildren<TextMeshProUGUI>(true) != null;
                if (method == nameof(ConfirmTrashScreen.OnPressConfirmBtn) && confirm == null && labelled)
                {
                    confirm = b;
                    b.onClick.AddListener(() => Choose(true));
                }
                else if (method == nameof(ConfirmTrashScreen.OnPressCancelBtn) && cancel == null && labelled)
                {
                    cancel = b;
                    b.onClick.AddListener(() => Choose(false));
                }
                else b.onClick.AddListener(Close);
            }
            if (confirm == null || cancel == null)
            {
                Plugin.Log.LogWarning($"ChoicePopup: confirm screen layout not recognised (confirm={confirm != null}, cancel={cancel != null})");
                Object.Destroy(go);
                _screen = null;
                return false;
            }
            _labelA = confirm.GetComponentInChildren<TextMeshProUGUI>(true);
            _labelB = cancel.GetComponentInChildren<TextMeshProUGUI>(true);

            // Message = the longest text outside the buttons; other free texts (headers) are cleared.
            var free = go.GetComponentsInChildren<TextMeshProUGUI>(true).Where(t => t.GetComponentInParent<Button>() == null).ToList();
            _message = free.OrderByDescending(t => (t.text ?? "").Length).FirstOrDefault();
            if (_message == null)
            {
                Plugin.Log.LogWarning("ChoicePopup: no message text in confirm screen");
                Object.Destroy(go);
                _screen = null;
                return false;
            }
            foreach (var t in free) if (t != _message) t.text = "";
            Plugin.Log.LogInfo($"ChoicePopup built from {src.name} (texts: {string.Join(" | ", free.Select(t => t.name))})");
            return true;
        }

        private static string PersistentMethod(UnityEvent e) =>
            e.GetPersistentEventCount() > 0 ? e.GetPersistentMethodName(0) : null;
    }

    /// <summary>Short on-screen message using the game's "not enough money"-style popup texts.</summary>
    internal static class Toast
    {
        public static void Show(string text)
        {
            Plugin.Log.LogInfo($"Toast: {text}");
            var p = CSingleton<NotEnoughResourceTextPopup>.Instance;
            if (p == null || p.m_ShowTextGameObjectList == null) return;
            for (int i = 0; i < p.m_ShowTextGameObjectList.Count && i < p.m_ShowTextList.Count; i++)
            {
                if (p.m_ShowTextGameObjectList[i].activeSelf) continue;
                p.m_ShowTextList[i].text = text;
                p.m_ShowTextGameObjectList[i].SetActive(true);
                return;
            }
        }
    }
}

namespace TCGCustomCards.UI
{
    /// <summary>IMGUI fallback for <see cref="ChoicePopup"/> (only used if the cloned game screen can't be built).</summary>
    internal class ImGuiChoice : MonoBehaviour
    {
        private static ImGuiChoice _inst;
        private string _message, _a, _b;
        private Action _onA, _onB;

        public static bool IsOpen => _inst != null && _inst.enabled;

        public static void Show(string message, string a, Action onA, string b, Action onB)
        {
            if (_inst == null)
            {
                var go = new GameObject("TCGCC_ImGuiChoice");
                DontDestroyOnLoad(go);
                _inst = go.AddComponent<ImGuiChoice>();
            }
            _inst._message = message; _inst._a = a; _inst._b = b; _inst._onA = onA; _inst._onB = onB;
            _inst.enabled = true;
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            ipc?.m_WalkerCtrl?.SetStopMovement(isStop: true);
            ipc?.EnterUIMode();
        }

        private void Pick(Action act)
        {
            enabled = false;
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            ipc?.m_WalkerCtrl?.SetStopMovement(isStop: false);
            ipc?.ExitUIMode();
            act?.Invoke();
        }

        private void OnGUI()
        {
            var r = new Rect(Screen.width / 2f - 220, Screen.height / 2f - 80, 440, 160);
            GUI.Box(r, "");
            GUI.Label(new Rect(r.x + 20, r.y + 20, r.width - 40, 60), _message);
            if (GUI.Button(new Rect(r.x + 20, r.y + 100, 190, 40), _a)) Pick(_onA);
            if (GUI.Button(new Rect(r.x + 230, r.y + 100, 190, 40), _b)) Pick(_onB);
            if (Event.current.type == EventType.KeyDown && Event.current.keyCode == KeyCode.Escape) Pick(null);
        }
    }
}
