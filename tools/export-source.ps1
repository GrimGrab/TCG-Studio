# Exports a clean snapshot of this repository into the public GitHub repo (tools\release.json) and commits it.
# The public repo never gets this repo's history: every export is a plain snapshot commit by the public identity.
#   .\tools\export-source.ps1               # export + commit locally in -PublicDir, no push (review first)
#   .\tools\export-source.ps1 -Push         # ... and push (publish.ps1 does this right before each release)
#   .\tools\export-source.ps1 -Fresh -Push  # start the public history over with a single commit (force-push)
# What goes out: every tracked file (working-tree content, so it matches what package.ps1 just built) except public\,
# plus public\* at the root and the docs listed in $docs. A leak scan aborts before committing if any file name or
# content matches a pattern: the Windows user name, this repo's git author email, the workspace folder name, user
# profile paths, and every line of <workspace>\leak-patterns.txt (a local file, never exported).
param(
    [switch]$Push,
    [switch]$Fresh,
    [string]$PublicDir = ""
)
$ErrorActionPreference = "Continue"
$root = Split-Path -Parent $PSScriptRoot
$workspace = Split-Path -Parent $root
if (-not $PublicDir) { $PublicDir = Join-Path $workspace "TCG-Studio-public" }
$repo = (Get-Content (Join-Path $PSScriptRoot "release.json") -Raw | ConvertFrom-Json).repo
$version = (Get-Content (Join-Path $root "VERSION") -Raw).Trim()
$authorName = "GrimGrab"
$authorEmail = "grimgrab31@gmail.com"
$docs = @("set-format.md", "mtg-forge.md", "runtime-facts.md")   # from <workspace>\docs, referenced by code comments
$utf8 = New-Object System.Text.UTF8Encoding($false)

function Git-Public { git -C $PublicDir @args; if ($LASTEXITCODE -ne 0) { throw "git $($args -join ' ') failed" } }

# Leak patterns.
$patternFile = Join-Path $workspace "leak-patterns.txt"
if (-not (Test-Path $patternFile)) { throw "Missing $patternFile (one leak pattern per line) - refusing to export without it." }
$patterns = @(Get-Content $patternFile | ForEach-Object { $_.Trim() } | Where-Object { $_ -and -not $_.StartsWith("#") })
if ($env:USERNAME.Length -ge 3) { $patterns += $env:USERNAME }
$privateEmail = (git -C $root config user.email)
if ($privateEmail -and $privateEmail -ne $authorEmail) { $patterns += $privateEmail; $patterns += $privateEmail.Split("@")[0] }
$patterns += (Split-Path -Leaf $workspace)
$profiles = Split-Path -Parent $env:USERPROFILE   # the folder holding user profiles, in Windows, forward-slash and Git Bash spelling
$patterns += @("$profiles\", ($profiles.Replace("\", "/") + "/"), ("/" + $profiles.Substring(0, 1).ToLower() + $profiles.Substring(2).Replace("\", "/") + "/"))
$patterns = $patterns | Where-Object { $_ } | Sort-Object -Unique

# Public working copy.
if ($Fresh) {
    New-Item -ItemType Directory -Force $PublicDir | Out-Null
    if (Test-Path (Join-Path $PublicDir ".git")) { Remove-Item -Recurse -Force (Join-Path $PublicDir ".git") }
    Git-Public init -q -b main
    Git-Public remote add origin "https://github.com/$repo.git"
} elseif (-not (Test-Path (Join-Path $PublicDir ".git"))) {
    git clone -q "https://github.com/$repo.git" $PublicDir
    if ($LASTEXITCODE -ne 0) { throw "git clone failed" }
} else {
    Git-Public fetch -q origin
    Git-Public reset -q --hard origin/main
}
Get-ChildItem $PublicDir -Force | Where-Object { $_.Name -ne ".git" } | Remove-Item -Recurse -Force

# Copy the snapshot.
$files = git -C $root -c core.quotepath=off ls-files
if ($LASTEXITCODE -ne 0) { throw "git ls-files failed" }
$n = 0
foreach ($rel in $files) {
    if ($rel -like "public/*") { continue }
    $src = Join-Path $root $rel
    if (-not (Test-Path -LiteralPath $src -PathType Leaf)) { continue }   # deleted in the working tree
    $dst = Join-Path $PublicDir $rel
    New-Item -ItemType Directory -Force (Split-Path -Parent $dst) | Out-Null
    Copy-Item -LiteralPath $src $dst -Force
    $n++
}
Get-ChildItem (Join-Path $root "public") -Force | Copy-Item -Destination $PublicDir -Recurse -Force
New-Item -ItemType Directory -Force (Join-Path $PublicDir "docs") | Out-Null
foreach ($d in $docs) {
    $text = [IO.File]::ReadAllText((Join-Path $workspace "docs\$d"), $utf8)
    $text = [regex]::Replace($text, '\s*Plan/phases:\s*`[^`]*`\.?', "")   # private planning notes
    [IO.File]::WriteAllText((Join-Path $PublicDir "docs\$d"), $text, $utf8)
}
Write-Host "Exported $n tracked files + public\ + $($docs.Count) docs to $PublicDir"

# Leak scan (names and contents, case-insensitive; binary files are read as Latin-1 so byte patterns still match).
$latin1 = [Text.Encoding]::GetEncoding(28591)
$hits = @()
foreach ($f in Get-ChildItem $PublicDir -Recurse -File -Force | Where-Object { $_.FullName -notlike "*\.git\*" }) {
    $rel = $f.FullName.Substring($PublicDir.Length + 1)
    $content = [IO.File]::ReadAllText($f.FullName, $latin1)
    foreach ($p in $patterns) {
        if ($rel.IndexOf($p, [StringComparison]::OrdinalIgnoreCase) -ge 0 -or $content.IndexOf($p, [StringComparison]::OrdinalIgnoreCase) -ge 0) {
            $hits += "$rel  (pattern #$([array]::IndexOf($patterns, $p)))"
        }
    }
}
if ($hits) {
    $hits | ForEach-Object { Write-Host "  LEAK: $_" }
    throw "Leak scan failed ($($hits.Count) hits) - nothing committed. Fix the files above and run again."
}
Write-Host "Leak scan clean ($($patterns.Count) patterns)."

# Commit as the public identity (no attribution trailers), then push.
Git-Public add -A
git -C $PublicDir diff --cached --quiet
if ($LASTEXITCODE -eq 0) {
    Write-Host "No source changes since the last export."
} else {
    Git-Public -c user.name=$authorName -c user.email=$authorEmail commit -q -m "Source for v$version"
    Write-Host "Committed: $(git -C $PublicDir log -1 --format='%h %an <%ae> %s')"
}
if ($Push) {
    # Authenticate with the GitHub CLI's login (gh auth login) rather than whatever credential helper git has.
    $env:Path += ";C:\Program Files\GitHub CLI"
    $gh = (Get-Command gh -ErrorAction SilentlyContinue).Source
    $auth = @()
    if ($gh) { $auth = @("-c", "credential.helper=", "-c", "credential.helper=!'$($gh.Replace('\', '/'))' auth git-credential") }
    if ($Fresh) { Git-Public @auth push -q --force -u origin main } else { Git-Public @auth push -q origin main }
    Write-Host "Pushed to https://github.com/$repo"
}
