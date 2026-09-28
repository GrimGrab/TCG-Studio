# Compiles the mod's shaders (TCG Custom Cards\shaders, a Unity 2021.3.38f1 built-in pipeline project) into the asset bundle
# src\TCGCustomCards\Assets\tcgcc_foil, which the mod build copies next to the DLL.
# Needs Unity 2021.3.38f1 (the game's exact version — bundles/shaders from newer editors may not load in the game).
param(
    [string]$Unity = "C:\Program Files\Unity\Hub\Editor\2021.3.38f1\Editor\Unity.exe"
)
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$project = Join-Path $root "shaders"
$log = Join-Path $project "Logs\build-shaders.log"
$out = Join-Path $root "src\TCGCustomCards\Assets"

if (-not (Test-Path $Unity)) { throw "Unity 2021.3.38f1 not found at $Unity (install it via Unity Hub)" }
New-Item -ItemType Directory -Force (Split-Path $log) | Out-Null

Write-Host "Building shader bundle with $Unity ..."
$p = Start-Process -FilePath $Unity -Wait -PassThru -NoNewWindow -ArgumentList @(
    "-batchmode", "-quit", "-nographics",
    "-projectPath", "`"$project`"",
    "-buildTarget", "Win64",
    "-executeMethod", "BuildBundles.Build",
    "-logFile", "`"$log`"")
Select-String -Path $log -Pattern "\[BuildBundles\]|error|Shader error|Exception" | ForEach-Object { $_.Line }
if ($p.ExitCode -ne 0) { throw "Unity exited with code $($p.ExitCode); see $log" }

New-Item -ItemType Directory -Force $out | Out-Null
Copy-Item (Join-Path $project "Build\tcgcc_foil") (Join-Path $out "tcgcc_foil") -Force
Write-Host "Copied bundle -> $out\tcgcc_foil"
