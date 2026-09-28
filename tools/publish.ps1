# Publishes a new TCG Studio release that installed clients pick up automatically ("Update now" banner / Check for updates).
#   .\tools\publish.ps1 -Notes "Fixed the binder set list"            # 0.3.0 -> 0.3.1
#   .\tools\publish.ps1 -Bump minor -Notes "Custom holo foil"          # 0.3.1 -> 0.4.0
#   .\tools\publish.ps1 -Same -Notes "First release"                   # publish the current VERSION as-is
#   -FreshSource                                                     # restart the public source history (one commit)
# Steps: bump VERSION -> build (tools\package.ps1) -> notices -> push a cleaned source snapshot (tools\export-source.ps1)
# -> GitHub release v<VERSION> on that commit in tools\release.json's repo with "TCG Studio.exe" + a "sha256: ..." line
# the client verifies -> commit VERSION + tag locally.
# Needs the GitHub CLI once: winget install GitHub.cli ; gh auth login
param(
    [Parameter(Mandatory = $true)][string]$Notes,
    [ValidateSet("patch", "minor", "major")][string]$Bump = "patch",
    [switch]$Same,
    [switch]$FreshSource
)
$ErrorActionPreference = "Continue"
$root = Split-Path -Parent $PSScriptRoot
$env:Path += ";C:\Program Files\GitHub CLI"
$repo = (Get-Content (Join-Path $PSScriptRoot "release.json") -Raw | ConvertFrom-Json).repo

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) { throw "GitHub CLI not found - run: winget install GitHub.cli ; then: gh auth login" }
gh auth status 2>$null | Out-Null
if ($LASTEXITCODE -ne 0) { throw "Not logged in to GitHub - run: gh auth login" }

# Next version.
$versionFile = Join-Path $root "VERSION"
$old = (Get-Content $versionFile -Raw).Trim()
$p = $old.Split(".") | ForEach-Object { [int]$_ }
$new = $old
if (-not $Same) {
    switch ($Bump) {
        "major" { $new = "$($p[0] + 1).0.0" }
        "minor" { $new = "$($p[0]).$($p[1] + 1).0" }
        default { $new = "$($p[0]).$($p[1]).$($p[2] + 1)" }
    }
}
$tag = "v$new"
gh release view $tag --repo $repo 2>$null | Out-Null
if ($LASTEXITCODE -eq 0) { throw "Release $tag already exists in $repo - bump the version." }
Write-Host "Publishing $tag to $repo (was $old)"
[IO.File]::WriteAllText($versionFile, "$new`n")

try {
    & (Join-Path $PSScriptRoot "package.ps1")
    if (-not $?) { throw "build failed" }
} catch {
    [IO.File]::WriteAllText($versionFile, "$old`n")
    throw "Build failed, VERSION restored to $old. $($_.Exception.Message)"
}

$exe = Join-Path $root "dist\TCG Studio.exe"
$hash = (Get-FileHash $exe -Algorithm SHA256).Hash.ToLower()

# Licenses: every release carries THIRD_PARTY_NOTICES.txt and the GPL-3.0 source of the Forge bridge jar it ships.
$bash = @("C:\Program Files\Git\bin\bash.exe", (Get-Command bash -ErrorAction SilentlyContinue).Source) | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
if (-not $bash) { [IO.File]::WriteAllText($versionFile, "$old`n"); throw "Git Bash not found (needed for tools\make-notices.sh). VERSION restored to $old." }
& $bash (Join-Path $PSScriptRoot "make-notices.sh")
if ($LASTEXITCODE -ne 0) { [IO.File]::WriteAllText($versionFile, "$old`n"); throw "make-notices.sh failed, VERSION restored to $old." }
$notices = Join-Path $root "THIRD_PARTY_NOTICES.txt"
$bridgeSrc = Join-Path $root "dist\tcgcc-forge-bridge-source.zip"
$licenseNote = "Licenses & source: THIRD_PARTY_NOTICES.txt (attached) lists the third-party software included and its licenses. " +
    "The Forge bridge (tcgcc-forge-bridge.jar) is GPL-3.0; its source is tcgcc-forge-bridge-source.zip (attached). " +
    "Unofficial fan project, not affiliated with or endorsed by OPNeon Games, Wizards of the Coast, Scryfall or the Forge project. " +
    "Magic: The Gathering, its card names, text and images are property of Wizards of the Coast LLC; no Magic card data or images are included.`n`n" +
    "Source code & build guide: https://github.com/$repo (BUILDING.md)."

# Public source: a cleaned, leak-scanned snapshot of what was just built, pushed so the release tag points at it.
try {
    & (Join-Path $PSScriptRoot "export-source.ps1") -Push -Fresh:$FreshSource
} catch {
    [IO.File]::WriteAllText($versionFile, "$old`n")
    throw "Source export failed, VERSION restored to $old. $($_.Exception.Message)"
}

$notesFile = Join-Path $env:TEMP "tcgstudio-release-notes.md"
[IO.File]::WriteAllText($notesFile, "$Notes`n`n$licenseNote`n`nsha256: $hash`n")
gh release create $tag "$exe#TCG Studio.exe" $bridgeSrc $notices --repo $repo --target main --title "TCG Studio $tag" --notes-file $notesFile --latest
if ($LASTEXITCODE -ne 0) { throw "gh release create failed" }

# Keep local history in step (only VERSION is committed here).
git -C $root add VERSION 2>$null
git -C $root commit -m "Release $tag" -- VERSION 2>$null | Out-Null
git -C $root tag $tag 2>$null
Write-Host "Published $tag - https://github.com/$repo/releases/tag/$tag"
