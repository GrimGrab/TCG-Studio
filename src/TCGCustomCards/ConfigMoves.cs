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
