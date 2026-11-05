using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Hooks;
using TCGCustomCards.Runtime.Customers;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// The panel at the right of the screen about the customer or employee you look at: portrait, name, what kind of person they are, what's known about them.
    /// The right, because the game's task panel and control hints have the left. Its look comes from <see cref="GameStyle"/>.
    /// </summary>
    internal static class CustomerCard
    {
        private const float Width = 430f, Margin = 8f, PortraitSize = 96f;
        private const float CardWidth = 72f, CardHeight = 100f;
        private const float FadeSeconds = 0.15f, RefreshSeconds = 1f;
        private const int MaxReviews = 2;

        private static CanvasGroup _group;
        private static GameStyle.GamePanel _panel;
        private static Image _portrait;
        private static TextMeshProUGUI _name, _kind, _perks, _body, _cardText, _footer;
        private static RawImage _card;
        private static GameStyle.GameBar _patience, _loyalty;
        private static object _shown;
        private static string _footerText;
        private static float _refreshAt;

        /// <summary>Shows this person (null: fades out). Call every frame.</summary>
        public static void Show(Persona p, Customer c, string footer)
        {
            if (Present(p, () => Fill(p, c), footer)) return;
            if (p != null && Impatience.PatienceLeft(c, p) is float left) _patience.Set(left);
        }

        /// <summary>Shows this employee. Call every frame.</summary>
        public static void ShowEmployee(Worker w, string footer) => Present(w, () => FillEmployee(w), footer);

        /// <summary>True when there's nothing to show.</summary>
        private static bool Present(object who, System.Action fill, string footer)
        {
            if (who != null && _group == null) Build();
            if (_group == null) return true;
            _group.alpha = Mathf.MoveTowards(_group.alpha, who != null ? 1f : 0f, Time.unscaledDeltaTime / FadeSeconds);
            if (who == null) return true;
            bool changed = false;
            if (who != _shown || Time.unscaledTime >= _refreshAt)
            {
                _shown = who;
                _refreshAt = Time.unscaledTime + RefreshSeconds;
                fill();
                changed = true;
            }
            if (footer != _footerText)
            {
                _footerText = footer;
                _footer.text = footer ?? "";
                _footer.gameObject.SetActive(!string.IsNullOrEmpty(footer));
                changed = true;
            }
            if (changed) _panel.Fit();
            return false;
        }

        /// <summary>The panel on screen (IMGUI's coordinates: y down); empty while it isn't.</summary>
        public static Rect ScreenRect() => _panel == null || _group.alpha <= 0f ? Rect.zero : GameStyle.ScreenRect(_panel.Root);

        private static void Fill(Persona p, Customer c)
        {
            string nick = Nicknames.Of(p);
            _name.text = p.name + (nick != null ? $"\n<size=70%>\"{nick}\"</size>" : "");
            _kind.text = !p.regular ? "Walk-in" : $"Regular · {p.visits} visit(s) · ${p.spent:0} spent";
            if (c != null) _portrait.sprite = CSingleton<CustomerManager>.Instance?.GetCustomerIconSprite(c.GetCustomerModelIndex(), c.m_IsFemale);
            _portrait.enabled = _portrait.sprite != null;
            _perks.text = "<b>Perks:</b> " + (p.traits.Count == 0 ? "none" : string.Join(", ", p.traits.Select(Traits.NameOf)));
            FillCard(p);
            _patience.Visible = Impatience.PatienceLeft(c, p) != null;
            var tier = Loyalty.TierOf(p);
            _loyalty.Visible = tier != null;
            if (tier != null)
            {
                _loyalty.Label = "Loyalty: " + tier.name;
                _loyalty.SetProgress(Loyalty.Progress(p), Traits.LoyaltyTiers.IndexOf(tier) == 0);
            }
            var lines = new List<string>(CustomerNameplates.Summaries(p));
            foreach (var (stars, day, text) in Reviews.Of(p).Take(MaxReviews)) lines.Add($"<b>{stars}/5</b>, day {day}: {text}");
            _body.text = string.Join("\n", lines);
            _body.gameObject.SetActive(lines.Count > 0);
        }

        private static void FillEmployee(Worker w)
        {
            var data = w.GetWorkerData();
            var custom = Runtime.Employees.Roster.Of(w);
            _name.text = data.GetName();
            _kind.text = "Employee · " + TaskWords(WorkerLife.TaskOf(w));
            _portrait.sprite = data.icon;
            _portrait.enabled = _portrait.sprite != null;
            _perks.text = custom == null ? "" : "<b>Perks:</b> " + (custom.Def.Perks == null || custom.Def.Perks.Count == 0 ? "none" : string.Join(", ", custom.Def.Perks.Select(id => Traits.EmployeePerk(id)?.name ?? id)));
            _card.gameObject.SetActive(false);
            _cardText.text = "";
            _patience.Visible = false;
            _loyalty.Visible = false;
            _body.text = $"Checkout: {data.GetCheckoutSpeedText()}\nRestock: {data.GetRestockSpeedText()}\nWage: {data.GetSalaryCostText()}";
            _body.gameObject.SetActive(true);
        }

        private static string TaskWords(EWorkerTask task)
        {
            switch (task)
            {
                case EWorkerTask.ManCounter: return "on the till";
                case EWorkerTask.RestockShelf: return "restocking shelves";
                case EWorkerTask.SetPrice: return "setting prices";
                case EWorkerTask.RefillCleanser: return "refilling cleanser";
                case EWorkerTask.RefillCardOpener: return "refilling the pack opener";
                case EWorkerTask.RestockCardDisplay: return "restocking card displays";
                case EWorkerTask.GoBackHome: return "going home";
                default: return "resting";
            }
        }

        /// <summary>Their prized card, as a picture; it may take a moment to be drawn, the next refresh picks it up.</summary>
        private static void FillCard(Persona p)
        {
            var best = Trading.BestCard(p);
            var picture = best == null ? null : CardRenderCache.Get(best);
            _card.texture = picture;
            _card.gameObject.SetActive(picture != null);
            _cardText.text = best == null ? "" : $"<b>Prized card</b>\n{CardInfo.Name(best)}\n{CardBubble.Value(CardInfo.Price(best))}";
        }

        private static void Build()
        {
            var canvas = GameStyle.Canvas("TCGCC_CustomerCard");
            _group = canvas.AddComponent<CanvasGroup>();
            _group.alpha = 0f;
            _group.blocksRaycasts = false;
            _group.interactable = false;
            _panel = GameStyle.Panel(canvas.transform, "Panel", Width);
            _panel.Root.anchorMin = _panel.Root.anchorMax = _panel.Root.pivot = new Vector2(1f, 0.5f);
            _panel.Root.anchoredPosition = new Vector2(-Margin, 0f);
            _portrait = _panel.PictureRow(PortraitSize);
            _name = _panel.TextBeside(GameStyle.TextKind.Title);
            _kind = _panel.TextBeside(GameStyle.TextKind.Small);
            _perks = _panel.TextRow(GameStyle.TextKind.Body);
            _patience = _panel.BarRow("Patience", GameStyle.TextKind.Small);
            _loyalty = _panel.BarRow("Loyalty", GameStyle.TextKind.Small);
            _card = _panel.TextureRow(CardWidth, CardHeight);
            _cardText = _panel.TextBeside(GameStyle.TextKind.Body);
            _body = _panel.TextRow(GameStyle.TextKind.Body);
            _footer = _panel.TextRow(GameStyle.TextKind.Small);
            _footer.gameObject.SetActive(false);
            _card.gameObject.SetActive(false);
            _footerText = null;
            _shown = null;
        }
    }
}
