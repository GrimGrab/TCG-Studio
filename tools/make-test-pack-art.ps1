# Builds placeholder pack/box art for the example sets from the vanilla templates the mod exports.
#   powershell -ExecutionPolicy Bypass -File tools\make-test-pack-art.ps1 [-Templates <dir>]
# Template layout notes (1024² textures, pixel coords from top-left):
#   pack: back panel x195-580, front panel x583-990, y108-722; crimps above/below.
#   box atlas (shared by Basic/Rare/Epic/Legendary boxes): Basic box uses the left strip (0-178, 0-305),
#   top banner (185-525, 10-145) and front panel (185-560, 150-388).
param([string]$Templates = 'D:\SteamLibrary\steamapps\common\TCG Card Shop Simulator\BepInEx\plugins\TCGCustomCards\templates')
Add-Type -AssemblyName System.Drawing
$root = Split-Path -Parent $PSScriptRoot

function New-HueAttributes([double]$deg) {
    $a = $deg * [Math]::PI / 180; $c = [Math]::Cos($a); $s = [Math]::Sin($a)
    $lr = 0.213; $lg = 0.715; $lb = 0.072
    # CSS hueRotate matrix: out_row = f(in); GDI+ wants the transpose.
    $css = @(
        @(($lr + $c*(1-$lr) - $s*$lr), ($lg - $c*$lg - $s*$lg), ($lb - $c*$lb + $s*(1-$lb))),
        @(($lr - $c*$lr + $s*0.143), ($lg + $c*(1-$lg) + $s*0.140), ($lb - $c*$lb - $s*0.283)),
        @(($lr - $c*$lr - $s*(1-$lr)), ($lg - $c*$lg + $s*$lg), ($lb + $c*(1-$lb) + $s*$lb))
    )
    $m = New-Object System.Drawing.Imaging.ColorMatrix
    for ($i = 0; $i -lt 3; $i++) { for ($j = 0; $j -lt 3; $j++) { $m.Item($i, $j) = [float]$css[$j][$i] } }
    $m.Item(3, 3) = 1; $m.Item(4, 4) = 1
    $ia = New-Object System.Drawing.Imaging.ImageAttributes
    $ia.SetColorMatrix($m)
    return $ia
}

function Draw-Label($g, [string]$text, [float]$x, [float]$y, [float]$w, [float]$h, [float]$size) {
    $g.FillRectangle([System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(200, 20, 20, 30)), $x, $y, $w, $h)
    $fmt = [System.Drawing.StringFormat]::new(); $fmt.Alignment = 'Center'; $fmt.LineAlignment = 'Center'
    $font = [System.Drawing.Font]::new('Segoe UI', $size, [System.Drawing.FontStyle]::Bold)
    $g.DrawString($text, $font, [System.Drawing.Brushes]::White, [System.Drawing.RectangleF]::new($x, $y, $w, $h), $fmt)
}

function Make-Image([string]$src, [string]$dst, [double]$hue, [scriptblock]$labels) {
    $img = [System.Drawing.Image]::FromFile($src)
    $bmp = New-Object System.Drawing.Bitmap($img.Width, $img.Height, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode = 'AntiAlias'; $g.TextRenderingHint = 'AntiAlias'
    $rect = [System.Drawing.Rectangle]::new(0, 0, $img.Width, $img.Height)
    $g.DrawImage($img, $rect, 0, 0, $img.Width, $img.Height, [System.Drawing.GraphicsUnit]::Pixel, (New-HueAttributes $hue))
    & $labels $g
    New-Item -ItemType Directory -Force (Split-Path $dst) | Out-Null
    $bmp.Save($dst, [System.Drawing.Imaging.ImageFormat]::Png)
    $g.Dispose(); $bmp.Dispose(); $img.Dispose()
}

$sets = @(
    @{ folder = 'test-fullimage'; label = 'FULL IMAGE'; hue = 120 },
    @{ folder = 'test-framed';    label = 'FRAMED';     hue = 270 }
)
foreach ($s in $sets) {
    $out = Join-Path $root "examples\$($s.folder)\images"
    $l = $s.label
    Make-Image "$Templates\BasicCardPack_texture.png" "$out\pack_texture.png" $s.hue { param($g)
        Draw-Label $g $l 600 150 380 110 44; Draw-Label $g 'TEST SET' 600 610 380 70 30; Draw-Label $g $l 215 380 345 90 36 }
    # Icon template is a 1024² canvas with the pack drawn at x261-782, y10-1011.
    Make-Image "$Templates\BasicCardPack_icon.png" "$out\pack_icon.png" $s.hue { param($g)
        Draw-Label $g $l 290 120 460 120 48 }
    Make-Image "$Templates\BasicCardBox_texture.png" "$out\box_texture.png" $s.hue { param($g)
        Draw-Label $g $l 385 60 140 60 18; Draw-Label $g "$l BOX" 200 170 220 70 26; Draw-Label $g $l 5 120 168 60 18 }
    Make-Image "$Templates\BasicCardBox_icon.png" "$out\box_icon.png" $s.hue { param($g)
        $w = $g.VisibleClipBounds.Width; Draw-Label $g $l ($w * 0.1) 20 ($w * 0.8) 90 36 }
}
Write-Output 'Wrote pack/box art for example sets'
