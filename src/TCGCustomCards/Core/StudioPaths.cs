using System;
using System.Collections.Generic;
using System.IO;
using Newtonsoft.Json;

namespace TCGCustomCards.Core
{
    /// <summary>
    /// The TCG Studio workspace folders all content is read from, named by TCG Studio in &lt;plugin&gt;\studio-paths.json — the
    /// game folder holds only the mod itself:
    ///   - setup: the active setup — its installed sets (setup.json "installedSets", each projects\&lt;id&gt;\set.json), its
    ///     accessory library (accessories\accessories.json), global card back and Shop look pictures (game\…);
    ///   - library: every set's files, &lt;library&gt;\&lt;set id&gt;;
    ///   - assets: accessory / furniture / decoration files ("assets/&lt;hash&gt;.&lt;ext&gt;").
    /// No fallback to the game folder. Not a setting: TCG Studio keeps the file in step. Read once at game start.
    /// </summary>
    internal static class StudioPaths
    {
        public const string FileName = "studio-paths.json";
        private const string AssetPrefix = "assets/";

        /// <summary>&lt;workspace&gt;\library, or null (no file, or the folder is gone).</summary>
        public static string Library { get; private set; }
        /// <summary>&lt;workspace&gt;\assets, or null.</summary>
        public static string Assets { get; private set; }
        /// <summary>The active setup's folder, or null.</summary>
        public static string Setup { get; private set; }
        /// <summary>Why custom content can't load (shown in game once), or null when TCG Studio's folders were found.</summary>
        public static string Problem { get; private set; }

        private class FileData
        {
            [JsonProperty("library")] public string Library { get; set; }
            [JsonProperty("assets")] public string Assets { get; set; }
            [JsonProperty("setup")] public string Setup { get; set; }
        }

        private class SetupInfo
        {
            [JsonProperty("installedSets")] public List<string> InstalledSets { get; set; }
        }

        public static void Load(string pluginDir)
        {
            Library = Assets = Setup = null;
            Problem = null;
            string file = Path.Combine(pluginDir, FileName);
            if (!File.Exists(file))
            {
                Fail($"no {FileName} next to the mod");
                return;
            }
            try
            {
                var d = JsonConvert.DeserializeObject<FileData>(File.ReadAllText(file)) ?? new FileData();
                Library = Folder(d.Library, "set files");
                Assets = Folder(d.Assets, "item files");
                Setup = Folder(d.Setup, "setup");
                if (Setup == null) Fail($"the setup folder '{d.Setup}' wasn't found");
                else if (!File.Exists(Path.Combine(Setup, "setup.json"))) Fail($"the setup '{Setup}' has no setup.json");
                else if (Library == null) Fail($"the set library '{d.Library}' wasn't found");
            }
            catch (Exception e)
            {
                Fail($"couldn't read {FileName}: {e.Message}");
            }
        }

        /// <summary>
        /// Custom content can't load. Saves stay safe (custom entries are kept until it's back, see SaveStripper), but the player must
        /// know: one clear error in the log and a notice in game.
        /// </summary>
        private static void Fail(string why)
        {
            Problem = why;
            Plugin.Log.LogError($"Custom content not loaded: {why}. Open TCG Studio once (it points the game at your content), then restart the game.");
        }

        private static string Folder(string dir, string what)
        {
            dir = dir?.Trim();
            if (!string.IsNullOrEmpty(dir) && Path.IsPathRooted(dir) && Directory.Exists(dir))
            {
                Plugin.Log.LogInfo($"Content files: {what} from {dir}");
                return dir;
            }
            Plugin.Log.LogWarning($"Content files: {what} folder '{dir}' not found");
            return null;
        }

        /// <summary>A file of the active setup (relative path, e.g. "game\card_back.png"), or null without a setup.</summary>
        public static string SetupFile(string relative) => Setup == null ? null : Path.Combine(Setup, relative);

        /// <summary>The set folders the active setup has installed (projects\&lt;id&gt;, each with its set.json).</summary>
        public static List<string> InstalledSetFolders()
        {
            var dirs = new List<string>();
            if (Setup == null) return dirs;
            try
            {
                string info = Path.Combine(Setup, "setup.json");
                var ids = File.Exists(info) ? JsonConvert.DeserializeObject<SetupInfo>(File.ReadAllText(info))?.InstalledSets : null;
                foreach (var id in ids ?? new List<string>())
                    if (SetLoader.IsSafeId(id)) dirs.Add(Path.Combine(Setup, "projects", id));
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Content files: couldn't read the setup's setup.json: {e.Message}");
            }
            return dirs;
        }

        /// <summary>
        /// Absolute path of a file an accessories.json entry names: "assets/…" from TCG Studio's store, any other relative
        /// path next to that accessories.json (folder). Null for an empty path, or a store file without a store.
        /// </summary>
        public static string ItemFile(string folder, string relative)
        {
            if (string.IsNullOrEmpty(relative)) return null;
            if (relative.StartsWith(AssetPrefix, StringComparison.Ordinal))
                return Assets == null ? null : Path.Combine(Assets, relative.Substring(AssetPrefix.Length));
            return Path.Combine(folder, relative);
        }
    }
}
