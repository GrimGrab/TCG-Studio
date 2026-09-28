# TCG Custom Cards — Forge bridge

Runs [Forge](https://github.com/Card-Forge/forge) (Magic: The Gathering rules engine and AI) without a window, so TCG Card Shop
Simulator can host MTG games at its play tables. The game starts this program and talks to it over stdin/stdout, one JSON
object per line (`Bridge.java` reads the game's messages; `StateWriter.java` and `BridgeGuiGame.java` write the game state,
prompts and questions). The full protocol is in the workspace doc `docs/mtg-forge.md`, under "Bridge protocol".

This is the source of `tcgcc-forge-bridge.jar`, which ships with TCG Custom Cards / TCG Studio
(https://github.com/GrimGrab/TCG-Studio/releases).

## Build

Needs a JDK 17 or newer and Forge **2.0.14** (`forge-gui-desktop-2.0.14-jar-with-dependencies.jar`, from
https://github.com/Card-Forge/forge/releases/tag/forge-2.0.14 — TCG Studio's Setup screen installs it into `<game>\TCGForge\forge`).

- Windows, with the TCG Custom Cards workspace: `tools\build-bridge.ps1` (included in the source download). Output:
  `src\TCGCustomCards\Assets\tcgcc-forge-bridge.jar`.
- Anywhere, by hand:

  ```
  javac --release 17 -encoding UTF-8 -cp forge-gui-desktop-2.0.14-jar-with-dependencies.jar -d out src/tcgcc/bridge/*.java
  jar --create --file tcgcc-forge-bridge.jar -C out .
  ```

The game runs it with Forge's jar on the class path and `tcgcc.bridge.Bridge` as the main class.

## License

Copyright (C) 2026 GrimGrab.

This bridge links against Forge, which is licensed under the GNU General Public License v3.0, so the bridge (this folder) is
also licensed under the **GNU General Public License v3.0** — see `LICENSE`. This program is distributed in the hope that it will
be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

The rest of TCG Custom Cards (the BepInEx mod and TCG Studio) only talks to the bridge as a separate program over stdin/stdout,
and is not covered by this license. Forge itself is not part of this download; TCG Studio downloads it from Forge's own
GitHub releases.
