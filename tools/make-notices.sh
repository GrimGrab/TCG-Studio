#!/usr/bin/env bash
# Builds THIRD_PARTY_NOTICES.txt (disclaimers + every third-party component we ship + full license texts) and
# dist/tcgcc-forge-bridge-source.zip (the GPL-3.0 bridge source that has to go with every release that ships the jar).
# Run from Git Bash after changing Go modules, the BepInEx bundle or the bridge:  bash "TCG Custom Cards/tools/make-notices.sh"
# Inputs: licenses/NOTICES_HEADER.txt (edit the text there), licenses/*.txt (upstream license texts, kept in the repo so this
# works offline) and the Go module cache (license of each module linked into TCG Studio.exe).
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
lic="$root/licenses"
out="$root/THIRD_PARTY_NOTICES.txt"
export PATH="$PATH:/c/Program Files/Go/bin"

# Go modules actually linked into the Windows build (not test-only or other-OS dependencies).
mods=$(cd "$root/studio" && GOOS=windows GOFLAGS=-tags=desktop,production \
  go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}|{{.Version}}|{{.Dir}}{{end}}{{end}}' . | sort -u)

kind() { # license family of a license file
  if grep -q "Apache License" "$1"; then echo "Apache-2.0"
  elif grep -qE "Permission is hereby granted" "$1"; then echo "MIT"
  elif grep -q "Redistribution and use in source and binary" "$1"; then echo "BSD"
  else echo "see text"; fi
}

list=""
texts=""
while IFS='|' read -r path ver dir; do
  [ -z "$path" ] && continue
  f=$(ls "$dir" | grep -iE '^(licen[cs]e|copying)' | head -1 || true)
  [ -z "$f" ] && { echo "no license file in $path" >&2; exit 1; }
  list+="     - $path $ver [$(kind "$dir/$f")]"$'\n'
  texts+=$'\n'"----- $path $ver ($f) -----"$'\n\n'"$(tr -d '\r' < "$dir/$f")"$'\n'
  [ -f "$dir/NOTICE" ] && texts+=$'\n'"----- $path NOTICE -----"$'\n\n'"$(tr -d '\r' < "$dir/NOTICE")"$'\n'
done <<< "$mods"

section() { printf '\n----- %s -----\n\n' "$1"; tr -d '\r' < "$lic/$2"; printf '\n'; }
{
  while IFS= read -r line; do
    if [ "$line" = "{{GO_MODULES}}" ]; then printf '%s' "$list"; else printf '%s\n' "$line"; fi
  done < <(tr -d '\r' < "$lic/NOTICES_HEADER.txt")
  section "GNU General Public License v3.0 (Forge bridge; also the base of the LGPL v3.0 below)" GPL-3.0.txt
  section "GNU Lesser General Public License v3.0 (BepInEx.ConfigurationManager)" ConfigurationManager-LGPL-3.0.txt
  section "GNU Lesser General Public License v2.1 (UnityDoorstop)" UnityDoorstop-LGPL-2.1.txt
  section "BepInEx (MIT)" BepInEx-MIT.txt
  section "HarmonyX (MIT)" HarmonyX-MIT.txt
  section "MonoMod (MIT)" MonoMod-MIT.txt
  section "Mono.Cecil (MIT)" Mono.Cecil-MIT.txt
  section "Go standard library (BSD-3-Clause)" Go-BSD-3-Clause.txt
  section "Svelte (MIT)" Svelte-MIT.txt
  section "libavif (BSD-2-Clause; inside github.com/gen2brain/avif)" libavif-BSD-2-Clause.txt
  section "dav1d (BSD-2-Clause; inside github.com/gen2brain/avif)" dav1d-BSD-2-Clause.txt
  section "libaom (BSD-2-Clause; inside github.com/gen2brain/avif)" aom-BSD-2-Clause.txt
  section "libyuv (BSD-3-Clause; inside github.com/gen2brain/avif)" libyuv-BSD-3-Clause.txt
  section "crunch (zlib; inside decoders.wasm)" crunch-Zlib.txt
  printf '%s' "$texts"
} > "$out"
echo "Wrote $out ($(wc -l < "$out") lines)"

# GPL source for the bridge jar: the bridge folder + its build script.
mkdir -p "$root/dist"
zip="$root/dist/tcgcc-forge-bridge-source.zip"
rm -f "$zip"
stage=$(mktemp -d)
mkdir -p "$stage/forge-bridge/tools"
cp -r "$root/forge-bridge/src" "$root/forge-bridge/README.md" "$root/forge-bridge/LICENSE" "$stage/forge-bridge/"
cp "$root/tools/build-bridge.ps1" "$stage/forge-bridge/tools/"
# Windows' bsdtar writes a standard zip (PowerShell 5.1's Compress-Archive stores backslash paths).
(cd "$stage" && /c/Windows/System32/tar.exe -a -c -f "$(cygpath -w "$zip")" forge-bridge)
rm -rf "$stage"
echo "Wrote $zip"
