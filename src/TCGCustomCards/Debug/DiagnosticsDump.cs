using System;
using System.Collections;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Text;
using HarmonyLib;
using UnityEngine;
using UnityEngine.Rendering;

namespace TCGCustomCards.Debug
{
    /// <summary>
    /// M0: one-shot dump of facts that only exist in Unity assets (list sizes, materials, restock table).
    /// Runs after a save has loaded (RestockManager.Init fires from GameDataFinishLoaded).
    /// </summary>
    [HarmonyPatch(typeof(RestockManager), "Init")]
    internal static class DiagnosticsDump
    {
        private static bool _done;

        [HarmonyPostfix]
        private static void Postfix()
        {
            TemplateExport.RunIfMissing();
            if (_done || !Plugin.DumpDiagnostics.Value) return;
            _done = true;

            var sb = new StringBuilder();
            try
            {
                Write(sb);
            }
            catch (Exception e)
            {
                sb.AppendLine("!! dump aborted: " + e);
            }

            string path = Path.Combine(Path.GetDirectoryName(typeof(Plugin).Assembly.Location), "diagnostics.txt");
            File.WriteAllText(path, sb.ToString());
            Plugin.Log.LogInfo($"Diagnostics written to {path}");
        }

        private static void Write(StringBuilder sb)
        {
            sb.AppendLine($"Game {Application.version} / Unity {Application.unityVersion} / {DateTime.Now:s}");

            var inv = CSingleton<InventoryBase>.Instance;
            var mso = inv.m_MonsterData_SO;
            var sso = inv.m_StockItemData_SO;

            Section(sb, "Monster lists");
            ShownList(sb, "m_ShownMonsterList", mso.m_ShownMonsterList);
            ShownList(sb, "m_ShownGhostMonsterList", mso.m_ShownGhostMonsterList);
            ShownList(sb, "m_ShownMegabotList", mso.m_ShownMegabotList);
            ShownList(sb, "m_ShownFantasyRPGList", mso.m_ShownFantasyRPGList);
            ShownList(sb, "m_ShownCatJobList", mso.m_ShownCatJobList);
            ShownList(sb, "m_ShownPOCGList", mso.m_ShownPOCGList);
            DataList(sb, "m_DataList", mso.m_DataList);
            DataList(sb, "m_MegabotDataList", mso.m_MegabotDataList);
            DataList(sb, "m_FantasyRPGDataList", mso.m_FantasyRPGDataList);
            DataList(sb, "m_CatJobDataList", mso.m_CatJobDataList);
            DataList(sb, "m_POCGDataList", mso.m_POCGDataList);

            Section(sb, "Per-expansion asset lists");
            sb.AppendLine($"m_CardUISettingList={Count(mso.m_CardUISettingList)} m_CardBackImageList={Count(mso.m_CardBackImageList)} " +
                          $"m_CardFoilMaskImageList={Count(mso.m_CardFoilMaskImageList)} m_GradedCardScratchTextureList={Count(mso.m_GradedCardScratchTextureList)}");
            var names = inv.m_TextSO.m_CardExpansionNameList;
            sb.AppendLine($"m_CardExpansionNameList={Count(names)}: {string.Join(" | ", names ?? new System.Collections.Generic.List<string>())}");
            for (int i = 0; i < Count(mso.m_CardUISettingList); i++)
            {
                var s = mso.m_CardUISettingList[i];
                sb.AppendLine($"  CardUISetting[{i} {(ECardExpansionType)i}]: {Fields(s)}");
                if (s != null && s.cardUISettingDataList != null)
                    for (int j = 0; j < s.cardUISettingDataList.Count; j++)
                        sb.AppendLine($"      data[{j}]: {Fields(s.cardUISettingDataList[j])}");
            }
            FoilMaterials(sb, mso);

            Section(sb, "Items");
            var itemMax = Enum.GetValues(typeof(EItemType)).Cast<int>().Max();
            sb.AppendLine($"EItemType max={itemMax} m_ItemDataList={Count(sso.m_ItemDataList)} m_ItemMeshDataList={Count(sso.m_ItemMeshDataList)} m_RestockDataList={Count(sso.m_RestockDataList)}");
            sb.AppendLine($"m_CardPackItemTypeList: {string.Join(", ", sso.m_CardPackItemTypeList)}");
            sb.AppendLine($"m_ShownItemType={Count(sso.m_ShownItemType)} m_ShownAllItemType={Count(sso.m_ShownAllItemType)}");
            ItemVisual(sb, EItemType.BasicCardPack);
            ItemVisual(sb, EItemType.BasicCardBox);
            sb.AppendLine($"ItemData[BasicCardPack]: {Fields(InventoryBase.GetItemData(EItemType.BasicCardPack))}");
            sb.AppendLine($"ItemData[BasicCardBox]: {Fields(InventoryBase.GetItemData(EItemType.BasicCardBox))}");

            Section(sb, "Restock table (index: itemType, bigBox, level, licensePrice, amount, hidden)");
            for (int i = 0; i < Count(sso.m_RestockDataList); i++)
            {
                var r = sso.m_RestockDataList[i];
                sb.AppendLine($"  [{i}] {r.itemType} big={r.isBigBox} lvl={r.licenseShopLevelRequired} license={r.licensePrice} amount={r.amount} hide={r.isHideItemUntilUnlocked} unlocked={SafeLicense(i)}");
            }

            Section(sb, "CPlayerData list sizes");
            sb.AppendLine($"m_CardCollectedList={Count(CPlayerData.m_CardCollectedList)} m_GenCardMarketPriceList={Count(CPlayerData.m_GenCardMarketPriceList)} " +
                          $"m_IsItemLicenseUnlocked={Count(CPlayerData.m_IsItemLicenseUnlocked)} m_CurrentTotalItemCountList={Count(CPlayerData.m_CurrentTotalItemCountList)} " +
                          $"m_CollectionSortingMethodIndexList={Count(CPlayerData.m_CollectionSortingMethodIndexList)} m_GenGradedCardPriceMultiplierList={Count(CPlayerData.m_GenGradedCardPriceMultiplierList)} " +
                          $"shopLevel={CPlayerData.m_ShopLevel}");

            Section(sb, "Play-table data");
            foreach (var so in Resources.FindObjectsOfTypeAll<PlayCardEffect_ScriptableObject>())
                sb.AppendLine($"{so.name}: effects={Count(so.m_PlayEffectDataList)} npcDecks={Count(so.m_PlayCardDeckDataList)}");

            Section(sb, "Card opening sequence");
            var seq = CSingleton<CardOpeningSequence>.Instance;
            foreach (var f in new[] { "m_Card3dUIList", "m_CardAnimList", "m_ShowAllCardPosList" })
                sb.AppendLine($"{f}={Count(Traverse.Create(seq).Field(f).GetValue() as ICollection)}");
            var packMesh = Traverse.Create(seq).Field("m_CardPackMesh").GetValue() as Renderer;
            if (packMesh != null) MaterialInfo(sb, "m_CardPackMesh", packMesh.sharedMaterial);
        }

        private static void ShownList(StringBuilder sb, string name, System.Collections.Generic.List<EMonsterType> list)
        {
            if (list == null) { sb.AppendLine($"{name}: null"); return; }
            var rarities = list.Select(InventoryBase.GetMonsterData)
                .GroupBy(m => m == null ? "null" : m.Rarity.ToString())
                .Select(g => $"{g.Key}={g.Count()}");
            sb.AppendLine($"{name}: count={list.Count} ids={(list.Count > 0 ? $"{(int)list.Min()}..{(int)list.Max()}" : "-")} rarity[{string.Join(", ", rarities)}]");
        }

        private static void DataList(StringBuilder sb, string name, System.Collections.Generic.List<MonsterData> list)
        {
            if (list == null) { sb.AppendLine($"{name}: null"); return; }
            sb.AppendLine($"{name}: count={list.Count} iconNull={list.Count(m => m.Icon == null)} ghostIconNull={list.Count(m => m.GhostIcon == null)} " +
                          $"iconList>0={list.Count(m => m.IconList != null && m.IconList.Count > 0)}");
        }

        private static void ItemVisual(StringBuilder sb, EItemType type)
        {
            var md = InventoryBase.GetItemMeshData(type);
            sb.AppendLine($"ItemMeshData[{type}]: mesh={md.mesh?.name} meshSecondary={md.meshSecondary?.name} materialList={Count(md.materialList)}");
            MaterialInfo(sb, $"  {type}.material", md.material);
            MaterialInfo(sb, $"  {type}.materialSecondary", md.materialSecondary);
            if (md.materialList != null)
                for (int i = 0; i < md.materialList.Count; i++) MaterialInfo(sb, $"  {type}.materialList[{i}]", md.materialList[i]);
        }

        private static void MaterialInfo(StringBuilder sb, string label, Material m)
        {
            if (m == null) { sb.AppendLine($"{label}: null"); return; }
            var shader = m.shader;
            var tex = Enumerable.Range(0, shader.GetPropertyCount())
                .Where(i => shader.GetPropertyType(i) == ShaderPropertyType.Texture)
                .Select(i => shader.GetPropertyName(i))
                .Select(p => { var t = m.GetTexture(p); return t == null ? $"{p}=-" : $"{p}={t.name}({t.width}x{t.height})"; });
            sb.AppendLine($"{label}: {m.name} shader={shader.name} textures[{string.Join(", ", tex)}]");
        }

        private static string Fields(object o)
        {
            if (o == null) return "null";
            return string.Join(" ", o.GetType().GetFields(BindingFlags.Public | BindingFlags.Instance).Select(f =>
            {
                var v = f.GetValue(o);
                if (v is ICollection c) return $"{f.Name}=[{c.Count}]";
                if (v is UnityEngine.Object u) return $"{f.Name}={(u == null ? "null" : u.name)}";
                return $"{f.Name}={v}";
            }));
        }

        private static string SafeLicense(int i) =>
            i < Count(CPlayerData.m_IsItemLicenseUnlocked) ? CPlayerData.m_IsItemLicenseUnlocked[i].ToString() : "?";

        private static int Count(ICollection c) => c?.Count ?? -1;

        private static void Section(StringBuilder sb, string title) => sb.AppendLine().AppendLine($"== {title} ==");

        /// <summary>
        /// Which foil material each card style uses (CardUI.m_FoilShowList order: 0 CardBGMask shine, 1 CardBorderMask glitter,
        /// 2 CenterFrameMask glitter, 3 CenterFrameMask glitter btm; blended: 0 CardBGMask blended), the generic lists in
        /// Card3dUISpawner (applied first, then overridden by the style's lists), and every material's shader properties.
        /// Textures are exported to templates\Foil_*.png.
        /// </summary>
        /// <summary>
        /// Layer tree of a pooled 3D card's front (draw order = listing order): which images are masks, which carry foil materials,
        /// and whether the foil layers sit above or below the monster art.
        /// </summary>
        private static void CardTree(StringBuilder sb, Card3dUISpawner spawner)
        {
            Section(sb, "Card front layer tree (pooled Card3dUIGroup, draw order top→bottom = back→front)");
            var group = spawner != null ? spawner.GetComponentsInChildren<Card3dUIGroup>(true).FirstOrDefault() : null;
            var front = group?.m_CardUI?.m_CardFront;
            if (front == null) { sb.AppendLine("no pooled card found"); return; }
            var ui = group.m_CardUI;
            void Walk(Transform t, int depth)
            {
                var parts = new System.Collections.Generic.List<string>();
                if (!t.gameObject.activeSelf) parts.Add("(off)");
                var img = t.GetComponent<UnityEngine.UI.Image>();
                if (img != null)
                    parts.Add($"Image sprite={img.sprite?.name} mat={(img.material != null && img.material != img.defaultMaterial ? img.material.name : "default")} " +
                              $"enabled={img.enabled} color={img.color}");
                var raw = t.GetComponent<UnityEngine.UI.RawImage>();
                if (raw != null) parts.Add($"RawImage tex={raw.texture?.name}");
                var mask = t.GetComponent<UnityEngine.UI.Mask>();
                if (mask != null) parts.Add($"MASK showGraphic={mask.showMaskGraphic}");
                if (t.GetComponent<UnityEngine.UI.RectMask2D>() != null) parts.Add("RECTMASK2D");
                var tmp = t.GetComponent<TMPro.TMP_Text>();
                if (tmp != null) parts.Add($"Text '{tmp.text}'");
                var tags = new System.Collections.Generic.List<string>();
                if (img != null && ui.m_FoilShowList != null && ui.m_FoilShowList.Contains(img)) tags.Add($"FoilShow[{ui.m_FoilShowList.IndexOf(img)}]");
                if (img != null && ui.m_FoilBlendedShowList != null && ui.m_FoilBlendedShowList.Contains(img)) tags.Add($"FoilBlended[{ui.m_FoilBlendedShowList.IndexOf(img)}]");
                if (img != null && ui.m_FoilDarkenImageList != null && ui.m_FoilDarkenImageList.Contains(img)) tags.Add("FoilDarken");
                if (img == ui.m_CenterFrameImage) tags.Add("MONSTER ART");
                if (img == ui.m_CardBGImage) tags.Add("CardBG");
                if (img == ui.m_CardFullBGImage) tags.Add("CardFullBG");
                if (img == ui.m_CardFrontImage) tags.Add("CardFrontFrame");
                if (img == ui.m_CardBorderImage) tags.Add("Border");
                if (t.gameObject == ui.m_CenterFoilGlitter) tags.Add("m_CenterFoilGlitter");
                if (t.gameObject == ui.m_CenterFoilGlitterBtm) tags.Add("m_CenterFoilGlitterBtm");
                if (t.gameObject == ui.m_BorderFoilGlitter) tags.Add("m_BorderFoilGlitter");
                if (t.gameObject == ui.m_FoilGrp) tags.Add("m_FoilGrp");
                if (tags.Count > 0) parts.Add("<" + string.Join(",", tags) + ">");
                sb.AppendLine($"{new string(' ', depth * 2)}{t.GetSiblingIndex()}:{t.name} {string.Join(" ", parts)}");
                if (depth < 10) for (int i = 0; i < t.childCount; i++) Walk(t.GetChild(i), depth + 1);
            }
            Walk(front.transform, 0);
        }

        private static void FoilMaterials(StringBuilder sb, MonsterData_ScriptableObject mso)
        {
            Section(sb, "Foil materials");
            var all = new System.Collections.Generic.List<Material>();
            string Names(System.Collections.Generic.List<Material> l)
            {
                if (l == null) return "null";
                foreach (var m in l) if (m != null && !all.Contains(m)) all.Add(m);
                return "[" + string.Join(", ", l.Select(m => m != null ? m.name : "null")) + "]";
            }
            for (int i = 0; i < Count(mso.m_CardUISettingList); i++)
            {
                var s = mso.m_CardUISettingList[i];
                if (s?.cardUISettingDataList == null) continue;
                for (int j = 0; j < s.cardUISettingDataList.Count; j++)
                {
                    var d = s.cardUISettingDataList[j];
                    sb.AppendLine($"{(ECardExpansionType)i} data[{j}] borders=[{string.Join(",", d.applicableBorderList)}] " +
                                  $"tangent={Names(d.foilMaterialTangentView)} world={Names(d.foilMaterialWorldView)} " +
                                  $"blendedTangent={Names(d.foilBlendedMaterialTangentView)} blendedWorld={Names(d.foilBlendedMaterialWorldView)}");
                }
            }
            var spawner = UnityEngine.Object.FindObjectOfType<Card3dUISpawner>();
            if (spawner != null)
                sb.AppendLine($"Card3dUISpawner tangent={Names(spawner.m_FoilMaterialTangentView)} world={Names(spawner.m_FoilMaterialWorldView)} " +
                              $"blendedTangent={Names(spawner.m_FoilBlendedMaterialTangentView)} blendedWorld={Names(spawner.m_FoilBlendedMaterialWorldView)}");
            else sb.AppendLine("Card3dUISpawner: not found");
            CardTree(sb, spawner);

            string dir = Path.Combine(Plugin.PluginDir, "templates");
            foreach (var m in all)
            {
                var sh = m.shader;
                if (sh == null) { sb.AppendLine($"  {m.name}: no shader"); continue; }
                var props = new System.Collections.Generic.List<string>();
                for (int p = 0; p < sh.GetPropertyCount(); p++)
                {
                    string n = sh.GetPropertyName(p);
                    if (n.StartsWith("_Stencil") || n == "_ColorMask" || n == "_UseUIAlphaClip") continue;
                    switch (sh.GetPropertyType(p))
                    {
                        case ShaderPropertyType.Texture:
                            var tex = m.GetTexture(n);
                            props.Add($"{n}={(tex != null ? $"{tex.name} {tex.width}x{tex.height}" : "none")}");
                            try
                            {
                                if (tex != null && Directory.Exists(dir) && !File.Exists(Path.Combine(dir, $"Foil_{tex.name}.png")))
                                    TemplateExport.SaveTexture(tex, Path.Combine(dir, $"Foil_{tex.name}.png"));
                            }
                            catch (Exception e) { props.Add($"(export failed: {e.Message})"); }
                            break;
                        case ShaderPropertyType.Color: props.Add($"{n}={m.GetColor(n)}"); break;
                        case ShaderPropertyType.Vector: props.Add($"{n}={m.GetVector(n)}"); break;
                        default: props.Add($"{n}={m.GetFloat(n):0.###}"); break;
                    }
                }
                sb.AppendLine($"  {m.name} ({sh.name}): {string.Join(", ", props)}");
            }
        }
    }
}
