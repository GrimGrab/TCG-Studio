# Compiles the Forge bridge (TCG Custom Cards\forge-bridge, Java, GPL-3.0) into src\TCGCustomCards\Assets\tcgcc-forge-bridge.jar,
# which the mod build copies next to the DLL. Compiles against the Forge jar that TCG Studio installed (<game>\TCGForge) — the
# bridge must be rebuilt when the pinned Forge version changes. Needs a JDK 17+ (javac + jar).
param(
    [string]$GamePath = "D:\SteamLibrary\steamapps\common\TCG Card Shop Simulator",
    [string]$Jdk = ""
)
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$src = Join-Path $root "forge-bridge\src"
$tmp = Join-Path $root "forge-bridge\out"
$out = Join-Path $root "src\TCGCustomCards\Assets\tcgcc-forge-bridge.jar"

if (-not $Jdk) {
    # javac on PATH is often Oracle's javapath shim (no jar.exe next to it): look for a real JDK folder.
    $candidates = @()
    if ($env:JAVA_HOME) { $candidates += $env:JAVA_HOME }
    $candidates += Get-ChildItem "C:\Program Files\Java", "C:\Program Files\Eclipse Adoptium" -Directory -ErrorAction SilentlyContinue |
        Sort-Object Name -Descending | ForEach-Object { $_.FullName }
    $Jdk = $candidates | Where-Object { Test-Path (Join-Path $_ "bin\jar.exe") } | Select-Object -First 1
}
$javacExe = Join-Path $Jdk "bin\javac.exe"
$jarExe = Join-Path $Jdk "bin\jar.exe"
if (-not (Test-Path $javacExe)) { throw "No JDK found (javac); pass -Jdk <folder>" }

$forgeJar = Get-ChildItem (Join-Path $GamePath "TCGForge\forge") -Filter "forge-gui-desktop-*-jar-with-dependencies.jar" | Select-Object -First 1
if (-not $forgeJar) { throw "Forge not installed in $GamePath\TCGForge (TCG Studio > Setup > Install MTG mode)" }

if (Test-Path $tmp) { Remove-Item -Recurse -Force $tmp }
New-Item -ItemType Directory -Force $tmp | Out-Null
$files = Get-ChildItem $src -Recurse -Filter *.java | ForEach-Object { $_.FullName }
Write-Host "Compiling $($files.Count) files against $($forgeJar.Name) ..."
& $javacExe --release 17 -encoding UTF-8 -cp $forgeJar.FullName -d $tmp @files
if ($LASTEXITCODE -ne 0) { throw "javac failed" }
New-Item -ItemType Directory -Force (Split-Path $out) | Out-Null
& $jarExe --create --file $out -C $tmp .
if ($LASTEXITCODE -ne 0) { throw "jar failed" }
Remove-Item -Recurse -Force $tmp
Write-Host "Built $out"
