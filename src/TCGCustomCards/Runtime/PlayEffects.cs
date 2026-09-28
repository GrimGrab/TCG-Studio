using System;
using System.Collections.Generic;
using System.Linq;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Turns a card's optional "play.effect" (raw PlayEffectData shape) into the game's effect data, served through the
    /// PlayCardEffect_ScriptableObject.GetPlayEffectData patch. Monster references ("effectMonsterTypeList", "monsterType",
    /// "searchData.specificMonsterTypeList") may be card ids from the same set or vanilla EMonsterType names.
    /// Effects that would hang or crash the game's interpreter are rejected (the card then plays with no effect).
    /// </summary>
    internal static class PlayEffects
    {
        private static readonly Dictionary<int, PlayEffectData> ByMonster = new Dictionary<int, PlayEffectData>();

        private static readonly HashSet<PlayEffectType> NeedsMonsterList = new HashSet<PlayEffectType>
            { PlayEffectType.DiscardHandCardSpecificMonsterType, PlayEffectType.InstantEvolve };
        private static readonly HashSet<PlayEffectTypeSecondary> SecondaryNeedsMonsterList = new HashSet<PlayEffectTypeSecondary>
            { PlayEffectTypeSecondary.IsCardOnYourField, PlayEffectTypeSecondary.ForEachMonsterTypeInFieldAndDiscardPile, PlayEffectTypeSecondary.TargetAllFieldCardMonsterType };

        public static PlayEffectData Get(EMonsterType monster) => ByMonster.TryGetValue((int)monster, out var e) ? e : null;

        public static void Build(CustomSet set)
        {
            for (int pos = 0; pos < set.Def.Cards.Count; pos++)
            {
                var card = set.Def.Cards[pos];
                if (card.Play?.Effect == null) continue;
                try
                {
                    var json = (JObject)card.Play.Effect.DeepClone();
                    ResolveMonsters(json, set);
                    var data = json.ToObject<PlayEffectData>(JsonSerializer.Create(new JsonSerializerSettings { MissingMemberHandling = MissingMemberHandling.Error }));
                    data.monsterType = set.Shown[pos];
                    if (string.IsNullOrEmpty(data.name)) data.name = card.Name;
                    var errors = Validate(data);
                    if (errors.Count > 0)
                    {
                        Plugin.Log.LogError($"Set '{set.Def.Id}' card '{card.Id}': effect ignored — " + string.Join("; ", errors));
                        continue;
                    }
                    ByMonster[(int)data.monsterType] = data;
                }
                catch (Exception e)
                {
                    Plugin.Log.LogError($"Set '{set.Def.Id}' card '{card.Id}': effect could not be read — {e.Message}");
                }
            }
        }

        private static void ResolveMonsters(JObject effect, CustomSet set)
        {
            void Fix(JToken arr)
            {
                if (!(arr is JArray a)) return;
                for (int i = 0; i < a.Count; i++)
                    if (a[i].Type == JTokenType.String) a[i] = Resolve((string)a[i], set);
            }
            Fix(effect["effectMonsterTypeList"]);
            Fix(effect["searchData"]?["specificMonsterTypeList"]);
            effect.Remove("monsterType"); // always the card itself
        }

        private static int Resolve(string reference, CustomSet set)
        {
            if (set.PosByCardId.TryGetValue(reference, out int pos)) return (int)set.Shown[pos];
            if (Enum.TryParse(reference, out EMonsterType vanilla)) return (int)vanilla;
            throw new ArgumentException($"unknown card reference '{reference}' (use a card id from this set or a vanilla monster name)");
        }

        private static List<string> Validate(PlayEffectData d)
        {
            var errors = new List<string>();
            if (d.playEffectQueueDataList == null || d.playEffectQueueDataList.Count == 0) { errors.Add("playEffectQueueDataList is empty"); return errors; }
            if (d.effectMonsterTypeList == null) d.effectMonsterTypeList = new List<EMonsterType>();
            for (int i = 0; i < d.playEffectQueueDataList.Count; i++)
            {
                var q = d.playEffectQueueDataList[i];
                string at = $"queue[{i}]";
                if (q == null) { errors.Add($"{at} is null"); continue; }
                if (q.playEffectType == PlayEffectType.None) errors.Add($"{at}: playEffectType None would hang the game");
                if (q.countList == null || q.countList.Count == 0) errors.Add($"{at}: countList needs at least one value");
                string secondary = q.playEffectTypeSecondary.ToString();
                if ((secondary.StartsWith("AmountCheck") || secondary.StartsWith("ForEach")) && (q.countList == null || q.countList.Count < 2))
                    errors.Add($"{at}: {secondary} needs countList with 2 values");
                if ((NeedsMonsterList.Contains(q.playEffectType) || SecondaryNeedsMonsterList.Contains(q.playEffectTypeSecondary)) && d.effectMonsterTypeList.Count == 0)
                    errors.Add($"{at}: {q.playEffectType}/{secondary} needs effectMonsterTypeList");
                if ((q.playEffectType == PlayEffectType.SearchDeck || q.playEffectType == PlayEffectType.SearchDiscardPile) && d.searchData == null)
                    errors.Add($"{at}: {q.playEffectType} needs searchData");
            }
            if (d.searchData != null && d.searchData.specificMonsterTypeList == null) d.searchData.specificMonsterTypeList = new List<EMonsterType>();
            return errors;
        }
    }
}
