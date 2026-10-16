package main

// Events the backend sends to the frontend (runtime.EventsEmit). frontend/src/lib/events.ts lists the same names.
const (
	evImportProgress    = "import:progress"
	evForgeProgress     = "forge:progress"
	evSetupProgress     = "setup:progress"   // Game & mod page: install / repair / uninstall
	evSetupsProgress    = "setups:progress"  // switching setups
	evSetupsExport      = "setups:export"    // {done, total}
	evStorageProgress   = "storage:progress" // library.Progress
	evStorageDone       = "storage:done"     // StorageResult
	evTemplatesProgress = "templates:progress"
	evTemplatesReady    = "templates:ready"
	evUpdateProgress    = "update:progress"
)
