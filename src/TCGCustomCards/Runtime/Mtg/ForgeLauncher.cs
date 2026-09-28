using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Text;
using Newtonsoft.Json;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>
    /// Forge installed by TCG Studio in &lt;game&gt;\TCGForge (studio/internal/forge): reads its manifest, writes decks into its
    /// private user dir, pre-selects them for the Constructed screen and starts Forge. Forge runs as its own process/window.
    /// </summary>
    internal static class ForgeLauncher
    {
        public const string DeckPrefix = "TCG - ";
        public const string OpponentDeckName = DeckPrefix + "Opponent";

#pragma warning disable CS0649 // filled by Json.NET
        private class Manifest
        {
            [JsonProperty("forgeVersion")] public string ForgeVersion;
            [JsonProperty("java")] public string Java;
            [JsonProperty("forge")] public string Forge;
            [JsonProperty("jar")] public string Jar;
            [JsonProperty("userDir")] public string UserDir;
            [JsonProperty("jvmArgs")] public List<string> JvmArgs;
        }
#pragma warning restore CS0649

        private static Process _forge;

        public static string Root
        {
            get
            {
                string custom = Plugin.MtgForgeFolder?.Value;
                return string.IsNullOrWhiteSpace(custom) ? Path.Combine(BepInEx.Paths.GameRootPath, "TCGForge") : custom.Trim();
            }
        }

        public static bool IsInstalled => Load() != null;

        public static bool IsRunning
        {
            get
            {
                try { return _forge != null && !_forge.HasExited; }
                catch { return false; }
            }
        }

        private static Manifest Load()
        {
            try
            {
                string file = Path.Combine(Root, "tcgforge.json");
                if (!File.Exists(file)) return null;
                var m = JsonConvert.DeserializeObject<Manifest>(File.ReadAllText(file));
                if (m == null || !File.Exists(Path.Combine(Root, m.Java)) || !File.Exists(Path.Combine(Root, m.Jar))) return null;
                return m;
            }
            catch (Exception e)
            {
                Plugin.Log.LogWarning($"Forge manifest unreadable: {e.Message}");
                return null;
            }
        }

        /// <summary>Writes both decks, selects them (player 1 = human, player 2 = AI) and starts Forge unless it's already open.</summary>
        /// <returns>null on success, else a short message for the player.</returns>
        public static string Launch(MtgDeck player, MtgDeck opponent)
        {
            var m = Load();
            if (m == null) return "MTG mode isn't installed - open TCG Studio > Setup > Install MTG mode";
            string user = Path.Combine(Root, m.UserDir ?? "userdata");
            string decks = Path.Combine(user, "decks", "constructed");
            Directory.CreateDirectory(decks);
            File.WriteAllText(Path.Combine(decks, FileName(player.Name)), player.ToDck(), new UTF8Encoding(false));
            if (opponent != null)
                File.WriteAllText(Path.Combine(decks, FileName(opponent.Name)), opponent.ToDck(), new UTF8Encoding(false));

            if (IsRunning) return "Forge is already open - close it and sit down again to load this deck";

            SetPrefs(Path.Combine(user, "preferences", "forge.preferences"), new Dictionary<string, string>
            {
                ["CONSTRUCTED_P1_DECK_STATE"] = "CUSTOM_DECK;" + player.Name,
                ["CONSTRUCTED_P2_DECK_STATE"] = opponent != null ? "CUSTOM_DECK;" + opponent.Name : "",
            });

            var args = new StringBuilder();
            foreach (var a in m.JvmArgs ?? new List<string>()) args.Append(Quote(a)).Append(' ');
            args.Append("-jar ").Append(Quote(Path.GetFullPath(Path.Combine(Root, m.Jar))));
            var psi = new ProcessStartInfo(Path.GetFullPath(Path.Combine(Root, m.Java)), args.ToString())
            {
                WorkingDirectory = Path.GetFullPath(Path.Combine(Root, m.Forge ?? "forge")), // forge.profile.properties lives here
                UseShellExecute = false,
                CreateNoWindow = true,
            };
            try
            {
                _forge = Process.Start(psi);
                Plugin.Log.LogInfo($"Started Forge {m.ForgeVersion}: {player.Name} ({player.Total} cards) vs {opponent?.Name ?? "Forge's pick"}");
                return null;
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Forge failed to start: {e}");
                return "Forge failed to start (see BepInEx log)";
            }
        }

        /// <summary>Forge's user dir (decks, preferences, logs) or null when not installed.</summary>
        public static string UserDir
        {
            get
            {
                var m = Load();
                return m == null ? null : Path.Combine(Root, m.UserDir ?? "userdata");
            }
        }

        /// <summary>Writes a deck into Forge's constructed deck folder; returns the file path.</summary>
        public static string WriteDeck(MtgDeck deck)
        {
            string dir = Path.Combine(UserDir, "decks", "constructed");
            Directory.CreateDirectory(dir);
            string file = Path.Combine(dir, FileName(deck.Name));
            File.WriteAllText(file, deck.ToDck(), new UTF8Encoding(false));
            return file;
        }

        /// <summary>
        /// Start info for the headless bridge (forge-bridge\, protocol in docs/mtg-forge.md): Forge's jar + the bridge jar next
        /// to our DLL, stdin/stdout redirected. Null when Forge or the bridge jar is missing.
        /// </summary>
        public static ProcessStartInfo BridgeStartInfo(string logFile)
        {
            var m = Load();
            string bridgeJar = Path.Combine(Plugin.PluginDir, "tcgcc-forge-bridge.jar");
            if (m == null || !File.Exists(bridgeJar)) return null;
            string java = Path.Combine(Root, m.Java);
            // javaw has no console; java.exe with CreateNoWindow works too but javaw never flashes a window
            var args = new StringBuilder();
            foreach (var a in m.JvmArgs ?? new List<string>()) args.Append(Quote(a)).Append(' ');
            args.Append("-cp ").Append(Quote(Path.GetFullPath(Path.Combine(Root, m.Jar)) + ";" + bridgeJar));
            args.Append(" tcgcc.bridge.Bridge ").Append(Quote(logFile));
            return new ProcessStartInfo(Path.GetFullPath(java), args.ToString())
            {
                WorkingDirectory = Path.GetFullPath(Path.Combine(Root, m.Forge ?? "forge")),
                UseShellExecute = false,
                CreateNoWindow = true,
                RedirectStandardInput = true,
                RedirectStandardOutput = true,
                StandardOutputEncoding = new UTF8Encoding(false),
            };
        }

        /// <summary>Deck file name: the deck name with characters Windows or Forge's deck-state string can't take replaced.</summary>
        public static string FileName(string deckName) => Safe(deckName) + ".dck";

        /// <summary>Deck names end up in file names and in the preference line "KEY=CUSTOM_DECK;name1::name2" (split on '=').</summary>
        public static string Safe(string s)
        {
            var sb = new StringBuilder();
            foreach (char c in s ?? "")
                sb.Append(Path.GetInvalidFileNameChars().Contains(c) || c == ';' || c == ':' || c == '=' ? '_' : c);
            string r = sb.ToString().Trim();
            return r.Length == 0 ? "Deck" : r;
        }

        /// <summary>Updates KEY=VALUE lines in Forge's preferences file, keeping everything else.</summary>
        private static void SetPrefs(string file, Dictionary<string, string> values)
        {
            Directory.CreateDirectory(Path.GetDirectoryName(file));
            var lines = File.Exists(file) ? File.ReadAllLines(file).ToList() : new List<string>();
            foreach (var kv in values)
            {
                int i = lines.FindIndex(l => l.StartsWith(kv.Key + "=", StringComparison.Ordinal));
                string line = kv.Key + "=" + kv.Value;
                if (i >= 0) lines[i] = line; else lines.Add(line);
            }
            File.WriteAllLines(file, lines, new UTF8Encoding(false));
        }

        private static string Quote(string a) => a.IndexOf(' ') >= 0 ? "\"" + a + "\"" : a;
    }
}
