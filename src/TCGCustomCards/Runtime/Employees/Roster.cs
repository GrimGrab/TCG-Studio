using System.Collections.Generic;
using System.Linq;
using CC;
using HarmonyLib;
using TCGCustomCards.Core;
using TCGCustomCards.Hooks;
using TCGCustomCards.UI;
using UnityEngine;

namespace TCGCustomCards.Runtime.Employees
{
    /// <summary>Side-car section "employees": per stable id, hired or not and the game's save data of that worker.</summary>
    internal class EmployeeSave
    {
        public Dictionary<string, bool> hired = new Dictionary<string, bool>();
        /// <summary>WorkerSaveData as JsonUtility text, as the game saves it: Newtonsoft fails on its pos and rot (their Data property leads to Vector3.normalized, a loop).</summary>
        public Dictionary<string, string> data = new Dictionary<string, string>();
    }

    /// <summary>
    /// Employees made in TCG Studio (accessories.json "employees"): each is a WorkerData and a Worker clone added after the game's own, so vanilla hires,
    /// pays, saves and runs them by index. CPlayerData.m_IsWorkerHired has 100 slots (CPlayerData.cs:641). Indices follow the library, which can change,
    /// so what the game keeps per index lives in the side-car by id, and SaveStripper keeps it out of the vanilla save.
    /// </summary>
    internal static class Roster
    {
        public class Custom
        {
            public EmployeeDef Def;
            public int Index = -1;
            public WorkerData Data;
            /// <summary>The preset of one of the game's workers of this body, for "Staff look".</summary>
            public string StaffLook;
        }

        private const int MaxHiredSlots = 100;
        private static readonly List<Custom> All = new List<Custom>();
        private static readonly Dictionary<WorkerData, Custom> ByData = new Dictionary<WorkerData, Custom>();
        private static readonly Dictionary<CharacterCustomization, Custom> ByCharacter = new Dictionary<CharacterCustomization, Custom>();
        private static EmployeeSave _carried = new EmployeeSave();
        private static WorkerManager _injectedInto;
        private static int _vanillaCount;

        public static int Count => All.Count;
        public static int VanillaCount => _vanillaCount;

        public static Custom Of(Worker w) => w != null && w.m_WorkerIndex >= _vanillaCount ? All.Find(c => c.Index == w.m_WorkerIndex) : null;
        public static Custom Of(CharacterCustomization cc) => cc != null && ByCharacter.TryGetValue(cc, out var c) ? c : null;

        public static void Init(List<EmployeeDef> defs)
        {
            foreach (var d in defs.OrderBy(d => d.Id, System.StringComparer.Ordinal)) All.Add(new Custom { Def = d });
            if (All.Count == 0) return;
            CustomerLooks.PresetName = cc => Of(cc) is Custom c ? (c.Def.Model >= 0 ? (c.Def.Female ? "Female" : "Male") + c.Def.Model : c.StaffLook) : null;
        }

        [HarmonyPatch(typeof(WorkerManager), "Awake")]
        private static class InjectPatch
        {
            private static void Postfix(WorkerManager __instance)
            {
                try { Inject(__instance); }
                catch (System.Exception e) { Plugin.Log.LogError($"Employees could not be added: {e}"); }
            }
        }

        private static void Inject(WorkerManager m)
        {
            // An empty manager made outside the shop (CSingleton) has no prefabs.
            if (All.Count == 0 || _injectedInto == m || m.m_WorkerDataList == null || m.m_WorkerParentGrp == null || m.m_WorkerPrefab == null || m.m_WorkerFemalePrefab == null) return;
            int vanilla = m.m_WorkerDataList.Count;
            if (m.m_WorkerParentGrp.childCount != vanilla)
            {
                Plugin.Log.LogError($"Employees: the game has {vanilla} worker(s) but {m.m_WorkerParentGrp.childCount} character(s); your employees are skipped");
                return;
            }
            _injectedInto = m;
            _vanillaCount = vanilla;
            ByData.Clear();
            ByCharacter.Clear();
            var first = vanilla > 0 ? m.m_WorkerDataList[0] : null;
            var materials = m.m_WorkerParentGrp.GetChild(0).GetComponent<Worker>()?.m_CharacterCustom?.MaterialsStorage;
            // Each of the game's worker presets belongs to one body (Worker6 is the only female one).
            var staff = new List<Worker>();
            for (int i = 0; i < vanilla; i++)
                if (m.m_WorkerParentGrp.GetChild(i).GetComponent<Worker>() is Worker w) staff.Add(w);
            int index = vanilla;
            foreach (var c in All)
            {
                if (index >= MaxHiredSlots) { Plugin.Log.LogError($"Employees: too many; {c.Def.Label} and later are skipped"); break; }
                var d = c.Def;
                c.Index = index;
                var sameBody = staff.FindAll(w => w.m_IsFemale == d.Female);
                if (sameBody.Count == 0) sameBody = staff;
                c.StaffLook = sameBody.Count == 0 ? null : "Worker" + sameBody[(d.Id.GetHashCode() & int.MaxValue) % sameBody.Count].m_WorkerIndex;
                c.Data = new WorkerData
                {
                    name = d.Name,
                    icon = Icon(d) ?? first?.icon,
                    restockSpeed = d.RestockSpeed,
                    checkoutSpeed = d.CheckoutSpeed,
                    walkSpeedMultiplier = d.WalkSpeed,
                    costPerDay = d.Wage,
                    hiringCost = d.HireCost,
                    shopLevelRequired = d.Level,
                    description = d.Description,
                    arriveEarlySpeedMin = first?.arriveEarlySpeedMin ?? 0f,
                    arriveEarlySpeedMax = first?.arriveEarlySpeedMax ?? 0f,
                    prologueShow = true,
                };
                // Perks that change the terms of the job.
                foreach (var id in d.Perks ?? new List<string>())
                {
                    var e = Customers.Traits.EmployeePerk(id)?.effects;
                    if (e == null) continue;
                    if (e.TryGetValue("early", out float early))
                    {
                        c.Data.arriveEarlySpeedMin += early;
                        c.Data.arriveEarlySpeedMax += early;
                    }
                    if (e.ContainsKey("onTime")) c.Data.goBackOnTime = true;
                }
                m.m_WorkerDataList.Add(c.Data);
                ByData[c.Data] = c;
                var worker = Object.Instantiate(d.Female ? m.m_WorkerFemalePrefab : m.m_WorkerPrefab, Vector3.zero, Quaternion.identity, m.m_WorkerParentGrp);
                worker.name = $"TCGCC_Employee_{d.Id}";
                worker.m_IsFemale = d.Female;
                worker.m_WorkerIndex = index;
                if (worker.m_CharacterCustom != null)
                {
                    if (materials != null) worker.m_CharacterCustom.MaterialsStorage = materials;
                    ByCharacter[worker.m_CharacterCustom] = c;
                }
                index++;
            }
            // One rest spot per worker index (Worker.cs:2927): ours share the game's.
            var rest = m.m_WorkerRestPointList;
            if (rest != null && rest.Count > 0)
                for (int i = 0, have = rest.Count; rest.Count < index; i++) rest.Add(rest[i % have]);
            Plugin.Log.LogInfo($"Employees: {index - vanilla} of yours after the game's {vanilla}");
        }

        private static Sprite Icon(EmployeeDef d)
        {
            if (string.IsNullOrEmpty(d.Portrait)) return null;
            var path = StudioPaths.ItemFile(d.FolderPath, d.Portrait);
            return path == null ? null : ImageCache.Get(path);
        }

        // Their text isn't an I2 term.
        [HarmonyPatch(typeof(WorkerData), nameof(WorkerData.GetDescription))]
        private static class DescriptionPatch
        {
            private static bool Prefix(WorkerData __instance, ref string __result)
            {
                if (!ByData.TryGetValue(__instance, out var c)) return true;
                var perks = (c.Def.Perks ?? new List<string>()).Select(id => Customers.Traits.EmployeePerk(id)?.name).Where(n => n != null).ToList();
                string list = perks.Count == 0 ? "" : "Perks: " + string.Join(", ", perks);
                __result = string.IsNullOrEmpty(c.Def.Description) ? list : list.Length == 0 ? c.Def.Description : c.Def.Description + "\n" + list;
                return false;
            }
        }

        [HarmonyPatch(typeof(WorkerData), nameof(WorkerData.GetBonusConversation))]
        private static class BonusLinePatch
        {
            private static bool Prefix(WorkerData __instance, ref string __result)
            {
                if (!ByData.TryGetValue(__instance, out var c)) return true;
                // One of what they say when they get a bonus (the same lines as everyone's, set in TCG Studio).
                var lines = Customers.Traits.EmployeeLines(c.Def, "empBonus");
                __result = lines == null ? "" : lines[Random.Range(0, lines.Count)];
                return false;
            }
        }

        /// <summary>The list holds a panel per worker; scene-authored for the game's own. With vanilla hidden, only those already hired stay.</summary>
        [HarmonyPatch(typeof(HireWorkerScreen), "Init")]
        private static class HireListPatch
        {
            private static readonly AccessTools.FieldRef<GenericSliderScreen, GameObject> ScrollEndParent =
                Compat.Field<GenericSliderScreen, GameObject>(typeof(HireListPatch), "m_ScrollEndPosParent");

            private static void Prefix(HireWorkerScreen __instance)
            {
                var data = CSingleton<WorkerManager>.Instance?.m_WorkerDataList;
                if (data != null) UiPool.Grow(__instance.m_HireWorkerPanelUIList, data.Count, "Hire workers");
                // A character's picture, once the customers' are there. Your own face has its portrait.
                var customers = Core.GameScene.Existing<CustomerManager>();
                if (customers != null)
                    foreach (var c in All)
                        if (c.Data != null && c.Def.Model >= 0 && string.IsNullOrEmpty(c.Def.Portrait))
                            c.Data.icon = customers.GetCustomerIconSprite(c.Def.Model, c.Def.Female) ?? c.Data.icon;
            }

            private static void Postfix(HireWorkerScreen __instance)
            {
                var panels = __instance.m_HireWorkerPanelUIList;
                if (!Plugin.ShowVanillaEmployees.Value)
                    for (int i = 0; i < _vanillaCount && i < panels.Count; i++)
                        if (!CPlayerData.GetIsWorkerHired(i)) panels[i].SetActive(isActive: false);
                if (All.Count == 0) return;
                // The list stops at the scroll marker, which the scene puts on the game's last panel: it belongs on the last one shown.
                var data = CSingleton<WorkerManager>.Instance?.m_WorkerDataList;
                int count = data == null ? 0 : Mathf.Min(data.Count, panels.Count);
                // Cheapest first, in the places the panels already hold. Only the order on screen: each panel keeps its worker's index.
                var byCost = Enumerable.Range(0, count).OrderBy(i => data[i].hiringCost).ThenBy(i => i).ToList();
                var places = byCost.Select(i => panels[i].transform.GetSiblingIndex()).OrderBy(s => s).ToList();
                for (int n = 0; n < count; n++) panels[byCost[n]].transform.SetSiblingIndex(places[n]);
                HireWorkerPanelUI last = null;
                for (int n = count - 1; n >= 0 && last == null; n--)
                    if (panels[byCost[n]].gameObject.activeSelf) last = panels[byCost[n]];
                if (last == null || __instance.m_ScrollEndPos == null) return;
                // The screen puts the marker on this one again a moment later (GenericSliderScreen.EvaluateActiveRestockUIScroller).
                ScrollEndParent(__instance) = last.gameObject;
                __instance.m_ScrollEndPos.transform.SetParent(last.transform, worldPositionStays: false);
                __instance.m_ScrollEndPos.transform.localPosition = new Vector3(0f, __instance.m_ScrollEndPosOffsetYAmount, 0f);
                __instance.m_MaxPosXClamp = float.MaxValue;
            }
        }

        // ---- Save

        public static EmployeeSave Export()
        {
            if (All.Count == 0 && _carried.hired.Count == 0 && _carried.data.Count == 0) return null;
            var save = new EmployeeSave
            {
                hired = new Dictionary<string, bool>(_carried.hired),
                data = new Dictionary<string, string>(_carried.data),
            };
            var list = CPlayerData.m_WorkerSaveDataList;
            foreach (var c in All)
            {
                if (c.Index < 0) continue;
                save.hired[c.Def.Id] = CPlayerData.GetIsWorkerHired(c.Index);
                if (list != null && c.Index < list.Count && list[c.Index] != null) save.data[c.Def.Id] = JsonUtility.ToJson(list[c.Index]);
            }
            return save;
        }

        /// <summary>From the side-car load, before the workers are loaded (WorkerManager.DelayLoadWorker indexes the list by worker). Null for a save without any.</summary>
        public static void Load(EmployeeSave save)
        {
            _carried = save ?? new EmployeeSave();
            if (All.Count == 0 || _vanillaCount == 0) return;
            var list = CPlayerData.m_WorkerSaveDataList;
            if (list == null || list.Count < _vanillaCount) return; // a new game: nobody is hired, the first save fills it
            while (list.Count > _vanillaCount) list.RemoveAt(list.Count - 1);
            foreach (var c in All)
            {
                if (c.Index < 0) continue;
                _carried.data.TryGetValue(c.Def.Id, out var text);
                _carried.hired.TryGetValue(c.Def.Id, out bool hired);
                var data = string.IsNullOrEmpty(text) ? null : JsonUtility.FromJson<WorkerSaveData>(text);
                list.Add(data ?? new WorkerSaveData());
                CPlayerData.SetIsWorkerHired(c.Index, hired && data != null);
                _carried.data.Remove(c.Def.Id);
                _carried.hired.Remove(c.Def.Id);
            }
        }

        /// <summary>The vanilla save without our entries: a copy of the data list cut to the game's own, and a copy of the hired list with ours off.</summary>
        public static (List<WorkerSaveData> data, List<bool> hired) ForVanillaSave(List<WorkerSaveData> data, List<bool> hired)
        {
            var d = data?.Take(_vanillaCount).ToList();
            var h = hired?.ToList();
            if (h != null)
                for (int i = _vanillaCount; i < h.Count; i++) h[i] = false;
            return (d, h);
        }
    }
}
