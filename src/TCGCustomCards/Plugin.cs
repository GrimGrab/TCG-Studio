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
        internal static ConfigEntry<Runtime.Mtg.AiDeckStyle> MtgAiDeckStyle;
        internal static ConfigEntry<Runtime.Mtg.AiDeckSets> MtgAiDeckSets;
        internal static ConfigEntry<int> MtgAiDeckSetsMin, MtgAiDeckSetsMax, MtgAiDeckSize, MtgAiSealedBoosters, MtgAiFullPowerShopLevel;
        internal static ConfigEntry<Runtime.Mtg.AiDeckPower> MtgAiDeckPower;
        internal static ConfigEntry<bool> MtgAiPowerFollowsShopLevel, MtgAiKeepDeckOnRematch, MtgAiRevealDeck;
        internal static ConfigEntry<Runtime.Mtg.AiPlayStyle> MtgAiPlayStyle;
        internal static ConfigEntry<int> MtgYourStartingLife, MtgCustomerStartingLife;
        internal static ConfigEntry<int> MtgAiDeckColorsMin, MtgAiDeckColorsMax;
        /// <summary>[MTG] AiDeckWeight1Color … AiDeckWeight5Colors: how often each colour count is rolled.</summary>
        internal static readonly ConfigEntry<int>[] MtgAiDeckColorWeights = new ConfigEntry<int>[5];
        internal static ConfigEntry<float> FullImageInset;
        internal static ConfigEntry<bool> FullImageFoilGlow;
        internal static ConfigEntry<bool> HidePrintedBorders;
        internal static ConfigEntry<float> PrintedBorderMax;
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

            var cfgBefore = ConfigMoves.Snapshot(Config); // see ConfigMoves: settings moved to a sub-section keep their value
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
            HidePrintedBorders = Config.Bind("Visuals", "HidePrintedBorders", false,
                "Full-image cards: crop the card scan's own printed border (the black/white/yellow rim of the real card) so the game's " +
                "border frames the card directly; the rest is stretched back to the full card size. Detected per image (borderless " +
                "printings are left alone); image files are not changed. Applies to newly shown cards.");
            PrintedBorderMax = Config.Bind("Visuals", "PrintedBorderMax", 0.1f, new ConfigDescription(
                "Full-image cards with HidePrintedBorders on: the widest printed border that is cropped, as a fraction of the card " +
                "width; anything wider is treated as artwork and left alone. Applies to newly shown cards.",
                new AcceptableValueRange<float>(0f, 0.2f)));
            FullImageFoilGlow = Config.Bind("Visuals", "FullImageFoilGlow", true,
                "Full-image foil cards: draw the game's foil glow over the artwork, like vanilla does over monster art. Applies live.");
            GradeLabel = Config.Bind("Visuals - Grade label", "GradeLabel", true,
                "Full-image cards: show the grade label (1st Edition / Silver / Gold / EX) like vanilla cards. Applies live.");
            GradeLabelOffsetX = Config.Bind("Visuals - Grade label", "GradeLabelOffsetX", 0f, new ConfigDescription(
                "Full-image cards: move the grade label sideways, in % of the card width (0 = the game's own spot, + = right). Applies live.",
                new AcceptableValueRange<float>(-50f, 50f)));
            GradeLabelOffsetY = Config.Bind("Visuals - Grade label", "GradeLabelOffsetY", -24.882f, new ConfigDescription(
                "Full-image cards: move the grade label up/down, in % of the card height (0 = the game's own spot, + = up; " +
                "default -24.882 = near the bottom, chosen in game 2026-09-24). Applies live.",
                new AcceptableValueRange<float>(-50f, 50f)));
            GradeLabelSize = Config.Bind("Visuals - Grade label", "GradeLabelSize", 1f, new ConfigDescription(
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
            string[] gradeNames = { "Base", "First edition", "Silver", "Gold", "EX", "Full art" };
            for (int g = 0; g < grades.Length; g++)
            {
                var preset = Runtime.HoloFoil.Presets[g];
                string foilSection = "Foil - " + gradeNames[g];
                HoloFoilColor[g] = Config.Bind(foilSection, $"{grades[g]}Color", preset.Color,
                    $"{grades[g]} foil colour (default: {preset.Name}). Applies live.");
                HoloFoilStrength[g] = Config.Bind(foilSection, $"{grades[g]}Strength", preset.Strength, new ConfigDescription(
                    $"{grades[g]} foil strength (0 = off). Applies live.", new AcceptableValueRange<float>(0f, 1.5f)));
                HoloFoilPattern[g] = Config.Bind(foilSection, $"{grades[g]}Pattern", preset.Pattern,
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
            // MTG AI opponent. Settings that only matter for some choice are hidden in TCG Studio by settings-meta.json
            // (studio/internal/modconfig); the F1 menu shows them all, so their descriptions say when they apply.
            const string aiDeck = "MTG - AI deck", aiOpp = "MTG - AI opponent", aiCol = "MTG - AI deck colours", match = "MTG - Match";
            const string next = " Applies to the next game (not in PlayInForgeWindow mode).";
            MtgAiDeckStyle = Config.Bind(aiDeck, "AiDeckStyle", Runtime.Mtg.AiDeckStyle.Random,
                "How the customer builds their deck. Random = a deck from Forge's deck generator (size, colours and power below). " +
                "Sealed = Forge opens boosters of the customer's sets and builds the best 40-card deck, like a sealed event." + next);
            MtgAiDeckSets = Config.Bind(aiDeck, "AiDeckSets", Runtime.Mtg.AiDeckSets.RandomLicensed,
                "Which sets the customer plays. RandomInstalled = random sets from every installed MTG set. RandomLicensed = random " +
                "sets among those whose packs you've unlocked in the shop (all installed sets until you have one). MatchMyDeck = the " +
                "sets your deck uses." + next);
            MtgAiDeckSetsMin = Config.Bind(aiDeck, "AiDeckSetsMin", 1, new ConfigDescription(
                "Random sets only: fewest sets a customer mixes into their deck." + next,
                new AcceptableValueRange<int>(1, 10)));
            MtgAiDeckSetsMax = Config.Bind(aiDeck, "AiDeckSetsMax", 10, new ConfigDescription(
                "Random sets only: most sets a customer mixes into their deck (set Min and Max equal for a fixed number)." + next, new AcceptableValueRange<int>(1, 10)));
            MtgAiDeckSize = Config.Bind(aiDeck, "AiDeckSize", 60, new ConfigDescription(
                "Random style: cards in the customer's deck (40 = quicker games)." + next,
                new AcceptableValueRange<int>(40, 100)));
            MtgAiSealedBoosters = Config.Bind(aiDeck, "AiSealedBoosters", 6, new ConfigDescription(
                "Sealed style: boosters a customer opens to build their deck (more = more cards to pick from = a stronger deck)." + next, new AcceptableValueRange<int>(3, 12)));
            MtgAiPowerFollowsShopLevel = Config.Bind(aiDeck, "AiDeckPowerFollowsShopLevel", true,
                "On = customers' decks get stronger as your shop levels up: weak at level 0, strong from AiFullPowerShopLevel. " +
                "Off = AiDeckPower decides." + next);
            MtgAiDeckPower = Config.Bind(aiDeck, "AiDeckPower", Runtime.Mtg.AiDeckPower.Normal, new ConfigDescription(
                "With AiDeckPowerFollowsShopLevel off: how strong customers' decks are, by Forge's card ratings. Weak = the best-rated cards are left out. Normal = every " +
                "card. Strong = mostly the best-rated cards (Sealed: more boosters). Random = anything in between, per customer." + next));
            MtgAiFullPowerShopLevel = Config.Bind(aiDeck, "AiFullPowerShopLevel", 35, new ConfigDescription(
                "With AiDeckPowerFollowsShopLevel on: shop level at which customers' decks reach full strength (it rises a little every level before that)." + next, new AcceptableValueRange<int>(1, 100)));
            MtgAiDeckColorsMin = Config.Bind(aiCol, "AiDeckColorsMin", 1, new ConfigDescription(
                "Random style: fewest colours the customer's deck may have. Decks with 3+ colours need mana fixing from the sets, so small or " +
                "old sets may give fewer colours." + next,
                new AcceptableValueRange<int>(1, 5)));
            MtgAiDeckColorsMax = Config.Bind(aiCol, "AiDeckColorsMax", 5, new ConfigDescription(
                "Random style: most colours the customer's deck may have (set Min and Max equal for a fixed count)." + next, new AcceptableValueRange<int>(1, 5)));
            int[] colorWeights = { 15, 45, 25, 10, 5 };
            for (int i = 0; i < 5; i++)
            {
                string what = i == 0 ? "1 colour" : $"{i + 1} colours";
                MtgAiDeckColorWeights[i] = Config.Bind(aiCol, i == 0 ? "AiDeckWeight1Color" : $"AiDeckWeight{i + 1}Colors", colorWeights[i],
                    new ConfigDescription($"Random style: how often a {what} deck is picked, relative to the other weights (0 = never; only " +
                                          "counts between Min and Max are used)." + next,
                        new AcceptableValueRange<int>(0, 100)));
            }
            MtgAiPlayStyle = Config.Bind(aiOpp, "AiPlayStyle", Runtime.Mtg.AiPlayStyle.Default,
                "How the customer plays (Forge's AI profiles). Default = Forge's standard AI. Cautious = holds back, avoids risky " +
                "attacks. Reckless = attacks a lot. Experimental = Forge's newest AI logic. Random = one of them per customer." + next);
            MtgAiKeepDeckOnRematch = Config.Bind(aiOpp, "AiKeepDeckOnRematch", true,
                "On = a rematch against the same customer uses the same deck (learn it and beat it). Off = a new deck every game." + next);
            MtgAiRevealDeck = Config.Bind(aiOpp, "AiRevealDeckAfterMatch", true,
                "On = after the game the customer shows their deck list (click Continue to close it). Off = straight to the result screen.");
            MtgYourStartingLife = Config.Bind(match, "YourStartingLife", 20, new ConfigDescription(
                "Your life at the start of an MTG game (Magic's normal is 20)." + next, new AcceptableValueRange<int>(1, 100)));
            MtgCustomerStartingLife = Config.Bind(match, "CustomerStartingLife", 20, new ConfigDescription(
                "The customer's life at the start of an MTG game (Magic's normal is 20)." + next, new AcceptableValueRange<int>(1, 100)));
            ShowVanillaCards.SettingChanged += (_, __) => VanillaFilter.Apply();
            ShowVanillaPacks.SettingChanged += (_, __) => VanillaFilter.Apply();
            DevTools.Init(Config);
            ConfigMoves.CarryMoved(Config, cfgBefore); // after every Bind

            Registry.Build(SetLoader.LoadAll(Path.Combine(PluginDir, "Sets")));
            Registry.BuildAccessories(AccessoryLoader.Load(PluginDir));
            Registry.BuildFurniture(AccessoryLoader.LoadFurniture(PluginDir));

            _harmony = new Harmony(Guid);
            _harmony.PatchAll(typeof(Plugin).Assembly);

            Log.LogInfo($"{Name} {Version} loaded with {Registry.Sets.Count} custom set(s), {Registry.Accessories.Count} accessor(ies) and {Registry.Furniture.Count} furniture piece(s)");
            Log.LogInfo("Supports " + Core.SetDef.LibraryCapability + " (shared card art in the plugin's Library folder)");
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
