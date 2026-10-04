# TCG Studio + Custom Cards mod

**Make your own cards, packs, accessories, figurines and furniture for TCG Card Shop Simulator — import real sets from Magic, Pokémon, Yu-Gi-Oh! and more, and play Magic: The Gathering at the shop's tables.**

TCG Studio is a Windows app that installs the mod for you and lets you build custom card sets, booster packs, deck boxes, playmats, figurines, shelves and more. Everything you make shows up in the game as new items. Your vanilla cards and shop are left alone unless you choose to hide them.

[![Support me on Ko-fi](https://storage.ko-fi.com/cdn/kofi2.png?v=3)](https://ko-fi.com/Z4N327UCRH)
&nbsp; 💬 **[Join our Discord](https://discord.gg/Pf75vnuN6B)**: share setups, request features and get help.

---

## Download

👉 **[Download the latest TCG Studio.exe](https://github.com/GrimGrab/TCG-Studio/releases/latest)**

You only need to download it once.

> **Prefer not to run a prebuilt exe?** Everything is open source. You can compile TCG Studio and the mod yourself with one
> command. See **[BUILDING.md](BUILDING.md)**.

### About updates

- When TCG Studio starts, it asks GitHub whether a newer version exists and shows a banner if there is one.
- **Nothing is ever downloaded or installed without you clicking "Update now".** "Later" hides the banner.
- Updates are only accepted from this repository, and their SHA-256 checksum must match the one in the release notes.
- The mod in your game only changes when you click **Install / Repair** on the Setup screen.
- **Don't want update checks at all?** Build it yourself with update checks turned off
  (`tools\package.ps1 -UpdateRepo ""`, see [BUILDING.md](BUILDING.md#updates-in-your-own-build)). That build never contacts GitHub for updates.

What changed in each version: **[CHANGELOG.md](CHANGELOG.md)**.

## Getting started

1. Download **TCG Studio.exe** and run it. It's a single file, so there's nothing to unzip.
2. Open **Setup** and click **Install / Repair**. This installs BepInEx, the in-game config menu and the mod into your game folder. Your other mods and settings are kept, and anything replaced is backed up first.
3. Open **Import sets** (or create a set from scratch), click **Install to game**, then start the game.

After an update, open **Setup** and click **Install / Repair** again to update the mod in the game.

You don't need to start the game before designing anything: TCG Studio reads the game's own models, art, icons and prices straight from your game folder.

## Features

### Import real card sets
One **Import sets** page with a source picker. Each import brings card art, rarities mapped to the game's rarities, real prices and a ready-made booster pack:

| Game | Source |
|---|---|
| Magic: The Gathering | Scryfall |
| Pokémon TCG (7 languages) | TCGdex |
| Yu-Gi-Oh! | YGOPRODeck |
| One Piece | TCGplayer |
| Star Wars: Unlimited | SWU-DB |
| Disney Lorcana | Lorcast |
| Flesh and Blood | TCGplayer |
| Union Arena | TCGplayer |

- Each game gets its own filters and sorts in the set editor (colour, type, cost, power, HP, ink, aspect, class…), and rarity sorting follows each game's real rarities.
- **Refresh prices** updates a set to today's real prices.
- Upcoming sets are marked and can't be imported before their card images exist.

### Custom card sets
- Build a set from scratch or edit an imported one: names, art, rarities, prices and play stats.
- Custom sets get their own **binder pages**, **market prices** (with daily price changes and price crashes) and **Check Price**, **Workbench** and **Bulk Donation** entries.
- **Gamify** turns real prices into game-like prices that fit the shop's economy (or a hybrid of both).
- Card backs per set or one global card back; holo foils with adjustable foil looks per border.
- Optional **Hide printed borders**: crops the printed border of full-art scans so the game's own frame sits right on the card.

### Packs & boxes
- Every set gets its own **booster packs and boxes** that you can order, stock on shelves, sell and open (by hand, by workers or in the auto pack opener).
- Set your own pack contents, rarity slots and prices.
- **Smart generate** builds pack and box art from the **real product photos** on TCGplayer (pick Play/Collector booster, 1st/Unlimited edition…, drag the corners), or from the set's most valuable card art. Imported sets get this art automatically.
- **Classic generate** makes simple pack and box art from a colour and a title in one click.
- Fine-tune everything in the **3D editor** with a live preview on the real game model: image, colour and text layers, free stretching, **Straighten** for photos taken at an angle, Match colours, copy layers to other faces, undo/redo and keyboard shortcuts.

### Accessories
- Make custom **deck boxes, playmats, sleeves, dice, comics, collection books and battle decks**. Paint them on the real game models in the 3D editor and sell them in your shop.
- **Painting templates**: export a texture template at the game's real size (every face outlined, named, measured and marked with an UP arrow), paint it in any image editor and import it back at full resolution (2048 or 4096 px stay sharp). Works for packs and boxes too.

### Figurines
Put **your own 3D models** on the shop shelves. Import `.glb`, `.gltf` or `.obj` (+ `.mtl`), turn and scale it, and check the fit next to the vanilla toy on a real shelf. Figurines are sold like the vanilla toys and fill the same shelf slots.

### Furniture
- Build new **shelves, card shelves, play tables, bins, storage, auto pack openers, workbenches, cash counters** and more on any vanilla piece, with your own name, price, shop level, colour, icon or 3D model.
- **3D spot editor**: add, move, turn and resize item spots, set how many items fit, and preview the shelf filled with any product.
- Edit where people sit, stand and work (table seats, cashier and queue, worker spots) and each piece's placement area.

### Magic: The Gathering mode
- Play **real MTG games at the shop's play tables** against customers, powered by the Forge rules engine. Cards are shown in 3D on the table in an arena-style layout.
- Build decks with the built-in **MTG deck builder** from the cards you've collected.
- **Smarter opponents**: customers build their decks with Forge's deck builder (Random 1–5 colours or Sealed), from the sets you choose, with a power level that can grow with your shop level, and a play style (Default / Cautious / Reckless / Experimental).
- See the customer's deck before and after the match; set deck size, starting life and same-deck rematches.
- Wins and losses are reported through the normal table game.
- TCG Studio installs Forge and Java for you from the Setup screen.

### Setups
- Keep several complete **setups** (sets, accessories, furniture, mod settings and card back) and switch the game between them from the sidebar. Each setup keeps its own game saves.
- **Export a setup** as one `.tcgsetup` file to share with friends, and import theirs. Exports never contain your saves or folder paths.

### Storage
The **Storage** page shows how much space your workspace, setups and sets use. Card art is stored once and shared between setups, can be shrunk to JPEG (about 6× smaller, with a before/after preview), and art no setup uses any more can be deleted.

### Custom-only mode
Want a shop that sells only your own content? Hide vanilla cards, packs, each accessory type, figurines and furniture separately. Turn them back on at any time.

### Debug log
**Setup → Debug log** shows the game's log from its last session with Copy and Save buttons, and warns you when the game loaded a different mod version, hasn't run since the mod was updated, or the mod didn't load. Your Windows user name is hidden in it.

## Settings

Change the mod's settings in TCG Studio's **Mod settings** page (searchable, no need to start the game), or press **F1** in game and look under **TCG Custom Cards** for:
- display, foil and grade label options
- vanilla content toggles
- MTG options (match, AI opponent, AI deck)

Changes are applied while the game is running.

## Saves

- Your custom collection, prices, placed custom furniture and MTG decks are stored in a **separate save file** next to the game's saves. The game's own save stays readable without the mod.
- Back up your saves before trying any mod: `%USERPROFILE%\AppData\LocalLow\OPNeonGames\Card Shop Simulator\`
- If you use several setups, turn off Steam Cloud for the game so each setup keeps its own saves.

## Requirements
- TCG Card Shop Simulator on Steam, Windows
- About 500 MB of free space if you use MTG mode (Forge + Java)

## Build it yourself

The full source of TCG Studio and the mod is in this repository. If you'd rather not run the prebuilt exe, see
**[BUILDING.md](BUILDING.md)** to compile it yourself with one command.

## Support

If you enjoy the mod, you can buy me a coffee on **[Ko-fi](https://ko-fi.com/Z4N327UCRH)**. It's completely optional, and everything stays free.

Found a bug? Open **Setup → Debug log**, copy it and post it on **[Discord](https://discord.gg/Pf75vnuN6B)** or in an issue here.

## Credits & licenses
- [BepInEx](https://github.com/BepInEx/BepInEx) and [ConfigurationManager](https://github.com/BepInEx/BepInEx.ConfigurationManager)
- [Forge](https://github.com/Card-Forge/forge) MTG rules engine (GPL-3.0). The bridge that talks to it is in [`forge-bridge/`](forge-bridge) (GPL-3.0).
- Card data and images are downloaded on your PC from [Scryfall](https://scryfall.com), [TCGdex](https://tcgdex.dev), [YGOPRODeck](https://ygoprodeck.com), [SWU-DB](https://swu-db.com), [Lorcast](https://lorcast.com) and TCGplayer (via [tcgcsv.com](https://tcgcsv.com)). No card data or images are included in this project.
- Magic: The Gathering is © Wizards of the Coast. Pokémon is © Nintendo / Creatures / GAME FREAK / The Pokémon Company. Yu-Gi-Oh! is © Konami. One Piece Card Game and Union Arena are © Bandai. Star Wars: Unlimited is © Fantasy Flight Games / Lucasfilm. Disney Lorcana is © Disney / Ravensburger. Flesh and Blood is © Legend Story Studios. This project is unofficial and not affiliated with any of them or with OPNeon Games.
- TCG Studio and the mod are released under the [MIT License](LICENSE). Third-party notices: [`THIRD_PARTY_NOTICES.txt`](THIRD_PARTY_NOTICES.txt).
