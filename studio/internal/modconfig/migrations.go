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
