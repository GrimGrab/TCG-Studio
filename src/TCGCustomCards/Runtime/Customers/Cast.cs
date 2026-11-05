using System;
using System.Collections.Generic;
using System.IO;
using HarmonyLib;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Hooks;
using static TCGCustomCards.Core.SafeEvents;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// The shop's cast ([Customers] Personalities): who each visiting customer is. Regulars come back with the same name and
    /// look; walk-ins get a name for one visit and may become regulars when they leave. Driven only by CustomerLife events;
    /// with the setting off nothing is attached and vanilla customers are untouched. Features build on it as
    /// <see cref="IPersonaModule"/>s through the events and pipelines here.
    /// </summary>
    internal static class Cast
    {
        private class Visit
        {
            public Persona Persona;
            public int Frame;
            /// <summary>The regular's memory before this visit counted (undone when a load replaces the visitor).</summary>
            public int VisitsBefore, LastDayBefore, FirstDayBefore;
            /// <summary>What they paid this visit is in the persona's spent already (at the till, or before a save).</summary>
            public bool PaidCounted;
            /// <summary>Their budget before VisitStarted's handlers changed it.</summary>
            public float MoneyBefore;
            /// <summary>Modules' data for this visit (<see cref="VisitData{T}"/>), by type.</summary>
            public Dictionary<Type, object> Data;
        }

        /// <summary>The visit being ended while VisitEnded runs (it is already out of Present).</summary>
        private static Customer _endingCustomer;
        private static Visit _ending;

        /// <summary>
        /// A module's data for the customer's current visit, created on first use. It ends with the visit (still readable in
        /// VisitEnded handlers): a pooled customer object never carries it to the next visitor and a load drops it. Null
        /// when the customer has no visit. Modules keep per-visit state here, not in their own dictionaries keyed by Customer.
        /// </summary>
        public static T VisitData<T>(Customer c) where T : class, new()
        {
            var v = VisitOf(c);
            if (v == null) return null;
            if (v.Data == null) v.Data = new Dictionary<Type, object>();
            if (!v.Data.TryGetValue(typeof(T), out var d)) v.Data[typeof(T)] = d = new T();
            return (T)d;
        }

        /// <summary>The kinds of per-visit data modules keep for this customer now (debug panel).</summary>
        public static IEnumerable<string> VisitDataNames(Customer c)
        {
            var v = VisitOf(c);
            if (v?.Data == null) yield break;
            foreach (var t in v.Data.Keys) yield return (t.DeclaringType != null ? t.DeclaringType.Name + "." : "") + t.Name;
        }

        /// <summary>Like <see cref="VisitData{T}"/> but never creates it: null when the module stored nothing this visit.</summary>
        public static T PeekVisitData<T>(Customer c) where T : class
        {
            var v = VisitOf(c);
            return v?.Data != null && v.Data.TryGetValue(typeof(T), out var d) ? (T)d : null;
        }

        private static Visit VisitOf(Customer c)
        {
            if (c is null) return null;
            if (Present.TryGetValue(c, out var v)) return v;
            return ReferenceEquals(c, _endingCustomer) ? _ending : null;
        }

        /// <summary>A visit began: (customer, persona, resumed = restored from a save rather than just arrived).</summary>
        public static event Action<Customer, Persona, bool> VisitStarted;

        /// <summary>The visitor walked into the shop (from the street; not when restored inside from a save).</summary>
        public static event Action<Customer, Persona> EnteredShop;

        /// <summary>A visit ended (customer back to the pool): (customer, persona, paid at the till).</summary>
        public static event Action<Customer, Persona, float> VisitEnded;

        /// <summary>A walk-in became a regular.</summary>
        public static event Action<Persona> Promoted;

        /// <summary>
        /// How likely a regular is to be the next visitor, relative to others (vanilla-free weight from days away). Each gets
        /// the previous weight; 0 = not today.
        /// </summary>
        public static readonly List<Func<Persona, float, float>> VisitWeight = new List<Func<Persona, float, float>>();

        public static readonly List<IPersonaModule> Modules = new List<IPersonaModule>();

        private static readonly Dictionary<string, Persona> Regulars = new Dictionary<string, Persona>();
        private static readonly Dictionary<Customer, Visit> Present = new Dictionary<Customer, Visit>();
        /// <summary>Made in TCG Studio (the setup item library's customers list); merged into Regulars on every load.</summary>
        private static List<Persona> _authored = new List<Persona>();
        private static PersonaSave _loaded;
        private static HashSet<string> _forgotten = new HashSet<string>();
        /// <summary>Who was behind each entry of the vanilla customer snapshot (see CustomerLife.Snapshot).</summary>
        private static List<Persona> _snapshot = new List<Persona>();
        public static readonly Random Rng = new Random();

        private static readonly AccessTools.FieldRef<CustomerManager, int> MaxMaleModel =
            AccessTools.FieldRefAccess<CustomerManager, int>("m_MaxMaleModelIndex");
        private static readonly AccessTools.FieldRef<CustomerManager, int> MaxFemaleModel =
            AccessTools.FieldRefAccess<CustomerManager, int>("m_MaxFemaleModelIndex");

        public static bool Enabled => Plugin.Personalities.Value;

        public static IEnumerable<Persona> AllRegulars => Regulars.Values;

        public static Persona Regular(string id) => id != null && Regulars.TryGetValue(id, out var p) ? p : null;

        /// <summary>Visitors with a persona right now (destroyed objects skipped).</summary>
        public static IEnumerable<KeyValuePair<Customer, Persona>> Visitors()
        {
            foreach (var kv in Present)
                if (kv.Key != null && kv.Key.m_IsActive) yield return new KeyValuePair<Customer, Persona>(kv.Key, kv.Value.Persona);
        }

        public static Persona Of(Customer c) => c != null && Present.TryGetValue(c, out var v) ? v.Persona : null;

        /// <summary>The customer object of a persona in the shop now, or null.</summary>
        public static Customer CustomerOf(Persona p)
        {
            foreach (var v in Visitors()) if (v.Value == p) return v.Key;
            return null;
        }

        public static bool IsInShop(Persona p) => CustomerOf(p) != null;

        /// <summary>The shop's name as customers say it.</summary>
        public static string ShopName => string.IsNullOrEmpty(CPlayerData.PlayerName) ? "boss" : CPlayerData.PlayerName;

        public static void Init(string setupDir)
        {
            var own = Read<CustomerLinesFile>(setupDir, "game", "customer-lines.json", "own customer lines");
            Traits.Load(own);
            _authored = Authored(Read<CustomersFile>(setupDir, "accessories", "accessories.json", "customers made in TCG Studio"));
            TableCards.Init();
            TillGuard.Init();
            Moves.Load(Read<MovesFile>(setupDir, "accessories", "accessories.json", "customer animations made in TCG Studio"));
            CustomerLife.Activated += OnActivated;
            ShopEvents.EntrantsSpawned += OnEntrantsSpawned;
            CustomerLife.Loaded += OnLoaded;
            CustomerLife.Leaving += OnLeaving;
            // Spent counts at the till, so a profile shows it while they are still in the shop (ripping their packs, …).
            CustomerLife.CheckedOut += c =>
            {
                if (Present.TryGetValue(c, out var v)) CountPaid(c, v);
            };
            CustomerLife.EnteredShop += c =>
            {
                if (Present.TryGetValue(c, out var v)) Raise(EnteredShop, c, v.Persona);
            };
            CustomerLife.Snapshot += list =>
            {
                _snapshot = new List<Persona>();
                foreach (var c in list) _snapshot.Add(Of(c)?.Clone());
            };
            // Reviews carry the customer's name instead of a random one.
            CustomerLife.ReviewAdded += (c, review) =>
            {
                var p = Of(c);
                if (Enabled && p != null) review.customerName = p.name;
            };
            Register(new Dialogue());
            Register(new Quirks());
            Register(new Wishlists());
            Register(new Loyalty());
            Register(new News());
            Register(new Crowd());
            Register(new TableBanter());
            Register(new Nicknames());
            Register(new Chatting());
            Register(new Browsing());
            Register(new Trading());
            Register(new Collecting());
            Register(new Friends());
            Register(new PackRipping());
            Register(new Rivalries());
            Register(new Reviews());
            Register(new KickOut());
            Register(new Shoplifting());
            Register(new Impatience());
        }

        /// <summary>A module by side-car id is loaded and switched on (traits that need a module are only rolled then).</summary>
        public static bool ModuleOn(string id) => Modules.Exists(m => m.Id == id && m.Enabled);

        private static void Register(IPersonaModule m)
        {
            Modules.Add(m);
            try { m.Init(); }
            catch (Exception e) { Plugin.Log.LogError($"Customers: module {m.Id} failed to start: {e}"); }
        }

        internal static T Read<T>(string setupDir, string folder, string name, string what) where T : class
        {
            if (setupDir == null) return null;
            string file = Path.Combine(setupDir, folder, name);
            if (!File.Exists(file)) return null;
            try
            {
                return JsonConvert.DeserializeObject<T>(File.ReadAllText(file));
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Could not read {file}: {e.Message}. The {what} are skipped.");
                return null;
            }
        }

        private static List<Persona> Authored(CustomersFile file)
        {
            var list = new List<Persona>();
            foreach (var p in file?.customers ?? new List<Persona>())
            {
                if (string.IsNullOrEmpty(p.id) || string.IsNullOrEmpty(p.name))
                {
                    Plugin.Log.LogWarning("Customers made in TCG Studio: one without id or name is skipped");
                    continue;
                }
                p.regular = true;
                p.origin = "authored";
                if (p.traits == null) p.traits = new List<string>();
                list.Add(p);
            }
            if (list.Count > 0) Plugin.Log.LogInfo($"Customers: {list.Count} regular(s) made in TCG Studio");
            return list;
        }

        // ---- Visits

        private static void OnActivated(Customer c)
        {
            Debug.HangWatch.Step("a customer arrives (Cast)");
            if (Present.TryGetValue(c, out var old)) End(c, old); // missed Leaving (should not happen)
            if (!Enabled) return;
            HooksRunner.Ensure();
            Attach(c, Pick(c.m_IsFemale), countVisit: true);
            Raise(VisitStarted, c, Present[c].Persona, false);
        }

        private static void OnLoaded(Customer c, int saveIndex)
        {
            // Loading activates the object first (OnActivated in the same frame): that visitor never really arrived.
            Present.TryGetValue(c, out var fresh);
            if (fresh != null && fresh.Frame != UnityEngine.Time.frameCount) fresh = null;
            string id = _loaded != null && saveIndex >= 0 && saveIndex < _loaded.present.Count ? _loaded.present[saveIndex] : null;
            Persona p = id == null ? null : Regulars.TryGetValue(id, out var r) ? r : _loaded.walkIns.Find(w => w.id == id);
            if (p == null || p.female != c.m_IsFemale)
            {
                // Saved without a persona: keep the fresh one, unless vanilla just restored a saved look (tournament
                // entrants, Customer.SetCustomerTournamentData) that isn't that regular's.
                if (fresh == null || !fresh.Persona.regular || c.GetCustomerModelIndex() == fresh.Persona.model) return;
                p = NewWalkIn(c.m_IsFemale);
            }
            if (fresh != null) Undo(fresh);
            Present.Remove(c);
            Attach(c, p, countVisit: false);
            Present[c].PaidCounted = CustomerLife.HasCheckedOut(c); // paid before the save: in the saved spent already
            Raise(VisitStarted, c, p, true);
        }

        private static void OnLeaving(Customer c)
        {
            Debug.HangWatch.Step("a customer's visit ends (Cast)");
            if (Present.TryGetValue(c, out var v)) End(c, v);
        }

        private static void CountPaid(Customer c, Visit v)
        {
            if (v.PaidCounted) return;
            float paid = CustomerLife.PaidThisVisit(c);
            if (paid <= 0f) return;
            v.PaidCounted = true;
            v.Persona.spent = (float)Math.Round(v.Persona.spent + paid, 2);
        }

        private static void End(Customer c, Visit v)
        {
            Present.Remove(c);
            var p = v.Persona;
            float paid = CustomerLife.PaidThisVisit(c);
            CountPaid(c, v);
            if (!p.regular && Enabled) MaybePromote(p, paid);
            _endingCustomer = c;
            _ending = v;
            try { Raise(VisitEnded, c, p, paid); }
            finally
            {
                _endingCustomer = null;
                _ending = null;
            }
        }

        /// <summary>The next customer of this body to walk in is them (a second visit today).</summary>
        public static void ComeBack(Persona p) => ComingBack.Add(p);

        private static readonly List<Persona> ComingBack = new List<Persona>();

        private static Persona Pick(bool female)
        {
            int back = ComingBack.FindIndex(p => p.female == female && !IsInShop(p));
            if (back < 0) return PickNew(female);
            var who = ComingBack[back];
            ComingBack.RemoveAt(back);
            return who;
        }

        private static Persona PickNew(bool female) =>
            (Rng.NextDouble() < Plugin.RegularChance.Value ? PickRegular(female) : null) ?? NewWalkIn(female);

        /// <summary>A regular of this body who may come in now (longer away = more likely; at most one visit a day), or null.</summary>
        private static Persona PickRegular(bool female)
        {
            int today = CPlayerData.m_CurrentDay;
            var due = new List<Persona>();
            var weights = new List<float>();
            float total = 0f;
            foreach (var p in Regulars.Values)
            {
                if (p.female != female || p.lastDay == today || IsInShop(p)) continue;
                float w = p.lastDay < 0 ? 4f : Math.Min(Math.Max(today - p.lastDay, 1), 7);
                w *= Math.Max(0f, Traits.Effect(p, "visit"));
                foreach (var f in VisitWeight)
                {
                    try { w = f(p, w); }
                    catch (Exception e) { Plugin.Log.LogError($"Visit weight modifier failed: {e}"); }
                }
                if (w <= 0f) continue;
                due.Add(p);
                weights.Add(w);
                total += w;
            }
            if (due.Count == 0) return null;
            double roll = Rng.NextDouble() * total;
            for (int i = 0; i < due.Count; i++)
                if ((roll -= weights[i]) < 0) return due[i];
            return due[due.Count - 1];
        }

        /// <summary>
        /// The game gives tournament slots to whoever came first in its morning crowd, so a slot a walk-in got is rolled again here, mostly for a regular. A body
        /// can't change, so one with no regular of its body left stays a walk-in. Nobody has registered yet.
        /// </summary>
        private static void OnEntrantsSpawned()
        {
            if (!Enabled) return;
            int regulars = 0, slots = 0;
            foreach (var c in CSingleton<CustomerManager>.Instance.GetTournamentCustomerList())
            {
                if (c == null || !Present.TryGetValue(c, out var v)) continue; // null = the player's slot
                slots++;
                if (!v.Persona.regular && Rng.NextDouble() < Plugin.TournamentRegularChance.Value && PickRegular(c.m_IsFemale) is Persona next)
                {
                    c.m_MaxMoney = v.MoneyBefore;
                    Present.Remove(c);
                    Attach(c, next, countVisit: true);
                    // Attach only reloads a different character, and a rolled face would stay on the same one.
                    if (Clothes.Wardrobe.HasFace(v.Persona.LookId) && c.GetCustomerModelIndex() == next.model) c.UpdateCharacterModel(next.model);
                    Raise(VisitStarted, c, next, false);
                }
                if (Present[c].Persona.regular) regulars++;
            }
            Plugin.Log.LogInfo($"Customers: {regulars} of {slots} tournament entrants are regulars");
        }

        private static Persona NewWalkIn(bool female)
        {
            var taken = new HashSet<string>();
            foreach (var p in Regulars.Values) taken.Add(p.name);
            foreach (var v in Visitors()) taken.Add(v.Value.name);
            var persona = new Persona
            {
                id = "c-" + Guid.NewGuid().ToString("N").Substring(0, 10),
                name = Names.Make(female, Rng, taken),
                female = female,
                traits = Traits.Roll(Rng),
                origin = "walk-in",
            };
            EnsureSkill(persona);
            RollLook(persona);
            return persona;
        }

        /// <summary>
        /// The faces you made and haven't given to anyone are in the pool with the game's characters, each as likely as one of those: a walk-in might have one, and
        /// keeps it if they become a regular. Otherwise the game's own pick stands (<see cref="Attach"/>).
        /// </summary>
        private static void RollLook(Persona p)
        {
            var faces = Clothes.Wardrobe.UnassignedFaces(p.female);
            faces.RemoveAll(f => FaceInUse(f.id));
            var cm = CSingleton<CustomerManager>.Instance;
            if (faces.Count == 0 || cm == null) return;
            int characters = (p.female ? MaxFemaleModel(cm) : MaxMaleModel(cm)) + 1;
            int roll = Rng.Next(characters + faces.Count);
            if (roll < characters) return;
            var face = faces[roll - characters];
            p.rolledFace = face.id;
            p.model = face.model;
        }

        /// <summary>A regular has rolled this face, or someone in the shop has it now.</summary>
        private static bool FaceInUse(string id)
        {
            foreach (var r in Regulars.Values) if (r.rolledFace == id) return true;
            foreach (var v in Visitors()) if (v.Value.rolledFace == id) return true;
            return false;
        }

        /// <summary>Rolls a table skill once (around 50, traits add).</summary>
        public static void EnsureSkill(Persona p)
        {
            if (p.skill >= 0) return;
            p.skill = Math.Max(1, Math.Min(100, 50 + Rng.Next(-20, 21) + (int)Traits.Effect(p, "skill")));
        }

        private static void Attach(Customer c, Persona p, bool countVisit)
        {
            var v = new Visit { Persona = p, Frame = UnityEngine.Time.frameCount, VisitsBefore = p.visits, LastDayBefore = p.lastDay, FirstDayBefore = p.firstDay, MoneyBefore = c.m_MaxMoney };
            EnsureSkill(p);
            if (countVisit && p.regular)
            {
                p.visits++;
                p.lastDay = CPlayerData.m_CurrentDay;
                if (p.firstDay < 0) p.firstDay = p.lastDay;
            }
            if (p.model < 0 || !ModelExists(p.female, p.model))
            {
                if (!p.regular) AvoidTakenModel(c, p.female);
                p.model = c.GetCustomerModelIndex();
            }
            else if (c.GetCustomerModelIndex() != p.model)
            {
                c.UpdateCharacterModel(p.model);
            }
            Present[c] = v;
        }

        private static void Undo(Visit v)
        {
            v.Persona.visits = v.VisitsBefore;
            v.Persona.lastDay = v.LastDayBefore;
            v.Persona.firstDay = v.FirstDayBefore;
        }

        private static void MaybePromote(Persona p, float paid)
        {
            if (Regulars.Count >= Plugin.MaxRegulars.Value) return;
            bool bigSpender = Plugin.PromoteSpend.Value > 0f && paid >= Plugin.PromoteSpend.Value;
            if (!bigSpender && (paid <= 0f || Rng.NextDouble() >= Plugin.PromoteChance.Value)) return;
            Promote(p, $"paid {paid:0.00}");
        }

        /// <summary>The player renames a regular from the whiteboard: false when the name is empty or another regular's (speech finds people by name).</summary>
        public static bool Rename(Persona p, string name)
        {
            if (string.IsNullOrEmpty(name)) return false;
            foreach (var o in Regulars.Values) if (o != p && o.name == name) return false;
            p.name = name;
            p.edited = true;
            Nicknames.NotifyChanged();
            return true;
        }

        /// <summary>The player removes a regular who isn't in the shop. Modules drop their rows on the next save (ids not kept).</summary>
        public static void Forget(Persona p)
        {
            if (IsInShop(p)) return;
            Regulars.Remove(p.id);
            if (p.origin == "authored") _forgotten.Add(p.id);
            Plugin.Log.LogInfo($"Customers: {p.name} was removed as a regular");
            Nicknames.NotifyChanged();
        }

        /// <summary>Makes a walk-in a regular (also used by modules for memorable moments, e.g. a big pull or beating you).</summary>
        public static bool Promote(Persona p, string why)
        {
            if (p == null || p.regular || Regulars.Count >= Plugin.MaxRegulars.Value) return false;
            p.regular = true;
            p.origin = "promoted";
            p.visits = 1;
            p.firstDay = p.lastDay = CPlayerData.m_CurrentDay;
            Regulars[p.id] = p;
            Plugin.Log.LogInfo($"Customers: {p.name} became a regular ({why}; traits {string.Join(", ", p.traits)})");
            Raise(Promoted, p);
            return true;
        }

        // ---- Looks: a regular keeps their model. Anyone may have it too, just not while they're in the shop. That goes for a face of their own as well: the game asks
        // for a portrait by character (Wardrobe.PortraitFor), so someone sharing theirs would show it.

        private static HashSet<int> TakenHere(bool female)
        {
            var taken = new HashSet<int>();
            foreach (var v in Visitors())
                if (v.Value.female == female && (v.Value.regular || Clothes.Wardrobe.HasFace(v.Value.LookId))) taken.Add(v.Value.model);
            foreach (var w in WorkerLife.Active())
                if (Employees.Roster.Of(w) is Employees.Roster.Custom e && e.Def.Female == female && e.Def.Model >= 0 && !Clothes.Wardrobe.HasFace(e.Def.Id))
                    taken.Add(e.Def.Model);
            return taken;
        }

        private static void AvoidTakenModel(Customer c, bool female)
        {
            var cm = CSingleton<CustomerManager>.Instance;
            var taken = TakenHere(female);
            if (cm == null || !taken.Contains(c.GetCustomerModelIndex())) return;
            var free = new List<int>();
            int max = female ? MaxFemaleModel(cm) : MaxMaleModel(cm);
            for (int m = 0; m <= max; m++) if (!taken.Contains(m)) free.Add(m);
            if (free.Count > 0) c.UpdateCharacterModel(free[Rng.Next(free.Count)]);
        }

        private static bool ModelExists(bool female, int model)
        {
            var cm = CSingleton<CustomerManager>.Instance;
            return model >= 0 && cm != null && model <= (female ? MaxFemaleModel(cm) : MaxMaleModel(cm));
        }

        // ---- Side-car

        public static void Load(PersonaSave save)
        {
            ComingBack.Clear();
            Present.Clear(); // a load rebuilds the scene: old Customer objects are gone
            _snapshot = new List<Persona>();
            Regulars.Clear();
            _loaded = save ?? new PersonaSave();
            _forgotten = new HashSet<string>(_loaded.forgotten ?? new List<string>());
            foreach (var p in _loaded.regulars)
                if (!string.IsNullOrEmpty(p.id)) Regulars[p.id] = p;
            foreach (var a in _authored)
            {
                if (_forgotten.Contains(a.id)) { Regulars.Remove(a.id); continue; }
                // Studio owns who they are; the save owns what happened.
                var p = a.Clone();
                if (Regulars.TryGetValue(a.id, out var saved))
                {
                    if (saved.edited)
                    {
                        p.name = saved.name;
                        p.nickname = saved.nickname;
                        p.edited = true;
                    }
                    if (!Clothes.Wardrobe.HasFace(a.id)) p.rolledFace = saved.rolledFace; // adopted in Studio: still the face they rolled
                    p.visits = saved.visits;
                    p.firstDay = saved.firstDay;
                    p.lastDay = saved.lastDay;
                    p.spent = saved.spent;
                    if (p.model < 0) p.model = saved.model;
                    if (p.skill < 0) p.skill = saved.skill;
                }
                Regulars[p.id] = p;
            }
            foreach (var m in Modules)
            {
                JToken state = null;
                _loaded.modules?.TryGetValue(m.Id, out state);
                try { m.Load(state); }
                catch (Exception e) { Plugin.Log.LogError($"Customers: module {m.Id} state could not be loaded ({e.Message}); it starts empty"); m.Load(null); }
            }
            Plugin.Log.LogInfo($"Customers: {Regulars.Count} regular(s) loaded");
        }

        public static PersonaSave Export()
        {
            var save = new PersonaSave { modules = new Dictionary<string, JToken>(_loaded?.modules ?? new Dictionary<string, JToken>()) };
            foreach (var old in new List<string>(save.modules.Keys))
                if (!Modules.Exists(m => m.Id == old)) save.modules.Remove(old); // a module that's gone
            if (_forgotten.Count > 0) save.forgotten = new List<string>(_forgotten);
            // Lines of customers made in TCG Studio are in the setup already; every save would carry another copy.
            foreach (var p in Regulars.Values)
            {
                if (p.origin != "authored" || (p.lines == null && p.removed == null)) { save.regulars.Add(p); continue; }
                var lean = p.Clone();
                lean.lines = null;
                lean.removed = null;
                save.regulars.Add(lean);
            }
            var keep = new HashSet<string>(Regulars.Keys);
            // A snapshot from another save (vanilla's list was replaced since) can't be matched: save nobody as present.
            bool matches = _snapshot.Count == CPlayerData.m_CustomerSaveDataList.Count;
            foreach (var p in matches ? _snapshot : new List<Persona>())
            {
                save.present.Add(p?.id);
                if (p == null || p.regular) continue;
                save.walkIns.Add(p);
                keep.Add(p.id);
            }
            foreach (var v in Visitors()) keep.Add(v.Value.id); // e.g. playing at a table (not in vanilla's list)
            foreach (var m in Modules)
            {
                JToken state = null;
                try { state = m.Save(keep); }
                catch (Exception e) { Plugin.Log.LogError($"Customers: module {m.Id} state could not be saved: {e}"); continue; }
                if (state == null) save.modules.Remove(m.Id);
                else save.modules[m.Id] = state;
            }
            return save;
        }
    }
}
