# Generates placeholder PNGs for the example sets (Windows PowerShell 5.1 / System.Drawing).
#   powershell -ExecutionPolicy Bypass -File tools\make-test-art.ps1
Add-Type -AssemblyName System.Drawing
$root = Split-Path -Parent $PSScriptRoot

function New-RoundedPath([float]$x, [float]$y, [float]$w, [float]$h, [float]$r) {
    $p = New-Object System.Drawing.Drawing2D.GraphicsPath
    $p.AddArc($x, $y, 2*$r, 2*$r, 180, 90)
    $p.AddArc($x + $w - 2*$r, $y, 2*$r, 2*$r, 270, 90)
    $p.AddArc($x + $w - 2*$r, $y + $h - 2*$r, 2*$r, 2*$r, 0, 90)
    $p.AddArc($x, $y + $h - 2*$r, 2*$r, 2*$r, 90, 90)
    $p.CloseFigure()
    return $p
}

function Save-Card([string]$path, [string]$title, [string]$sub, [System.Drawing.Color]$color, [int]$w, [int]$h, [bool]$fullCard) {
    $bmp = New-Object System.Drawing.Bitmap($w, $h, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode = 'AntiAlias'; $g.TextRenderingHint = 'AntiAlias'
    $g.Clear([System.Drawing.Color]::Transparent)
    $fg = [System.Drawing.Brushes]::White
    $fmt = [System.Drawing.StringFormat]::new(); $fmt.Alignment = 'Center'; $fmt.LineAlignment = 'Center'
    $bold = [System.Drawing.FontStyle]::Bold
    [float]$fw = $w; [float]$fh = $h
    if ($fullCard) {
        $outer = New-RoundedPath 0 0 ($fw - 1) ($fh - 1) 36
        $g.FillPath([System.Drawing.Brushes]::Black, $outer)
        $inner = New-RoundedPath 24 24 ($fw - 49) ($fh - 49) 18
        $g.FillPath([System.Drawing.SolidBrush]::new($color), $inner)
        $g.FillRectangle([System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(90, 0, 0, 0)), [float]48, [float]90, [float]($fw - 96), [float]($fh * 0.45))
        $g.DrawString($title, [System.Drawing.Font]::new('Segoe UI', [float]40, $bold), $fg, [System.Drawing.RectangleF]::new(40, 20, $fw - 80, 70), $fmt)
        $g.DrawString('FULL IMAGE', [System.Drawing.Font]::new('Segoe UI', [float]56, $bold), $fg, [System.Drawing.RectangleF]::new(48, 90, $fw - 96, $fh * 0.45), $fmt)
        $g.DrawString($sub, [System.Drawing.Font]::new('Segoe UI', [float]30), $fg, [System.Drawing.RectangleF]::new(48, $fh * 0.62, $fw - 96, $fh * 0.3), $fmt)
    } else {
        $g.Clear($color)
        $g.FillEllipse([System.Drawing.SolidBrush]::new([System.Drawing.Color]::FromArgb(80, 255, 255, 255)), [float]($fw * 0.15), [float]($fh * 0.15), [float]($fw * 0.7), [float]($fh * 0.7))
        $g.DrawString($title, [System.Drawing.Font]::new('Segoe UI', [float]44, $bold), $fg, [System.Drawing.RectangleF]::new(0, 0, $fw, $fh), $fmt)
    }
    $bmp.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
    $g.Dispose(); $bmp.Dispose()
}

$palette = @('#C0392B','#27AE60','#2980B9','#8E44AD','#D35400','#16A085')
$names = @('Ember Drake','Moss Golem','Tide Sprite','Void Wisp','Sun Lion','Jade Serpent')

$full = Join-Path $root 'examples\test-fullimage\images'
$framed = Join-Path $root 'examples\test-framed\images'
New-Item -ItemType Directory -Force $full, $framed | Out-Null
for ($i = 0; $i -lt 6; $i++) {
    $c = [System.Drawing.ColorTranslator]::FromHtml($palette[$i])
    Save-Card (Join-Path $full "card$($i+1).png") $names[$i] "Test card #$($i+1)" $c 745 1040 $true
    Save-Card (Join-Path $framed "art$($i+1).png") $names[$i] '' $c 512 512 $false
}
Write-Output "Wrote placeholder art to $full and $framed"
