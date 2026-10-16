// Events the backend sends (app_events.go: keep both in step). Subscribe with EventsOn(Ev.X, fn); it returns the function
// that unsubscribes this handler only.
export const Ev = {
  ImportProgress: 'import:progress',
  ForgeProgress: 'forge:progress',
  SetupProgress: 'setup:progress', // Game & mod page: install / repair / uninstall
  SetupsProgress: 'setups:progress', // switching setups
  SetupsExport: 'setups:export', // {done, total}
  StorageProgress: 'storage:progress',
  StorageDone: 'storage:done',
  TemplatesProgress: 'templates:progress',
  TemplatesReady: 'templates:ready',
  UpdateProgress: 'update:progress',
} as const;
