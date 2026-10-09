using System;
using System.Collections.Generic;
using System.Linq;
using System.Reflection;
using System.Reflection.Emit;
using HarmonyLib;
using UnityEngine;
using Random = UnityEngine.Random;

namespace TCGCustomCards.Hooks
{
    /// <summary>
    /// The logic of one tournament format (e.g. MTG draft). Every member is optional in spirit: return the "vanilla" answer to
    /// leave the vanilla tournament behaviour in place. Calls happen on the main thread.
    /// </summary>
    internal interface IShopEventFormat
    {
        /// <summary>
        /// One think step of an entrant who hasn't registered yet (it would walk to the prize shelf and pay the fee next).
        /// True = handled this step (the format keeps the entrant busy); false = vanilla goes on to registration. Called again on
        /// every later think until it returns false.
        /// </summary>
        bool BeforeRegister(Customer entrant);
        /// <summary>The entrant paid the fee. Fires once per entrant per day; also once after loading a save mid-event (idempotent).</summary>
        void OnRegistered(Customer entrant);
        /// <summary>False = seated customer pairs at tournament tables wait (their timed match doesn't run).</summary>
        bool CanStartMatches { get; }
        /// <summary>Winner of a customer-vs-customer tournament match: true = seat 0, false = seat 1, null = vanilla coin flip.</summary>
        bool? DecideCustomerMatch(Customer seat0, Customer seat1);
        /// <summary>The day's entrants were spawned (tournament day start; the player's slot is a null entry).</summary>
        void OnEntrantsSpawned();
        /// <summary>Every entrant reported; <paramref name="nextRound"/> (0-based) is about to start.</summary>
        void OnRoundFinished(int nextRound);
        /// <summary>
        /// The event is over: placements given (<paramref name="forced"/> false) or the day ended before it finished. Pack stock
        /// still in <see cref="ShopEvents.Stock"/> afterwards is returned to the player as delivered boxes.
        /// </summary>
        void OnEventEnded(bool forced);
        /// <summary>Added to the entry fee when customers decide whether to sign up (e.g. overpriced packs); 0 = vanilla.</summary>
        float SignUpExtra();
    }

    /// <summary>A tournament format offered on the host screen (vanilla Tetramon = no format).</summary>
    internal sealed class ShopEventFormat
    {
        public string Id, Name, Description;
        /// <summary>False = not offered right now (e.g. Forge not installed); <see cref="Unavailable"/> says why.</summary>
        public Func<bool> Available = () => true;
        public string Unavailable = "";
        /// <summary>Which installed sets can be picked for the event's set list.</summary>
        public Func<Runtime.CustomSet, bool> SetEligible = _ => true;
        public bool UsesPacks;
        /// <summary>The event's sets are the sets of the packs in <see cref="ShopEvents.Stock"/> (no set list on the host screen).</summary>
        public bool SetsFromStock;
        public int MaxPacksPerPlayer = 12;
        /// <summary>Packs per player when the format is picked on the host screen.</summary>
        public int DefaultPacksPerPlayer = 3;
        /// <summary>Format-specific reason the pending event can't be hosted yet (config, capacity), or null.</summary>
        public Func<ShopEventConfig, int, string> Problem;
        /// <summary>"How it works" text for the host panel (collapsible), or null.</summary>
        public string Help;
        /// <summary>Extra lines for the host panel (config, capacity), e.g. "17 cards per player".</summary>
        public Func<ShopEventConfig, int, string> Info;
        /// <summary>Shop-side logic (null = vanilla tournament flow with this format's options).</summary>
        public IShopEventFormat Logic;
    }

    /// <summary>Options of the hosted event beyond vanilla's fee/capacity/prizes (side-car; null = plain vanilla tournament).</summary>
    internal sealed class ShopEventConfig
    {
        /// <summary>Format id registered with <see cref="ShopEvents.Register"/>.</summary>
        public string format;
        /// <summary>Installed set ids usable in this event (1..n).</summary>
        public List<string> sets = new List<string>();
        /// <summary>Draft/sealed: packs each entrant opens.</summary>
        public int packsPerPlayer = 3;
    }

    /// <summary>
    /// Single owner of the vanilla tournament pipeline patches (Customer.DetermineShopAction for entrants,
    /// InteractablePlayTable.Update / StopTableGame, CustomerManager.SpawnGameStartCustomer / OnCustomerFinishTournamentRound /
    /// OnDayStarted). With no <see cref="Config"/> or no format registered for it, every patch is a pass-through. One active format
    /// at a time. See docs/hooks.md for the vanilla lines owned here.
    /// </summary>
    internal static class ShopEvents
    {
        private static readonly List<ShopEventFormat> Formats = new List<ShopEventFormat>();

        /// <summary>The hosted event's options (null = vanilla). Saved in the side-car.</summary>
        public static ShopEventConfig Config;

        public static void Register(ShopEventFormat format)
        {
            Formats.RemoveAll(f => f.Id == format.Id);
            Formats.Add(format);
        }

        public static IReadOnlyList<ShopEventFormat> FormatList => Formats;

        /// <summary>The hosted event's format, or null (vanilla).</summary>
        public static ShopEventFormat CurrentFormat => Config?.format == null ? null : Formats.FirstOrDefault(f => f.Id == Config.format);

        /// <summary>The shop-side logic of the hosted event's format, or null (vanilla behaviour).</summary>
        public static IShopEventFormat Handler => CurrentFormat?.Logic;

        /// <summary>True while an event of <paramref name="formatId"/> is hosted or running.</summary>
        public static bool Is(string formatId) => Config?.format == formatId;

        /// <summary>True on a tournament day that hasn't finished yet.</summary>
        public static bool EventRunning => CPlayerData.m_TournamentData.m_IsTournamentDay && !CPlayerData.m_TournamentData.m_IsTournamentDayOver;

        /// <summary>Entrants in current standing order (sorted index); a null entry is the player.</summary>
        public static List<Customer> SortedEntrants() =>
            Traverse.Create(CSingleton<CustomerManager>.Instance).Field("m_TournamentSortedCustomerList").GetValue<List<Customer>>()
            ?? new List<Customer>();

        // ---- side-car

        public static void Load(ShopEventConfig saved)
        {
            Config = saved;
            _registered.Clear();
            if (Config == null) return;
            Config.sets = (Config.sets ?? new List<string>()).Where(id =>
            {
                bool ok = Runtime.Registry.Sets.Any(s => s.Def.Id == id);
                if (!ok) Plugin.Log.LogWarning($"Shop event: set '{id}' is no longer installed, removed from the event");
                return ok;
            }).ToList();
            if (Config.format != null && CurrentFormat == null)
            {
                Plugin.Log.LogWarning($"Shop event: format '{Config.format}' isn't available in this mod version, the event runs as a vanilla tournament");
                Config = null;
            }
            else if (Config.format != null && Config.sets.Count == 0)
            {
                Plugin.Log.LogWarning($"Shop event: no installed set left for format '{Config.format}', the event runs as a vanilla tournament");
                UI.Toast.Show("Tournament: none of its sets are installed any more, it runs as a normal tournament");
                Config = null;
            }
            else Plugin.Log.LogInfo($"Shop event loaded: format {Config.format ?? "vanilla"}, sets {string.Join(", ", Config.sets)}, packs {Config.packsPerPlayer}");
        }

        public static ShopEventConfig Export() => Config;

        // ---- pack stock (packs the player put up for a pack event: virtual, saved, only entrants get them)

        /// <summary>Pack stock by stable pack key (<see cref="Runtime.CustomPack.ItemKey"/> false). Saved in the side-car.</summary>
        public static readonly Dictionary<string, int> Stock = new Dictionary<string, int>();

        public static int StockTotal => Stock.Values.Sum();

        public static void LoadStock(Dictionary<string, int> saved)
        {
            Stock.Clear();
            foreach (var kv in saved ?? new Dictionary<string, int>())
                if (kv.Value > 0) Stock[kv.Key] = kv.Value;
            if (Stock.Count > 0) Plugin.Log.LogInfo($"Shop event stock loaded: {string.Join(", ", Stock.Select(kv => kv.Key + " x" + kv.Value))}");
        }

        public static Dictionary<string, int> ExportStock() => Stock.Count == 0 ? null : new Dictionary<string, int>(Stock);

        /// <summary>Set ids of the packs in the stock (for <see cref="ShopEventFormat.SetsFromStock"/> formats).</summary>
        public static List<string> StockSetIds() =>
            Stock.Where(kv => kv.Value > 0).Select(kv => PackOf(kv.Key)?.Set.Def.Id).Where(id => id != null).Distinct().ToList();

        public static Runtime.CustomPack PackOf(string key) => Runtime.Registry.Packs.FirstOrDefault(p => p.ItemKey(false) == key);

        public static void AddStock(string key, int n)
        {
            if (n <= 0) return;
            Stock[key] = (Stock.TryGetValue(key, out int c) ? c : 0) + n;
        }

        /// <summary>Takes one random pack (weighted by count, optionally limited to <paramref name="allowed"/> keys); null when empty.</summary>
        public static string TakeRandomPack(Func<string, bool> allowed = null)
        {
            var keys = Stock.Where(kv => kv.Value > 0 && (allowed == null || allowed(kv.Key))).ToList();
            int total = keys.Sum(kv => kv.Value);
            if (total == 0) return null;
            int roll = Random.Range(0, total);
            foreach (var kv in keys)
            {
                if (roll < kv.Value)
                {
                    if (kv.Value == 1) Stock.Remove(kv.Key); else Stock[kv.Key] = kv.Value - 1;
                    return kv.Key;
                }
                roll -= kv.Value;
            }
            return null;
        }

        /// <summary>
        /// Gives the whole stock back as delivered boxes (vanilla RestockManager.SpawnPackageBoxItem, up to 32 packs per small
        /// box, like a restock order). Packs of sets that are no longer installed are dropped (logged).
        /// </summary>
        public static void ReturnStock(string why)
        {
            if (Stock.Count == 0) return;
            int boxes = 0, packs = 0;
            foreach (var kv in Stock.ToList())
            {
                var pack = PackOf(kv.Key);
                if (pack == null || pack.PackItem == EItemType.None)
                {
                    Plugin.Log.LogWarning($"Shop event stock: {kv.Value} pack(s) of '{kv.Key}' can't be returned (set not installed)");
                    continue;
                }
                for (int left = kv.Value; left > 0; left -= 32)
                {
                    RestockManager.SpawnPackageBoxItem(pack.PackItem, Math.Min(32, left), isBigBox: false);
                    boxes++;
                }
                packs += kv.Value;
            }
            Stock.Clear();
            Plugin.Log.LogInfo($"Shop event stock returned ({why}): {packs} pack(s) in {boxes} box(es)");
            if (packs > 0) UI.Toast.Show($"{packs} tournament pack(s) returned as {boxes} delivered box(es)");
        }

        // ---- host screen (format / sets / packs panel, UI/ShopEventHostUI)

        /// <summary>The format picked on the host screen before Confirm (null = Tetramon / vanilla).</summary>
        public static ShopEventConfig Pending = new ShopEventConfig();

        /// <summary>Why the pending choice can't be hosted, or null.</summary>
        public static string PendingProblem()
        {
            if (Pending.format == null) return null;
            var f = Formats.FirstOrDefault(x => x.Id == Pending.format);
            if (f == null) return "Unknown tournament format";
            if (!f.Available()) return f.Unavailable;
            if (f.SetsFromStock) Pending.sets = StockSetIds();
            if (Pending.sets.Count == 0)
                return f.SetsFromStock ? "Put up packs first: hold them and right-click the Tournament Prize Shelf" : "Pick at least one set for this tournament";
            return f.Problem?.Invoke(Pending, Capacity);
        }

        private static HostTournamentScreen _hostScreen;

        /// <summary>Player capacity on the host screen right now (vanilla's private m_CapacityAmount; hosted value while hosting).</summary>
        public static int Capacity
        {
            get
            {
                if (CPlayerData.m_TournamentData.m_IsHostingTournament || _hostScreen == null)
                    return Math.Max(8, CPlayerData.m_TournamentData.m_TournamentMaxPlayerCount);
                return Traverse.Create(_hostScreen).Field("m_CapacityAmount").GetValue<int>();
            }
        }

        /// <summary>Entry fee on the host screen right now (vanilla's private m_PriceSet; the hosted fee while hosting).</summary>
        public static float EntryFee
        {
            get
            {
                if (CPlayerData.m_TournamentData.m_IsHostingTournament || _hostScreen == null) return CPlayerData.m_TournamentData.m_TournamentFee;
                return Traverse.Create(_hostScreen).Field("m_PriceSet").GetValue<float>();
            }
        }

        [HarmonyPatch(typeof(HostTournamentScreen), "OnOpenScreen")]
        private static class HostOpenPatch
        {
            private static void Postfix(HostTournamentScreen __instance)
            {
                _hostScreen = __instance;
                if (Formats.Count == 0) return;
                var t = CPlayerData.m_TournamentData;
                if (t.m_IsHostingTournament || t.m_IsTournamentDay) // read-only: show what's hosted
                    Pending = Config != null ? Copy(Config) : new ShopEventConfig();
                UI.ShopEventHostUI.Open();
            }
        }

        [HarmonyPatch(typeof(HostTournamentScreen), "OnCloseScreen")]
        private static class HostClosePatch
        {
            private static void Postfix() => UI.ShopEventHostUI.Close();
        }

        [HarmonyPatch(typeof(HostTournamentScreen), nameof(HostTournamentScreen.OnPressConfirm))]
        private static class HostConfirmPatch
        {
            private static bool Prefix()
            {
                if (CPlayerData.m_TournamentData.m_IsHostingTournament || CPlayerData.m_TournamentData.m_IsTournamentDay) return true;
                string problem = PendingProblem();
                if (problem == null) return true;
                UI.Toast.Show(problem);
                return false;
            }

            private static void Postfix(bool __runOriginal)
            {
                if (!__runOriginal || !CPlayerData.m_TournamentData.m_IsHostingTournament || CPlayerData.m_TournamentData.m_IsTournamentDay) return;
                Config = Pending.format == null ? null : Copy(Pending);
                Plugin.Log.LogInfo($"Shop event hosted: format {Config?.format ?? "Tetramon"}" +
                                   (Config == null ? "" : $", sets {string.Join(", ", Config.sets)}, packs {Config.packsPerPlayer}"));
            }
        }

        [HarmonyPatch(typeof(HostTournamentScreen), nameof(HostTournamentScreen.ConfirmCancelTournament))]
        private static class HostCancelPatch
        {
            private static void Postfix()
            {
                if (Config != null) Plugin.Log.LogInfo($"Shop event cancelled (format {Config.format})");
                Config = null;
                ReturnStock("tournament cancelled");
            }
        }

        private static ShopEventConfig Copy(ShopEventConfig c) =>
            new ShopEventConfig { format = c.format, sets = new List<string>(c.sets), packsPerPlayer = c.packsPerPlayer };

        /// <summary>Stable id of a tournament entrant for the day (vanilla's unsorted tournament index; sign-ups are anonymous).</summary>
        public static int EntrantId(Customer c) => c.GetCustomerTournamentData().m_TournamentCustomerIndex;

        public static int PlayerEntrantId => CPlayerData.m_PlayerTournamentData.m_TournamentCustomerIndex;

        // ---- sign-up decision (SignUpExtra)

        /// <summary>Stands in for <c>TournamentData.m_TournamentFee</c> in Customer.EvaluateJoinTournament.</summary>
        public static float SignUpFee(TournamentData t)
        {
            float extra = 0f;
            try { extra = Handler?.SignUpExtra() ?? 0f; }
            catch (Exception e) { Plugin.Log.LogError($"ShopEvents: SignUpExtra failed: {e}"); }
            return t.m_TournamentFee + Math.Max(0f, extra);
        }

        [HarmonyPatch(typeof(Customer), "EvaluateJoinTournament")]
        private static class SignUpPatch
        {
            private static IEnumerable<CodeInstruction> Transpiler(IEnumerable<CodeInstruction> instructions)
            {
                var fee = AccessTools.Field(typeof(TournamentData), nameof(TournamentData.m_TournamentFee));
                var call = AccessTools.Method(typeof(ShopEvents), nameof(SignUpFee));
                int n = 0;
                foreach (var ci in instructions)
                {
                    if (ci.LoadsField(fee)) { n++; yield return new CodeInstruction(OpCodes.Call, call).MoveLabelsFrom(ci); }
                    else yield return ci;
                }
                if (n == 0) Plugin.Log.LogError("ShopEvents: EvaluateJoinTournament no longer reads m_TournamentFee - sign-ups ignore pack prices; re-check after the game update");
            }
        }

        // ---- entrant think step (BeforeRegister / OnRegistered)

        /// <summary>Entrants whose registration was reported (keyed per Customer instance; cleared on day start / load).</summary>
        private static readonly HashSet<Customer> _registered = new HashSet<Customer>();

        [HarmonyPatch(typeof(Customer), "DetermineShopAction")]
        private static class ThinkPatch
        {
            private static bool Prefix(Customer __instance)
            {
                var h = Handler;
                if (h == null || !EventRunning || EndOfDayReportScreen.IsActive()) return true;
                var td = __instance.GetCustomerTournamentData();
                if (td == null || !td.m_IsTournamentCustomer) return true;
                if (!td.m_HasRegisteredTournamentStart)
                {
                    if (td.m_PrizeDataList.Count > 0 || td.m_HasFinishCurrentTournamentRound) return true;
                    return !h.BeforeRegister(__instance);
                }
                _registered.RemoveWhere(c => c == null); // destroyed instances (scene reload)
                if (_registered.Add(__instance)) h.OnRegistered(__instance); // vanilla goes Idle → think within ~2 s of paying
                return true;
            }
        }

        // ---- match gate (CanStartMatches)

        private static readonly AccessTools.FieldRef<InteractablePlayTable, bool> HasStartPlay =
            AccessTools.FieldRefAccess<InteractablePlayTable, bool>("m_HasStartPlay");
        private static readonly AccessTools.FieldRef<InteractablePlayTable, float> PlayTime =
            AccessTools.FieldRefAccess<InteractablePlayTable, float>("m_CurrentPlayTime");
        private static readonly AccessTools.FieldRef<InteractablePlayTable, bool> IsTournamentTable =
            AccessTools.FieldRefAccess<InteractablePlayTable, bool>("m_IsTournamentPlayTable");

        /// <summary>Holds a seated customer pair's match timer at 0 while the format says matches can't start.</summary>
        [HarmonyPatch(typeof(InteractablePlayTable), "Update")]
        private static class MatchGatePatch
        {
            private static void Postfix(InteractablePlayTable __instance)
            {
                if (!HasStartPlay(__instance) || !IsTournamentTable(__instance)) return;
                var h = Handler;
                if (h != null && EventRunning && !h.CanStartMatches) PlayTime(__instance) = 0f;
            }
        }

        // ---- customer-vs-customer result (DecideCustomerMatch)

        private static readonly AccessTools.FieldRef<InteractablePlayTable, List<Customer>> Occupied =
            AccessTools.FieldRefAccess<InteractablePlayTable, List<Customer>>("m_OccupiedCustomer");

        /// <summary>Stands in for StopTableGame's coin flip <c>Random.Range(0, 100) &lt; 50</c> (true there = seat 1 wins).</summary>
        public static int MatchRoll(int min, int max, InteractablePlayTable table)
        {
            var h = Handler;
            if (h != null && EventRunning && IsTournamentTable(table))
            {
                var seats = Occupied(table);
                if (seats != null && seats.Count >= 2 && seats[0] != null && seats[1] != null)
                {
                    bool? seat0Wins = h.DecideCustomerMatch(seats[0], seats[1]);
                    if (seat0Wins.HasValue) return seat0Wins.Value ? max - 1 : min;
                }
            }
            return Random.Range(min, max);
        }

        [HarmonyPatch(typeof(InteractablePlayTable), "StopTableGame")]
        private static class MatchResultPatch
        {
            private static IEnumerable<CodeInstruction> Transpiler(IEnumerable<CodeInstruction> instructions)
            {
                var range = AccessTools.Method(typeof(Random), nameof(Random.Range), new[] { typeof(int), typeof(int) });
                var roll = AccessTools.Method(typeof(ShopEvents), nameof(MatchRoll));
                var list = instructions.ToList();
                int hits = list.Count(c => c.Calls(range));
                if (hits != 1)
                {
                    Plugin.Log.LogError($"ShopEvents: StopTableGame has {hits} Random.Range(int, int) calls (expected 1) — " +
                                        "tournament match results stay vanilla coin flips; re-check after the game update");
                    return list;
                }
                int i = list.FindIndex(c => c.Calls(range));
                var call = new CodeInstruction(OpCodes.Call, roll);
                list[i].MoveLabelsTo(call); // labels can't sit on the call that loses its place
                list.RemoveAt(i);
                list.InsertRange(i, new[] { new CodeInstruction(OpCodes.Ldarg_0), call });
                return list;
            }
        }

        // ---- day / round lifecycle

        [HarmonyPatch(typeof(CustomerManager), "SpawnGameStartCustomer")]
        private static class SpawnPatch
        {
            private static void Postfix()
            {
                _registered.Clear();
                if (CPlayerData.m_TournamentData.m_IsTournamentDay) Handler?.OnEntrantsSpawned();
            }
        }

        [HarmonyPatch(typeof(CustomerManager), nameof(CustomerManager.OnCustomerFinishTournamentRound))]
        private static class RoundPatch
        {
            private static void Prefix(out (int round, bool over) __state) =>
                __state = (CPlayerData.m_TournamentData.m_TournamentCurrentRound, CPlayerData.m_TournamentData.m_IsTournamentDayOver);

            private static void Postfix((int round, bool over) __state)
            {
                var t = CPlayerData.m_TournamentData;
                if (t.m_IsTournamentDayOver && !__state.over) EndEvent(forced: false); // last round just finished, placements given
                else if (t.m_TournamentCurrentRound != __state.round) Handler?.OnRoundFinished(t.m_TournamentCurrentRound);
            }
        }

        /// <summary>A tournament day that never finished is force-ended (and review-bombed) by vanilla at the next day start.</summary>
        [HarmonyPatch(typeof(CustomerManager), "OnDayStarted")]
        private static class DayStartPatch
        {
            private static void Prefix()
            {
                _registered.Clear();
                if (CPlayerData.m_TournamentData.m_IsTournamentDay) EndEvent(forced: true);
            }
        }

        private static void EndEvent(bool forced)
        {
            var h = Handler;
            Plugin.Log.LogInfo($"Shop event ended ({(forced ? "day ended first" : "finished")}), format {Config?.format ?? "vanilla"}");
            h?.OnEventEnded(forced);
            Config = null; // vanilla stops hosting at the end too (m_IsHostingTournament = false)
            ReturnStock(forced ? "tournament day ended early" : "tournament over");
        }
    }
}
