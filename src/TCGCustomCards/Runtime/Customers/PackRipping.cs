using System.Collections;
using System.Collections.Generic;
using System.Linq;
using HarmonyLib;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using UnityEngine;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// [Customers - Behaviour] PackRipping: after paying, a customer may walk to a free spot in the shop, away from the tills,
    /// and open the packs they bought there before leaving (trait effect "rip"; mod state <see cref="RipState"/>, then the
    /// built-in leave state). Contents are rolled by the game's own pack roll (custom packs through our patches), so pulls
    /// follow the real odds. A hit shows the card above their head, they cheer, hype customers nearby react and the shop gets a little
    /// XP. Pulls are the customer's: nothing goes into your collection. Saved: packs opened + the 3 best pulls, per persona.
    /// </summary>
    internal class PackRipping : IPersonaModule
    {
        public class Pull
        {
            public string k;  // card key
            public float p;   // market price when pulled
        }

        public class Row
        {
            public int n;          // packs opened in the shop
            public List<Pull> top; // best pulls, at most 3
        }

        /// <summary>Packs they have opened in the shop.</summary>
        public static int PacksOpened(Persona p) => Table.Get(p, create: false)?.n ?? 0;

        /// <summary>The market price of their best pull, 0 when they have none.</summary>
        public static float BestPull(Persona p)
        {
            var top = Table.Get(p, create: false)?.top;
            return top == null || top.Count == 0 ? 0f : top.Max(t => t.p);
        }

        private const int MaxPacksPerSession = 3;
        private const float SecondsPerPack = 3.6f; // looking through a pack's cards: the built-in animation's length and its fade
        private const float TearAt = 0.95f;        // seconds into the opening: the pack is torn (the built-in animation's pose)
        private const float TillClearance = 4f; // metres the rip spot keeps from every till
        private const float WalkTimeout = 12f;  // seconds; after that they rip where they got to
        private const float BigHitSeconds = 9f, HugeHitSeconds = 14f;
        private const float HitSeconds = 7f;    // a hit stays above their head this long

        /// <summary>A customer pulled a hit (worth at least PackHitPrice): (customer, persona, card, price). Binders, news, …</summary>
        public static event System.Action<Customer, Persona, CardData, float> PulledHit;

        /// <summary>A customer opened their packs here and pulled nothing worth PackHitPrice: (customer, persona).</summary>
        public static event System.Action<Customer, Persona> Bricked;

        public string Id => "packRipping";
        public bool Enabled => On;
        private static bool On => Cast.Enabled && Plugin.CustomerPackRipping.Value;

        private static readonly PersonaTable<Row> Table = new PersonaTable<Row>();
        /// <summary>One customer's rip: who and which packs.</summary>
        private class Session
        {
            public Persona Persona;
            public List<EItemType> Packs;
            public bool Opening;
            public PackInHand Pack;
            public Vector3 FanScale;
        }

        /// <summary>Walk to a free spot (or rip where they are after a while), open the packs, then leave.</summary>
        private class RipState : ModState
        {
            public const string StateId = "packRipping.rip";
            public override string Id => StateId;

            public override void Enter(Customer c, StateContext ctx)
            {
                var spot = CustomerStates.FreeSpot(c, TillClearance);
                if (spot.HasValue) ctx.WalkTo(spot.Value);
                else Open(c, ctx);
            }

            public override void Tick(Customer c, StateContext ctx)
            {
                var s = (Session)ctx.Data;
                if (!s.Opening && ctx.Elapsed > WalkTimeout)
                {
                    CustomerLife.ResetPath(c); // stop where they got to (Tick runs after vanilla's Update)
                    Open(c, ctx);
                }
            }

            public override void Arrived(Customer c, StateContext ctx) => Open(c, ctx);

            public override void Exit(Customer c, StateContext ctx)
            {
                if (c == null) return;
                CardBubble.Hide(c);
                HandCards.Clear(c);
                c.m_Anim.SetBool("IsPlaying", false);
                if (ctx.Data is Session s)
                {
                    PackInHand.Hide(ref s.Pack);
                    if (s.FanScale != Vector3.zero) c.m_GameCardFanOut.transform.localScale = s.FanScale;
                }
                c.m_GameCardFanOut.SetActive(false);
                // The bag was put away to free their hands.
                if (c.m_ItemInBagList.Count > 0)
                {
                    c.m_ShoppingBagTransform.gameObject.SetActive(true);
                    c.m_Anim.SetBool("HoldingBag", true);
                }
            }

            private static void Open(Customer c, StateContext ctx)
            {
                var s = (Session)ctx.Data;
                if (s.Opening) return;
                s.Opening = true;
                // No ResetPath here: Arrived runs inside vanilla's path follower, which reads the path right after.
                ctx.Run(Rip(c, s));
            }
        }

        private static readonly AccessTools.FieldRef<CardOpeningSequence, ECollectionPackType> OpeningPackType =
            AccessTools.FieldRefAccess<CardOpeningSequence, ECollectionPackType>("m_CollectionPackType");
        private static readonly AccessTools.FieldRef<CardOpeningSequence, bool> OpeningScreenActive =
            AccessTools.FieldRefAccess<CardOpeningSequence, bool>("m_IsScreenActive");
        private static readonly AccessTools.FieldRef<CardOpeningSequence, bool> OpeningHasFoil =
            AccessTools.FieldRefAccess<CardOpeningSequence, bool>("m_HasFoilCard");

        public void Init()
        {
            CustomerStates.Register(new RipState());
            UI.CustomerDebugPanel.Section("Pack ripping", (c, p, rows) =>
            {
                if (!On) return;
                rows.Add(("chance", $"{Mathf.Clamp01(RipChance(p)) * 100f:0}% if they bought packs"));
                rows.Add(("packs in bag", BoughtPacks(c).Count.ToString()));
                var ctx = CustomerStates.Of(c);
                if (ctx?.State.Id == RipState.StateId)
                {
                    var s = (Session)ctx.Data;
                    rows.Add(("now", s.Opening ? $"opening {s.Packs.Count} pack(s)" : "walking to a spot"));
                }
                var row = Table.Get(p, create: false);
                if (row != null) rows.Add(("record", $"{row.n} pack(s) opened here"));
            });
            // Paid and turned to the exit (vanilla's ExitShop already did the visit's bookkeeping).
            CustomerLife.ExitingShop += c =>
            {
                var p = On ? Cast.Of(c) : null;
                if (p == null || CustomerStates.Of(c) != null || CustomerLife.IsTournamentEntrant(c) || KickOut.WasKicked(c) || Shoplifting.IsStealing(c)) return;
                var packs = BoughtPacks(c);
                if (packs.Count == 0 || Random.value >= RipChance(p)) return;
                CustomerStates.Goto(c, RipState.StateId, new Session { Persona = p, Packs = packs });
            };
        }

        /// <summary>PackRipChance for everyone + the traits' "rip" (Pack ripper +1 = always).</summary>
        private static float RipChance(Persona p) => Plugin.CustomerPackRipChance.Value / 100f + Traits.Effect(p, "rip");

        private static List<EItemType> BoughtPacks(Customer c)
        {
            var list = new List<EItemType>();
            var player = CSingleton<InteractionPlayerController>.Instance;
            if (player == null) return list;
            foreach (var item in c.m_ItemInBagList)
            {
                if (item == null) continue;
                var t = item.GetItemType();
                if (player.IsCardPackType(t)) list.Add(t);
                if (list.Count >= MaxPacksPerSession) break;
            }
            return list;
        }

        private static IEnumerator Rip(Customer c, Session s)
        {
            var p = s.Persona;
            c.m_Anim.SetBool("IsPlaying", true);
            c.m_ShoppingBagTransform.gameObject.SetActive(false);
            c.m_Anim.SetBool("HoldingBag", false);
            // The fan stays active (HandCards draws on it) but out of sight until the pack is open.
            var fan = c.m_GameCardFanOut.transform;
            s.FanScale = HandCards.FanScale(c);
            c.m_GameCardFanOut.SetActive(true);
            Dialogue.Say(c, "ripStart", null, 0.7f);
            var row = Table.Get(p);
            bool anyHit = false;
            foreach (var pack in s.Packs)
            {
                Debug.SpikeWatch.Mark("a pack roll");
                var cards = Roll(pack);
                fan.localScale = Vector3.zero;
                s.Pack = PackInHand.Show(c, c.m_ItemInBagList.Find(i => i != null && i.GetItemType() == pack));
                Moves.Play(c, "ripPack");
                float tear = Time.time + TearAt;
                if (cards != null) yield return HandCards.Show(c, cards); // draws the pictures while they open it
                if (Time.time < tear) yield return new WaitForSeconds(tear - Time.time);
                PackInHand.Hide(ref s.Pack);
                fan.localScale = s.FanScale;
                Moves.Play(c, "ripLook"); // they look through this pack's cards
                yield return new WaitForSeconds(SecondsPerPack);
                if (cards == null) continue;
                row.n++;
                CardData best = null;
                float bestPrice = 0f;
                foreach (var card in cards)
                {
                    float price = CardInfo.Price(card);
                    if (best == null || price > bestPrice) { best = card; bestPrice = price; }
                }
                if (best == null) continue;
                bool hit = bestPrice >= Plugin.CustomerPackHitPrice.Value;
                if (!hit) continue;
                anyHit = true;
                Remember(row, best, bestPrice);
                var tokens = new Dialogue.Tokens { Card = CardInfo.Name(best), Price = bestPrice };
                Dialogue.Say(c, "ripHit", tokens, 1f);
                // A big one (3× a hit) or a huge one (10×) has its own moment, replacing the hit's.
                float times = bestPrice / Mathf.Max(0.01f, Plugin.CustomerPackHitPrice.Value);
                if (times >= 10f) Moves.Play(c, "ripHugeHit");
                else if (times >= 3f) Moves.Play(c, "ripBigHit");
                CEventManager.QueueEvent(new CEventPlayer_AddShopExp(Mathf.Clamp(Mathf.RoundToInt(bestPrice / 5f), 1, 15)));
                Plugin.Log.LogInfo($"Customers: {p.name} pulled {CardInfo.Name(best)} ({bestPrice:0.00}) from {pack}");
                // They stay with a big pull, so the people coming over find them still here.
                float showFor = times >= 10f ? HugeHitSeconds : times >= 3f ? BigHitSeconds : HitSeconds;
                Debug.SpikeWatch.Mark("a pack hit");
                using (Debug.SpikeWatch.Measure("pack hit"))
                {
                    CardBubble.Show(c, best, showFor);
                    Crowd.React(c, p, best, bestPrice);
                }
                PulledHit?.Invoke(c, p, best, bestPrice);
                // Big moments make people come back.
                if (bestPrice >= Plugin.CustomerPackHitPrice.Value * 3f) Cast.Promote(p, $"pulled {CardInfo.Name(best)}");
                yield return new WaitForSeconds(showFor);
            }
            if (!anyHit)
            {
                Dialogue.Say(c, "ripBrick", null, 0.8f);
                Moves.Play(c, s.Packs.Count >= 3 ? "ripBrickStack" : "ripBrick"); // whether or not they said it; a whole stack for nothing has its own
                Bricked?.Invoke(c, p);
            }
            yield return new WaitForSeconds(1f);
            Dialogue.Say(c, "leaveHappy", null, 0.25f); // only now are they really going
            CustomerStates.Goto(c, CustomerStates.LeaveId);
        }

        /// <summary>
        /// Rolls a pack the way the auto pack opener does (CardOpeningSequence.GetPackContentCardDataList, no cards added to the
        /// player). That call overwrites the opening screen's pack type and foil flag, so they are put back, and nothing is
        /// rolled while the player is opening a pack.
        /// </summary>
        private static List<CardData> Roll(EItemType pack)
        {
            using var measured = Debug.SpikeWatch.Measure("pack roll");
            var seq = CSingleton<CardOpeningSequence>.Instance;
            if (seq == null || OpeningScreenActive(seq)) return null;
            var type = InventoryBase.ItemTypeToCollectionPackType(pack);
            if (type == ECollectionPackType.None) return null;
            var before = OpeningPackType(seq);
            bool foil = OpeningHasFoil(seq);
            try
            {
                // The game hands back its one list, refilled by the next roll: they're looking through these for seconds, while others open packs too.
                var cards = new List<CardData>();
                foreach (var rolled in seq.GetPackContentCardDataList(type))
                {
                    var card = new CardData();
                    card.CopyData(rolled);
                    cards.Add(card);
                }
                return cards;
            }
            catch (System.Exception e)
            {
                Plugin.Log.LogWarning($"Customers: could not roll a {pack} for a customer: {e.Message}");
                return null;
            }
            finally
            {
                OpeningPackType(seq) = before;
                OpeningHasFoil(seq) = foil;
            }
        }

        private static void Remember(Row row, CardData card, float price)
        {
            if (row.top == null) row.top = new List<Pull>();
            row.top.Add(new Pull { k = CardInfo.Key(card), p = (float)System.Math.Round(price, 2) });
            row.top.Sort((a, b) => b.p.CompareTo(a.p));
            if (row.top.Count > 3) row.top.RemoveRange(3, row.top.Count - 3);
        }

        public void Load(JToken state) => Table.Load(state);

        public JToken Save(ICollection<string> keep) => Table.Save(keep);

        public string Summary(Persona p)
        {
            if (!On) return null;
            var row = Table.Get(p, create: false);
            if (row == null || row.n == 0) return null;
            string best = row.top != null && row.top.Count > 0 ? $", best {CardInfo.NameOfKey(row.top[0].k) ?? "?"} (${row.top[0].p:0})" : "";
            return $"opened {row.n} pack(s) here{best}";
        }
    }
}
