# TCG Studio + Custom Cards mod

**Make your own cards, packs and accessories for TCG Card Shop Simulator — and play Magic: The Gathering at the shop's tables.**

TCG Studio is a Windows app that installs the mod for you and lets you build custom card sets, booster packs, deck boxes, playmats and more. Everything you make shows up in the game as new items. Your vanilla cards and shop are left alone unless you choose to hide them.

[![Support me on Ko-fi](https://storage.ko-fi.com/cdn/kofi2.png?v=3)](https://ko-fi.com/Z4N327UCRH)

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

## Getting started

1. Download **TCG Studio.exe** and run it. It's a single file, so there's nothing to unzip.
2. Open **Setup** and click **Install / Repair**. This installs BepInEx, the in-game config menu and the mod into your game folder. Your other mods and settings are kept.
3. Create or import a card set, click **Install to game**, then start the game.

After an update, open **Setup** and click **Install / Repair** again to update the mod in the game.

## Features

### Custom card sets
- Import any **Magic: The Gathering set from Scryfall** with card art, rarities and prices, or build a set from scratch.
- Custom sets get their own **binder pages**, **market prices** (with daily price changes) and **Check Price** entries.
- Card backs can be set per set or globally, and holo foils are supported.

### Packs & boxes
- Every set gets its own **booster packs and boxes** that you can order, stock on shelves, sell and open.
- Design pack and box art in a **3D editor** with a live preview on the real game model. Use layers, free stretching and the **Straighten** tool to fix photos taken at an angle.
- Set your own pack contents, rarities and prices.

### Accessories
Make custom **deck boxes, playmats, sleeves, dice, comics, collection books and battle decks**. Paint them on the real game models and sell them in your shop.

### Magic: The Gathering mode
- Play **real MTG games at the shop's play tables** against the AI, powered by the Forge rules engine. Cards are shown in 3D on the table.
- Build decks with the built-in **MTG deck builder** from the cards you've collected.
- Wins and losses are reported through the normal table game.
- TCG Studio installs Forge and Java for you from the Setup screen.

### Custom-only mode
Want a shop that sells only your own sets? In the in-game config menu (**F1**), you can hide vanilla cards, packs and each accessory type separately. Turn them back on at any time.

## Settings

Press **F1** in game to open the config menu. Look under **TCG Custom Cards** for:
- display and foil options
- vanilla content toggles
- MTG options

Changes are applied while the game is running.

## Saves

- Your custom collection, prices and MTG decks are stored in a **separate save file** next to the game's saves. The game's own save stays readable without the mod.
- Back up your saves before trying any mod: `%USERPROFILE%\AppData\LocalLow\OPNeonGames\Card Shop Simulator\`

## Requirements
- TCG Card Shop Simulator on Steam, Windows
- About 500 MB of free space if you use MTG mode (Forge + Java)

## Build it yourself

The full source of TCG Studio and the mod is in this repository. If you'd rather not run the prebuilt exe, see
**[BUILDING.md](BUILDING.md)** to compile it yourself with one command.

## Support

If you enjoy the mod, you can buy me a coffee on **[Ko-fi](https://ko-fi.com/Z4N327UCRH)**. It's completely optional, and everything stays free.

Found a bug? Open an issue and include your `BepInEx\LogOutput.log` from the game folder.

## Credits & licenses
- [BepInEx](https://github.com/BepInEx/BepInEx) and [ConfigurationManager](https://github.com/BepInEx/BepInEx.ConfigurationManager)
- [Forge](https://github.com/Card-Forge/forge) MTG rules engine (GPL-3.0). The bridge that talks to it is in [`forge-bridge/`](forge-bridge) (GPL-3.0).
- Card data and images from [Scryfall](https://scryfall.com). Magic: The Gathering is © Wizards of the Coast. This project is unofficial and not affiliated with Wizards of the Coast or OPNeon Games. No Magic card data or images are included.
- TCG Studio and the mod are released under the [MIT License](LICENSE). Third-party notices: [`THIRD_PARTY_NOTICES.txt`](THIRD_PARTY_NOTICES.txt).
