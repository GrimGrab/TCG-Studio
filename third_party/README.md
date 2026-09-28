# Third-party files shipped by the TCG Studio installer

`BepInEx_5.4.23.5_x64_ConfigurationManager.zip` — BepInEx 5.4.23.5 (x64, Mono) with BepInEx.ConfigurationManager, in
game-folder layout (as downloaded from the Nexus "BepInEx With Configuration Manager" package). Embedded into `studio.exe` by
`tools\package.ps1` and installed by the Setup screen.

- BepInEx — https://github.com/BepInEx/BepInEx — LGPL-2.1
- BepInEx.ConfigurationManager — https://github.com/BepInEx/BepInEx.ConfigurationManager — LGPL-3.0 (licence file inside the zip)

Replace the zip to ship a newer BepInEx 5 build (keep the same layout: `BepInEx/…`, `winhttp.dll`, `doorstop_config.ini`).
