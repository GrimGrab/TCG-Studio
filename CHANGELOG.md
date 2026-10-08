# Changelog

What changed in each TCG Studio release (the mod and TCG Studio share one version). Downloads, licenses and checksums are on the
[releases page](https://github.com/GrimGrab/TCG-Studio/releases).

## v1.3.0 - 2026-10-08

Custom decorations: your own wall, floor and ceiling looks, posters and 3D decorations.

- New: Decorations page in TCG Studio with tabs for Walls, Floors, Ceilings, Posters and Objects. Everything you make is sold in the phone's Buy Decoration app.
- New: Wall, floor and ceiling looks from any repeating texture, with optional normal and shine maps, colour and smoothness. Buy them once and switch them in the Decorate screen, for both shop areas.
- New: Posters from your own pictures: pick a picture, set the width and an optional frame and frame colour.
- New: 3D decorations from your own models (GLB, glTF or OBJ), hung on a wall or stood on the floor, with height and rotation. A vanilla poster or plant is shown beside it for scale.
- New: Textures for imported models can be picked separately (colour texture, normal map, ambient occlusion), for models that come without their textures. Normal and occlusion maps are baked in as shading.
- New: Importing an EPL mod now also converts its decorations (placeable decorations and wall/floor/ceiling looks).
- New: Decorations are in the catalog, Add from catalog, Storage page and setup sharing like furniture.
- New: Mod settings → Content - Decorations: ShowVanillaPosters, ShowVanillaDecoObjects, ShowVanillaSurfaces hide the vanilla ones from the Buy Decoration app (what you own stays).
- Fixed: Colours of GLB/glTF models came out much too dark (figurines, furniture and decorations).
- Changed: Normal and bump maps that come with a model are now baked into its texture, so surface detail shows in game.
- Changed: Furniture with its own model keeps its placement guides, and models with several materials draw all of them.
- Changed: Your decorations stay out of the vanilla save, like other custom content, so the save still loads without the mod.

After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.2.0 - 2026-10-08

Custom rarities: sets can have rarities of their own, and imports keep the real ones.

- New: Sets can have their own rarities (Set tab → Rarities): add, rename, reorder and colour them. Each rarity is its own in the game: its name shows on the cards, in the binder, Check Price and on graded slabs, and it has its own weight in pack slots.
- New: Imports keep the set's own rarities (Secret Rare, SR, Mythic…). The option "Keep the set's own rarities" is on the Import page and on by default. Sets imported earlier: Set tab → "Use the source's rarities".
- New: EPL mod imports keep every tier of the mod as a rarity, with the mod's exact pack odds, and cards get the mod's own prices instead of $0.10.
- New: The workbench and bulk donation rarity filter lists a set's own rarities.
- New: Pack contents is now a table of rarities × slots, showing each weight's share of the slot, cards per pack and how often you pull a particular card.
- New: View a card image full screen: double-click a card or click its picture; ← → to browse.
- New: EPL import: "Choose folder…" for unzipped mods, and Select all / Deselect all for accessories, figurines and furniture.
- Changed: The binder's rarity sort for custom sets follows the set's rarity order.
- Changed: Imported card prices can now go down to $0.01 (was $0.10).
- Fixed: EPL imports put the mod's pack art, box art and card back in the shared folder, so Storage no longer offers to move them right after an import.
- Fixed: Image-folder imports sorted images outside any subfolder (Common) as the highest rarity.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.1.0 - 2026-10-08

Paint your own furniture, and build custom furniture from every piece in the game.

- New: Furniture → Paint. Put images, text and colour fills on a furniture piece in the same editor as pack and box art, with a live 3D preview. The paint replaces the piece's tint; "Remove paint" brings the plain look back.
- New: all 43 vanilla furniture pieces can now be the base of custom furniture, including the warehouse shelf and the tournament prize shelf. Boxes stored on a custom warehouse shelf are kept when you save and load.
- New: Spots & positions can now move and resize the working parts of a piece (cash drawer, card machine, signs, the prize shelf's TV), with a size setting against the vanilla piece.
- Changed: a furniture piece with your own model can be moved, turned and resized right in the 3D view (keys 1 / 2 / 3), and you can move its spots onto the model.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.0.2 - 2026-10-07

Hide-vanilla settings now always work.

- Fixed: turning off a Content → ShowVanilla… setting sometimes left the vanilla items in the shop even though you had custom content (for example with only custom sets or furniture installed, or when a custom item failed to load).
- Changed: the Content → ShowVanilla… settings are now a plain switch: off always hides that vanilla content, even if you have no custom replacement. With no custom sets, customers stop coming to trade cards; with no custom packs, play-table matches give no pack prize.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.0.1 - 2026-10-06

Imported sets no longer leave files behind in your setup, and the Storage page now shows exactly what it will move.

- Fixed: Importing a set kept its pack & box art and set icon inside the setup, so Storage asked you to "Move everything to shared" after every import. New and edited pack/box art, Smart generate images, card backs, product photos and rotated card images now go straight to the set's shared folder.
- Fixed: "Move everything to shared" could keep the catalog's older pack art instead of the art made in your setup, and list yours as "differs". Your setup's art now wins.
- Changed: A set that's already in your catalog (imported in another setup or earlier) is added with "Add from catalog" instead of being imported again: no second download, and the shared art other setups use is never overwritten. To import it again, remove it on the Catalog page first.
- Changed: Editing the pack/box art of a set, or rotating its card images, updates it in every setup that uses that set. Sets you kept as their own art keep their own copy.
- New: Storage → Move everything to shared → "Show the files" lists every file it will move or delete, and the summary shows how much is moved and how much is freed. Accessory files are now included in the count.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.0.0 - 2026-10-06

TCG Studio 1.0: bring in content from other mods and your own card pictures, and reuse everything you've imported across all your setups.

- New: Import → Image folder. Turn a folder of your own card pictures into a set, with an optional cards.csv for names, rarities and prices. Handy for games that have no online card database.
- New: Import → EPL mod (experimental). Converts an Enhanced Prefab Loader mod (folder or zip) into TCG Studio sets, with their pack and box art, card back and odds, plus their accessories, figurines and furniture, in one go.
- New: Catalog page. Everything you've ever imported or made (sets, accessories, furniture) in any setup, and which setups use it. Delete something here to remove it everywhere.
- New: Add from catalog… on the Sets, Accessories and Furniture pages, and on the Import page for sets you already imported somewhere else: adds it to this setup instantly, with no download or re-import. Prices and tiers stay per setup.
- New: Storage → Move everything to shared. Card art, accessory and furniture files are kept once for all your setups instead of once per setup, which saves disk space. You choose what happens when two setups have different copies of the same file. Storage also lists files no setup uses, so you can delete them.
- New: Sets, Accessories and Furniture can be sorted by source, and converted content shows which EPL mod it came from.
- Changed: Mod settings now shows which setup the settings belong to. Each setup keeps its own mod settings, so switching setups also switches them (for example "Show vanilla cards").
- Changed: on the Import page, "Add from catalog" sits under the Import button.
- Fixed: custom packs and boxes showed the vanilla pack art in the game after their art was moved to the shared library.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.15.3 - 2026-10-05

Magic games now work in tournaments.

- Fixed: with Magic mode on, sitting at your tournament table said "deck incomplete" and you couldn't play your round. Tournament tables now offer Magic (Forge) like any other table, and your win or loss counts for the tournament.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.15.2 - 2026-10-04

Rotate card pictures right in TCG Studio, and delete single cards.

- New: ⟲ / ⟳ buttons next to "Change image…" turn a card's picture. With several cards selected you can rotate them all, or click "Turn landscape cards upright" to fix only the sideways ones. The game is updated automatically; restart the game to see it.
- New: Delete card button when one card is selected (before, delete only showed with two or more selected).
- Fixed: pictures turned with the Rotate button in Windows Photos (or taken sideways on a phone) showed upright in Studio but sideways in the game. They are now turned upright when you add them or use "Change image…". Cards added before this update: use the new rotate buttons.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.15.1 - 2026-10-04

Setup now has a Debug log to help us sort out problems.

- New: Setup → Debug log shows the game's log from its last session, with Copy to clipboard and Save as file buttons. If something doesn't work in game, close the game, copy it and send it to us on Discord. Your Windows user name is hidden in it.
- New: the Debug log tells you when the game loaded a different mod version than this TCG Studio installs (for example when Studio is set to a different game folder than the one Steam starts), when the game hasn't run since the mod was updated, or when the mod didn't load.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.15.0 - 2026-10-04

Paint your own high-resolution textures for accessories, packs and boxes in any image editor.

- New: Export template… in the face editor (deck boxes, playmats, sleeves, comics, collection books, battle decks, card packs and card boxes). It saves an image of the game texture at its real size with every face outlined, named, measured in pixels and marked with an UP arrow; areas the game stores mirrored or on their side say so.
- New: Import painted texture… uses the image you painted on a template as the whole texture, at the resolution you painted it (2048 or 4096 px stay sharp). Image, colour and text layers still go on top, and the painted texture can be replaced or removed at any time.
- Changed: Smart generate asks before replacing pack or box art that uses a painted texture.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.14.0 - 2026-10-04

Your sets take much less disk space: card art is shared between setups and can be stored as JPEG.

- New: Storage page (sidebar). See how much space your workspace, each setup and each set use. Move card art into a shared library so a set that is in several setups is stored once. Shrink card art to JPEG (about 6× smaller) per set, with a before/after preview. Delete the card art of sets no setup uses any more.
- New: a set's card art is kept once and shared by every setup that has it. Importing a set you already have in another setup downloads nothing, and switching setups copies far less into the game.
- New: Settings → Downloads → default card image format (PNG or JPEG). The Import page offers the same choice for each import, and asks before downloading a separate copy of a set that is already on your PC in the other format.
- Changed: card art you change in one setup stays that setup's own; your other setups keep the original.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.13.3 - 2026-10-03

Shop lists no longer cut off when you have many sets.

- Fixed: the phone's stock shop stopped after about 24 rows, so packs and boxes of later sets couldn't be bought. All items now show, however many sets you install.
- Fixed: the same limit in Check Price → Items, the board game tab of the stock shop and the furniture shop.
- Fixed: the cart no longer reports "full" early when it holds many different products.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.13.2 - 2026-10-03

Union Arena sets can now be imported.

- New: Import sets → Union Arena. Every Union Arena set from TCGplayer (boosters, starter decks, promos), with card scans, real prices, colour filter and Energy / BP sorts. Star parallels, box toppers and serial-numbered cards come in as variants; Action Point cards are left out.
- New: Find product photos and Smart generate work for Union Arena packs and boxes.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.13.1 - 2026-10-03

The classic pack & box art generator is back.

- New: "Classic generate…" in the Packs tab's Art section, next to "Smart generate…": the original generator. Pick a colour and a title (or your own image for the pack front) and it makes the pack and box art in one click.
- Note: the 3D editor keeps its own layers. To edit on top of classic art there, set "Start from" to "This pack's current art" first.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.13.0 - 2026-10-03

Pack and box art from real product photos, and a better art editor.

- New: Smart generate (Packs → Art) builds a set's pack and booster box from the real product photos on TCGplayer, for Magic, Pokémon, Yu-Gi-Oh!, One Piece, Star Wars: Unlimited, Lorcana and Flesh and Blood. Sets without photos use their most valuable card art.
- New: Smart generate lets you pick which pack and box photos to use (e.g. Play or Collector booster, 1st or Unlimited Edition) and drag the corners of the pack, the box's front panel and its lid before building.
- New: imported sets get this pack and box art automatically.
- New: Find product photos and Match colours buttons in the pack/box editor.
- New: the 3D art editor (accessories, packs, boxes) can copy a layer to another face ("Opposite" for front/back, left/right), duplicate layers, and has Ctrl+C / Ctrl+V, Ctrl+D, Delete, Ctrl+Z undo and Ctrl+Y redo.
- Changed: generated packs no longer print the set name; the old Quick start generator is gone (its simple design is the fallback when a set has no photos or card images).
- Changed: Gamify now defaults to Game-like pricing (Hybrid is still available). Sets you already gamified keep their setting.

After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.12.0 - 2026-10-02

Import Yu-Gi-Oh!, One Piece, Star Wars: Unlimited, Lorcana and Flesh and Blood sets.

- New: "Import sets" now also offers Yu-Gi-Oh! (YGOPRODeck), One Piece (TCGplayer), Star Wars: Unlimited (SWU-DB), Disney Lorcana (Lorcast) and Flesh and Blood (TCGplayer), with card art, real prices and a ready-made booster pack.
- New: Each game gets its own filter and sorts in the set editor's Cards tab: Card type, Level and ATK for Yu-Gi-Oh!; Color, Cost and Power for One Piece; Aspect for Star Wars: Unlimited; Ink, Ink cost and Strength for Lorcana; Class for Flesh and Blood. Rarity sorting follows each game's own rarities. Magic gains Mana value and Power sorts, Pokémon an HP sort.
- New: Optional setting Visuals → HidePrintedBorders (off by default) crops the printed border of full-image cards so the game's border frames the card directly. Visuals → PrintedBorderMax sets the widest border it will crop.
- Changed: Default boosters no longer use a rarity a set doesn't have (for example modern Yu-Gi-Oh! sets have no Rares); that slot uses the next rarity down.
- Changed: Sets that aren't released yet are marked "upcoming" in the Import sets list and can't be imported before card images exist. Cards whose image fails to download are left out instead of being kept without art.
- Fixed: Yu-Gi-Oh! cards now fill the card frame (their art is narrower than other games' cards).
After updating: open Setup and click Install / Repair to update the mod in the game.

## v0.11.0 - 2026-10-02

Import Pokémon sets, on a new "Import sets" page.

- New: "Import sets" page (replaces "Import from Scryfall"). Pick where to import from at the top: Magic: The Gathering (Scryfall) or Pokémon TCG (TCGdex).
- New: Pokémon sets from TCGdex: card art, real TCGplayer/Cardmarket prices (the reverse holo price sets the foil price), rarities mapped to the game's rarities and a ready-made booster pack. Choose the card language (English, French, German, Italian, Spanish, Portuguese, Japanese). Refresh prices works for Pokémon sets too.
- New: In the set editor's Cards tab, Pokémon sets can be filtered by type (Fire, Water, Grass… plus Trainer and Energy) and by the card's own rarity (e.g. Special illustration rare). Sorting by rarity now orders cards by their real rarity within each game rarity (for Magic: mythics before rares).
- Fixed: The Sets page could show no sets right after Studio started.
After updating: open Setup and click Install / Repair to update the mod in the game.

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
