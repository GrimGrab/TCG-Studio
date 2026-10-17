# Changelog

What changed in each TCG Studio release (the mod and TCG Studio share one version). Downloads, licenses and checksums are on the
[releases page](https://github.com/GrimGrab/TCG-Studio/releases).

## v1.10.1 - 2026-10-16

Accessory art now covers every visible part of the model.

- Fixed: card sleeves — the hang tab's art now continues over the crimp below it, and the tab's thin edges and the left box edge no longer show the vanilla colour.
- Fixed: deck boxes — no more red lines along the top of the sides, around the lid or at the front cover's edges when seen from a distance.
- Fixed: battle decks — the lid is covered right to its edges, and the rim around the top of the box takes your side colour instead of the vanilla band.
- Fixed: comics — thin uncovered lines along the cover and spine edges.
- New: comics have a "Page edges" face to repaint the page block (front edge, top and bottom).
- New: playmats have an "Underside" face (and a "Rim" colour) to paint the rubber back.
After updating: open your accessories in Studio and save them again to apply the fixes to their textures.

## v1.10.0 - 2026-10-16

Share single sets and items with other players, without sending a whole setup.

- New: Catalog → select sets, accessories, furniture or decorations → Export… saves them as one .tcgitems file (card art and all their files included).
- New: Import → "TCG Studio file": open or drop a .tcgitems file, pick which items to take, say where they came from (name, made by, link), and import. They go into your catalog and, if ticked, into the setup you're in. Nothing you already have is ever replaced: anything with an id you already use gets a new one.
- Changed: setup files (.tcgsetup) use the same format now; older setup files still import.
- Changed: new sets, accessories, furniture and decorations get random ids, so items from different players never clash. "New empty set" only asks for a name.
- Changed: the Catalog's selection bar (Export…, Delete) stays at the bottom of the window while you scroll.
- Changed: content converted from EPL mods stays on your PC and is left out of exports (its art belongs to the mod's authors).
- Fixed: the Import page could show a stuck progress bar after exporting or fetching back faces.
After updating: open Game & mod and click Install / Repair to update the mod in the game.

## v1.9.2 - 2026-10-16

Storage clean-up: several GB back for anyone who imported zipped EPL mods.

- Fixed: Importing an EPL mod from a .zip/.rar/.7z left a full unpacked copy of the download in TCG Studio's cache folder (often several GB per mod). Studio now removes it once the import is done, and clears any leftovers when it starts.
- Changed: Switching setups no longer makes an extra copy of all your game saves before the first switch. The switch already undoes or finishes itself if it gets interrupted. Old copies (the "saves-backup-…" folder in your TCG Studio folder) are deleted when Studio starts.
- Changed: Storage → Workspace now shows shared storage as one number, with setups & saves, download cache and other files listed separately.
After updating: open Game & mod and click Install / Repair to update the mod in the game.

## v1.9.1 - 2026-10-16

Storage: clean up TCG Studio's backup folders.

- New: Storage → Backup folders in the game folder lists every TCGStudio_backup folder (from Game & mod → Install / Repair and from "Move the game's copies out"), with its date, what made it and its size. Open it, delete one, or delete them all. Close the game first.
- Changed: "Move the game's copies out" now points you to that list to delete the backup once the game shows everything.
After updating: start TCG Studio with the game closed; it updates the mod in the game for you. If Game & mod shows a dot, click Install / Repair there.

## v1.9.0 - 2026-10-16

Your content now lives in one place, and TCG Studio moves it there for you.

- New: the game reads your sets, accessories, card back and Shop look straight from your TCG Studio setup. The game folder only holds the mod. On its first start TCG Studio moves your content over and updates the mod in the game by itself; close the game first (it waits while the game runs). Nothing is deleted: files that aren't exact copies are kept (Storage → Kept by the upgrade).
- New: Storage → Remove the game's copies moves the old copies out of the game folder into a backup folder, once the game shows everything.
- New: Storage → Compress mod save files makes the mod's save files about 20× smaller (the mod reads both kinds).
- New: rename your shop on the Shop look page (game closed).
- New: Shop look has a tab per part (sign, phone wallpaper, open/closed sign, card back). The global card back moved there from Settings, and you can now move and resize the part of the picture it uses. A set can still have its own back on its Set tab.
- New: Furniture has a tab per type instead of the type list.
- Changed: the sidebar groups your setup's pages under the setup picker, with the pages all setups share below. The Setup page is now called Game & mod.
- Changed: Shop look saves only when you click Save (with Revert, and a warning when you leave with unsaved changes).
- Changed: accessories, furniture and decorations reach the game when you save them; there's no separate install step any more.
- Fixed: the binder no longer crashes on sets the game keeps no collection for (another mod's sets, or custom sets that aren't loaded).
- Fixed: when content a save uses is missing, the mod keeps that save data until the content is back, and tells you in game.
After updating: start TCG Studio with the game closed; it updates the mod in the game for you. If Game & mod shows a dot, click Install / Repair there.

## v1.8.2 - 2026-10-12

Binder: see every version of a card on its own.

- New: "Every version separately" in the binder panel (Sort Album → Show). Each border and foil version gets its own slot, 12 per card like the game's binder, and keeps every enhanced binder feature: search, filters, sorts, several sets at once, favourites and Home/End. The counter at the top then counts versions. With "Gaps for missing", versions you don't own show as empty slots.
- Changed: the choice is saved per save file, so an early-game save can show versions apart while a late-game save keeps one slot per card.
- Removed: the Binder → Layout setting (it used to turn the enhanced binder off). Saves that don't have their own choice yet start with your old setting.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.8.1 - 2026-10-11

Controller support for the improved binder, version totals, and EPL mods grouped by name.

- New: Controller support in the binder panel: move with the d-pad or left stick, A to select, B to close, Y to apply, X to reset, LB/RB to scroll a list. The search box opens the on-screen keyboard.
- New: While browsing the binder with a controller, R3 switches the card you're aiming at to its next version and L3 marks it as a favourite (Binder → ControllerVersionButton / ControllerFavouriteButton). The button hints at the bottom of the screen show both.
- New: First page / Last page buttons in the binder panel.
- New: A second count under the binder's cards collected: every border and foil version you own, out of all versions in the shown sets. The panel header shows it too.
- Changed: Sets converted from EPL mods are now grouped by their mod in the binder panel instead of all under "Other sets". Installed sets pick this up when Studio starts; a collection name you typed yourself is kept.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.8.0 - 2026-10-11

A rebuilt card binder: one slot per card, search and filters, and whole collections in one binder.

- New: Enhanced binder. Each card takes one slot showing your best version; the small squares beside its count show which border/foil versions you own. Mouse wheel over a card switches between them.
- New: Binder panel (your Sort Album key): pick sets, sort (set order, name, price, total value, copies, rarity, element, or mana value/colour for Magic), search by name, artist, number or type line, and filter by border, finish, rarity, duplicates, favourites, and Magic colour/type/mana value. The header shows collected cards, value and matches before you press Apply.
- New: Collections. View any mix of sets as one binder: tick a whole collection (e.g. every Magic set) or single sets, or All sets.
- New: Favourites. Press T over a card (or up close) to mark it with a red heart; filter to favourites in the panel.
- New: The graded album gets the same panel: sort by grade, newest, price and more; filter by grade, set, rarity and more.
- New: Home / End jump to the first / last binder page; the close-up shows the card's number and set.
- New: Your binder choices, favourites and picked versions are saved with your game.
- New (TCG Studio): Collection and Release date on the Set tab. Imported sets get them automatically; type one for image-folder or EPL sets (e.g. Gundam) so the binder can group them.
- Changed: New settings Binder → EnhancedBinder (turn the new binder off to get the game's own), Binder → Layout (OnePerCard / Classic) and Binder → FavouriteKey.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.7.2 - 2026-10-11

Phone wallpaper and your own open/closed sign.

- New: Shop look page in TCG Studio (sidebar). The shop sign settings moved here from Settings.
- New: Phone wallpaper: pick any picture and crop it to the phone's screen. The preview shows your app buttons on top.
- New: Open/closed sign at the door: give each side a board colour or your own picture, and write your own words on top in any font, colour, size and outline. The sign's black frame and screws stay.
- Changed: the shop sign's font, colour, size and outline controls are the same as the door sign's.
- New F1 settings: Visuals - Shop look → PhoneWallpaperImage, PhoneWallpaperCrop, OpenSignImage, OpenSignCrop, ClosedSignImage, ClosedSignCrop (set from the Shop look page).
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.7.1 - 2026-10-10

See the top of your library in Magic games, plus an import queue in TCG Studio.

- New: Magic games show cards Forge lets you see in hidden zones. With Future Sight, Courser of Kruphix, Bolas's Citadel and similar cards, a "Top: <card>" button appears next to Graveyard and Exile: hover it to see the card, click it and then the card to play it. The opponent's revealed top card shows on their side, Telepathy shows their hand, and face-down cards you may look at (e.g. Gonti's) show their real face.
- New: Graveyard, Exile and library buttons light up when they hold a card you can play or use right now (flashback, cast from exile, the top of your library).
- Fixed: after clicking a card in the graveyard or exile view (e.g. to flashback it), the view now closes so you can pay the cost and pick targets.
- New: TCG Studio imports queue up. Click Import on more sets while one is importing, or use Import all to import every set in the list. The queue keeps going while you use other pages; the Import tab shows a dot while it runs, and you can take sets out of the queue or cancel one or all.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.7.0 - 2026-10-09

Magic: Commander against customers, plus double-faced cards you can turn over.

- New: Commander decks in the MTG deck builder: + New Commander deck, right-click a legendary creature to make it your commander (Forge checks it can be one), then add 99 different cards. Forge checks the finished deck (colour identity, legal commander, ban list).
- New: Play Commander at a table: pick a Commander deck when you sit down. 40 life, the customer brings a Commander deck Forge builds from your sets, and a commander row shows commander tax and commander damage (click your commander to cast it from the command zone).
- New: Magic: Commander tournament format with its own set list; your deck is checked against those sets.
- New: Double-faced cards show their back face: when a card transforms in a game, in the deck builder and draft previews (both sides side by side), and on the back of the card (display racks, your hand). Press F to turn a double-faced card over in the binder close-up or while holding it.
- New: TCG Studio downloads back-face pictures for double-faced cards. For sets you already imported, open the set and click Download back faces. The card detail, full-screen viewer and grid show both sides.
- New settings: MTG → PreloadForge (starts Forge in the background after a save loads, so the first game doesn't wait), MTG - Match → YourCommanderLife and CustomerCommanderLife.
After updating: open Setup and click Install / Repair to update the mod in the game, then use Download back faces on your Magic sets.

## v1.6.0 - 2026-10-09

Magic tournaments: Constructed with your own set list, plus booster Draft and Sealed with packs you put up.

- New: Host Tournament has a format panel: Tetramon, Magic: Constructed, Magic: Draft or Magic: Sealed. It can be minimized.
- New: Magic: Constructed tournaments use the sets you pick (one or many). Forge checks every deck against them and only legal decks can play; the customers' decks use the same sets.
- New: Choose your deck when you sit down to play Magic at any table.
- New: Magic: Draft and Magic: Sealed tournaments. Hold MTG packs and right-click the Tournament Prize Shelf to put them up for the tournament: only entrants get them, and leftovers come back as delivered boxes (cancelling returns them all). Entrants pay your pack price on top of the entry fee; overpriced packs mean fewer sign-ups. On the day, go to your table to draft (or open your sealed pool) and build a 40-card deck; the cards are yours to keep. Forge drafts and builds the customers' decks, and the stronger deck wins customer matches more often.
- New settings: MTG - Draft → PodSize, MinPicksPerPlayer, StrengthWeight, PausesClock (the shop clock stops while you draft). Debug → TestDraft (Shift+F10) tries a draft without a tournament.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.5.0 - 2026-10-08

Bigger packs, real booster layouts for every game, and per-set card versions.

- New: packs can hold 1 to 24 cards. Big packs open like normal ones - every card is revealed in turn and the final screen lays them out in rows of up to 8. Vanilla packs open exactly as before.
- New: imported sets get their game's real booster for their era: Magic (Play, Draft, Classic, 12- and 8-card), Pokemon (Scarlet & Violet, 2007-2023, e-Card/EX, Base Set-Neo), Yu-Gi-Oh!, One Piece, Star Wars: Unlimited, Lorcana, Flesh and Blood and Union Arena. The rarest slot comes last, like opening a real pack.
- New: the Packs tab "Apply preset" list has the same real boosters, grouped by game. Applying one sets the slots, card count and foil chance.
- New: Set tab "Card versions" - choose which of the 12 border/foil versions a set has. Versions that are off never come out of packs or trades and don't take binder or Check Price slots; cards you already own in them stay visible, and the binder's total counts only the versions that are on.
- Changed: a pack's size now follows its slots. Gamify prices packs by their card count.
- Changed: the old 7-card "MTG-like" preset is gone.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.4.0 - 2026-10-08

MTG mode: choices that used to freeze the game now work, and the table shows more of what is going on.

- Fixed: searching your library (Cultivate, tutors and similar) - you now pick the cards in a dialog instead of the game getting stuck.
- Fixed: cards that make you choose from revealed cards, like the opponent's Karn, Scion of Urza, no longer stall after the reveal.
- Fixed: choosing a card from the opponent's hand (Duress and similar) now opens a picker.
- Fixed: Scry and Surveil with one card now show the card you are deciding about.
- New: when a cost or target uses cards in a graveyard or exile, those cards appear in a strip at the top of the table, and the Graveyard/Exile buttons light up.
- New: attacking creatures show what they attack (the player or a planeswalker), with a line to an attacked planeswalker. While you declare attackers, the panel says what new attackers will attack and how to switch.
- New: while a spell or ability is on the stack, its targets are framed in purple (hover a stack entry to see that one's targets).
- New: "Other..." in a number list (X spells) opens a number box instead of freezing the game.
- New: very long lists, such as naming a card for Pithing Needle, have a search box.
- Changed: reveal windows now say there is nothing to choose, so they aren't mistaken for a choice.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.3.2 - 2026-10-08

Customise the shop name on your shop sign.

- New: Settings → Shop sign now lets you change the shop name's font: the game's own font, other fonts from the game, any font installed on Windows, or your own .ttf/.otf file.
- New: pick your own colour, size and outline (with outline colour) for the shop name, with a live preview of the sign before you go in game. Also in F1 under Visuals - Shop sign text.
- Fixed: removing the sign picture no longer deletes other sign files.
After updating: open Setup and click Install / Repair to update the mod in the game.

## v1.3.1 - 2026-10-08

Put your own picture on the sign above your shop.

- New: TCG Studio Settings → Shop sign: choose any picture, then drag the box over it (or drag a corner to resize) to pick the part shown on the sign above your shop's entrance. It glows at night like the original sign, and changes show in a running game right away. Remove brings the original sign back.
- New: "Show the shop name on the sign" option (same place) — turn it off when your picture already has a name or logo.
After updating: open Setup and click Install / Repair to update the mod in the game.

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
