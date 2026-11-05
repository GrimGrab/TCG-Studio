using System;
using System.Collections;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using BepInEx.Configuration;

namespace TCGCustomCards
{
    /// <summary>
    /// Keeps a player's value when a setting moves to another section (e.g. <c>[Foil] BaseColor</c> → <c>[Foil - Base] BaseColor</c>).
    /// Convention: a moved setting keeps its key, and keys are unique across sections. Take a <see cref="Snapshot"/> of the file
    /// before binding, then <see cref="CarryMoved"/> after every Bind: each bound entry the file didn't have takes the value of the
    /// unbound entry with the same key in another section, and that old entry is dropped. TCG Studio does the same
    /// (modconfig.AddMissing), so whichever runs first moves it.
    /// </summary>
    internal static class ConfigMoves
    {
        /// <summary>Every settings change since players first had the setting; Studio's modconfig/migrations.go has the same lists.</summary>
        public static void Run(ConfigFile cfg, Dictionary<(string section, string key), string> before)
        {
            CarryMoved(cfg, before); // migration:settings-moved
            // migration:settings-bulk-boxes
            // Game 1.03 replaced the ten Tetramon/Destiny bulk boxes with six by grade: Tetramon designs carry over, Destiny ones go.
            const string boxes = "Visuals - Game look - Boxes";
            CarryRenamed(cfg, before, (boxes, "BulkBox_TetramonBaseTexture", "BulkBoxNGTexture"),
                (boxes, "BulkBox_TetramonBaseGradeHGTexture", "BulkBoxHGTexture"), (boxes, "BulkBox_TetramonBaseGradeMGTexture", "BulkBoxMGTexture"),
                (boxes, "BulkBox_TetramonBaseGradePGTexture", "BulkBoxPGTexture"), (boxes, "BulkBox_TetramonBaseGradeUGTexture", "BulkBoxUGTexture"));
            // migration:settings-binder-layout, migration:settings-credit-card, and the bulk boxes above
            DropRemoved(cfg, ("Binder", "Layout"), ("Debug", "TestDraft"), ("Visuals - Game look", "CreditCardImage"), ("Visuals - Game look", "CreditCardCrop"),
                (boxes, "BulkBox_TetramonBaseTexture"), (boxes, "BulkBox_TetramonDestinyTexture"),
                (boxes, "BulkBox_TetramonBaseGradeHGTexture"), (boxes, "BulkBox_TetramonBaseGradeMGTexture"),
                (boxes, "BulkBox_TetramonBaseGradePGTexture"), (boxes, "BulkBox_TetramonBaseGradeUGTexture"),
                (boxes, "BulkBox_TetramonDestinyGradeHGTexture"), (boxes, "BulkBox_TetramonDestinyGradeMGTexture"),
                (boxes, "BulkBox_TetramonDestinyGradePGTexture"), (boxes, "BulkBox_TetramonDestinyGradeUGTexture"));
        }

        /// <summary>Every (section, key) → raw value in the config file as it is on disk now.</summary>
        public static Dictionary<(string section, string key), string> Snapshot(ConfigFile cfg)
        {
            var map = new Dictionary<(string, string), string>();
            try
            {
                if (!File.Exists(cfg.ConfigFilePath)) return map;
                string section = null;
                foreach (var raw in File.ReadAllLines(cfg.ConfigFilePath))
                {
                    string l = raw.Trim();
                    if (l.Length == 0 || l.StartsWith("#")) continue;
                    if (l.StartsWith("[") && l.EndsWith("]")) { section = l.Substring(1, l.Length - 2); continue; }
                    int eq = l.IndexOf('=');
                    if (section != null && eq > 0) map[(section, l.Substring(0, eq).Trim())] = l.Substring(eq + 1).Trim();
                }
            }
            catch (Exception e) { Plugin.Log.LogWarning($"Config snapshot failed: {e.Message}"); }
            return map;
        }

        public static void CarryMoved(ConfigFile cfg, Dictionary<(string section, string key), string> before)
        {
            int moved = 0;
            var orphans = OrphanedEntries(cfg);
            foreach (var def in cfg.Keys.ToList())
            {
                if (before.ContainsKey((def.Section, def.Key))) continue; // already in the file: nothing to carry
                var old = before.Where(kv => kv.Key.key == def.Key && kv.Key.section != def.Section &&
                                             !cfg.ContainsKey(new ConfigDefinition(kv.Key.section, kv.Key.key))).ToList();
                if (old.Count != 1) continue;
                try
                {
                    cfg[def].SetSerializedValue(old[0].Value);
                    orphans?.Remove(new ConfigDefinition(old[0].Key.section, def.Key));
                    moved++;
                    Plugin.Log.LogInfo($"Config: moved [{old[0].Key.section}] {def.Key} = {old[0].Value} to [{def.Section}]");
                }
                catch (Exception e) { Plugin.Log.LogWarning($"Config: couldn't move {def.Key}: {e.Message}"); }
            }
            if (moved > 0) cfg.Save();
        }

        /// <summary>Settings renamed (new name ← old name, same section): an old value moves to the new setting when the new one
        /// isn't in the file yet (Studio's modconfig renamedSettings does the same when it opens the file first).</summary>
        public static void CarryRenamed(ConfigFile cfg, Dictionary<(string section, string key), string> before,
            params (string section, string from, string to)[] renames)
        {
            int moved = 0;
            foreach (var (section, from, to) in renames)
            {
                var def = new ConfigDefinition(section, to);
                if (before.ContainsKey((section, to)) || !cfg.ContainsKey(def)) continue;
                if (!before.TryGetValue((section, from), out var value) || string.IsNullOrEmpty(value)) continue;
                try
                {
                    cfg[def].SetSerializedValue(value);
                    moved++;
                    Plugin.Log.LogInfo($"Config: [{section}] {from} = {value} carried over to {to}");
                }
                catch (Exception e) { Plugin.Log.LogWarning($"Config: couldn't carry {from} over to {to}: {e.Message}"); }
            }
            if (moved > 0) cfg.Save();
        }

        /// <summary>Settings the mod no longer has: dropped from the file (BepInEx would keep them forever).</summary>
        public static void DropRemoved(ConfigFile cfg, params (string section, string key)[] removed)
        {
            var orphans = OrphanedEntries(cfg);
            if (orphans == null) return;
            int n = 0;
            foreach (var (section, key) in removed)
            {
                var def = new ConfigDefinition(section, key);
                if (!orphans.Contains(def)) continue;
                orphans.Remove(def);
                n++;
            }
            if (n > 0) cfg.Save();
        }

        /// <summary>BepInEx's entries read from the file but not bound (non-public in 5.4), or null.</summary>
        private static IDictionary OrphanedEntries(ConfigFile cfg)
        {
            try
            {
                return typeof(ConfigFile).GetProperty("OrphanedEntries", BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic)
                    ?.GetValue(cfg) as IDictionary;
            }
            catch { return null; }
        }
    }
}
