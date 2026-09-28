# TCG Studio

The editor and installer for the TCG Custom Cards mod (Go + Wails v2, Svelte 5 frontend).

- Full build (mod + installer payload + exe): `tools\package.ps1` from the repository root. See `BUILDING.md`.
- Live development: `wails dev` in this folder. A dev build has no installer payload, so the Setup screen says so.
- After changing a Go method on `App`: `wails generate module`.
- Checks: `go test ./internal/...` here, `npx svelte-check` in `frontend\`.

Layout: `app*.go` holds every method the frontend calls. `internal/` has the backend packages (`setfmt` mirrors the
mod's set.json format, plus `importer`, `installer`, `updater`, `art`, `uvmap`, `forge`, ...). `frontend/src/` is the UI.
