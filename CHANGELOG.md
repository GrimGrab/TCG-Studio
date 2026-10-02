# Changelog

What changed in each TCG Studio release (the mod and TCG Studio share one version). Downloads, licenses and checksums are on the
[releases page](https://github.com/GrimGrab/TCG-Studio/releases).

## v0.10.0 - 2026-10-02

Smarter MTG opponents built by Forge, more control over them, and a tidier Mod settings page.

- New: Customers build their MTG decks with Forge's own deck builder: Random (1–5 colours) or Sealed (they open boosters and build the best 40 cards). F1 → MTG - AI deck → AiDeckStyle.
- New: Pick which sets customers play: random sets you've licensed in the shop (default), any installed set, or the sets your deck uses, and how many sets they mix (AiDeckSets, AiDeckSetsMin/Max).
- New: Deck power from Forge's card ratings (Weak / Normal / Strong / Random), or let it grow with your shop level (AiDeckPowerFollowsShopLevel; full strength at level 35 by default).
- New: Deck size, Sealed booster count and colour weights (MTG - AI deck, MTG - AI deck colours).
- New: Customer play style (Forge's Default / Cautious / Reckless / Experimental AI, or random), same deck on rematch, and starting life for you and the customer (MTG - AI opponent, MTG - Match).
- New: A "Customer's deck" panel at the start of a match; after the match the customer shows their deck list. Hover a name to see the card.
- New: TCG Studio Mod settings: collapsible sections and groups, a search box, "customised" markers, settings that don't apply to your choices are hidden, and new settings show up right away without starting the game.
- New: "Join our Discord" button in TCG Studio. Share workspaces, request features and get help.
- Fixed: Blocking. Attackers a creature can't legally block (e.g. flyers) are marked while you drag, blocks show their line straight away, and you can move or take back a blocker by dragging or clicking it.
- Fixed: An equipped or enchanted creature lost its power/toughness tag and combat outline.
- Fixed: A morph turned face up showed a card from your hand. The AI's face-down cards no longer reveal themselves on hover.
- Changed: Foil grade and grade label settings moved into their own groups (Foil - Base … Foil - Full art, Visuals - Grade label). Your values are kept.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.9.0 - 2026-10-02

Setups: keep several custom-content setups, switch the game between them, and share them.

- New: Setups. Keep complete setups (sets, accessories, furniture, mod settings and the global card back) and switch the game between them from the setup menu at the top of the sidebar. Each setup keeps its own game saves, which are swapped too (close the game first). The first switch makes a one-time backup of your saves in the workspace folder.
- New: Export a setup as one .tcgsetup file to share with friends, and Import theirs. Exports never contain saves or folder paths from your PC.
- New: Mod settings works before you've ever started the game; Studio creates the settings file with default values.
- Changed: "My Sets" is now called "Sets".
- Fixed: installing a set that has no images failed.
- Note: on the first start, your existing sets and accessories become the setup "My Setup". Nothing in the game changes.
- Note: if Steam Cloud is on for the game, switching to a setup without saves may bring the previous setup's saves back from Steam. Turn off Steam Cloud for TCG Card Shop Simulator to keep setups apart.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.8.0 - 2026-09-30

Custom furniture: make your own shelves, card shelves, play tables and more in TCG Studio.

- New: Furniture page in TCG Studio. Build new furniture on any vanilla piece's type (shelf, card shelf, play table, bulk donation bin, trash bin, empty box storage, card storage shelf, auto pack opener, auto cleanser, workbench, cash counter) with your own name, description, price, shop level, colour, icon or your own 3D model (.glb/.gltf/.obj).
- New: 3D spot editor for shelves and card shelves: add, delete, move, turn and resize spots with drag handles, set how many items fit, and preview the shelf filled with any pack, box, accessory or figurine. Price tags and customer points are editable too.
- New: edit where people sit, stand and work: play table seats, the cashier and queue at the cash counter, worker spots at machines, customer stand points at bins and storage.
- New: edit each piece's placement area (the space kept free around it when placing).
- New: Mod settings → Content → ShowVanillaFurniture: turn off to sell only your own furniture (cash counter, workbench, trash bin and empty box storage stay until you make your own).
- New: full-view button on the 3D previews, undo/redo and keyboard shortcuts in the furniture editor (press ? in the view).
- Fixed: custom sets' progress could be lost when quitting the game (the save on quit didn't write the mod's save file).

Placed custom furniture is kept in the mod's save file, so your game still loads if you remove the mod.

After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.7.6 - 2026-09-29

TCG Studio no longer needs you to start the game before you can design packs, accessories and figurines.

- New: TCG Studio reads the game's own art, models, icons and prices straight from your game folder (a few seconds on first start, and again after a game update). Settings → Game shows the status and has a Read again button.
- Changed: the mod no longer exports templates in game; Setup → Install / Repair removes the old export folder (about 230 MB).
- Fixed: figurines, the "On a shelf" view and the pack/box 3D editor work right away on a fresh install.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.7.5 - 2026-09-29

Figurines now come out right even before you've played with the mod.

- Fixed: a figurine saved before the game had been started once with the mod could end up the wrong size, floating or sunk into the shelf. TCG Studio now knows the vanilla toy sizes and shelf slots from the start, so the shelf fit check works right away too.
No changes to the mod in the game this time.

## v0.7.4 - 2026-09-29

Custom figurines: put your own 3D models on the shop shelves.

- New: Figurines tab in Accessories. Import a model (.glb, .gltf or .obj with its .mtl), turn and scale it, and preview it next to the vanilla toy on a real shop shelf with a fit check. Your figurines are sold in the shop like the vanilla toys and fill the same shelf slots.
- New: setting Content → ShowVanillaFigurines to hide the vanilla figurines from the restock list and customer demand.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.7.3 - 2026-09-28

The source code is now public!

- New: the full source of TCG Studio and the mod is on GitHub under the MIT license, with a build guide (BUILDING.md) if you'd rather compile it yourself than run the prebuilt exe.
- Changed: the README explains exactly how updates work - nothing is ever downloaded or installed until you click Update now.
No changes to the mod in the game this time.
