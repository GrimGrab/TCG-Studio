using System;
using System.Collections.Generic;
using HarmonyLib;
using UnityEngine;

namespace TCGCustomCards.Hooks
{
    /// <summary>
    /// Single owner of the patches on a customer's visit (Customer / CustomerManager / CustomerReviewManager outside the
    /// tournament pipeline, which ShopEvents owns). Vanilla always runs: patches only observe, adjust an argument or a return
    /// value through the pipelines below, and raise plain C# events. Customers are pooled: one Customer object is many
    /// visitors over a day, so features key their data by visit, not by object.
    /// </summary>
    internal static class CustomerLife
    {
        /// <summary>A pooled customer became a new visitor (spawn, tournament spawn, or the first step of loading a saved one).</summary>
        public static event Action<Customer> Activated;

        /// <summary>A saved customer was restored: <c>saveIndex</c> = its place in CPlayerData.m_CustomerSaveDataList. Follows Activated.</summary>
        public static event Action<Customer, int> Loaded;

        /// <summary>The visitor is about to go back to the pool (left the shop or the day was reset). Fields are still set.</summary>
        public static event Action<Customer> Leaving;

        /// <summary>
        /// The visitor walked in through the entrance (vanilla's OnCustomerReachInsideShop, once per visit; not for customers
        /// restored inside the shop from a save). They spawn out on the street, so anything they should be seen doing on
        /// arrival starts here.
        /// </summary>
        public static event Action<Customer> EnteredShop;

        /// <summary>
        /// Every frame for each active customer, after vanilla's Update: per-customer logic (timers, mod states) lives here,
        /// on the customer's own update, not in loops of our own. Handlers must be cheap.
        /// </summary>
        public static readonly List<Action<Customer>> Tick = new List<Action<Customer>>();

        /// <summary>
        /// About <see cref="SlowTickSeconds"/> apart for each active customer, by the clock (not frames), spread over
        /// customers: for checks that don't need every frame (timers in seconds, moods).
        /// </summary>
        public static readonly List<Action<Customer>> SlowTick = new List<Action<Customer>>();

        public const float SlowTickSeconds = 0.25f;
        private static readonly Dictionary<Customer, float> NextSlowTick = new Dictionary<Customer, float>();

        static CustomerLife()
        {
            // Loading a save rebuilds the shop scene: its customers are gone.
            UnityEngine.SceneManagement.SceneManager.sceneLoaded += (scene, mode) =>
            {
                NextSlowTick.Clear();
                Glances.Clear();
            };
        }

        /// <summary>
        /// Vanilla took its snapshot of the customers to save (CustomerManager.SaveCustomerData, run before every save but
        /// sometimes earlier, e.g. by play tables). Argument: the customers behind CPlayerData.m_CustomerSaveDataList, in its
        /// order (null where unknown). Record per-visit data here, not at write time: the objects may be reused by then.
        /// </summary>
        public static event Action<List<Customer>> Snapshot;

        /// <summary>The customer judged a price on a shelf (after the BuyChance pipeline). Whether they buy is rolled next.</summary>
        public static event Action<BuyContext, int> PriceJudged;

        /// <summary>The customer put a card from a card shelf into their bag.</summary>
        public static event Action<Customer, CardData, float> CardBought;

        /// <summary>The customer decided to pay and joined a till queue (ThinkWantToPay; again when they switch queues).</summary>
        public static event Action<Customer> HeadingToPay;

        /// <summary>The customer paid at the till (bag contents still there).</summary>
        public static event Action<Customer> CheckedOut;

        /// <summary>The customer turned towards the exit (state ExitingShop; not tournament entrants). Their review was just sent.</summary>
        public static event Action<Customer> ExitingShop;
        /// <summary>After every <see cref="ExitingShop"/> listener: someone not in a mod state now (<see cref="CustomerStates.Of"/>) really is walking out.</summary>
        public static event Action<Customer> ExitDecided;
        /// <summary>A customer's game at a play table began (their cards are in hand) / ended. Runs for each player.</summary>
        public static event Action<Customer> TableGameStarted, TableGameEnded;

        [HarmonyPatch(typeof(Customer), nameof(Customer.PlayTableGameStarted))]
        private static class TableGameStartedPatch
        {
            private static void Postfix(Customer __instance) => Raise(TableGameStarted, __instance, "table game started");
        }

        [HarmonyPatch(typeof(Customer), nameof(Customer.PlayTableGameEnded))]
        private static class TableGameEndedPatch
        {
            private static void Prefix(Customer __instance) => Raise(TableGameEnded, __instance, "table game ended");
        }

        /// <summary>A review was written by this customer (vanilla picked a random name; handlers may change it).</summary>
        public static event Action<Customer, CustomerReviewData> ReviewAdded;

        /// <summary>What a customer is looking at on a shelf while vanilla rolls the buy chance.</summary>
        public class BuyContext
        {
            public Customer Customer;
            public bool IsCard;
            /// <summary>Card on a card shelf (IsCard), else null.</summary>
            public CardData Card;
            public float Price, Market;
        }

        /// <summary>Adjusts vanilla's buy chance (0–100) in order; each gets the previous result. Clamped to 0–100 after.</summary>
        public static readonly List<Func<BuyContext, int, int>> BuyChance = new List<Func<BuyContext, int, int>>();

        /// <summary>Adjusts a review's higherStarChanceAdd (vanilla: + 50% chance of one more star) for the reviewing customer.</summary>
        public static readonly List<Func<Customer, int, int>> ReviewStars = new List<Func<Customer, int, int>>();

        private static readonly AccessTools.FieldRef<Customer, float> ScannedCost =
            AccessTools.FieldRefAccess<Customer, float>("m_TotalScannedItemCost");
        private static readonly AccessTools.FieldRef<Customer, bool> CheckedOutFlag =
            AccessTools.FieldRefAccess<Customer, bool>("m_HasCheckedOut");
        private static readonly AccessTools.FieldRef<Customer, bool> PausingAction =
            AccessTools.FieldRefAccess<Customer, bool>("m_IsPausingAction");
        private static readonly AccessTools.FieldRef<Customer, CardShelf> CurrentCardShelf =
            AccessTools.FieldRefAccess<Customer, CardShelf>("m_CurrentCardShelf");
        private static readonly AccessTools.FieldRef<Customer, InteractableCardCompartment> CurrentCardCompartment =
            AccessTools.FieldRefAccess<Customer, InteractableCardCompartment>("m_CurrentCardCompartment");

        /// <summary>What the visitor paid at the till this visit (0 before checking out).</summary>
        public static float PaidThisVisit(Customer c) => CheckedOutFlag(c) ? ScannedCost(c) : 0f;

        public static bool HasCheckedOut(Customer c) => CheckedOutFlag(c);

        public static bool IsTournamentEntrant(Customer c) => c.GetCustomerTournamentData()?.m_IsTournamentCustomer == true;

        /// <summary>
        /// Holds the customer where they are using vanilla's own pause (the flag the trade screen sets, Customer.cs:242/:266):
        /// their Update skips all decisions and movement until released. Animation and rotation keep running.
        /// </summary>
        public static void SetPaused(Customer c, bool paused) => PausingAction(c) = paused;

        public static bool IsPaused(Customer c) => PausingAction(c);

        private static readonly AccessTools.FieldRef<Customer, bool> Smelly = AccessTools.FieldRefAccess<Customer, bool>("m_IsSmelly");
        private static readonly System.Reflection.MethodInfo ExitShopMethod = AccessTools.Method(typeof(Customer), "ExitShop");

        public static bool IsStinky(Customer c) => Smelly(c);

        private static readonly AccessTools.FieldRef<Customer, InteractableCashierCounter> QueueCounter =
            AccessTools.FieldRefAccess<Customer, InteractableCashierCounter>("m_CurrentQueueCashierCounter");
        private static readonly System.Reflection.MethodInfo ThinkWantToPayMethod = AccessTools.Method(typeof(Customer), "ThinkWantToPay");
        private static readonly System.Reflection.MethodInfo ThinkWantToPlayTableMethod = AccessTools.Method(typeof(Customer), "ThinkWantToPlayTable");
        private static readonly AccessTools.FieldRef<Customer, bool> HasPlayedGame = AccessTools.FieldRefAccess<Customer, bool>("m_HasPlayedGame");
        private static readonly AccessTools.FieldRef<Customer, bool> Angry = AccessTools.FieldRefAccess<Customer, bool>("m_IsAngry");

        /// <summary>
        /// Vanilla would let them play at a table now (DetermineShopAction :944): not played yet, nothing unpaid, not angry, and
        /// a table free or a player waiting at one.
        /// </summary>
        public static bool CanPlayTable(Customer c) =>
            !HasPlayedGame(c) && !Angry(c) && (CheckedOutFlag(c) || (c.m_ItemInBagList.Count == 0 && c.m_CardInBagList.Count == 0)) &&
            (ShelfManager.GetPlayTableToPlay(findTableWithPlayerWaiting: false) != null || ShelfManager.HasPlayTableWithPlayerWaiting());

        /// <summary>To a table through vanilla's ThinkWantToPlayTable (:1073). False = <see cref="CanPlayTable"/> says not now.</summary>
        private static readonly AccessTools.FieldRef<Customer, CustomerTradeData> TradeData = AccessTools.FieldRefAccess<Customer, CustomerTradeData>("m_CustomerTradeData");
        private static readonly AccessTools.FieldRef<Customer, InteractableCashierCounter> TradeCounter = AccessTools.FieldRefAccess<Customer, InteractableCashierCounter>("m_CurrentTradeCardCashierCounter");
        private static readonly AccessTools.FieldRef<Customer, bool> HasTraded = AccessTools.FieldRefAccess<Customer, bool>("m_HasTradedCard");
        private static readonly AccessTools.FieldRef<Customer, CardData> SoldCardData = AccessTools.FieldRefAccess<Customer, CardData>("m_SoldCardData");
        private static readonly AccessTools.FieldRef<Customer, Transform> WalkTarget = AccessTools.FieldRefAccess<Customer, Transform>("m_TargetTransform");

        public static bool PlayedAtTable(Customer c) => HasPlayedGame(c);
        public static bool TradedWithYou(Customer c) => HasTraded(c);

        /// <summary>The card the player bought from them this visit, or null.</summary>
        public static CardData SoldCard(Customer c) => SoldCardData(c);

        /// <summary>
        /// They walk to the counter to sell the player this card, the way vanilla's sell customers do (Customer.cs:916-927) but with their own card and price: the
        /// screen uses trade data it's given as it does after "let me think" (CustomerTradeCardScreen.SetCustomer). False when no counter can take them.
        /// </summary>
        public static bool GoSellCard(Customer c, CardData card, float askPrice)
        {
            if (!InsideShop(c) || HasTraded(c)) return false;
            var counter = ShelfManager.GetCashierCounterToTradeCard();
            var target = counter != null ? counter.GetTradeCardStandLoc() : null;
            if (target == null) return false;
            ResetPath(c);
            TradeData(c) = new CustomerTradeData { m_IsTrading = false, m_CardData_L = card, m_SellCardAskPrice = askPrice, m_MaxDeclineCount = UnityEngine.Random.Range(0, 5) };
            TradeCounter(c) = counter;
            WalkTarget(c) = target;
            c.m_Seeker.StartPath(c.transform.position, target.position, c.OnPathComplete);
            WaitingForPath(c) = true;
            SetStateMethod.Invoke(c, new object[] { ECustomerState.WantToTradeCard });
            return true;
        }

        public static bool GoPlayTable(Customer c)
        {
            if (!CanPlayTable(c)) return false;
            ResetPath(c);
            ThinkWantToPlayTableMethod.Invoke(c, null);
            return true;
        }

        private static readonly AccessTools.FieldRef<Customer, bool> AtPayingPosition =
            AccessTools.FieldRefAccess<Customer, bool>("m_IsAtPayingPosition");
        private static readonly AccessTools.FieldRef<Customer, bool> CountUpdated =
            AccessTools.FieldRefAccess<Customer, bool>("m_HasUpdatedCustomerCount");
        private static readonly AccessTools.FieldRef<Customer, bool> LookingForCardOnly =
            AccessTools.FieldRefAccess<Customer, bool>("m_LookingForCardOnly");
        private static readonly AccessTools.FieldRef<Customer, UnityEngine.Coroutine> DecideFinishShopping =
            AccessTools.FieldRefAccess<Customer, UnityEngine.Coroutine>("m_DecideFinishShopping");
        private static readonly AccessTools.FieldRef<Customer, ShelfCompartment> CurrentItemCompartment =
            AccessTools.FieldRefAccess<Customer, ShelfCompartment>("m_CurrentItemCompartment");
        private static readonly AccessTools.FieldRef<Item, bool> ItemHideAfterLerp =
            AccessTools.FieldRefAccess<Item, bool>("m_IsHideAfterFinishLerp");

        /// <summary>Standing at the till. A till takes its next customer while they're still walking over.</summary>
        public static bool IsAtTill(Customer c) => c != null && AtPayingPosition(c);

        /// <summary>Waiting in a till queue, not yet at the till (their items aren't on the counter).</summary>
        public static bool IsWaitingInQueue(Customer c) =>
            c.m_CurrentState == ECustomerState.QueuingToPay && !AtPayingPosition(c) && QueueCounter(c) != null;

        /// <summary>
        /// Takes the customer out of their till queue from any place in it. Everyone keeps their own spot object and spots slide forward
        /// when the line moves up (RemoveCurrentCustomerFromQueue does that from the front): the ones behind take the place of the one ahead,
        /// the leaver's spot goes, everyone walks to their spot again. Removing the last spot instead (vanilla's queue switch, which only
        /// the last in line uses) left the next customer standing short of the till, where they can't be served.
        /// </summary>
        public static void LeaveQueue(Customer c)
        {
            var counter = QueueCounter(c);
            if (counter == null || CheckedOutFlag(c)) return; // paid: the game's own DelayRemoveCustomerFromQueue takes them and their spot out
            int place = counter.m_CustomerListInQueue.IndexOf(c);
            if (place >= 0) RemoveFromLine(counter, place);
        }

        /// <summary>The customer at this place leaves the line: the spots behind move up, everyone walks to their spot again.</summary>
        public static void RemoveFromLine(InteractableCashierCounter counter, int place)
        {
            counter.m_CustomerListInQueue.RemoveAt(place);
            var spots = counter.m_ValidQueuePosList;
            if (place >= spots.Count)
            {
                Plugin.Log.LogWarning($"Customers: a till queue had {spots.Count} spots for place {place + 1}");
                counter.RemoveLastCustomerFromQueue();
            }
            else
            {
                for (int n = spots.Count - 1; n > place; n--) spots[n].SetPositionAndRotation(spots[n - 1].position, spots[n - 1].rotation);
                spots[place].gameObject.SetActive(false);
                spots.RemoveAt(place);
                counter.m_CustomerQueueCount--;
            }
            for (int i = 0; i < counter.m_CustomerListInQueue.Count; i++) counter.m_CustomerListInQueue[i].OnCashierCounterQueueMoved(i);
        }

        /// <summary>
        /// Drops what the customer planned (the pending "done shopping?" decision, items they still wanted), so a following
        /// SendOut isn't turned around by it.
        /// </summary>
        public static void DropPlans(Customer c)
        {
            var pending = DecideFinishShopping(c);
            if (pending != null) c.StopCoroutine(pending);
            DecideFinishShopping(c) = null;
            c.m_TargetBuyItemList.Clear();
            LookingForCardOnly(c) = false;
        }

        /// <summary>
        /// The customer leaves their unpaid things behind: what fits goes back where it came from (the same shelf compartment:
        /// still there, room, same type), other items are left in delivery boxes at their feet (RestockManager.SpawnPackageBoxItem,
        /// one type per box) and other cards go back into the player's collection (they came out of it onto the card shelf).
        /// Returns how many items and cards didn't go back on a shelf.
        /// </summary>
        public static int Abandon(Customer c)
        {
            Debug.HangWatch.Step("customer leaves their things (Abandon)");
            if (CheckedOutFlag(c)) return 0;
            var plan = PlanReturn(c);
            if (plan.LeftItems.Count + plan.LeftCards.Count > 0)
                Plugin.Log.LogInfo($"Customers: {plan.LeftItems.Count} item(s) and {plan.LeftCards.Count} card(s) can't go back on their shelf " +
                    $"({plan.Unknown} taken before this session, e.g. a customer restored from the save); dropped where the customer stands");
            Return(plan);
            if (plan.LeftItems.Count > 0) DropInBoxes(c, plan.LeftItems);
            foreach (var card in plan.LeftCards)
            {
                CPlayerData.AddCard(card.m_Card3dUI.m_CardUI.GetCardData(), 1);
                CardSource.Remove(card);
                card.OnDestroyed();
            }
            EmptyBag(c);
            return plan.LeftItems.Count + plan.LeftCards.Count;
        }

        private class ReturnPlan
        {
            public readonly List<Item> Items = new List<Item>();
            public readonly List<InteractableCard3d> Cards = new List<InteractableCard3d>();
            public readonly List<Item> LeftItems = new List<Item>();
            public readonly List<InteractableCard3d> LeftCards = new List<InteractableCard3d>();
            public int Unknown; // left ones with no remembered shelf
        }

        /// <summary>Which bag items and cards fit back in their own compartment (room, type). Changes nothing.</summary>
        private static ReturnPlan PlanReturn(Customer c)
        {
            var plan = new ReturnPlan();
            var room = new Dictionary<ShelfCompartment, int>();
            var typeOf = new Dictionary<ShelfCompartment, EItemType>();
            foreach (var item in c.m_ItemInBagList)
            {
                if (item == null) continue;
                if (!ItemSource.TryGetValue(item, out var from) || from == null || !from.gameObject.activeInHierarchy)
                {
                    if (!ItemSource.ContainsKey(item)) plan.Unknown++;
                    plan.LeftItems.Add(item);
                    continue;
                }
                var type = item.GetItemType();
                if (!typeOf.TryGetValue(from, out var holds))
                {
                    // An emptied compartment takes any type (Return's CheckItemType sizes its slots for it).
                    holds = from.GetItemCount() > 0 ? from.GetItemType() : type;
                    typeOf[from] = holds;
                    room[from] = from.GetMaxItemCount() - from.GetItemCount();
                }
                if (holds != type || room[from] <= 0)
                {
                    plan.LeftItems.Add(item);
                    continue;
                }
                room[from]--;
                plan.Items.Add(item);
            }
            var cardSlots = new HashSet<InteractableCardCompartment>();
            foreach (var card in c.m_CardInBagList)
            {
                if (card == null) continue;
                if (CardSource.TryGetValue(card, out var from) && from != null && from.gameObject.activeInHierarchy &&
                    from.m_StoredCardList.Count == 0 && cardSlots.Add(from))
                    plan.Cards.Add(card);
                else
                {
                    if (!CardSource.ContainsKey(card)) plan.Unknown++;
                    plan.LeftCards.Add(card);
                }
            }
            return plan;
        }

        // Vanilla's own moves: the player's put-on-shelf (InteractionPlayerController :2603-2606) and the worker's card restock
        // (Worker :2010).
        private static void Return(ReturnPlan plan)
        {
            foreach (var item in plan.Items)
            {
                var from = ItemSource[item];
                from.CheckItemType(item.GetItemType());
                item.gameObject.SetActive(true);
                item.m_Mesh.enabled = true;
                ItemHideAfterLerp(item) = false;
                item.LerpToTransform(from.GetEmptySlotTransform(), from.GetEmptySlotParent());
                from.AddItem(item, addToFront: false);
                ItemSource.Remove(item);
            }
            foreach (var card in plan.Cards)
            {
                var from = CardSource[card];
                card.StopLerpToTransform();
                card.gameObject.SetActive(true);
                card.m_Card3dUI.SetVisibility(isVisible: true); // as Customer does: the card's culling keeps its own state
                card.m_Card3dUI.m_CardUIAnimGrp.gameObject.SetActive(true);
                card.m_Card3dUI.m_LastCullState = 0;
                from.SetCardOnShelf(card);
                CardSource.Remove(card);
            }
        }

        /// <summary>
        /// Leftover items in small delivery boxes on the floor behind the customer. A box spawns its own items, so the bag's go
        /// back to the pool and out of the shop's item count, as DeactivateCustomer does with a bag (:470).
        /// </summary>
        private static void DropInBoxes(Customer c, List<Item> items)
        {
            var counts = new Dictionary<EItemType, int>();
            foreach (var item in items)
            {
                var type = item.GetItemType();
                counts[type] = counts.TryGetValue(type, out var n) ? n + 1 : 1;
                CPlayerData.AddCurrentTotalItemCount(type, -1);
                ItemSource.Remove(item);
                item.DisableItem();
            }
            var at = c.transform.position - c.transform.forward * 0.8f + Vector3.up * 0.4f;
            int boxes = 0;
            foreach (var kv in counts)
            {
                int perBox = Mathf.Max(1, RestockManager.GetMaxItemCountInBox(kv.Key, isBigBox: false));
                for (int left = kv.Value; left > 0; left -= perBox)
                {
                    var box = RestockManager.SpawnPackageBoxItem(kv.Key, Mathf.Min(left, perBox), isBigBox: false);
                    box.transform.position = at + Vector3.up * (0.35f * boxes++);
                }
            }
        }

        private static void EmptyBag(Customer c)
        {
            c.m_ItemInBagList.Clear();
            c.m_CardInBagList.Clear();
            c.m_CurrentCostTotal = 0f;
            c.m_ShoppingBagTransform.gameObject.SetActive(false);
            c.m_Anim.SetBool("HoldingBag", false);
        }

        private static readonly AccessTools.FieldRef<Customer, int> ScannedCount = AccessTools.FieldRefAccess<Customer, int>("m_ItemScannedCount");
        private static readonly AccessTools.FieldRef<Customer, bool> CheckScanOutOfBound =
            AccessTools.FieldRefAccess<Customer, bool>("m_IsCheckScanItemOutOfBound");
        private static readonly System.Reflection.MethodInfo SetStateMethod = AccessTools.Method(typeof(Customer), "SetState");

        /// <summary>
        /// At the till with their things on the counter and nothing scanned yet. A till manned by a worker doesn't count: the
        /// worker scans on a timer.
        /// </summary>
        public static bool IsWaitingAtTill(Customer c)
        {
            var counter = QueueCounter(c);
            return c.m_CurrentState == ECustomerState.ReadyToPay && AtPayingPosition(c) && counter != null && !counter.IsMannedByNPC() &&
                ScannedCount(c) == 0 && !CheckedOutFlag(c);
        }

        /// <summary>
        /// Steps away from the till before anything was scanned: their things come off the counter into the bag again and the
        /// till is free for the next customer (the steps of OnPayingDone :2677 and DelayRemoveCustomerFromQueue :3046, without
        /// a sale). Follow with <see cref="Abandon"/>. False = not <see cref="IsWaitingAtTill"/>.
        /// </summary>
        public static bool LeaveTill(Customer c)
        {
            Debug.HangWatch.Step("customer leaves the till (LeaveTill)");
            if (!IsWaitingAtTill(c)) return false;
            var counter = QueueCounter(c);
            var bag = c.m_ShoppingBagTransform;
            foreach (var item in c.m_ItemInBagList)
            {
                if (item == null) continue;
                item.m_InteractableScanItem.RegisterScanItem(null, null);
                item.m_InteractableScanItem.enabled = false;
                item.m_Collider.enabled = false;
                item.GetOrCreateRigidbody().isKinematic = true;
                item.transform.parent = bag;
                item.transform.position = bag.position;
                item.gameObject.SetActive(false);
            }
            foreach (var card in c.m_CardInBagList)
            {
                if (card == null) continue;
                card.RegisterScanCard(null, null);
                card.m_Collider.enabled = false;
                card.GetOrCreateRigidbody().isKinematic = true;
                card.transform.parent = bag;
                card.transform.position = bag.position;
                card.gameObject.SetActive(false);
            }
            CheckScanOutOfBound(c) = false;
            AtPayingPosition(c) = false;
            counter.SetPlsaticBagVisibility(isShow: false);
            counter.UpdateCashierCounterState(ECashierCounterState.Idle);
            counter.UpdateCurrentCustomer(null);
            counter.RemoveCustomerFromQueue(c);
            counter.RemoveCurrentCustomerFromQueue();
            SetStateMethod.Invoke(c, new object[] { ECustomerState.Idle });
            return true;
        }

        /// <summary>Customers sent out by WalkOutWithoutPaying whose customer count we took (undone by SendToPay).</summary>
        private static readonly HashSet<Customer> SneakedOut = new HashSet<Customer>();

        /// <summary>
        /// Shoplifting: the customer leaves their till queue and walks out as if they had paid. They are counted out of the
        /// shop here (UpdateCustomerCount, as ExitShop would), so ExitShop skips its sales tally (:2758-2800, it only runs for
        /// a customer it counts out): stolen goods never show as sold.
        /// </summary>
        public static void WalkOutWithoutPaying(Customer c)
        {
            LeaveQueue(c);
            if (!CountUpdated(c))
            {
                CSingleton<CustomerManager>.Instance.UpdateCustomerCount(-1);
                CountUpdated(c) = true;
                SneakedOut.Add(c);
            }
            CheckedOutFlag(c) = true; // ExitShop only lets paid customers leave (:2739)
            CustomerStates.Leave(c);
        }

        /// <summary>
        /// Undoes <see cref="WalkOutWithoutPaying"/>: back in the shop's count and to a till queue with the same bag, through
        /// vanilla's ThinkWantToPay; their sale is tallied normally when they leave after paying.
        /// </summary>
        public static void SendToPay(Customer c)
        {
            Announced.Remove(c);
            if (SneakedOut.Remove(c))
            {
                CSingleton<CustomerManager>.Instance.UpdateCustomerCount(1);
                CountUpdated(c) = false;
            }
            CheckedOutFlag(c) = false;
            ResetPath(c);
            ThinkWantToPayMethod.Invoke(c, null);
        }

        /// <summary>In a till queue or at the till.</summary>
        public static bool IsQueued(Customer c) =>
            c.m_CurrentState == ECustomerState.ReadyToPay || QueueCounter(c)?.m_CustomerListInQueue.Contains(c) == true;

        /// <summary>
        /// Sends the customer out through vanilla's ExitShop (Customer.cs:2732). It sends anyone with unpaid items to the till
        /// instead (ThinkWantToPay :1699); someone queued or at the till is left alone (they would join a queue twice).
        /// True = they are walking out now. Features that send someone out for good use <see cref="CustomerStates.Leave"/>.
        /// </summary>
        public static bool SendOut(Customer c)
        {
            if (IsQueued(c) && !CheckedOutFlag(c)) return false; // paid: still listed for a moment, by the game's own timer
            ResetPath(c);
            ExitShopMethod.Invoke(c, null);
            return c.m_CurrentState == ECustomerState.ExitingShop;
        }

        private static readonly System.Reflection.MethodInfo DetermineShopActionMethod = AccessTools.Method(typeof(Customer), "DetermineShopAction");

        private static readonly AccessTools.FieldRef<Customer, bool> InsideShop = AccessTools.FieldRefAccess<Customer, bool>("m_IsInsideShop");
        private static readonly AccessTools.FieldRef<Customer, Quaternion> TargetRotation =
            AccessTools.FieldRefAccess<Customer, Quaternion>("m_TargetLerpRotation");

        /// <summary>
        /// Free to be drawn into an activity (a chat, gawking, browsing): inside the shop, run by vanilla and between tasks —
        /// idle or on the way to a shelf / the bulk box (DetermineShopAction picks again afterwards). Not while taking
        /// something, queuing or paying, at a table, trading at the counter, leaving, in a tournament or in a mod state.
        /// </summary>
        public static bool IsFree(Customer c)
        {
            if (c == null || !c.m_IsActive || !InsideShop(c) || PausingAction(c) || IsTournamentEntrant(c) || CustomerStates.Of(c) != null) return false;
            switch (c.m_CurrentState)
            {
                case ECustomerState.Idle:
                case ECustomerState.WantToBuyItem:
                case ECustomerState.WalkToShelf:
                case ECustomerState.WantToBuyCard:
                case ECustomerState.WalkToCardShelf:
                case ECustomerState.WantToTakeBulkDonationCard:
                case ECustomerState.WalkToBulkDonationBox:
                    return true;
                default:
                    return false;
            }
        }

        private class GlanceAt
        {
            public Quaternion Before, Look;
            public float Until;
        }

        private static readonly Dictionary<Customer, GlanceAt> Glances = new Dictionary<Customer, GlanceAt>();

        /// <summary>
        /// Looks at a point for a few seconds, then turns back to how they faced before — for customers vanilla runs (seated at
        /// a table, queuing: vanilla sets their facing once and never again, so a plain <see cref="FaceTowards"/> stuck). If
        /// something else turned them meanwhile (they walked off), that wins and nothing is restored.
        /// </summary>
        public static void Glance(Customer c, Vector3 point, float seconds = 3f)
        {
            var d = point - c.transform.position;
            d.y = 0f;
            if (d.sqrMagnitude < 0.0001f) return;
            if (!Glances.TryGetValue(c, out var g)) Glances[c] = g = new GlanceAt { Before = TargetRotation(c) };
            g.Look = Quaternion.LookRotation(d, Vector3.up);
            g.Until = Time.time + seconds;
            TargetRotation(c) = g.Look;
        }

        private static void EndGlance(Customer c)
        {
            if (Glances.Count == 0 || !Glances.TryGetValue(c, out var g) || Time.time < g.Until) return;
            Glances.Remove(c);
            if (Quaternion.Angle(TargetRotation(c), g.Look) < 1f) TargetRotation(c) = g.Before;
        }

        /// <summary>Turns the customer towards a point and leaves them so (for customers in a mod state; others: <see cref="Glance"/>).</summary>
        public static void FaceTowards(Customer c, Vector3 point)
        {
            var d = point - c.transform.position;
            d.y = 0f;
            if (d.sqrMagnitude > 0.0001f) TargetRotation(c) = Quaternion.LookRotation(d, Vector3.up);
        }

        /// <summary>
        /// For a customer in a mod state who reached their walk's end: steps straight towards <paramref name="other"/> at a
        /// walking pace until <paramref name="distance"/> metres away (vanilla's path follower calls a walk done a little before
        /// its end, so face-to-face activities closed the last gap themselves). Animates through vanilla's Update (MoveSpeed
        /// from the position change). True = close enough.
        /// </summary>
        public static bool StepCloser(Customer c, Vector3 other, float distance)
        {
            var to = other - c.transform.position;
            to.y = 0f;
            float gap = to.magnitude - distance;
            if (gap <= 0.05f) return true;
            c.transform.position += to.normalized * Mathf.Min(gap, 1.1f * Time.deltaTime);
            FaceTowards(c, other);
            return false;
        }

        /// <summary>Stops where they are (drops the path; for customers in a mod state, from a Tick — not inside Arrived).</summary>
        public static void Stand(Customer c) => ResetPath(c);

        /// <summary>Debug panel (UI/CustomerDebugPanel): what drives the customer now and where they are going.</summary>
        public static void StateRows(Customer c, List<(string label, string value)> rows)
        {
            var mod = CustomerStates.Of(c);
            rows.Add(("game state", mod != null ? $"<color=#7CC8FF>mod: {mod.State.Id}</color> for {mod.Elapsed:0.0} s" : c.m_CurrentState.ToString()));
            if (PausingAction(c)) rows.Add(("paused", "yes (vanilla pause flag)"));
            var target = c.m_TargetTransform;
            if (target != null)
            {
                float d = Vector3.Distance(c.transform.position, target.position);
                string path = WaitingForPath(c) ? "finding a path" : CurrentPath(c) == null ? "no path" : ReachedEnd(c) ? "arrived" : "walking";
                rows.Add(("heading to", $"{target.name} · {d:0.0} m · {path}"));
            }
            if (IsStinky(c)) rows.Add(("smelly", "yes"));
        }

        /// <summary>Debug panel: queue, till and bag.</summary>
        public static void TillRows(Customer c, List<(string label, string value)> rows)
        {
            var counter = QueueCounter(c);
            int queue = counter == null ? -1 : counter.m_CustomerListInQueue.IndexOf(c);
            string where = AtPayingPosition(c) ? "at the till" : queue >= 0 ? $"queue spot {queue + 1} of {counter.m_CustomerListInQueue.Count}" : "not queued";
            if (IsWaitingAtTill(c)) where += " · waiting, nothing scanned";
            else if (IsWaitingInQueue(c)) where += " · waiting";
            rows.Add(("till", where));
            if (AtPayingPosition(c) || ScannedCount(c) > 0) rows.Add(("scanned", $"{ScannedCount(c)} of {c.m_ItemInBagList.Count + c.m_CardInBagList.Count}"));
            rows.Add(("bag", $"{c.m_ItemInBagList.Count} item(s), {c.m_CardInBagList.Count} card(s) · ${c.m_CurrentCostTotal:0.00} of ${c.m_MaxMoney:0} budget"));
            rows.Add(("paid", CheckedOutFlag(c) ? $"yes, ${PaidThisVisit(c):0.00}" : "no"));
        }

        private static readonly System.Reflection.MethodInfo CustomerEnterShopMethod = AccessTools.Method(typeof(CustomerManager), "CustomerEnterShop");

        /// <summary>One more visitor walks in now: vanilla's CustomerManager.CustomerEnterShop (:461, respects the customer limit).</summary>
        public static void SpawnVisitor()
        {
            var cm = CSingleton<CustomerManager>.Instance;
            if (cm != null && CustomerEnterShopMethod != null) CustomerEnterShopMethod.Invoke(cm, null);
        }

        /// <summary>Back to shopping: vanilla's DetermineShopAction (:676) picks what they do next.</summary>
        public static void Shop(Customer c)
        {
            ResetPath(c);
            DetermineShopActionMethod.Invoke(c, null);
        }

        private static readonly AccessTools.FieldRef<Customer, Pathfinding.Path> CurrentPath =
            AccessTools.FieldRefAccess<Customer, Pathfinding.Path>("m_Path");
        private static readonly AccessTools.FieldRef<Customer, bool> ReachedEnd = AccessTools.FieldRefAccess<Customer, bool>("m_ReachedEndOfPath");
        private static readonly AccessTools.FieldRef<Customer, bool> WaitingForPath = AccessTools.FieldRefAccess<Customer, bool>("m_IsWaitingForPathCallback");

        /// <summary>
        /// Drops the path the customer is walking. Vanilla calls its entry methods (ExitShop, ThinkWantToPay, …) only on a
        /// customer standing at a path end; on one still walking, the old path's end fires OnReachedPathEnd in the new state
        /// (an exiting customer far from the exit = "the door is blocked" → back to shopping, :1750). Call before any entry.
        /// </summary>
        public static void ResetPath(Customer c)
        {
            CurrentPath(c) = null;
            ReachedEnd(c) = false;
            WaitingForPath(c) = false;
        }

        /// <summary>
        /// Walks the customer to <paramref name="target"/> with vanilla's own path follower (Customer.Update :3737-3788) and
        /// path callback, as its entry methods do (e.g. ExitShop :2745-2747). For customers in a <see cref="ModState"/>.
        /// </summary>
        public static void WalkTo(Customer c, Transform target)
        {
            ResetPath(c);
            c.m_TargetTransform = target;
            WaitingForPath(c) = true;
            c.m_Seeker.StartPath(c.transform.position, target.position, c.OnPathComplete);
        }

        private static void Raise<T>(Action<T> handlers, T arg, string what)
        {
            if (handlers == null) return;
            foreach (Action<T> h in handlers.GetInvocationList())
            {
                try { h(arg); }
                catch (Exception e) { Plugin.Log.LogError($"Customer {what} handler failed: {e}"); }
            }
        }

        private static void Raise<T1, T2>(Action<T1, T2> handlers, T1 a, T2 b, string what)
        {
            if (handlers == null) return;
            foreach (Action<T1, T2> h in handlers.GetInvocationList())
            {
                try { h(a, b); }
                catch (Exception e) { Plugin.Log.LogError($"Customer {what} handler failed: {e}"); }
            }
        }

        private static void Raise<T1, T2, T3>(Action<T1, T2, T3> handlers, T1 a, T2 b, T3 c, string what)
        {
            if (handlers == null) return;
            foreach (Action<T1, T2, T3> h in handlers.GetInvocationList())
            {
                try { h(a, b, c); }
                catch (Exception e) { Plugin.Log.LogError($"Customer {what} handler failed: {e}"); }
            }
        }

        // ---- Visit lifecycle

        [HarmonyPatch(typeof(Customer), nameof(Customer.ActivateCustomer))]
        private static class ActivatePatch
        {
            // [Gameplay] StinkyCustomers off: the stinky roll (Customer.cs:381) only runs with canSpawnSmelly set.
            // Saved ones are restored by LoadCustomerSaveData / the play table and stay.
            private static void Prefix(ref bool canSpawnSmelly)
            {
                if (!Plugin.StinkyCustomers.Value) canSpawnSmelly = false;
            }

            private static void Postfix(Customer __instance)
            {
                Announced.Remove(__instance);
                SneakedOut.Remove(__instance);
                Raise(Activated, __instance, "activate");
            }
        }

        [HarmonyPatch(typeof(Customer), nameof(Customer.LoadCustomerSaveData))]
        private static class LoadPatch
        {
            private static void Postfix(Customer __instance, CustomerSaveData data) =>
                Raise(Loaded, __instance, CPlayerData.m_CustomerSaveDataList.IndexOf(data), "load");
        }

        [HarmonyPatch(typeof(Customer), nameof(Customer.DeactivateCustomer))]
        private static class DeactivatePatch
        {
            // DeactivateCustomer also sends a tournament entrant's review (:501).
            private static void Prefix(Customer __instance)
            {
                _reviewer = __instance;
                Announced.Remove(__instance);
                SneakedOut.Remove(__instance);
                if (__instance.m_IsActive) Raise(Leaving, __instance, "leave");
                NextSlowTick.Remove(__instance);
                Glances.Remove(__instance);
                ForgetSources(__instance);
            }

            private static void Finalizer() => _reviewer = null;
        }

        [HarmonyPatch(typeof(CustomerManager), nameof(CustomerManager.SaveCustomerData))]
        private static class SaveListPatch
        {
            private static void Prefix() => SavedBy.Clear();

            private static void Postfix()
            {
                var list = new List<Customer>();
                foreach (var data in CPlayerData.m_CustomerSaveDataList)
                    list.Add(data != null && SavedBy.TryGetValue(data, out var c) ? c : null);
                SavedBy.Clear();
                Raise(Snapshot, list, "snapshot");
            }
        }

        // SaveCustomerData asks each active customer for its CustomerSaveData and keeps some of them (CustomerManager.cs:1211-1225).
        // We remember which customer made which entry instead of copying its filter.
        private static readonly Dictionary<CustomerSaveData, Customer> SavedBy = new Dictionary<CustomerSaveData, Customer>();

        [HarmonyPatch(typeof(Customer), nameof(Customer.GetCustomerSaveData))]
        private static class SaveDataPatch
        {
            private static void Postfix(Customer __instance, CustomerSaveData __result)
            {
                if (__result != null) SavedBy[__result] = __instance;
            }
        }

        // ---- Shopping: GetCustomerBuyItemChance knows only prices, so the shelf methods that call it say who is asking.

        [ThreadStatic] private static BuyContext _buying;
        [ThreadStatic] private static InteractableCardCompartment _takingFrom;

        // Where each unpaid bag item came from, so it can go back (Abandon). Pooled objects: forgotten once paid or gone.
        private static readonly Dictionary<Item, ShelfCompartment> ItemSource = new Dictionary<Item, ShelfCompartment>();
        private static readonly Dictionary<InteractableCard3d, InteractableCardCompartment> CardSource =
            new Dictionary<InteractableCard3d, InteractableCardCompartment>();

        private static void ForgetSources(Customer c)
        {
            foreach (var item in c.m_ItemInBagList) if (item != null) ItemSource.Remove(item);
            foreach (var card in c.m_CardInBagList) if (card != null) CardSource.Remove(card);
        }

        [HarmonyPatch(typeof(Customer), "TakeItemFromShelf")]
        private static class TakeItemPatch
        {
            private static void Prefix(Customer __instance, out (int bag, ShelfCompartment from) __state)
            {
                _buying = new BuyContext { Customer = __instance };
                __state = (__instance.m_ItemInBagList.Count, CurrentItemCompartment(__instance));
            }

            private static void Postfix(Customer __instance, (int bag, ShelfCompartment from) __state)
            {
                if (__state.from == null) return;
                for (int i = __state.bag; i < __instance.m_ItemInBagList.Count; i++)
                    if (__instance.m_ItemInBagList[i] != null) ItemSource[__instance.m_ItemInBagList[i]] = __state.from;
            }

            private static void Finalizer() => _buying = null;
        }

        [HarmonyPatch(typeof(Customer), "TakeCardFromShelf")]
        private static class TakeCardPatch
        {
            private static void Prefix(Customer __instance, out int __state)
            {
                __state = __instance.m_CardInBagList.Count;
                CardData card = null;
                var compartment = CurrentCardCompartment(__instance);
                _takingFrom = compartment;
                if (CurrentCardShelf(__instance) != null && compartment != null && compartment.m_StoredCardList.Count > 0)
                    card = compartment.m_StoredCardList[0]?.m_Card3dUI?.m_CardUI?.GetCardData();
                _buying = new BuyContext { Customer = __instance, IsCard = true, Card = card };
            }

            private static void Postfix(Customer __instance, int __state)
            {
                var ctx = _buying;
                if (__instance.m_CardInBagList.Count > __state && _takingFrom != null)
                    CardSource[__instance.m_CardInBagList[__instance.m_CardInBagList.Count - 1]] = _takingFrom;
                if (ctx?.Card != null && __instance.m_CardInBagList.Count > __state)
                    Raise(CardBought, __instance, ctx.Card, ctx.Price, "card bought");
            }

            private static void Finalizer()
            {
                _buying = null;
                _takingFrom = null;
            }
        }

        [HarmonyPatch(typeof(CustomerManager), nameof(CustomerManager.GetCustomerBuyItemChance))]
        private static class BuyChancePatch
        {
            private static void Postfix(float currentPrice, float marketPrice, ref int __result)
            {
                var ctx = _buying;
                if (ctx == null) return;
                ctx.Price = currentPrice;
                ctx.Market = marketPrice;
                int chance = __result;
                foreach (var f in BuyChance)
                {
                    try { chance = f(ctx, chance); }
                    catch (Exception e) { Plugin.Log.LogError($"Buy chance modifier failed: {e}"); }
                }
                __result = Math.Max(0, Math.Min(100, chance));
                Raise(PriceJudged, ctx, __result, "price judged");
            }
        }

        /// <summary>
        /// Vanilla's "done shopping?" decision comes 1.5 s after taking something (Customer.DecideIfFinishShopping) and doesn't look at what they're doing by
        /// then. Someone drawn into a mod state meanwhile (a chat) would join a till queue from it, keep the spot and never walk there: everyone queues behind
        /// nobody. They decide again when the mod state hands them back.
        /// </summary>
        private static bool HeldByModState(Customer c) => CustomerStates.Of(c) != null;

        [HarmonyPatch(typeof(Customer), "DetermineShopAction")]
        private static class DetermineShopActionPatch
        {
            private static bool Prefix(Customer __instance) => !HeldByModState(__instance);
        }

        [HarmonyPatch(typeof(Customer), "ThinkWantToPay")]
        private static class ThinkWantToPayPatch
        {
            // ThinkWantToPay (:1699) sets WantToPay, then AttemptFindQueue joins a till queue (QueuingToPay) — the normal case;
            // WantToPay stays only when no till has room. Checking WantToPay alone missed nearly every customer (no thefts).
            private static bool Prefix(Customer __instance) => !HeldByModState(__instance);

            private static void Postfix(Customer __instance)
            {
                var s = __instance.m_CurrentState;
                if (!CheckedOutFlag(__instance) && (s == ECustomerState.WantToPay || s == ECustomerState.QueuingToPay))
                    Raise(HeadingToPay, __instance, "heading to pay");
            }
        }

        [HarmonyPatch(typeof(Customer), nameof(Customer.Update))]
        private static class TickPatch
        {
            private static void Postfix(Customer __instance)
            {
                if (!__instance.m_IsActive) return;
                EndGlance(__instance);
                Debug.HangWatch.Step("customer tick");
                using var measured = Debug.SpikeWatch.Measure("customer ticks");
                Run(Tick, __instance);
                if (SlowTick.Count == 0) return;
                float now = Time.time;
                if (!NextSlowTick.TryGetValue(__instance, out float next))
                {
                    NextSlowTick[__instance] = now + UnityEngine.Random.value * SlowTickSeconds; // spread customers out
                    return;
                }
                if (now < next) return;
                NextSlowTick[__instance] = now + SlowTickSeconds;
                Debug.HangWatch.Step("customer slow tick");
                Run(SlowTick, __instance);
            }

            // Vanilla's Update sometimes throws (no stack: it's all one patched method): say who and in what state, once per state.
            private static Exception Finalizer(Exception __exception, Customer __instance)
            {
                if (__exception != null && Plugin.CustomerDebug.Value && ThrewIn.Add(__instance.m_CurrentState))
                    Plugin.Log.LogWarning($"Customers: Customer.Update threw {__exception.GetType().Name} in state {__instance.m_CurrentState} (active {__instance.m_IsActive}, target {(__instance.m_TargetTransform != null ? __instance.m_TargetTransform.name : "none")})");
                return __exception;
            }

            private static readonly HashSet<ECustomerState> ThrewIn = new HashSet<ECustomerState>();

            private static void Run(List<Action<Customer>> handlers, Customer c)
            {
                for (int i = 0; i < handlers.Count; i++)
                {
                    try { handlers[i](c); }
                    catch (Exception e) { Plugin.Log.LogError($"Customers: tick handler failed: {e}"); }
                }
            }
        }

        [HarmonyPatch(typeof(Customer), "OnCustomerReachInsideShop")]
        private static class EnteredPatch
        {
            private static void Postfix(Customer __instance) => Raise(EnteredShop, __instance, "entered");
        }

        [HarmonyPatch(typeof(Customer), "OnPayingDone")]
        private static class PaidPatch
        {
            private static void Postfix(Customer __instance)
            {
                ForgetSources(__instance);
                Raise(CheckedOut, __instance, "checkout");
            }
        }

        // ---- Leaving and reviews: ExitShop sends the visit's review (:2866), then walks to the exit (state ExitingShop).

        [ThreadStatic] private static Customer _reviewer;

        /// <summary>Customers whose current walk to the exit was announced (ExitingShop); cleared when they turn back or leave.</summary>
        private static readonly HashSet<Customer> Announced = new HashSet<Customer>();

        [HarmonyPatch(typeof(Customer), "ExitShop")]
        private static class ExitShopPatch
        {
            private static void Prefix(Customer __instance, out bool __state)
            {
                __state = __instance.m_CurrentState == ECustomerState.ExitingShop;
                _reviewer = __instance;
            }

            // ExitShop can run inside itself (unpaid → ThinkWantToPay → a handler sends them out after all): announce once.
            private static void Postfix(Customer __instance, bool __state)
            {
                if (!__state && __instance.m_CurrentState == ECustomerState.ExitingShop && Announced.Add(__instance))
                {
                    Raise(ExitingShop, __instance, "exiting");
                    Raise(ExitDecided, __instance, "exit decided");
                }
            }

            private static void Finalizer() => _reviewer = null;
        }

        [HarmonyPatch(typeof(Customer), nameof(Customer.ForceEndTournamentEarly))]
        private static class ForceEndPatch
        {
            private static void Prefix(Customer __instance) => _reviewer = __instance;
            private static void Finalizer() => _reviewer = null;
        }

        [HarmonyPatch(typeof(CustomerReviewManager), nameof(CustomerReviewManager.AddCustomerReview))]
        private static class ReviewPatch
        {
            private static void Prefix(ref int higherStarChanceAdd, out int __state)
            {
                __state = CPlayerData.m_CustomerReviewCount;
                var c = _reviewer;
                if (c == null) return;
                foreach (var f in ReviewStars)
                {
                    try { higherStarChanceAdd = f(c, higherStarChanceAdd); }
                    catch (Exception e) { Plugin.Log.LogError($"Review modifier failed: {e}"); }
                }
            }

            // Vanilla adds the review last to CPlayerData.m_CustomerReviewDataList and counts it (:126-137), or drops it.
            private static void Postfix(int __state)
            {
                var c = _reviewer;
                var list = CPlayerData.m_CustomerReviewDataList;
                if (c == null || CPlayerData.m_CustomerReviewCount == __state || list.Count == 0) return;
                Raise(ReviewAdded, c, list[list.Count - 1], "review");
            }
        }
    }
}
