package modconfig

// migration:settings-binder-layout
// migration:settings-bulk-boxes
// The mod's ConfigMoves.Run has the same lists: either can open the file first.

// migration:settings-moved
// removedSettings are "section/key" of settings the mod dropped; BepInEx keeps such entries, so Studio removes them.
var removedSettings = map[string]bool{
	"Binder/Layout":   true, // now per save, in the binder panel
	"Debug/TestDraft": true, // the draft test hotkey is gone
	// Game 1.03 replaced the ten Tetramon/Destiny bulk boxes with six by grade (Tetramon designs carry over: renamedSettings).
	"Visuals - Game look - Boxes/BulkBox_TetramonBaseTexture": true, "Visuals - Game look - Boxes/BulkBox_TetramonDestinyTexture": true,
	"Visuals - Game look - Boxes/BulkBox_TetramonBaseGradeHGTexture": true, "Visuals - Game look - Boxes/BulkBox_TetramonBaseGradeMGTexture": true,
	"Visuals - Game look - Boxes/BulkBox_TetramonBaseGradePGTexture": true, "Visuals - Game look - Boxes/BulkBox_TetramonBaseGradeUGTexture": true,
	"Visuals - Game look - Boxes/BulkBox_TetramonDestinyGradeHGTexture": true, "Visuals - Game look - Boxes/BulkBox_TetramonDestinyGradeMGTexture": true,
	"Visuals - Game look - Boxes/BulkBox_TetramonDestinyGradePGTexture": true, "Visuals - Game look - Boxes/BulkBox_TetramonDestinyGradeUGTexture": true,
	// migration:settings-foil-looks
	// The holo foil became stock card shaders (their values are the shaders' parameters now).
	"Foil/HoloFoilMotion": true, "Foil/HoloFoilViewReact": true,
	"Foil - Base/BaseColor": true, "Foil - Base/BaseStrength": true, "Foil - Base/BasePattern": true,
	"Foil - First edition/FirstEditionColor": true, "Foil - First edition/FirstEditionStrength": true, "Foil - First edition/FirstEditionPattern": true,
	"Foil - Silver/SilverColor": true, "Foil - Silver/SilverStrength": true, "Foil - Silver/SilverPattern": true,
	"Foil - Gold/GoldColor": true, "Foil - Gold/GoldStrength": true, "Foil - Gold/GoldPattern": true,
	"Foil - EX/EXColor": true, "Foil - EX/EXStrength": true, "Foil - EX/EXPattern": true,
	"Foil - Full art/FullArtColor": true, "Foil - Full art/FullArtStrength": true, "Foil - Full art/FullArtPattern": true,
}

// renamedSettings: "section/new key" → old key (same section). A new setting the file doesn't have yet takes the old one's
// value (the mod's ConfigMoves.CarryRenamed does the same when the game writes the file first).
var renamedSettings = map[string]string{
	"Visuals - Game look - Boxes/BulkBoxNGTexture": "BulkBox_TetramonBaseTexture",
	"Visuals - Game look - Boxes/BulkBoxHGTexture": "BulkBox_TetramonBaseGradeHGTexture",
	"Visuals - Game look - Boxes/BulkBoxMGTexture": "BulkBox_TetramonBaseGradeMGTexture",
	"Visuals - Game look - Boxes/BulkBoxPGTexture": "BulkBox_TetramonBaseGradePGTexture",
	"Visuals - Game look - Boxes/BulkBoxUGTexture": "BulkBox_TetramonBaseGradeUGTexture",
}
