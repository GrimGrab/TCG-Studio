using TCGCustomCards.Patches;
using UnityEngine;
using Object = UnityEngine.Object;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// Two extra hints in the game's button-hint bar while browsing the enhanced binder: "Switch version" (mouse wheel or
    /// the controller button) and "Favourite" (key or button; also in the close-up). They are clones of one of the bar's
    /// own pooled hints, kept outside its pool, so the game's clear/show calls never touch them; we show, hide and order
    /// them ourselves every frame from the binder's Update.
    /// </summary>
    internal static class BinderTooltips
    {
        private static InputTooltipListDisplay _display;
        private static InputTooltipUI _version, _favourite;
        private static string _shown;

        public static void Update(CollectionBinderFlipAnimCtrl ctrl)
        {
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            var display = ipc != null ? ipc.m_InputTooltipListDisplay : null;
            if (display == null) return;
            bool on = Plugin.EnhancedBinder.Value && ipc.m_CurrentGameState == EGameState.ViewAlbumState &&
                      CSingleton<CGameManager>.Instance.m_EnableTooltip && EnhancedBinderPatches.Active(ctrl) &&
                      !(ctrl.m_CollectionBinderUI != null && ctrl.m_CollectionBinderUI.IsSortAlbumScreenOpen());
            bool pad = PadInput.Active;
            var versionBtn = Plugin.BinderPadVersion.Value;
            var favBtn = Plugin.BinderPadFavourite.Value;
            var favKey = Plugin.BinderFavouriteKey.Value.MainKey;
            bool showVersion = on && !Binder.BinderIndex.Separate && !ctrl.m_IsHoldingCardCloseUp && (!pad || versionBtn != BinderPadButton.None);
            bool showFav = on && (pad ? favBtn != BinderPadButton.None : favKey != KeyCode.None);

            if (display != _display || _version == null || _favourite == null)
            {
                if (!showVersion && !showFav) return; // nothing to show yet: make them when needed
                if (!Make(display)) return;
            }
            string sig = $"{showVersion}|{showFav}|{pad}|{PadInput.IsPS}|{versionBtn}|{favBtn}|{favKey}";
            if (sig != _shown)
            {
                _shown = sig;
                if (pad)
                {
                    _version.SetGamepadInputTooltip(EGameAction.None, PadInput.ToGame(versionBtn), "Switch version", false);
                    _favourite.SetGamepadInputTooltip(EGameAction.None, PadInput.ToGame(favBtn), "Favourite", false);
                }
                else
                {
                    _version.SetInputTooltip(EGameAction.None, KeyCode.Mouse2, "Switch version", false); // the wheel picture
                    _favourite.SetInputTooltip(EGameAction.None, favKey, "Favourite", false);
                }
                _version.SetActive(showVersion);
                _favourite.SetActive(showFav);
            }
            // The game re-adds its own hints to the bar when they change: keep ours at the end.
            var parent = display.m_ActiveLayoutParent;
            foreach (var t in new[] { _version, _favourite })
                if (t.gameObject.activeSelf && t.transform.GetSiblingIndex() < parent.childCount - 2) t.transform.SetAsLastSibling();
        }

        private static bool Make(InputTooltipListDisplay display)
        {
            if (display.m_InputTooltipUIPool == null || display.m_InputTooltipUIPool.Count == 0 || display.m_ActiveLayoutParent == null) return false;
            var template = display.m_InputTooltipUIPool[0];
            _display = display;
            _shown = null;
            _version = Clone(template, display.m_ActiveLayoutParent, "TCGCC_Tooltip_Version");
            _favourite = Clone(template, display.m_ActiveLayoutParent, "TCGCC_Tooltip_Favourite");
            return _version != null && _favourite != null;
        }

        private static InputTooltipUI Clone(InputTooltipUI template, Transform parent, string name)
        {
            var go = Object.Instantiate(template.gameObject, parent, false);
            go.name = name;
            var t = go.GetComponent<InputTooltipUI>();
            t.m_Transform = go.transform;
            t.SetActive(false);
            return t;
        }
    }
}
