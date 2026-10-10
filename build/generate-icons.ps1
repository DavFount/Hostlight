# Regenerate Windows assets from the geometry in frontend/src/assets/hostlight.svg.
# No third-party dependencies; run with PowerShell on Windows.
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$projectRoot = Split-Path -Parent $PSScriptRoot
$svgPath = Join-Path $projectRoot 'frontend/src/assets/hostlight.svg'
[xml]$svg = Get-Content -LiteralPath $svgPath -Raw

function New-IconBitmap([int]$size) {
    $bitmap = [System.Drawing.Bitmap]::new($size, $size)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $graphics.ScaleTransform($size / 256.0, $size / 256.0)
    foreach ($shape in $svg.DocumentElement.ChildNodes) {
        if ($shape.LocalName -notin @('rect', 'circle')) { continue }
        $color = [System.Drawing.ColorTranslator]::FromHtml($shape.GetAttribute('fill'))
        $opacity = $shape.GetAttribute('fill-opacity')
        if ($opacity) { $color = [System.Drawing.Color]::FromArgb([int](255 * [double]::Parse($opacity, [cultureinfo]::InvariantCulture)), $color) }
        $brush = [System.Drawing.SolidBrush]::new($color)
        try {
            if ($shape.LocalName -eq 'circle') {
                $radius = [single]$shape.GetAttribute('r')
                $graphics.FillEllipse($brush, [single]$shape.GetAttribute('cx')-$radius, [single]$shape.GetAttribute('cy')-$radius, 2*$radius, 2*$radius)
            } else {
                $x = [single]$shape.GetAttribute('x'); $y = [single]$shape.GetAttribute('y')
                $width = [single]$shape.GetAttribute('width'); $height = [single]$shape.GetAttribute('height')
                $diameter = 2 * [single]$shape.GetAttribute('rx')
                $path = [System.Drawing.Drawing2D.GraphicsPath]::new()
                try {
                    $path.AddArc($x, $y, $diameter, $diameter, 180, 90)
                    $path.AddArc($x+$width-$diameter, $y, $diameter, $diameter, 270, 90)
                    $path.AddArc($x+$width-$diameter, $y+$height-$diameter, $diameter, $diameter, 0, 90)
                    $path.AddArc($x, $y+$height-$diameter, $diameter, $diameter, 90, 90)
                    $path.CloseFigure()
                    $graphics.FillPath($brush, $path)
                } finally { $path.Dispose() }
            }
        } finally { $brush.Dispose() }
    }
    $graphics.Dispose()
    return $bitmap
}

$master = New-IconBitmap 1024
try { $master.Save((Join-Path $PSScriptRoot 'appicon.png'), [System.Drawing.Imaging.ImageFormat]::Png) }
finally { $master.Dispose() }

$sizes = @(16, 20, 24, 32, 40, 48, 64, 128, 256)
$frames = @()
foreach ($size in $sizes) {
    $bitmap = New-IconBitmap $size
    $stream = [System.IO.MemoryStream]::new()
    try {
        $bitmap.Save($stream, [System.Drawing.Imaging.ImageFormat]::Png)
        $frames += ,$stream.ToArray()
    } finally { $stream.Dispose(); $bitmap.Dispose() }
}
$output = [System.IO.File]::Create((Join-Path $PSScriptRoot 'windows/icon.ico'))
$writer = [System.IO.BinaryWriter]::new($output)
try {
    $writer.Write([uint16]0); $writer.Write([uint16]1); $writer.Write([uint16]$sizes.Count)
    $offset = 6 + 16 * $sizes.Count
    for ($index = 0; $index -lt $sizes.Count; $index++) {
        $dimension = if ($sizes[$index] -eq 256) { 0 } else { $sizes[$index] }
        $writer.Write([byte]$dimension); $writer.Write([byte]$dimension)
        $writer.Write([byte]0); $writer.Write([byte]0)
        $writer.Write([uint16]1); $writer.Write([uint16]32)
        $writer.Write([uint32]$frames[$index].Length); $writer.Write([uint32]$offset)
        $offset += $frames[$index].Length
    }
    foreach ($frame in $frames) { $writer.Write([byte[]]$frame) }
} finally { $writer.Dispose(); $output.Dispose() }
Write-Output 'Generated appicon.png and a nine-resolution Windows icon from hostlight.svg.'
