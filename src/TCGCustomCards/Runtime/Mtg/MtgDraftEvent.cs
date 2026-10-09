using System;
using System.Collections.Generic;
using System.Linq;
using Newtonsoft.Json;
using TCGCustomCards.Hooks;
using TCGCustomCards.UI;
using UnityEngine;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>Saved state of today's Magic: Draft / Sealed tournament (side-car <c>draftEvent</c>; entrant ids = vanilla tournament index).</summary>
    internal sealed class DraftEventSave
    {
        /// <summary>Draft tables (pods). Sealed: one group with everyone.</summary>
        [JsonProperty("pods")] public List<List<int>> Pods = new List<List<int>>();
        /// <summary>Pack keys each entrant opens (taken from the stock at day start).</summary>
        [JsonProperty("packs")] public Dictionary<int, List<string>> Packs = new Dictionary<int, List<string>>();
        /// <summary>
        /// The cards in each entrant's packs, rolled once at day start (per pack, the exact copies), so leaving and redoing a
        /// draft — or reloading — opens the same packs.
        /// </summary>
        [JsonProperty("rolled")] public Dictionary<int, List<List<DraftCopySave>>> Rolled = new Dictionary<int, List<List<DraftCopySave>>>();
        /// <summary>Entrants who paid for their packs at registration.</summary>
        [JsonProperty("paid")] public List<int> Paid = new List<int>();
        /// <summary>Customers' decks (with the copies in them, for the table's card faces).</summary>
        [JsonProperty("decks")] public Dictionary<int, DraftDeckSave> Decks = new Dictionary<int, DraftDeckSave>();
        /// <summary>The player's built deck + the copies in it (null until built).</summary>
        [JsonProperty("player")] public DraftDeckSave Player;
    }

    internal sealed class DraftDeckSave
    {
        [JsonProperty("deck")] public List<DraftLineSave> Deck = new List<DraftLineSave>();
        [JsonProperty("rating")] public double Rating;
        [JsonProperty("colors")] public string Colors = "";
        /// <summary>The seat's own copies in the deck (set id, card id, border, foil); free basics aren't copies.</summary>
        [JsonProperty("copies")] public List<DraftCopySave> Copies;
    }

    internal sealed class DraftLineSave
    {
        [JsonProperty("n")] public int N;
        [JsonProperty("name")] public string Name;
        [JsonProperty("set")] public string Set;
        [JsonProperty("type")] public string Type;
    }

    internal sealed class DraftCopySave
    {
        [JsonProperty("set")] public string Set;
        [JsonProperty("card")] public string Card;
        [JsonProperty("border")] public int Border;
        [JsonProperty("foil")] public bool Foil;
    }

    /// <summary>
    /// Magic: Draft and Magic: Sealed tournaments (user decisions 2026-10-09, plan okay-i-d-like-to-cheeky-rabin.md). The player puts
    /// packs into the virtual stock (<see cref="ShopEvents.Stock"/>: right-click the Tournament Prize Shelf holding them); their
    /// sets are the event's sets; hosting needs capacity × packs per player. At day start every seat (the player's too) gets its
    /// packs from the stock and they are rolled once (saved). Entrants pay your pack price on top of the fee when they register.
    /// <b>Draft</b>: pods of ≤ [MTG - Draft] PodSize; pods without the player are drafted by Forge right away, the player's pod
    /// drafts live at the player's table. <b>Sealed</b>: Forge builds every customer's deck from their own packs right away; the
    /// player builds from theirs at their table. Customer matches wait until every deck exists, then follow the decks' Forge
    /// ratings; the player's matches are real MTG games against that customer's deck (their own copies on the table).
    /// </summary>
    internal sealed class MtgDraftEvent : IShopEventFormat
    {
        public const string DraftId = "mtg-draft", SealedId = "mtg-sealed";
        public static readonly MtgDraftEvent Draft = new MtgDraftEvent(sealedFormat: false);
        public static readonly MtgDraftEvent Sealed = new MtgDraftEvent(sealedFormat: true);

        public readonly bool IsSealed;
        public string Id => IsSealed ? SealedId : DraftId;
        private string Word => IsSealed ? "sealed" : "draft";

        private MtgDraftEvent(bool sealedFormat) => IsSealed = sealedFormat;

        /// <summary>The hosted draft/sealed event's logic, or null.</summary>
        private static MtgDraftEvent Active => ShopEvents.Handler as MtgDraftEvent;

        private sealed class Job
        {
            public List<int> Members;
            public int HumanSeat = -1;
            public string Name;
        }

        private static DraftEventSave _state;
        private static readonly Queue<Job> Jobs = new Queue<Job>(); // customers' drafts/builds waiting for Forge
        private static bool _busy;                                   // Forge is on one of ours now
        private const string ClockReason = "draft";

        public static void Register()
        {
            const string common =
                "• Put up packs: hold MTG packs and right-click the Tournament Prize Shelf. Only entrants get them; they never go on sale.\n" +
                "• Their sets are the tournament's sets. You need enough packs for every player (you too) to confirm.\n" +
                "• Before confirming, \"Take back\" returns the packs as delivered boxes. After confirming they're locked: you can add more until the tournament day, but not take any back.\n" +
                "• Cancelling the tournament returns every pack as delivered boxes (one box per pack type, up to 32 each).\n" +
                "• On the day each player gets their packs at random from the stock. Entrants pay the entry fee + their packs at your shelf price when they register.\n" +
                "• Packs left over (extra stock, players who never showed up) come back as delivered boxes when the tournament ends.\n";
            foreach (var e in new[] { Draft, Sealed })
            {
                ShopEvents.Register(new ShopEventFormat
                {
                    Id = e.Id,
                    Name = e.IsSealed ? "Magic: Sealed" : "Magic: Draft",
                    Description = e.IsSealed
                        ? "Everyone builds a 40-card deck from their own packs (no picking). Put up packs at the Tournament Prize Shelf."
                        : "Booster draft with packs you put up: hold MTG packs and right-click the Tournament Prize Shelf.",
                    Available = () => MtgMode.Enabled && MtgSession.CanPlayInGame,
                    Unavailable = "Needs MTG mode: TCG Studio > Setup > Install MTG mode, and [MTG] MtgMode on.",
                    SetEligible = s => MtgEvents.IsMtgSet(s) && Registry.Packs.Any(p => p.Set == s && p.PackItem != EItemType.None),
                    UsesPacks = true,
                    SetsFromStock = true,
                    DefaultPacksPerPlayer = e.IsSealed ? 6 : 3,
                    Help = common + (e.IsSealed
                        ? "• Go to your table to open your packs and build your deck; your whole pool is yours to keep. Customers build theirs right away; matches start once you've built yours."
                        : "• Go to your table to draft with your table's players; the cards you pick are yours to keep. Customers' matches start once everyone has drafted."),
                    Problem = Problem,
                    Info = Info,
                    Logic = e,
                });
            }
            PlayerItems.PackTargetLabel = DepositLabel;
            PlayerItems.PackTargetRun = Deposit;
        }

        // ------------------------------------------------------------------ hosting: picks per player, stock

        /// <summary>The pack kinds in the stock (they decide the event's sets and the cards per player).</summary>
        private static IEnumerable<CustomPack> PacksOf(ShopEventConfig c) =>
            ShopEvents.Stock.Where(kv => kv.Value > 0).Select(kv => ShopEvents.PackOf(kv.Key)).Where(p => p != null && p.PackItem != EItemType.None);

        /// <summary>Packs that can go into the stock: any pack of an installed MTG set.</summary>
        private static bool Eligible(CustomPack p) => p != null && MtgEvents.IsMtgSet(p.Set);

        private static int CardsIn(CustomPack p) => p.Def.Slots.Count > 0 ? p.Def.Slots.Sum(s => s.Count) : p.Def.CardsPerPack;

        /// <summary>Cards each player gets in the worst case (smallest pack kind in the stock).</summary>
        private static int MinPicks(ShopEventConfig c)
        {
            var packs = PacksOf(c).ToList();
            return packs.Count == 0 ? 0 : c.packsPerPlayer * packs.Min(CardsIn);
        }

        private static int Needed(ShopEventConfig c, int capacity) => capacity * c.packsPerPlayer;

        private static int StockFor(ShopEventConfig c) => PacksOf(c).Sum(p => ShopEvents.Stock.TryGetValue(p.ItemKey(false), out int n) ? n : 0);

        private static string Problem(ShopEventConfig c, int capacity)
        {
            int min = Plugin.MtgDraftMinPicks?.Value ?? 23;
            int picks = MinPicks(c);
            if (picks < min)
            {
                int size = PacksOf(c).Select(CardsIn).DefaultIfEmpty(1).Min();
                return $"Only {picks} cards per player (smallest pack: {size} cards) - at least {min} are needed: use {(min + size - 1) / size}+ packs per player";
            }
            int have = StockFor(c), need = Needed(c, capacity);
            if (have < need) return $"Put up {need - have} more pack(s): {have}/{need} in the tournament stock ({capacity} players × {c.packsPerPlayer})";
            return null;
        }

        private static string Info(ShopEventConfig c, int capacity)
        {
            var lines = new List<string> { $"{MinPicks(c)} cards per player (packs × smallest pack)" };
            // What an entrant pays: vanilla's entry fee + their packs at your shelf price, charged at registration (user 2026-10-09:
            // keep them separate but show the total). Average over the stock: which packs each entrant gets is random.
            var priced = PacksOf(c).Select(p => (price: CPlayerData.GetItemPrice(p.PackItem), n: ShopEvents.Stock.TryGetValue(p.ItemKey(false), out int k) ? k : 0))
                .Where(x => x.n > 0).ToList();
            if (priced.Count > 0)
            {
                float avg = priced.Sum(x => x.price * x.n) / priced.Sum(x => x.n);
                float fee = ShopEvents.EntryFee, packs = avg * c.packsPerPlayer;
                lines.Add($"Entrants pay: fee {GameInstance.GetPriceString(fee)} + {c.packsPerPlayer} packs × ~{GameInstance.GetPriceString(avg)}" +
                          $" = <b>{GameInstance.GetPriceString(fee + packs)}</b> each <color=#888888>(packs at your shelf price, paid at registration)</color>");
            }
            int have = StockFor(c), need = Needed(c, capacity);
            lines.Add($"Tournament stock: <b>{have}/{need}</b> packs" + (have >= need ? "" : " - hold packs and right-click the Tournament Prize Shelf"));
            foreach (var kv in ShopEvents.Stock.OrderBy(k => k.Key))
            {
                var p = ShopEvents.PackOf(kv.Key);
                string name = p == null ? kv.Key : $"{p.Set.Def.Name ?? p.Set.Def.Id}: {p.Def.Name ?? p.Def.Id}";
                lines.Add($"   {kv.Value} × {name}");
            }
            return string.Join("\n", lines);
        }

        private static bool IsOurs(string format) => format == DraftId || format == SealedId;

        /// <summary>The event (pending on the host screen, or hosted) whose stock right-clicks feed; null = none.</summary>
        private static ShopEventConfig DepositTarget()
        {
            var t = CPlayerData.m_TournamentData;
            if (t.m_IsTournamentDay) return null; // locked once the day starts
            if (t.m_IsHostingTournament) return IsOurs(ShopEvents.Config?.format) ? ShopEvents.Config : null;
            return IsOurs(ShopEvents.Pending?.format) ? ShopEvents.Pending : null;
        }

        private static bool Wanted(EItemType t) => Eligible(Registry.GetByItem(t));

        private static string DepositLabel(Transform hit)
        {
            if (hit.GetComponentInParent<TournamentPrizeShelf>() == null) return null;
            if (DepositTarget() == null) return null;
            int n = PlayerItems.HeldCount(Wanted);
            return n > 0 ? $"Add {n} pack(s) to the tournament stock" : null;
        }

        private static void Deposit(Transform hit)
        {
            var c = DepositTarget();
            if (c == null) return;
            var taken = PlayerItems.ConsumeHeld(Wanted, int.MaxValue);
            foreach (var t in taken) ShopEvents.AddStock(Registry.GetByItem(t).ItemKey(false), 1);
            c.sets = ShopEvents.StockSetIds(); // the stock decides the event's sets (also when adding while hosting)
            SoundManager.GenericPop();
            Toast.Show($"{taken.Count} pack(s) added - tournament stock: {StockFor(c)}/{Needed(c, ShopEvents.Capacity)}");
            Plugin.Log.LogInfo($"Tournament stock +{taken.Count}: {string.Join(", ", ShopEvents.Stock.Select(kv => kv.Key + " x" + kv.Value))}");
        }

        // ------------------------------------------------------------------ the day

        /// <summary>Today's draft/sealed tournament is running and the player plays in it.</summary>
        public static bool PlayerInEvent => Active != null && ShopEvents.EventRunning && CPlayerData.m_IsPlayerRegisteredForTournament && _state != null;

        public static bool PlayerDeckReady => _state?.Player != null;

        private static int PlayerId => ShopEvents.PlayerEntrantId;

        public void OnEntrantsSpawned()
        {
            _state = new DraftEventSave();
            Jobs.Clear();
            _busy = false;
            var c = ShopEvents.Config;
            var ids = ShopEvents.SortedEntrants().Select(e => e == null ? PlayerId : ShopEvents.EntrantId(e)).ToList();
            foreach (int id in ids)
            {
                var keys = new List<string>();
                var rolled = new List<List<DraftCopySave>>();
                for (int i = 0; i < c.packsPerPlayer; i++)
                {
                    string k = ShopEvents.TakeRandomPack();
                    var pack = k == null ? null : ShopEvents.PackOf(k);
                    if (pack == null) continue;
                    keys.Add(k);
                    // Rolled once, now: the same packs whenever this seat's draft/build (re)starts.
                    rolled.Add((MtgDraft.Roll(pack.PackItem) ?? new List<DraftCard>()).Select(d => SaveCopy(d.Data)).Where(x => x != null).ToList());
                }
                _state.Packs[id] = keys;
                _state.Rolled[id] = rolled;
            }
            if (IsSealed) _state.Pods.Add(ids); // no tables to draft at: one group
            else
            {
                // Pods: as even as possible, at most PodSize each, in standing order.
                int podSize = Math.Max(2, Plugin.MtgDraftPodSize?.Value ?? 8);
                int pods = Math.Max(1, (ids.Count + podSize - 1) / podSize);
                for (int p = 0, at = 0; p < pods; p++)
                {
                    int size = ids.Count / pods + (p < ids.Count % pods ? 1 : 0);
                    _state.Pods.Add(ids.Skip(at).Take(size).ToList());
                    at += size;
                }
            }
            Plugin.Log.LogInfo($"{Word} day: {ids.Count} players, groups {string.Join(" / ", _state.Pods.Select(p => p.Count))}, " +
                               $"{c.packsPerPlayer} packs each, stock left {ShopEvents.StockTotal}");
            QueueCustomerJobs();
            if (CPlayerData.m_IsPlayerRegisteredForTournament)
                Toast.Show($"{(IsSealed ? "Sealed" : "Draft")} day! Go to table {CPlayerData.m_PlayerTournamentData.m_TournamentCustomerPlayTableIndex} to open your packs" +
                           (IsSealed ? " and build your deck" : " and draft"));
        }

        /// <summary>Forge's work for the customers: each draft pod without the player; Sealed: every customer (one build).</summary>
        private void QueueCustomerJobs()
        {
            MtgSession.EnsureRunner();
            bool playerIn = CPlayerData.m_IsPlayerRegisteredForTournament;
            if (IsSealed)
            {
                var customers = _state.Pods.SelectMany(p => p).Where(id => !(playerIn && id == PlayerId) && !_state.Decks.ContainsKey(id)).ToList();
                if (customers.Count > 0) Jobs.Enqueue(new Job { Members = customers, Name = "customers' sealed decks" });
                return;
            }
            for (int p = 0; p < _state.Pods.Count; p++)
            {
                var pod = _state.Pods[p];
                if (playerIn && pod.Contains(PlayerId)) continue; // drafts live with the player
                if (pod.Any(id => !_state.Decks.ContainsKey(id))) Jobs.Enqueue(new Job { Members = pod, Name = $"pod {p + 1}" });
            }
        }

        /// <summary>Every frame (MtgSession.Pump): hand the next customer job to Forge when it's free.</summary>
        public static void Tick()
        {
            if (_state == null || Jobs.Count == 0 || _busy || MtgDraft.Running) return;
            var ev = Active;
            if (ev == null || !ShopEvents.EventRunning) { Jobs.Clear(); return; }
            var job = Jobs.Dequeue();
            string err = ev.StartJob(job, e => ev.CustomerJobDone(job, e));
            if (err != null) ev.CustomerJobDone(job, err);
        }

        /// <summary>The job's packs as draft cards (from the saved rolls), [round][seat] for a draft, [seat][card] for sealed.</summary>
        private string StartJob(Job job, Action<string> done)
        {
            string landSet = MtgDraft.LandSetOf(MtgEvents.EventSets());
            var packsBySeat = job.Members.Select(id => (_state.Rolled.TryGetValue(id, out var r) ? r : new List<List<DraftCopySave>>())
                .Select(pack => pack.Select(LoadCopy).Select(MtgDraft.FromData).Where(d => d != null).ToList()).ToList()).ToList();
            string err;
            if (IsSealed)
                err = MtgDraft.StartSealed(packsBySeat.Select(packs => packs.SelectMany(p => p).ToList()).ToList(), job.HumanSeat, landSet, done);
            else
            {
                int rounds = packsBySeat.Max(p => p.Count);
                if (rounds == 0) return "no packs";
                var packs = Enumerable.Range(0, rounds)
                    .Select(r => packsBySeat.Select(seat => r < seat.Count ? seat[r] : new List<DraftCard>()).ToList()).ToList();
                err = MtgDraft.Start(packs, job.HumanSeat, landSet, done);
            }
            _busy = err == null;
            return err;
        }

        private void CustomerJobDone(Job job, string error)
        {
            _busy = false;
            if (_state == null) return;
            if (error != null || MtgDraft.Seats == null)
            {
                Plugin.Log.LogWarning($"{Word} day: Forge couldn't build the {job.Name} ({error}) - those customers play coin-flip matches");
                foreach (int id in job.Members) if (!_state.Decks.ContainsKey(id)) _state.Decks[id] = new DraftDeckSave();
                return;
            }
            StoreCustomerDecks(job);
            Plugin.Log.LogInfo($"{Word} day: {job.Name} done by Forge ({job.Members.Count} customers)");
        }

        private static void StoreCustomerDecks(Job job)
        {
            for (int i = 0; i < job.Members.Count && i < MtgDraft.Seats.Count; i++)
            {
                if (i == job.HumanSeat) continue;
                var seat = MtgDraft.Seats[i];
                _state.Decks[job.Members[i]] = new DraftDeckSave
                {
                    Rating = seat.Rating, Colors = seat.Colors,
                    Deck = (seat.Deck?.Main ?? new List<MtgDeckLine>()).Select(l => new DraftLineSave
                    {
                        N = l.Count, Name = l.Card.Name, Set = l.Card.SetCode, Type = l.Card.TypeLine,
                    }).ToList(),
                    Copies = MtgDraft.DeckCopies(seat, seat.Deck).Select(SaveCopy).Where(x => x != null).ToList(),
                };
            }
        }

        /// <summary>The player right-clicked their tournament table before building: draft with their pod / open their sealed pool.</summary>
        public static void StartPlayerDeck()
        {
            var ev = Active;
            if (ev == null || MtgDraftUI.IsOpen) return;
            if (MtgDraft.Running) { Toast.Show("Forge is still working on another table - try again in a moment"); return; }
            Job job;
            if (ev.IsSealed) job = new Job { Members = new List<int> { PlayerId }, HumanSeat = 0, Name = "your sealed pool" };
            else
            {
                var pod = _state.Pods.FirstOrDefault(p => p.Contains(PlayerId));
                if (pod == null) { Toast.Show("You're not in a draft pod today"); return; }
                job = new Job { Members = pod, HumanSeat = pod.IndexOf(PlayerId), Name = "your pod" };
            }
            string err = ev.StartJob(job, e =>
            {
                _busy = false;
                if (e == null && MtgDraft.Seats != null) StoreCustomerDecks(job); // a draft pod's customers drafted with you
                MtgDraftUI.DraftFinished(e);
            });
            if (err != null) { Toast.Show($"{(ev.IsSealed ? "Sealed" : "Draft")}: {err}"); return; }
            if (Plugin.MtgDraftPausesClock?.Value ?? true) ShopClock.Hold(ClockReason);
            MtgDraftUI.Open((deck, copies) =>
            {
                ShopClock.Release(ClockReason);
                foreach (var c in MtgDraft.Seats[MtgDraft.HumanSeat].Picks) CPlayerData.AddCard(c.Data, 1); // your picks / pool are yours
                _state.Player = new DraftDeckSave
                {
                    Deck = deck.Main.Select(l => new DraftLineSave { N = l.Count, Name = l.Card.Name, Set = l.Card.SetCode, Type = l.Card.TypeLine }).ToList(),
                    Copies = copies.Select(SaveCopy).Where(x => x != null).ToList(),
                };
                Toast.Show($"Deck ready - play your round at table {CPlayerData.m_PlayerTournamentData.m_TournamentCustomerPlayTableIndex}");
                Plugin.Log.LogInfo($"{ev.Word} day: player's deck built ({deck.Total} cards)");
            }, why =>
            {
                ShopClock.Release(ClockReason);
                Toast.Show(why == "aborted" || why == "left"
                    ? "Left - right-click your table to start again (same packs)"
                    : "Stopped: " + why);
            });
        }

        /// <summary>The player's match at their table: their deck vs the waiting customer's deck (both sides' own copies on the table).</summary>
        public static MtgSession.DeckPair PairAgainst(Customer opponent)
        {
            if (_state?.Player == null || opponent == null) return null;
            if (!_state.Decks.TryGetValue(ShopEvents.EntrantId(opponent), out var opp) || opp.Deck.Count == 0) return null;
            var player = ToDeck(_state.Player, ForgeLauncher.DeckPrefix + "My deck");
            var oppDeck = ToDeck(opp, ForgeLauncher.DeckPrefix + "Opponent");
            List<CardData> Copies(DraftDeckSave s) => (s.Copies ?? new List<DraftCopySave>()).Select(LoadCopy).Where(x => x != null).ToList();
            return new MtgSession.DeckPair
            {
                Player = player, Opponent = oppDeck, Faces = new MtgCardFaces(Copies(_state.Player), Copies(opp)),
                OpponentInfo = MtgDraft.DeckInfo(oppDeck, opp.Rating, opp.Colors),
            };
        }

        private static MtgDeck ToDeck(DraftDeckSave s, string name)
        {
            var deck = new MtgDeck { Name = ForgeLauncher.Safe(name) };
            foreach (var l in s.Deck)
                deck.Main.Add(new MtgDeckLine { Count = l.N, Card = new MtgCard { Name = l.Name, SetCode = l.Set, TypeLine = l.Type ?? "" } });
            return deck;
        }

        private static DraftCopySave SaveCopy(CardData c)
        {
            var set = c != null && Registry.IsCustom(c.expansionType) ? Registry.Get(c.expansionType) : null;
            if (set == null || !set.PosByMonster.TryGetValue(c.monsterType, out int pos)) return null;
            return new DraftCopySave { Set = set.Def.Id, Card = set.Card(pos).Id, Border = (int)c.borderType, Foil = c.isFoil };
        }

        private static CardData LoadCopy(DraftCopySave s)
        {
            var set = Registry.Sets.FirstOrDefault(x => x.Def.Id == s.Set);
            int pos = set?.Def.Cards.FindIndex(c => c.Id == s.Card) ?? -1;
            if (pos < 0) return null;
            return new CardData { expansionType = set.Expansion, monsterType = set.Shown[pos], borderType = (ECardBorderType)s.Border, isFoil = s.Foil };
        }

        // ------------------------------------------------------------------ IShopEventFormat

        public bool BeforeRegister(Customer entrant) => false; // packs come from the stock: straight to registration

        public void OnRegistered(Customer entrant)
        {
            if (_state == null) return;
            int id = ShopEvents.EntrantId(entrant);
            if (_state.Paid.Contains(id) || !_state.Packs.TryGetValue(id, out var keys) || keys.Count == 0) return;
            _state.Paid.Add(id);
            float total = 0f;
            foreach (var k in keys)
            {
                var p = ShopEvents.PackOf(k);
                if (p != null) total += CPlayerData.GetItemPrice(p.PackItem);
            }
            if (total <= 0f) return;
            CPlayerData.m_GameReportDataCollect.itemAmountSold += keys.Count;
            CPlayerData.m_GameReportDataCollectPermanent.itemAmountSold += keys.Count;
            PriceChangeManager.AddTransaction(total, ETransactionType.ItemSold, 0);
            CEventManager.QueueEvent(new CEventPlayer_AddCoin(total));
            CSingleton<PricePopupSpawner>.Instance.ShowPricePopup(total, 2.2f, entrant.transform);
            Plugin.Log.LogInfo($"{Word} day: entrant {id} paid {total:0.00} for {keys.Count} pack(s)");
        }

        public bool CanStartMatches => _state != null && _state.Pods.SelectMany(p => p).All(id =>
            id == PlayerId && CPlayerData.m_IsPlayerRegisteredForTournament ? _state.Player != null : _state.Decks.ContainsKey(id));

        public bool? DecideCustomerMatch(Customer seat0, Customer seat1)
        {
            if (_state == null) return null;
            if (!_state.Decks.TryGetValue(ShopEvents.EntrantId(seat0), out var a) || a.Deck.Count == 0) return null;
            if (!_state.Decks.TryGetValue(ShopEvents.EntrantId(seat1), out var b) || b.Deck.Count == 0) return null;
            float k = Plugin.MtgDraftStrengthWeight?.Value ?? 0.015f;
            float p = Mathf.Clamp(0.5f + k * (float)(a.Rating - b.Rating), 0.2f, 0.8f);
            return UnityEngine.Random.value < p;
        }

        public void OnRoundFinished(int nextRound) { }

        public void OnEventEnded(bool forced)
        {
            if (_state != null)
            {
                // Packs of entrants who never registered (day ended early) go back to the stock, which ShopEvents returns.
                bool playerIn = CPlayerData.m_IsPlayerRegisteredForTournament;
                foreach (var kv in _state.Packs)
                    if (!(playerIn && kv.Key == PlayerId) && !_state.Paid.Contains(kv.Key))
                        foreach (var k in kv.Value) ShopEvents.AddStock(k, 1);
            }
            _state = null;
            Jobs.Clear();
            _busy = false;
            ShopClock.Release(ClockReason);
        }

        public float SignUpExtra()
        {
            var c = ShopEvents.Config;
            if (c == null) return 0f;
            // Packs at market price are fair (entrants get the cards); only what you charge above market counts against you.
            var over = ShopEvents.Stock.Select(kv => (p: ShopEvents.PackOf(kv.Key), n: kv.Value)).Where(x => x.p != null)
                .Select(x => (extra: Math.Max(0f, CPlayerData.GetItemPrice(x.p.PackItem) - CPlayerData.GetItemMarketPrice(x.p.PackItem)), x.n)).ToList();
            int n = over.Sum(x => x.n);
            return n == 0 ? 0f : c.packsPerPlayer * over.Sum(x => x.extra * x.n) / n;
        }

        // ------------------------------------------------------------------ side-car

        public static DraftEventSave Export() => _state;

        public static void Load(DraftEventSave saved)
        {
            var ev = Active;
            _state = ev != null && CPlayerData.m_TournamentData.m_IsTournamentDay ? saved : null;
            Jobs.Clear();
            _busy = false;
            if (_state == null) return;
            if (_state.Rolled == null || _state.Rolled.Count == 0)
            {
                Plugin.Log.LogWarning($"{ev.Word} day loaded from an older save without rolled packs - customers' matches fall back to coin flips");
                foreach (int id in _state.Pods.SelectMany(p => p)) if (!_state.Decks.ContainsKey(id)) _state.Decks[id] = new DraftDeckSave();
            }
            ev.QueueCustomerJobs(); // customers Forge hadn't finished when the game was saved
            Plugin.Log.LogInfo($"{ev.Word} day loaded: {_state.Pods.Count} group(s), {_state.Decks.Count} customer deck(s), player deck {(_state.Player != null ? "built" : "not built")}");
        }
    }
}
