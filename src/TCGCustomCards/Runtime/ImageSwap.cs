using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// One vanilla UI Image whose sprite a Game look picture replaces (phone wallpaper, loading screen, main menu): remembers the
    /// vanilla sprite, colour, aspect and rect, and puts them back when the picture is cleared. A covering image (a full-screen
    /// background) keeps the picture's shape and fills the screen with it, cutting off what doesn't fit (AspectRatioFitter
    /// EnvelopeParent), like a wallpaper on a monitor of another shape.
    /// </summary>
    internal sealed class ImageSwap
    {
        private Image _image;
        private Sprite _sprite;
        private Color _color;
        private bool _preserveAspect;
        private Vector2 _anchorMin, _anchorMax, _sizeDelta, _position;
        private AspectRatioFitter _fitter;
        private readonly bool _white;

        /// <param name="white">Show pictures untinted (the loading screen's vanilla background is a black tint without a sprite);
        /// otherwise the vanilla colour stays.</param>
        internal ImageSwap(bool white = false) { _white = white; }

        /// <summary>The image is known and not destroyed (scene reloads destroy it).</summary>
        internal bool Valid => _image != null;

        /// <summary>Starts managing <paramref name="image"/> (no change when it already is the managed one).</summary>
        internal void Capture(Image image)
        {
            if (image == null || image == _image) return;
            _image = image;
            _sprite = image.sprite;
            _color = image.color;
            _preserveAspect = image.preserveAspect;
            var rt = image.rectTransform;
            _anchorMin = rt.anchorMin;
            _anchorMax = rt.anchorMax;
            _sizeDelta = rt.sizeDelta;
            _position = rt.anchoredPosition;
            _fitter = null;
        }

        /// <summary>Shows <paramref name="sprite"/> (null = vanilla). <paramref name="coverAspect"/> &gt; 0: the picture's width ÷
        /// height, kept while it covers the image's parent; 0 = stretched over the image's own rect.</summary>
        internal void Show(Sprite sprite, float coverAspect = 0)
        {
            if (_image == null) return;
            if (sprite == null)
            {
                _image.sprite = _sprite;
                _image.color = _color;
                _image.preserveAspect = _preserveAspect;
                if (_fitter != null && _fitter.enabled)
                {
                    _fitter.enabled = false;
                    var rt = _image.rectTransform;
                    rt.anchorMin = _anchorMin;
                    rt.anchorMax = _anchorMax;
                    rt.sizeDelta = _sizeDelta;
                    rt.anchoredPosition = _position;
                }
                return;
            }
            _image.sprite = sprite;
            if (_white) _image.color = Color.white;
            _image.preserveAspect = false;
            if (coverAspect <= 0) return;
            if (_fitter == null) _fitter = _image.GetComponent<AspectRatioFitter>() ?? _image.gameObject.AddComponent<AspectRatioFitter>();
            _fitter.aspectMode = AspectRatioFitter.AspectMode.EnvelopeParent;
            _fitter.aspectRatio = coverAspect;
            _fitter.enabled = true;
        }
    }
}
