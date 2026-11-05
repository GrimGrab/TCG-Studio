using System.Collections.Generic;
using System.Linq;
using HarmonyLib;
using TCGCustomCards.Hooks;
using TCGCustomCards.Runtime.Customers;
using UnityEngine;

namespace TCGCustomCards.Runtime.Employees
{
    /// <summary>
    /// [Customers - Employees]: what employees say (the game's and yours, text over them and a voice) and what perks do (defaults.json employeePerks, yours
    /// only). Lines add up like customers': built-in − the setup's switched off + the setup's + the perks' − their own switched off + their own.
    /// Effects: multipliers work, morning, evening (1 = no change); chances chat, slack, fumble, thief, gossip, upsell; amounts tip (chance), loyalty (per checkout), exp, early; onTime.
    /// </summary>
    internal static class EmployeeLife
    {
        private const float Cooldown = 6f;
        private const float ChatSeconds = 5f, SlackMin = 8f, SlackMax = 14f, FumbleSeconds = 3f;
        private const float CalledOutRange = 20f;
        private const int EveningHour = 14;
        private const float RushSeconds = 12f;     // Early bird: long enough to get from the street to their spot
        private const float BounceEvery = 30f, BounceRange = 10f;
        private const float SmallTalk = 0.15f; // anyone at the till, without holding up the line
        /// <summary>What an employee can bring up at the till, each with the customer lines that answer it, so the two always go together.</summary>
        private static readonly (string say, string reply)[] Banter =
        {
            ("empTillChat", "tillChatReply"), ("empChatMoan", "chatMoanReply"), ("empChatBrag", "chatBragReply"),
            ("empChatDeck", "chatDeckReply"), ("empChatPull", "chatPullReply"), ("empChatShop", "tillShopReply"),
        };

        private const float ReplyChance = 0.6f, ReplyDelay = 1.6f; // a customer answering what an employee said to them
        private const float UpsellEvery = 15f;   // seconds between one employee's pitches
        private const float UpsellBudget = 1.25f; // they came with this much more in mind than they'd planned

        private class Talk
        {
            public float NextAllowed;
            public Persona Self;
            public bool NoStockSaid;
            public float NextUpsell, RushUntil, NextBounce;
            public bool? Evening;
        }

        /// <summary>Per visit: an upseller has tried this customer.</summary>
        private class Pitched
        {
            public string By;
            public float Bag;
        }

        private static readonly Dictionary<Worker, Talk> Talks = new Dictionary<Worker, Talk>();

        private static bool TalkOn => Plugin.EmployeeTalk.Value;
        private static readonly AccessTools.FieldRef<Worker, InteractablePackagingBox_Item> HoldingBox =
            Core.Compat.Field<Worker, InteractablePackagingBox_Item>(typeof(EmployeeLife), "m_CurrentHoldItemBox");
        private static readonly AccessTools.FieldRef<Worker, float> WalkSpeed =
            Core.Compat.Field<Worker, float>(typeof(EmployeeLife), "m_ExtraSpeedMultiplier");
        private static readonly System.Reflection.MethodInfo EvaluateStats = AccessTools.Method(typeof(Worker), "EvaluateTaskLevelStat");

        public static void Init()
        {
            UnityEngine.SceneManagement.SceneManager.sceneLoaded += (scene, mode) => Talks.Clear(); // a load rebuilds the scene: its workers are gone
            WorkerLife.Arrived += w =>
            {
                Say(w, "empArrive", 0.5f);
                if (Effect(w, "early", 0f) > 0f) TalkOf(w).RushUntil = Time.time + RushSeconds;
            };
            WorkerLife.GoingHome += w => Say(w, "empGoHome", 0.6f);
            WorkerLife.GotBonus += w => Say(w, "empBonus", 1f);
            WorkerLife.Fired += w => Say(w, "empFired", 1f);
            WorkerLife.StateChanged += OnState;
            WorkerLife.NewCustomer += OnCustomer;
            WorkerLife.SlowTick += OnSecond;
            WorkerLife.StatsSet += ApplySpeed;
            CustomerLife.CheckedOut += OnCheckedOut;
            Shoplifting.StartedStealing += OnThief;
        }

        // ---- Perks

        public static float Effect(Worker w, string key, float none)
        {
            var c = Roster.Of(w);
            if (c == null) return none;
            bool multiplies = key == "work" || key == "morning" || key == "evening";
            float v = none;
            foreach (var id in c.Def.Perks ?? new List<string>())
                if (Traits.EmployeePerk(id)?.effects is Dictionary<string, float> e && e.TryGetValue(key, out float x))
                    v = multiplies ? v * x : v + x;
            return v;
        }

        private static void ApplySpeed(Worker w)
        {
            float work = Effect(w, "work", 1f) * Effect(w, IsEvening ? "evening" : "morning", 1f);
            if (Mathf.Approximately(work, 1f)) return;
            w.m_ScanItemTime *= work;
            w.m_GiveChangeTime *= work;
            w.m_RestockTime *= work;
        }

        private static bool IsEvening => LightManager.GetTimeHour() >= EveningHour;

        /// <summary>Morning and evening speeds change at the hour; the game only works speeds out when something changes.</summary>
        private static void WatchTheClock(Worker w, Talk talk)
        {
            if (Effect(w, "morning", 1f) == 1f && Effect(w, "evening", 1f) == 1f) return;
            bool evening = IsEvening;
            if (talk.Evening == evening) return;
            bool first = talk.Evening == null;
            talk.Evening = evening;
            if (first) return;
            EvaluateStats?.Invoke(w, null);
            if (evening) Say(w, "empMidday", 1f);
        }

        /// <summary>Early bird: the game's arrival boost lasts under 2 seconds (WorkerManager.OnDayStarted); they keep it the whole way in.</summary>
        private static void Rush(Worker w, Talk talk)
        {
            if (talk.RushUntil <= 0f) return;
            if (Time.time < talk.RushUntil)
            {
                WalkSpeed(w) = 1f + Effect(w, "early", 0f);
                return;
            }
            talk.RushUntil = 0f;
            WalkSpeed(w) = 1f + w.GetWorkerData().walkSpeedMultiplier;
            EvaluateStats?.Invoke(w, null); // adds their skill back, when they have a task
        }

        /// <summary>Bouncer: someone stinky or caught stealing before, near them, gets thrown out, off the record: no ban comes of it.</summary>
        private static void Bounce(Worker w, Talk talk)
        {
            float chance = Effect(w, "bounce", 0f);
            if (chance <= 0f || Time.time < talk.NextBounce || Random.value >= chance) return;
            foreach (var v in Cast.Visitors())
            {
                var c = v.Key;
                if (!CustomerLife.IsStinky(c) && Shoplifting.Caught(v.Value) == 0) continue;
                if (Vector3.Distance(c.transform.position, w.transform.position) > BounceRange || CustomerLife.HasCheckedOut(c) || KickOut.Refusal(c) != null) continue;
                talk.NextBounce = Time.time + BounceEvery;
                Say(w, "empBounce", 1f, new Dialogue.Tokens { Other = v.Value.name });
                KickOut.Kick(c, onTheRecord: false);
                return;
            }
        }

        [HarmonyPatch(typeof(Worker), nameof(Worker.AddExp))]
        private static class ExpPatch
        {
            private static void Prefix(Worker __instance, ref int expAddAmount)
            {
                float more = Effect(__instance, "exp", 0f);
                float teach = 0f;
                foreach (var other in WorkerLife.Active())
                    if (other != __instance) teach = Mathf.Max(teach, Effect(other, "teach", 0f));
                more += teach;
                if (more != 0f) expAddAmount = Mathf.CeilToInt(expAddAmount * (1f + more));
            }
        }

        private static void OnState(Worker w, EWorkerState before, EWorkerState now)
        {
            if (now == EWorkerState.RestockingShelf)
            {
                Say(w, "empRestock", 0.1f);
                if (Random.value < Effect(w, "fumble", 0f) && WorkerLife.Pause(w, FumbleSeconds)) Say(w, "empFumble", 1f);
            }
            else if (before == EWorkerState.RestockingShelf && now != EWorkerState.RestockingShelf) Say(w, "empRestockDone", 0.06f);
            else if (now == EWorkerState.SettingShelfPrice || now == EWorkerState.SettingCardPrice) Say(w, "empSetPrice", 0.1f);
        }

        private static void OnCustomer(Worker w, Customer c)
        {
            var p = Cast.Of(c);
            var tokens = new Dialogue.Tokens { Other = p?.name };
            var answer = new Dialogue.Tokens { Other = NameOf(w) };
            var topic = Banter[Random.Range(0, Banter.Length)];
            if (Random.value < Effect(w, "chat", 0f) && WorkerLife.Pause(w, ChatSeconds))
            {
                if (Say(w, topic.say, 1f, tokens)) Dialogue.Say(c, topic.reply, answer, 1f, 2.2f);
                return;
            }
            if (Random.value < Effect(w, "gossip", 0f) && News.Pick(p?.name, null) is News.Item news)
            {
                if (Say(w, "empGossip", 1f, new Dialogue.Tokens { Other = news.w })) Reply(c, w, "tillGossipReply");
                return;
            }
            if (Say(w, topic.say, SmallTalk, tokens)) Dialogue.Say(c, topic.reply, answer, 1f, ReplyDelay);
            else if (Say(w, "empGreet", 0.5f, tokens)) Reply(c, w, "tillGreetReply");
        }

        private static void Reply(Customer c, Worker w, string situation) =>
            Dialogue.Say(c, situation, new Dialogue.Tokens { Other = NameOf(w) }, ReplyChance, ReplyDelay);

        private static void OnCheckedOut(Customer c)
        {
            var pitched = Cast.PeekVisitData<Pitched>(c);
            if (pitched?.By != null && Plugin.CustomerDebug.Value)
            {
                float paid = CustomerLife.PaidThisVisit(c);
                Plugin.Log.LogInfo($"Employees: {pitched.By} upsold {Cast.Of(c)?.name}: ${pitched.Bag:0.00} in their bag when pitched, paid ${paid:0.00} (+${paid - pitched.Bag:0.00})");
            }
            var w = WorkerLife.ServedBy(c);
            if (w == null) return;
            var p = Cast.Of(c);
            bool thanked = Say(w, "empThanks", 0.6f, new Dialogue.Tokens { Other = p?.name });
            if (p == null) return;
            Loyalty.Change(p, Mathf.RoundToInt(Effect(w, "loyalty", 0f)), $"served by {NameOf(w)}");
            float tip = Effect(w, "tip", 0f);
            if (tip > 0f && Random.value < tip)
            {
                Quirks.GiveTip(c, Mathf.Max(1f, Mathf.Round(CustomerLife.PaidThisVisit(c) * Random.Range(0.05f, 0.1f))));
                Dialogue.Say(c, "tip", null, 1f, 0.6f);
            }
            else if (thanked) Reply(c, w, "tillThanksReply");
        }

        private static void OnSecond(Worker w)
        {
            if (Roster.Of(w) is Roster.Custom own && Clothes.Wardrobe.HasFace(own.Def.Id)) Clothes.FaceHeads.Hold(w);
            var talk = TalkOf(w);
            Rush(w, talk);
            WatchTheClock(w, talk);
            Bounce(w, talk);
            bool stuck = w.m_ExclaimationMesh != null && w.m_ExclaimationMesh.activeSelf;
            if (stuck && !talk.NoStockSaid) Say(w, "empNoStock", 0.7f);
            talk.NoStockSaid = stuck;
            if (WorkerLife.IsPaused(w)) return;
            Upsell(w, talk);
            // A per-minute chance, about.
            if (Random.value < Effect(w, "slack", 0f) / 60f && WorkerLife.Pause(w, Random.Range(SlackMin, SlackMax))) Say(w, "empBreak", 1f);
            else if (w.m_CurrentState == EWorkerState.RestOutside || w.m_CurrentState == EWorkerState.Idle) Say(w, "empIdle", 0.015f);
            else if (CustomerLife.IsAtTill(WorkerLife.Serving(w))) Say(w, "empScan", 0.04f);
        }

        /// <summary>
        /// "You know what goes great with that?" Someone in line at their till, with nothing on the counter yet (CustomerLife.IsWaitingInQueue), goes back for one
        /// more thing with a bigger budget, once a visit. They leave the line properly and queue again through vanilla when they're done.
        /// </summary>
        private static void Upsell(Worker w, Talk talk)
        {
            float chance = Effect(w, "upsell", 0f);
            if (chance <= 0f || Time.time < talk.NextUpsell || !CPlayerData.m_IsShopOpen || LightManager.GetHasDayEnded()) return;
            var till = WorkerLife.Till(w);
            if (till == null) return;
            foreach (var c in till.m_CustomerListInQueue)
            {
                if (c == null || !CustomerLife.IsWaitingInQueue(c) || CustomerLife.HasCheckedOut(c) || CustomerStates.Of(c) != null || c == till.m_CurrentCustomer) continue;
                if (Cast.Of(c) == null || Cast.PeekVisitData<Pitched>(c) != null) continue;
                var pitched = Cast.VisitData<Pitched>(c);
                if (Random.value >= chance) continue;
                pitched.By = NameOf(w);
                pitched.Bag = c.m_CurrentCostTotal;
                talk.NextUpsell = Time.time + UpsellEvery;
                var p = Cast.Of(c);
                Say(w, "empUpsell", 1f, new Dialogue.Tokens { Other = p.name });
                c.m_MaxMoney = Mathf.Max(c.m_MaxMoney, c.m_CurrentCostTotal) * UpsellBudget;
                Dialogue.Say(c, "upsellYes", new Dialogue.Tokens { Other = NameOf(w) }, 1f, 1.5f);
                if (Plugin.CustomerDebug.Value) Plugin.Log.LogInfo($"Employees: {NameOf(w)} upsold {p.name}, who goes back for more");
                CustomerLife.LeaveQueue(c);
                CustomerStates.ToVanilla(c, CustomerLife.Shop);
                return; // the line just changed
            }
        }

        /// <summary>A hawk-eyed employee in the shop may stop a thief before the player has to.</summary>
        private static void OnThief(Customer c)
        {
            foreach (var w in WorkerLife.Active().ToList())
            {
                if (Vector3.Distance(w.transform.position, c.transform.position) > CalledOutRange) continue;
                if (Random.value >= Effect(w, "thief", 0f)) continue;
                Say(w, "empCaughtThief", 1f, new Dialogue.Tokens { Other = Cast.Of(c)?.name });
                Shoplifting.Catch(c);
                return;
            }
        }

        // ---- Lines

        private static Talk TalkOf(Worker w)
        {
            if (!Talks.TryGetValue(w, out var t)) Talks[w] = t = new Talk { Self = new Persona { id = "worker-" + w.m_WorkerIndex, name = NameOf(w) } };
            return t;
        }

        private static string NameOf(Worker w) => w.GetWorkerData()?.GetName() ?? WorkerManager.GetWorkerData(w.m_WorkerIndex).GetName();

        /// <summary>Says a line of the situation, by this chance × [Customers - Employees] EmployeeChattiness (1 = always). Nothing when they said something just now. True when they spoke.</summary>
        private static bool Say(Worker w, string situation, float chance, Dialogue.Tokens tokens = null)
        {
            if (w == null || !w.m_IsActive || !TalkOn) return false;
            if (chance < 1f && Random.value >= chance * Plugin.EmployeeChattiness.Value) return false;
            var talk = TalkOf(w);
            float now = Time.time;
            if (chance < 1f && now < talk.NextAllowed) return false;
            var custom = Roster.Of(w);
            var lines = Traits.EmployeeLines(custom?.Def, situation);
            if (lines == null) return false;
            // Nobody to name (Personalities off): {name} would become the speaker.
            if (tokens?.Other == null) lines = lines.FindAll(l => !l.Contains("{name}"));
            if (lines.Count == 0) return false;
            talk.NextAllowed = now + Cooldown;
            string text = Dialogue.Fill(lines[Random.Range(0, lines.Count)], talk.Self, tokens);
            UI.WorldText.Show(w.transform, text, 1.8f, Plugin.CustomerLineSeconds.Value, Plugin.CustomerLineTextSize.Value);
            TalkSounds.Say(w.transform, custom?.Def.Voice, talk.Self.id, w.m_IsFemale, text, Traits.EmployeeMood(situation));
            // Not with a box in their arms: the animation would take their arms off it.
            if (HoldingBox(w) == null) Moves.Play(w, custom?.Def, situation, talk.Self.name);
            return true;
        }
    }
}
