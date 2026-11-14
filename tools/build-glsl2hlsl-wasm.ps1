# Builds studio\internal\cardshader\glsl2hlsl.wasm (committed) from studio\internal\cardshader\glsl2hlsl\glsl2hlsl.cpp
# plus glslang (GLSL -> SPIR-V) and SPIRV-Cross (SPIR-V -> HLSL), both unchanged release sources downloaded once to
# %LOCALAPPDATA%\TCG Studio\buildtools (checksums verified). Studio runs the .wasm with wazero to turn a player's card
# shader (GLSL) into HLSL for Direct3D's compiler. Only needed when glsl2hlsl.cpp or the pinned versions change.
# Uses Zig (MIT, build-time only) as the C++ -> WebAssembly compiler, like build-decoders-wasm.ps1.
$ErrorActionPreference = "Stop"
$ZigVersion = "0.17.0"
$ZigSha256 = "b5663f69581dcf391293fbf16c06cb80d81d806545ce618b4d0bab7f0eb8c428"
$Glslang = "16.6.0"
$GlslangSha256 = "a405dc18a69941044cf0cae820c0c2bd960f92dcb3810d0bc249447cc46f04e9"
$SpirvCross = "vulkan-sdk-1.4.363.0"
$SpirvCrossSha256 = "637ee99192da3cea88bb81c97bcb7177d7b0010f75d2afd6c97f8e7b7d2a0c90"

$root = Split-Path -Parent $PSScriptRoot
$src = Join-Path $root "studio\internal\cardshader\glsl2hlsl"
$out = Join-Path $root "studio\internal\cardshader\glsl2hlsl.wasm"
$tools = Join-Path $env:LOCALAPPDATA "TCG Studio\buildtools"
$zigDir = Join-Path $tools "zig-x86_64-windows-$ZigVersion"
$zig = Join-Path $zigDir "zig.exe"
$glslangDir = Join-Path $tools "glslang-$Glslang"
$crossDir = Join-Path $tools "SPIRV-Cross-$SpirvCross"
New-Item -ItemType Directory -Force $tools | Out-Null

function Get-Source($name, $url, $sha, $dir) {
    if (Test-Path $dir) { return }
    $zip = Join-Path $tools "$name.zip"
    Write-Host "Downloading $name ..."
    Invoke-WebRequest $url -OutFile $zip
    $hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
    if ($hash -ne $sha) { Remove-Item $zip; throw "$name download checksum mismatch ($hash)" }
    Expand-Archive $zip -DestinationPath $tools -Force
    Remove-Item $zip
}
Get-Source "zig-$ZigVersion" "https://ziglang.org/download/$ZigVersion/zig-x86_64-windows-$ZigVersion.zip" $ZigSha256 $zigDir
Get-Source "glslang-$Glslang" "https://github.com/KhronosGroup/glslang/archive/refs/tags/$Glslang.zip" $GlslangSha256 $glslangDir
Get-Source "spirv-cross-$SpirvCross" "https://github.com/KhronosGroup/SPIRV-Cross/archive/refs/tags/$SpirvCross.zip" $SpirvCrossSha256 $crossDir

# glslang's one generated header (its CMake build makes it from the template): only the version numbers.
$gen = Join-Path $tools "glslang-$Glslang-generated"
New-Item -ItemType Directory -Force (Join-Path $gen "glslang") | Out-Null
$v = $Glslang.Split(".")
$info = (Get-Content (Join-Path $glslangDir "build_info.h.tmpl") -Raw).Replace("@major@", $v[0]).Replace("@minor@", $v[1]).Replace("@patch@", $v[2]).Replace("@flavor@", "")
[IO.File]::WriteAllText((Join-Path $gen "glslang\build_info.h"), $info)

# WebAssembly's C++ library has no file streams, and one glslang file has two helpers that write SPIR-V to a file
# (never called here). That file is compiled from a copy in which their stream is one that cannot be opened.
$spv = Get-Content (Join-Path $glslangDir "SPIRV\GlslangToSpv.cpp") -Raw
$uses = ([regex]::Matches($spv, "std::ofstream out;")).Count
if ($uses -ne 2) { throw "GlslangToSpv.cpp: expected 2 file streams, found $uses (glslang changed; check this step)" }
$stub = "#include <sstream>`nstruct NoFile : std::ostringstream { void open(const char*, std::ios::openmode) { setstate(std::ios::failbit); } void close() {} };`n"
$spvCopy = Join-Path $gen "GlslangToSpv.cpp"
[IO.File]::WriteAllText($spvCopy, $stub + $spv.Replace("#include <fstream>", "").Replace("std::ofstream out;", "NoFile out;"))

$sources = @((Join-Path $src "glsl2hlsl.cpp"), (Join-Path $src "os.cpp"), $spvCopy)
$sources += Get-ChildItem (Join-Path $glslangDir "glslang\MachineIndependent") -Filter *.cpp | ForEach-Object FullName
$sources += Get-ChildItem (Join-Path $glslangDir "glslang\MachineIndependent\preprocessor") -Filter *.cpp | ForEach-Object FullName
$sources += Get-ChildItem (Join-Path $glslangDir "glslang\GenericCodeGen") -Filter *.cpp | ForEach-Object FullName
$sources += Join-Path $glslangDir "glslang\ResourceLimits\ResourceLimits.cpp"
$sources += "InReadableOrder.cpp", "Logger.cpp", "SpvBuilder.cpp", "SpvPostProcess.cpp", "SpvTools.cpp", "doc.cpp" |
    ForEach-Object { Join-Path $glslangDir "SPIRV\$_" }
$sources += "spirv_cross.cpp", "spirv_parser.cpp", "spirv_cross_parsed_ir.cpp", "spirv_cfg.cpp", "spirv_glsl.cpp", "spirv_hlsl.cpp" |
    ForEach-Object { Join-Path $crossDir $_ }

$exports = "cs_alloc", "cs_convert", "cs_out", "cs_out_len"
$args = @("c++", "-target", "wasm32-wasi", "-mexec-model=reactor", "-Oz", "-std=c++17",
    "-I", $glslangDir, "-I", $gen, "-I", $crossDir, "-I", (Join-Path $glslangDir "SPIRV"),
    "-DSPIRV_CROSS_EXCEPTIONS_TO_ASSERTIONS", "-DNDEBUG",
    "-fno-exceptions", "-fno-rtti", "-Wno-everything",
    # Shipped binary: no debug info or local build paths (the export's leak scan rejects them).
    "-g0", "-Wl,--strip-all", "-ffile-prefix-map=$src=glsl2hlsl", "-ffile-prefix-map=$tools=buildtools",
    "-o", $out)
$args += $sources
Write-Host "Compiling glsl2hlsl.wasm ($($sources.Count) files, a few minutes the first time) ..."
# Zig reports on stderr; PowerShell would stop at its first line.
$ErrorActionPreference = "Continue"
& $zig @args 2>&1 | ForEach-Object { "$_" }
$ErrorActionPreference = "Stop"
if ($LASTEXITCODE -ne 0) { throw "zig exited with code $LASTEXITCODE" }
Write-Host ("Built {0} ({1:N0} bytes)" -f $out, (Get-Item $out).Length)
