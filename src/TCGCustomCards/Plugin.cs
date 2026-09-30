using System.IO;
using BepInEx;
using BepInEx.Configuration;
using BepInEx.Logging;
using HarmonyLib;
using TCGCustomCards.Core;
using TCGCustomCards.Debug;
using TCGCustomCards.Runtime;

namespace TCGCustomCards
{
    [BepInPlugin(Guid, Name, Version)]
    public class Plugin : BaseUnityPlugin
    {
        public const string Guid = "tcgcustomcards";
        public const string Name = "TCG Custom Cards";
        public const string Version = BuildInfo.Version; // from TCG Custom Cards\VERSION

        internal static ManualLogSource Log;
        internal static ConfigEntry<bool> DumpDiagnostics;
        internal static ConfigEntry<bool> ShowVanillaCards;
        internal static ConfigEntry<bool> ShowVanillaPacks;
        internal static ConfigEntry<bool> ShowVanillaFurniture;
        /// <summary>[Content] ShowVanilla&lt;Kind&gt; per accessory kind.</summary>
        internal static readonly System.Collections.Generic.Dictionary<AccessoryKind, ConfigEntry<bool>> ShowVanillaAccessory =
            new System.Collections.Generic.Dictionary<AccessoryKind, ConfigEntry<bool>>();
        internal static ConfigEntry<float> CardBackScale;
        internal static ConfigEntry<bool> MtgModeEnabled;
        internal static ConfigEntry<string> MtgForgeFolder;
        internal static ConfigEntry<bool> MtgPlayInWindow;
        internal static ConfigEntry<bool> MtgBoard3D;
        internal static ConfigEntry<float> MtgHandHeight;
        internal static ConfigEntry<bool> MtgDeckBuilder;
        internal static ConfigEntry<float> FullImageInset;
        internal static ConfigEntry<bool> FullImageFoilGlow;
        internal static ConfigEntry<bool> GradeLabel;
        internal static ConfigEntry<float> GradeLabelOffsetX;
        internal static ConfigEntry<float> GradeLabelOffsetY;
        internal static ConfigEntry<float> GradeLabelSize;
        internal static ConfigEntry<bool> HoloFoil;
        internal static ConfigEntry<bool> HoloFoilRim;
        internal static ConfigEntry<float> HoloFoilMotion;
        internal static ConfigEntry<float> HoloFoilViewReact;
        internal static readonly ConfigEntry<UnityEngine.Color>[] HoloFoilColor = new ConfigEntry<UnityEngine.Color>[6];
        internal static readonly ConfigEntry<float>[] HoloFoilStrength = new ConfigEntry<float>[6];
        internal static readonly ConfigEntry<HoloPattern>[] HoloFoilPattern = new ConfigEntry<HoloPattern>[6];
        internal static string PluginDir;

        private Harmony _harmony;

        private void Awake()
        {
            Log = Logger;
            PluginDir = Path.GetDirectoryName(Info.Location);

            DumpDiagnostics = Config.Bind("Debug", "DumpDiagnostics", true,
                "Write runtime game data (list sizes, materials, restock entries) to diagnostics.txt once per session after a save loads.");
            ShowVanillaCards = Config.Bind("Content", "ShowVanillaCards", true,
                "Show the vanilla card sets (binder tabs, set pickers, trade customers). Off = custom sets only. Nothing is deleted; turning it back on restores vanilla.");
            ShowVanillaPacks = Config.Bind("Content", "ShowVanillaPacks", true,
                "Sell vanilla card packs/boxes (restock list, customer demand, worker pack list, play-table prizes). Off = custom packs only. Items you already own stay.");
            foreach (var (kind, key, what, where) in new[]
            {
                (AccessoryKind.Deckbox, "ShowVanillaDeckBoxes", "deck boxes", "restock list, deck box picker, play tables, customer demand"),
                (AccessoryKind.Playmat, "ShowVanillaPlaymats", "playmats", "restock list, playmat picker, play tables, customer demand"),
                (AccessoryKind.Sleeve, "ShowVanillaSleeves", "card sleeves", "restock list, customer demand"),
                (AccessoryKind.Dice, "ShowVanillaDice", "dice boxes", "restock list, customer demand"),
                (AccessoryKind.Comic, "ShowVanillaComics", "comics", "restock list, play tables, customer demand"),
                (AccessoryKind.Binder, "ShowVanillaCollectionBooks", "collection books", "restock list, customer demand"),
                (AccessoryKind.BattleDeck, "ShowVanillaBattleDecks", "battle decks", "booster-pack restock list, customer demand"),
                (AccessoryKind.Figurine, "ShowVanillaFigurines", "figurines", "restock list, customer demand"),
            })
            {
                var entry = Config.Bind("Content", key, true,
                    $"Sell and use the vanilla {what} ({where}). Off = only your custom {what} from TCG Studio — needs at least one, " +
                    "otherwise the vanilla ones stay. Items you already own stay.");
                entry.SettingChanged += (_, __) => VanillaFilter.Apply();
                ShowVanillaAccessory[kind] = entry;
            }
            ShowVanillaFurniture = Config.Bind("Content", "ShowVanillaFurniture", true,
                "Sell the vanilla furniture in the furniture shop. Off = only your custom furniture from TCG Studio (the cash counter, " +
                "workbench, trash bin and empty box storage stay unless you have a custom one of that kind). Placed furniture stays.");
            CardBackScale = Config.Bind("Visuals", "CardBackScale", 1.08f, new ConfigDescription(
                "Size of custom card-back art on 3D cards relative to the vanilla art window (1 = same as the vanilla art, larger also covers the dark border). Applies live to newly shown cards.",
                new AcceptableValueRange<float>(0.9f, 1.25f)));
            CardBackScale.SettingChanged += (_, __) => CardBackMesh.ClearCache();
            FullImageInset = Config.Bind("Visuals", "FullImageInset", 0.9f, new ConfigDescription(
                "Full-image cards: art size of Base/1st Edition/Silver/Gold/EX cards relative to Full Art, so the grade's border rim shows " +
                "around it (1 = no rim). Full Art cards always fill the card. Applies to newly shown cards.",
                new AcceptableValueRange<float>(0.7f, 1f)));
            FullImageFoilGlow = Config.Bind("Visuals", "FullImageFoilGlow", true,
                "Full-image foil cards: draw the game's foil glow over the artwork, like vanilla does over monster art. Applies live.");
            GradeLabel = Config.Bind("Visuals", "GradeLabel", true,
                "Full-image cards: show the grade label (1st Edition / Silver / Gold / EX) like vanilla cards. Applies live.");
            GradeLabelOffsetX = Config.Bind("Visuals", "GradeLabelOffsetX", 0f, new ConfigDescription(
                "Full-image cards: move the grade label sideways, in % of the card width (0 = the game's own spot, + = right). Applies live.",
                new AcceptableValueRange<float>(-50f, 50f)));
            GradeLabelOffsetY = Config.Bind("Visuals", "GradeLabelOffsetY", -24.882f, new ConfigDescription(
                "Full-image cards: move the grade label up/down, in % of the card height (0 = the game's own spot, + = up; " +
                "default -24.882 = near the bottom, chosen in game 2026-09-24). Applies live.",
                new AcceptableValueRange<float>(-50f, 50f)));
            GradeLabelSize = Config.Bind("Visuals", "GradeLabelSize", 1f, new ConfigDescription(
                "Full-image cards: size of the grade label (1 = the game's size). Applies live.",
                new AcceptableValueRange<float>(0.3f, 4f)));
            HoloFoil = Config.Bind("Foil", "HoloFoil", true,
                "Full-image foil cards: our own holographic foil over the artwork, a different look per grade (needs tcgcc_foil next to the DLL). Applies live.");
            HoloFoilRim = Config.Bind("Foil", "HoloFoilRim", true,
                "Full-image foil cards: use the same holo on the border rim instead of the game's foil (needs HoloFoil). Applies live.");
            HoloFoilMotion = Config.Bind("Foil", "HoloFoilMotion", 1.49f, new ConfigDescription(
                "Speed of the holo's own animation (drifting bands + light sweep), like the game's foil. 0 = still. Applies live.",
                new AcceptableValueRange<float>(0f, 3f)));
            HoloFoilViewReact = Config.Bind("Foil", "HoloFoilViewReact", 0f, new ConfigDescription(
                "How much the holo reacts to the camera angle (0 = not at all, like the game's foil; 1 = shifts as you move/look around). Applies live.",
                new AcceptableValueRange<float>(0f, 1f)));
            HoloFoilMotion.SettingChanged += (_, __) => Runtime.HoloFoil.Invalidate();
            HoloFoilViewReact.SettingChanged += (_, __) => Runtime.HoloFoil.Invalidate();
            string[] grades = { "Base", "FirstEdition", "Silver", "Gold", "EX", "FullArt" };
            for (int g = 0; g < grades.Length; g++)
            {
                var preset = Runtime.HoloFoil.Presets[g];
                HoloFoilColor[g] = Config.Bind("Foil", $"{grades[g]}Color", preset.Color,
                    $"{grades[g]} foil colour (default: {preset.Name}). Applies live.");
                HoloFoilStrength[g] = Config.Bind("Foil", $"{grades[g]}Strength", preset.Strength, new ConfigDescription(
                    $"{grades[g]} foil strength (0 = off). Applies live.", new AcceptableValueRange<float>(0f, 1.5f)));
                HoloFoilPattern[g] = Config.Bind("Foil", $"{grades[g]}Pattern", preset.Pattern,
                    $"{grades[g]} foil pattern: None, Sparkle, Etched (lines that catch the light) or Cosmos (nebula + sparkles). Applies live.");
                HoloFoilColor[g].SettingChanged += (_, __) => Runtime.HoloFoil.Invalidate();
                HoloFoilStrength[g].SettingChanged += (_, __) => Runtime.HoloFoil.Invalidate();
                HoloFoilPattern[g].SettingChanged += (_, __) => Runtime.HoloFoil.Invalidate();
            }
            MtgModeEnabled = Config.Bind("MTG", "MtgMode", true,
                "Offer Magic: The Gathering (played in Forge against the AI) when you sit at a play table with a deck that contains " +
                "MTG cards. Needs TCG Studio > Setup > Install MTG mode.");
            MtgForgeFolder = Config.Bind("MTG", "ForgeFolder", "",
                "Folder of the Forge install made by TCG Studio. Empty = <game folder>\\TCGForge.");
            MtgPlayInWindow = Config.Bind("MTG", "PlayInForgeWindow", false,
                "Off = play MTG at the table inside the game (customer opponent, win/lose counts like a normal duel). " +
                "On = open Forge's own window with your deck instead (no result comes back to the shop).");
            MtgBoard3D = Config.Bind("MTG", "Board3D", true,
                "On = MTG games show your real cards in 3D on the play table (top-down view). Off = the flat 2D board. Applies to the next game.");
            MtgHandHeight = Config.Bind("MTG", "HandHeight", 0.0756f, new ConfigDescription(
                "3D table: height of your hand's centre on screen (0 = bottom edge, 0.12 = a bit above it; negative = partly below " +
                "the edge, like MTG Arena). Hover a card to see it whole. Applies live.", new AcceptableValueRange<float>(-0.15f, 0.3f)));
            MtgDeckBuilder = Config.Bind("MTG", "MtgDeckBuilder", true,
                "On = the workbench's Edit Deck opens the MTG deck builder (search/filter your MTG cards, free basic lands, 60-card " +
                "decks) and MTG games use its active deck. Off = the vanilla deck builder (MTG games then pad your 50-card deck " +
                "with free lands). Your vanilla decks are kept either way.");
            ShowVanillaCards.SettingChanged += (_, __) => VanillaFilter.Apply();
            ShowVanillaPacks.SettingChanged += (_, __) => VanillaFilter.Apply();
            DevTools.Init(Config);

            Registry.Build(SetLoader.LoadAll(Path.Combine(PluginDir, "Sets")));
            Registry.BuildAccessories(AccessoryLoader.Load(PluginDir));
            Registry.BuildFurniture(AccessoryLoader.LoadFurniture(PluginDir));

            _harmony = new Harmony(Guid);
            _harmony.PatchAll(typeof(Plugin).Assembly);

            Log.LogInfo($"{Name} {Version} loaded with {Registry.Sets.Count} custom set(s), {Registry.Accessories.Count} accessor(ies) and {Registry.Furniture.Count} furniture piece(s)");
            WatchConfig();
        }

        // ---- Live config: TCG Studio's "Mod settings" edits the .cfg file; reload it so "Applies live" settings change in game.

        private FileSystemWatcher _cfgWatcher;
        private volatile bool _cfgTouched;
        private float _cfgReloadAt = -1f;

        private void WatchConfig()
        {
            try
            {
                string file = Config.ConfigFilePath;
                _cfgWatcher = new FileSystemWatcher(Path.GetDirectoryName(file), Path.GetFileName(file))
                {
                    NotifyFilter = NotifyFilters.LastWrite | NotifyFilters.FileName | NotifyFilters.Size,
                };
                FileSystemEventHandler touched = (_, __) => _cfgTouched = true; // watcher thread: only set a flag
                _cfgWatcher.Changed += touched;
                _cfgWatcher.Created += touched;
                _cfgWatcher.Renamed += (_, __) => _cfgTouched = true; // studio writes a temp file and renames it over the cfg
                _cfgWatcher.EnableRaisingEvents = true;
            }
            catch (System.Exception e)
            {
                Log.LogWarning($"Config file watcher unavailable ({e.Message}); settings changed outside the game apply on restart");
            }
        }

        private void Update()
        {
            if (_cfgTouched)
            {
                // Debounce: editors may write several times in a row.
                _cfgTouched = false;
                _cfgReloadAt = UnityEngine.Time.unscaledTime + 0.4f;
            }
            if (_cfgReloadAt >= 0f && UnityEngine.Time.unscaledTime >= _cfgReloadAt)
            {
                _cfgReloadAt = -1f;
                try
                {
                    // Entries whose value changed raise SettingChanged (same handlers as the F1 menu); unchanged ones don't,
                    // so our own saves (after an F1 edit) are a harmless no-op.
                    Config.Reload();
                }
                catch (System.Exception e)
                {
                    Log.LogWarning($"Config reload failed: {e.Message}");
                }
            }
        }

        private void OnDestroy() => _cfgWatcher?.Dispose();
    }
}
