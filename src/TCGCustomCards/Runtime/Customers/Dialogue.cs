using System.Collections;
using System.Collections.Generic;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] Dialogue: customers say lines (Traits.LinesFor) in vanilla's speech popup. Other modules call
    /// <see cref="Say"/> for their own situations; this module covers arriving, prices and leaving. No saved state.
    /// </summary>
    internal class Dialogue : IPersonaModule
    {
        public string Id => "dialogue";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerDialogue.Value;

        /// <summary>Per visit: when this customer may talk again (cooldown).</summary>
        private class Talk
        {
            public float NextAllowed;
            public List<Line> Later;
        }

        private class Line
        {
            public float At;
            public string Text;
            public bool Important;
            public string Mood;
        }
        private const float Cooldown = 6f;

        /// <summary>Seconds between two lines of a back-and-forth: the first has gone (LineSeconds) before the next shows.</summary>
        public static float Gap => Plugin.CustomerLineSeconds.Value + 0.3f;

        public class Tokens
        {
            public string Card, Rival, Other;
            /// <summary>{nick}: a nickname. In gossip it travels as the news item's card.</summary>
            public string Nick;
            /// <summary>Who a line is about when it isn't the listener ({about}: gossip about a third customer).</summary>
            public string About;
            /// <summary>A second card ({card2}: what they got for {card}) and when something happened ({when}: "yesterday").</summary>
            public string Card2, When;
            public float Price;
        }

        /// <summary>
        /// Says a line of <paramref name="situation"/> with probability <paramref name="chance"/> × the persona's talk × Chattiness,
        /// after <paramref name="delay"/> seconds, if it is still the same visit then. Important lines (chance ≥ 1) skip the cooldown.
        /// </summary>
        public static void Say(Customer c, string situation, Tokens tokens = null, float chance = 1f, float delay = 0f)
        {
            var p = c != null ? Cast.Of(c) : null;
            if (p == null) return;
            bool important = chance >= 1f;
            if (!important && Random.value >= chance * Traits.Effect(p, "talk") * Plugin.CustomerChattiness.Value) return;
            // The animation goes with the line; only an important one breaks off what they're playing, and not when that's nearly done.
            Moves.Play(c, situation, delay, important, Moves.ImportantWait);
            if (!On) return;
            var lines = Traits.LinesFor(p, situation);
            if (lines == null) return;
            string text = Fill(lines[Random.Range(0, lines.Count)], p, tokens);
            string mood = Traits.MoodOf(situation);
            if (delay <= 0f) { Show(c, p, text, important, mood); return; }
            var talk = Cast.VisitData<Talk>(c);
            if (talk == null) return;
            if (talk.Later == null) talk.Later = new List<Line>();
            talk.Later.Add(new Line { At = Time.time + delay, Text = text, Important = important, Mood = mood });
        }

        /// <summary>Delayed lines whose time has come (slow tick); they belong to the visit, so they die with it.</summary>
        private static void SayDue(Customer c)
        {
            var later = Cast.PeekVisitData<Talk>(c)?.Later;
            if (later == null || later.Count == 0) return;
            var p = Cast.Of(c);
            float now = Time.time;
            for (int i = later.Count - 1; i >= 0; i--)
            {
                if (later[i].At > now) continue;
                var line = later[i];
                later.RemoveAt(i);
                if (p != null) Show(c, p, line.Text, line.Important, line.Mood);
            }
        }

        private static void Show(Customer c, Persona p, string text, bool important, string mood)
        {
            float now = Time.time;
            var talk = Cast.VisitData<Talk>(c);
            if (talk == null || (!important && now < talk.NextAllowed)) return;
            talk.NextAllowed = now + Cooldown;
            UI.WorldText.Show(c.transform, text, 1.8f - SitDrop(c), Plugin.CustomerLineSeconds.Value, Plugin.CustomerLineTextSize.Value);
            TalkSounds.Say(c, p, text, mood);
        }

        /// <summary>
        /// Text over a customer that isn't a line (a card's value): <paramref name="offsetUp"/> metres above their feet,
        /// shown <paramref name="seconds"/>, in <paramref name="colour"/> (default white). Plain text, no colour tags.
        /// Not subject to the talk cooldown.
        /// </summary>
        public static void Popup(Customer c, string text, float offsetUp, float seconds, Color? colour = null)
        {
            if (c != null) UI.WorldText.Show(c.transform, text, offsetUp - SitDrop(c), seconds, 1f, colour);
        }

        /// <summary>How much lower a customer's head is while they sit (the Animator's parameters still count on the GPU path).</summary>
        public static float SitDrop(Customer c) => c != null && c.m_Anim != null && c.m_Anim.GetBool("IsSitting") ? 0.6f : 0f;

        public static string Fill(string line, Persona p, Tokens t)
        {
            // A line about the nickname itself ("{name} is {nick}") needs their real name beside it.
            bool byName = line.Contains("{nick}");
            string Said(string who) => byName ? who : Nicknames.Spoken(p, who);
            return line.Replace("{name}", t?.Other != null ? Said(t.Other) : p.name)
                       .Replace("{nick}", t?.Nick ?? t?.Card ?? "that")
                       .Replace("{shop}", Cast.ShopName)
                       .Replace("{card}", t?.Card ?? "that card")
                       .Replace("{rival}", t?.Rival != null ? Said(t.Rival) : "you")
                       .Replace("{about}", t?.About != null ? Said(t.About) : "someone")
                       .Replace("{card2}", t?.Card2 ?? "some card")
                       .Replace("{when}", t?.When ?? "the other day")
                       .Replace("{price}", t == null || t.Price <= 0f ? "that" : "$" + t.Price.ToString("0.##"));
        }

        public void Init()
        {
            UI.CustomerDebugPanel.Section("Talk", (c, p, rows) =>
            {
                if (!On) return;
                var talk = Cast.PeekVisitData<Talk>(c);
                float wait = talk == null ? 0f : talk.NextAllowed - Time.time;
                rows.Add(("next line", wait > 0f ? $"in {wait:0} s (cooldown)" : "any time"));
                if (talk?.Later != null && talk.Later.Count > 0) rows.Add(("queued", $"{talk.Later.Count} line(s)"));
            });
            CustomerLife.SlowTick.Add(SayDue);
            Cast.EnteredShop += (c, p) =>
            {
                if (!On) return;
                string situation = "enter";
                if (p.regular && p.visits > 1)
                {
                    var tier = Loyalty.TierOf(p);
                    situation = tier?.greeting ?? "greet";
                }
                Say(c, situation, null, 0.45f, Random.Range(0.5f, 2f));
            };
            CustomerLife.PriceJudged += (ctx, chance) =>
            {
                if (!On) return;
                var tokens = new Tokens { Price = ctx.Price, Card = ctx.IsCard ? CardInfo.Name(ctx.Card) : null };
                if (chance >= 97) Say(ctx.Customer, "cheap", tokens, 0.08f);
                else if (chance < 40) Say(ctx.Customer, "pricey", tokens, 0.12f);
            };
            CustomerLife.ExitDecided += c =>
            {
                if (!On || Shoplifting.IsStealing(c) || CustomerStates.Of(c) != null) return; // staying a while says goodbye later
                if (!CustomerLife.IsInsideShop(c)) return; // gave up outside a closed shop: no "nothing for me" goodbye
                Say(c, CustomerLife.PaidThisVisit(c) > 0f ? "leaveHappy" : "leaveEmpty", null, 0.25f, 0.5f);
            };
        }

        public void Load(JToken state) { }
        public JToken Save(ICollection<string> keep) => null;
        public string Summary(Persona p) => null;
    }
}
