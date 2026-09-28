# Builds a shareable TCG Studio: Release mod + shader bundle + BepInEx package embedded into studio.exe, whose Setup screen
# installs everything into a friend's game with one button. Version = TCG Custom Cards\VERSION (also used by the mod);
# the exe checks tools\release.json's repo for newer GitHub releases.
# Output: TCG Custom Cards\dist\TCG Studio.exe
#   -GamePath "<game folder>"  game install to compile the mod against (default: the csproj's GamePath)
#   -UpdateRepo ""             build an exe that never checks for updates (default: tools\release.json's repo)
# Native tools (wails, go, dotnet) write progress to stderr, so rely on exit codes instead of ErrorActionPreference=Stop.
param(
    [string]$GamePath = "",
    [string]$UpdateRepo
)
$ErrorActionPreference = "Continue"
$root = Split-Path -Parent $PSScriptRoot
$studio = Join-Path $root "studio"
$payload = Join-Path $studio "internal\installer\payload"
$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\go\bin"

$version = (Get-Content (Join-Path $root "VERSION") -Raw).Trim()
$repo = if ($PSBoundParameters.ContainsKey("UpdateRepo")) { $UpdateRepo } else { (Get-Content (Join-Path $PSScriptRoot "release.json") -Raw | ConvertFrom-Json).repo }
if ($version -notmatch '^\d+\.\d+\.\d+$') { throw "VERSION must look like 1.2.3 (got '$version')" }

Write-Host "1/4 Building the mod v$version (Release)..."
$gameArg = @()
if ($GamePath) {
    if (-not (Test-Path (Join-Path $GamePath "Card Shop Simulator_Data\Managed\Assembly-CSharp.dll"))) { throw "Not the game folder: $GamePath" }
    if (-not (Test-Path (Join-Path $GamePath "BepInEx\core\BepInEx.dll"))) { throw "BepInEx 5 isn't installed in $GamePath (see BUILDING.md)" }
    $gameArg = @("-p:GamePath=$GamePath")
}
dotnet build (Join-Path $root "src\TCGCustomCards\TCGCustomCards.csproj") -c Release -v q -nologo @gameArg
if ($LASTEXITCODE -ne 0) { throw "mod build failed" }

Write-Host "2/4 Collecting installer payload..."
$dll = Join-Path $root "src\TCGCustomCards\bin\Release\netstandard2.1\TCGCustomCards.dll"
$bundle = Join-Path $root "src\TCGCustomCards\Assets\tcgcc_foil"
$bridge = Join-Path $root "src\TCGCustomCards\Assets\tcgcc-forge-bridge.jar"   # tools\build-bridge.ps1
$bep = Join-Path $root "third_party\BepInEx_5.4.23.5_x64_ConfigurationManager.zip"
foreach ($f in $dll, $bundle, $bridge, $bep) { if (-not (Test-Path $f)) { throw "missing $f" } }
Copy-Item $dll (Join-Path $payload "TCGCustomCards.dll") -Force
Copy-Item $bundle (Join-Path $payload "tcgcc_foil") -Force
Copy-Item $bridge (Join-Path $payload "tcgcc-forge-bridge.jar") -Force
Copy-Item $bep (Join-Path $payload "bepinex.zip") -Force
[IO.File]::WriteAllText((Join-Path $payload "version.txt"), $version)

Write-Host "3/4 Testing the installer and updater..."
Push-Location $studio
try {
    go test ./internal/installer ./internal/updater
    if ($LASTEXITCODE -ne 0) { throw "tests failed" }
    Write-Host "4/4 Building TCG Studio v$version..."
    # -trimpath: no local build paths (user name, folders) in the shipped exe
    wails build -clean -trimpath -webview2 embed -ldflags "-X main.Version=$version -X main.UpdateRepo=$repo"
    if ($LASTEXITCODE -ne 0) { throw "wails build failed" }
} finally { Pop-Location }

$dist = Join-Path $root "dist"
New-Item -ItemType Directory -Force $dist | Out-Null
$out = Join-Path $dist "TCG Studio.exe"
try { Copy-Item (Join-Path $studio "build\bin\studio.exe") $out -Force -ErrorAction Stop }
catch { throw "Couldn't write $out - close TCG Studio if it's running from the dist folder, then run this again. ($($_.Exception.Message))" }
Write-Host "Done: $out (v$version) - send this one file; its Setup screen installs everything."
