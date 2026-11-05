using System;
using System.Collections.Generic;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] Chatting: when deciding what to do next (CustomerChoices), a customer may walk up to a free one nearby; they face each
    /// other and trade a few lines — hello, then gossip about today's news (<see cref="News"/>: pulls, thefts, kicks, trades,
    /// rip-offs) or small talk, then goodbye — and both go back to shopping. Gossips, Hype and Tire-kickers start chats most
    /// (trait effect "chat", a multiplier). Other modules join in: <see cref="Exchange"/> (a trade mid-chat), <see cref="Avoid"/>
    /// (who won't talk to whom). No saved state.
    /// </summary>
    internal class Chatting : IPersonaModule
    {
        public string Id => "chatting";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerChatting.Value;

        private const float ChatWeight = 0.08f;     // × their "chat" effect, against vanilla's 1 at each decision
        private const int MaxChatsPerVisit = 2;
        private const int MaxChatsAtOnce = 3;
        private const float Reach = 10f;            // metres to a chat partner
        private const float ChatDistance = 0.8f;    // metres between two customers chatting

        /// <summary>Something to do before goodbye (a trade): returns the seconds its own lines take (0 = nothing said).</summary>
        public static Func<Customer, Customer, float> Exchange;

        /// <summary>True = these two don't talk (a grudge).</summary>
        public static readonly List<Func<Persona, Persona, bool>> Avoid = new List<Func<Persona, Persona, bool>>();

        private class Chatty
        {
            public int Chats;
            public float NextTry;
        }

        private class Step
        {
            public bool ByA;
            public string Situation;
            public Dialogue.Tokens Tokens;
        }

        private class Chat
        {
            public Customer A, B; // A walked up to B
            public Persona PA, PB;
            public List<Step> Script;
            public int Next;
            public float NextAt;
            public bool Started, Exchanged, Ended;
            public string Topic;
        }

        private class Role
        {
            public Chat Chat;
            public bool Talker;
        }

        public void Init()
        {
            CustomerStates.Register(new ChatState());
            CustomerChoices.Register("chat", Weight, TryStart);
            UI.CustomerDebugPanel.Section("Chatting", (c, p, rows) =>
            {
                if (!On) return;
                var role = CustomerStates.Of(c)?.Data as Role;
                if (role != null)
                    rows.Add(("now", $"chatting with {(role.Talker ? role.Chat.PB : role.Chat.PA).name}" + (role.Chat.Topic != null ? $" about {role.Chat.Topic}" : "")));
                var ch = Cast.PeekVisitData<Chatty>(c);
                rows.Add(("chats", $"{ch?.Chats ?? 0} of {MaxChatsPerVisit} this visit · chance ×{Traits.Effect(p, "chat"):0.#}"));
            });
        }

        private static float Weight(Customer c)
        {
            var p = On ? Cast.Of(c) : null;
            var me = Cast.PeekVisitData<Chatty>(c);
            if (p == null || (me != null && (me.Chats >= MaxChatsPerVisit || Time.time < me.NextTry)) || ChatsNow() >= MaxChatsAtOnce) return 0f;
            return ChatWeight * Traits.Effect(p, "chat");
        }

        /// <summary>A chat with the nearest free customer (not one they avoid); false = nobody around.</summary>
        private static bool TryStart(Customer c)
        {
            var p = Cast.Of(c);
            var me = Cast.VisitData<Chatty>(c);
            if (p == null || me == null) return false;
            me.NextTry = Time.time + 20f;
            Customer best = null;
            float bestDist = Reach;
            foreach (var v in Cast.Visitors())
            {
                if (v.Key == c || !CustomerLife.IsFree(v.Key)) continue;
                if ((Cast.PeekVisitData<Chatty>(v.Key)?.Chats ?? 0) >= MaxChatsPerVisit) continue;
                if (Avoids(p, v.Value)) continue;
                float d = Vector3.Distance(c.transform.position, v.Key.transform.position);
                if (d < bestDist) { bestDist = d; best = v.Key; }
            }
            if (best == null) return false;
            Start(c, best);
            return true;
        }

        private static bool Avoids(Persona a, Persona b)
        {
            foreach (var f in Avoid) if (f(a, b) || f(b, a)) return true;
            return false;
        }

        private static int ChatsNow()
        {
            int n = 0;
            foreach (var v in Cast.Visitors()) if (CustomerStates.Of(v.Key)?.Data is Role r && r.Talker) n++;
            return n;
        }

        /// <summary>A chat that ran to its goodbye: (talker, listener). Friendships grow from these.</summary>
        public static event Action<Customer, Customer> ChatFinished;

        /// <summary>
        /// Starts a chat between two free customers now (another module's choice: friends meeting up). <paramref name="open"/>
        /// replaces the hello's situation ("friendHello"). False = one of them isn't free.
        /// </summary>
        public static bool StartWith(Customer a, Customer b, string open = null)
        {
            if (!On || a == null || b == null || Cast.Of(a) == null || Cast.Of(b) == null || !CustomerLife.IsFree(b)) return false;
            Start(a, b, open);
            return true;
        }

        private static void Start(Customer a, Customer b, string open = null)
        {
            var chat = new Chat { A = a, B = b, PA = Cast.Of(a), PB = Cast.Of(b) };
            chat.Script = Script(chat);
            if (open != null) chat.Script[0].Situation = open;
            var ca = Cast.VisitData<Chatty>(a);
            var cb = Cast.VisitData<Chatty>(b);
            if (ca != null) ca.Chats++;
            if (cb != null) cb.Chats++;
            CustomerStates.Goto(b, ChatState.StateId, new Role { Chat = chat, Talker = false });
            CustomerStates.Goto(a, ChatState.StateId, new Role { Chat = chat, Talker = true });
        }

        private const float AnotherExchange = 0.45f; // chance of going on to one more topic, tried twice

        // Small talk: what's said, and the replies written for it.
        private static readonly KeyValuePair<string, string>[] Topics =
        {
            new KeyValuePair<string, string>("chatSmall", "chatSmallReply"),
            new KeyValuePair<string, string>("chatAsk", "chatAskReply"),
            new KeyValuePair<string, string>("chatMoan", "chatMoanReply"),
            new KeyValuePair<string, string>("chatBrag", "chatBragReply"),
            new KeyValuePair<string, string>("chatAskDeck", "chatDeckReply"),
            new KeyValuePair<string, string>("chatAskPull", "chatPullReply"),
        };

        /// <summary>Hello, reply, a topic (gossip when one of them is a Gossip or by chance, else small talk), a reaction, goodbye.</summary>
        private static List<Step> Script(Chat chat)
        {
            var s = new List<Step>
            {
                new Step { ByA = true, Situation = "chatOpen", Tokens = new Dialogue.Tokens { Other = chat.PB.name } },
                new Step { ByA = false, Situation = "chatReply", Tokens = new Dialogue.Tokens { Other = chat.PA.name } },
            };
            bool aGossip = chat.PA.traits.Contains("gossip"), bGossip = chat.PB.traits.Contains("gossip");
            var news = (aGossip || bGossip || UnityEngine.Random.value < 0.4f) ? News.Pick(chat.PA.name, chat.PB.name) : null;
            bool teller = !(bGossip && !aGossip); // the gossip tells it
            if (news != null)
            {
                if (teller) s[0].Situation = "chatOpenNews"; // only someone with news says they have some
                bool nice = news.k == "pull" || news.k == "friends" || news.k == "trade" || news.k == "nick" || news.k == "nickTaken";
                chat.Topic = $"{news.w}'s {news.k} ({News.When(news)})";
                s.Add(new Step
                {
                    ByA = teller, Situation = "gossip" + char.ToUpperInvariant(news.k[0]) + news.k.Substring(1),
                    Tokens = new Dialogue.Tokens { About = news.w, Rival = news.o, Card = news.c, Card2 = news.c2, Price = news.p, When = News.When(news) },
                });
                s.Add(new Step { ByA = !teller, Situation = nice ? "chatReactNice" : "chatReact", Tokens = new Dialogue.Tokens { About = news.w, When = News.When(news) } });
            }
            // Matched pairs, so that an answer fits what was said: each topic has its own replies. They take turns starting one.
            int start = UnityEngine.Random.Range(0, Topics.Length);
            int step = UnityEngine.Random.value < 0.5f ? 1 : 5; // either way through the list: both reach every topic, so none repeats
            int topics = news != null ? 0 : 1;
            if (UnityEngine.Random.value < AnotherExchange) topics++;
            if (topics > 0 && UnityEngine.Random.value < AnotherExchange) topics++;
            for (int i = 0; i < topics; i++)
            {
                var topic = Topics[(start + i * step) % Topics.Length];
                bool byA = i % 2 == 0;
                s.Add(new Step { ByA = byA, Situation = topic.Key, Tokens = new Dialogue.Tokens { Other = byA ? chat.PB.name : chat.PA.name } });
                s.Add(new Step { ByA = !byA, Situation = topic.Value, Tokens = new Dialogue.Tokens { Other = byA ? chat.PA.name : chat.PB.name } });
            }
            s.Add(new Step { ByA = true, Situation = "chatBye", Tokens = new Dialogue.Tokens { Other = chat.PB.name } });
            return s;
        }

        private static void End(Chat chat)
        {
            if (chat.Ended) return;
            chat.Ended = true;
            if (chat.Next >= chat.Script.Count)
            {
                try { ChatFinished?.Invoke(chat.A, chat.B); }
                catch (Exception e) { Plugin.Log.LogError($"Customers: chat finished handler failed: {e.Message}"); }
            }
            foreach (var x in new[] { chat.A, chat.B })
                if (x != null && CustomerStates.Of(x)?.Data is Role r && r.Chat == chat) CustomerStates.ToVanilla(x, CustomerLife.Shop);
        }

        /// <summary>The talker walks up and drives the lines; the listener stands and faces them. Either leaving ends it for both.</summary>
        private class ChatState : ModState
        {
            public const string StateId = "chatting.chat";
            public override string Id => StateId;

            public override void Enter(Customer c, StateContext ctx)
            {
                var role = (Role)ctx.Data;
                if (!role.Talker)
                {
                    CustomerLife.Stand(c);
                    CustomerLife.FaceTowards(c, role.Chat.A.transform.position);
                    return;
                }
                var to = role.Chat.B.transform.position;
                var dir = c.transform.position - to;
                dir.y = 0f;
                dir = dir.sqrMagnitude < 0.01f ? Vector3.forward : dir.normalized;
                ctx.WalkTo(to + dir * ChatDistance);
            }

            public override void Tick(Customer c, StateContext ctx)
            {
                var role = (Role)ctx.Data;
                var chat = role.Chat;
                if (chat.Ended) return;
                var other = role.Talker ? chat.B : chat.A;
                if (other == null || !other.m_IsActive || !(CustomerStates.Of(other)?.Data is Role r) || r.Chat != chat)
                {
                    End(chat);
                    return;
                }
                if (!role.Talker)
                {
                    CustomerLife.FaceTowards(c, other.transform.position);
                    if (ctx.Elapsed > 150f) End(chat);
                    return;
                }
                if (!chat.Started)
                {
                    if (!ctx.HasArrived || !CustomerLife.StepCloser(c, other.transform.position, ChatDistance))
                    {
                        if (ctx.Elapsed > 15f) End(chat);
                        return;
                    }
                    chat.Started = true;
                    chat.NextAt = Time.time + 0.3f;
                }
                CustomerLife.FaceTowards(c, other.transform.position);
                if (Time.time < chat.NextAt) return;
                if (chat.Next < chat.Script.Count)
                {
                    // The exchange (a trade) gets its own slot before goodbye, after the previous line has gone.
                    if (chat.Next == chat.Script.Count - 1 && !chat.Exchanged)
                    {
                        chat.Exchanged = true;
                        float took = Exchange?.Invoke(chat.A, chat.B) ?? 0f;
                        if (took > 0f)
                        {
                            chat.NextAt = Time.time + took;
                            return;
                        }
                    }
                    var step = chat.Script[chat.Next++];
                    Dialogue.Say(step.ByA ? chat.A : chat.B, step.Situation, step.Tokens, 1f);
                    chat.NextAt = Time.time + Dialogue.Gap;
                    return;
                }
                End(chat);
            }

            public override void Exit(Customer c, StateContext ctx)
            {
                var chat = ((Role)ctx.Data).Chat;
                if (!chat.Ended) End(chat); // interrupted (kicked, visit over): the other one goes back too
            }
        }

        public void Load(JToken state) { }
        public JToken Save(ICollection<string> keep) => null;
        public string Summary(Persona p) => null;
    }
}
