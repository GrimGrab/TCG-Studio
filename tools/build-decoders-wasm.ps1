# Builds studio\internal\unityfs\decoders.wasm (committed) from studio\internal\unityfs\decoders\: Unity's crunch
# transcoder (crn_decomp.h, zlib) and bcdec's BC7 decoder (bcdec.h, Unlicense), both unchanged, plus decoders.cpp.
# Studio runs the .wasm with wazero to read crunched (DXT1/DXT5 Crunched) and BC7 textures from mod asset bundles.
# Only needed when those sources change. Uses Zig (MIT, build-time only) as the C++ -> WebAssembly compiler; it is
# downloaded once to %LOCALAPPDATA%\TCG Studio\buildtools (checksum verified). No C library is linked into the .wasm.
$ErrorActionPreference = "Stop"
$ZigVersion = "0.17.0"
$ZigSha256 = "b5663f69581dcf391293fbf16c06cb80d81d806545ce618b4d0bab7f0eb8c428"

$root = Split-Path -Parent $PSScriptRoot
$src = Join-Path $root "studio\internal\unityfs\decoders"
$out = Join-Path $root "studio\internal\unityfs\decoders.wasm"
$tools = Join-Path $env:LOCALAPPDATA "TCG Studio\buildtools"
$zigDir = Join-Path $tools "zig-x86_64-windows-$ZigVersion"
$zig = Join-Path $zigDir "zig.exe"

if (-not (Test-Path $zig)) {
    New-Item -ItemType Directory -Force $tools | Out-Null
    $zip = Join-Path $tools "zig-$ZigVersion.zip"
    Write-Host "Downloading Zig $ZigVersion (~95 MB) ..."
    Invoke-WebRequest "https://ziglang.org/download/$ZigVersion/zig-x86_64-windows-$ZigVersion.zip" -OutFile $zip
    $hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
    if ($hash -ne $ZigSha256) { Remove-Item $zip; throw "Zig download checksum mismatch ($hash)" }
    Expand-Archive $zip -DestinationPath $tools -Force
    Remove-Item $zip
}

$exports = "dec_reset", "dec_alloc", "crunch_unpack", "bc7_decode"
$args = @("c++", "-target", "wasm32-freestanding", "-O2", "-nostdlib", "-nostdinc++", "-I", (Join-Path $src "include"),
    "-fno-exceptions", "-fno-rtti", "-fno-strict-aliasing",
    "-Wno-everything", "-Wl,--no-entry",
    # Shipped binary: no debug info or local build paths (the export's leak scan rejects them).
    "-g0", "-Wl,--strip-all", "-ffile-prefix-map=$src=decoders", "-ffile-prefix-map=$zigDir=zig",
    "-o", $out, (Join-Path $src "decoders.cpp"))
foreach ($e in $exports) { $args += "-Wl,--export=$e" }
Write-Host "Compiling decoders.wasm ..."
& $zig @args
if ($LASTEXITCODE -ne 0) { throw "zig exited with code $LASTEXITCODE" }
Write-Host ("Built {0} ({1:N0} bytes)" -f $out, (Get-Item $out).Length)
