# Building TCG Studio and the mod yourself

You don't need to build anything to use the mod. The prebuilt `TCG Studio.exe` on the
[releases page](https://github.com/GrimGrab/TCG-Studio/releases/latest) is built from this source.
This guide is for anyone who would rather compile it themselves or wants to change something.

## What gets built

| Part | Language | Output |
|---|---|---|
| **Mod** (`src/TCGCustomCards`) | C# (BepInEx 5 + Harmony) | `TCGCustomCards.dll`, loaded by the game |
| **TCG Studio** (`studio/`) | Go + Wails v2, Svelte 5 frontend | `TCG Studio.exe`, the editor and installer |
| Forge bridge (`forge-bridge/`) | Java, GPL-3.0 | `tcgcc-forge-bridge.jar`, MTG mode. **Prebuilt** in `src/TCGCustomCards/Assets/` |
| Foil shaders (`shaders/`) | Unity 2021.3.38f1 project | `tcgcc_foil` asset bundle. **Prebuilt** in `src/TCGCustomCards/Assets/` |

The finished `TCG Studio.exe` contains the mod DLL, the shader bundle, the bridge jar and BepInEx, and its **Setup** screen
installs them into the game.

## Requirements

- Windows 10 or 11
- **TCG Card Shop Simulator** installed, with **BepInEx 5** in the game folder. The mod compiles against the game's DLLs and
  BepInEx's DLLs. The easiest way to get BepInEx is to run the official TCG Studio once and click **Setup → Install / Repair**.
  Or install [BepInEx 5.4.23.x x64](https://github.com/BepInEx/BepInEx/releases) by hand and start the game once.
- [.NET SDK](https://dotnet.microsoft.com/download) 9 or newer
- [Go](https://go.dev/dl/) 1.26 or newer
- [Node.js](https://nodejs.org/) 18 or newer (with npm)
- Wails CLI v2.16:
  ```
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
  ```
  Make sure `%USERPROFILE%\go\bin` is on your PATH. `wails doctor` checks that everything is set up.

Only needed to rebuild the prebuilt parts (you can skip these):
- **Shaders:** Unity **2021.3.38f1** (the game's exact version), then `tools\build-shaders.ps1`
- **Forge bridge:** a JDK 17+ and Forge installed through TCG Studio (Setup → Install MTG mode), then
  `tools\build-bridge.ps1 -GamePath "<game folder>"`

## Build everything (one command)

From the repository root, in PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File tools\package.ps1 -GamePath "C:\Program Files (x86)\Steam\steamapps\common\TCG Card Shop Simulator"
```

Use your own game folder for `-GamePath`. In Steam: right-click the game → Manage → Browse local files.

The script:
1. builds the mod (Release),
2. collects the mod, the shader bundle, the bridge jar and BepInEx into the installer payload,
3. runs the installer and updater tests,
4. builds TCG Studio with Wails.

Result: **`dist\TCG Studio.exe`**. Run it and use **Setup → Install / Repair** like the official build.

### Updates in your own build

By default your exe checks this repository for new releases and offers to update itself, like the official one.
An update replaces your build with the official exe. To build an exe that never checks for updates:

```powershell
powershell -ExecutionPolicy Bypass -File tools\package.ps1 -GamePath "<game folder>" -UpdateRepo ""
```

Builds are not byte-for-byte identical to the official exe, so their SHA-256 won't match the one in the release notes.

## Build only the mod

```powershell
dotnet build src\TCGCustomCards\TCGCustomCards.csproj -c Release -p:GamePath="<game folder>"
```

If BepInEx is installed in that game folder, the build also copies the DLL, the shader bundle and the bridge jar into
`<game folder>\BepInEx\plugins\TCGCustomCards\`, so you can start the game right away.

## Working on TCG Studio

In `studio\`:

| Command | What it does |
|---|---|
| `wails dev` | Runs Studio with hot reload. A dev build has no installer payload, so its Setup screen says so. Use `tools\package.ps1` for a full build. |
| `wails generate module` | Regenerates the frontend bindings after you change a Go method on `App` |
| `go test ./internal/...` | Go tests. Some are skipped unless their test data is present. |
| `npx svelte-check` (in `studio\frontend\`) | Type-checks the Svelte frontend |

Keep Vite on version 6 if you're on Node 18.

## Repository layout

| Path | What |
|---|---|
| `src/TCGCustomCards/` | The mod: `Core` (set.json definitions and loader), `Runtime`, `Patches` (Harmony), `Save` (side-car save), `UI`, `Debug` |
| `studio/` | TCG Studio: `app*.go` (methods the UI calls), `internal/` (importer, installer, updater, art, UV maps, ...), `frontend/` (Svelte) |
| `forge-bridge/` | Headless Forge bridge for MTG mode (Java, GPL-3.0) |
| `shaders/` | Unity project for the foil shaders |
| `examples/` | Small example sets |
| `tools/` | Build scripts |
| `third_party/` | BepInEx 5 + ConfigurationManager package that Setup installs |
| `licenses/`, `THIRD_PARTY_NOTICES.txt` | Third-party licenses |
| `VERSION` | Single version for the mod and TCG Studio |

## Licenses

TCG Studio and the mod are MIT licensed (see [LICENSE](LICENSE)). `forge-bridge/` is GPL-3.0.
Third-party components keep their own licenses, listed in [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt).
