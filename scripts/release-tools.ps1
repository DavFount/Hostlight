function ConvertTo-ReleaseVersion {
    param([string]$Version)
    if ($Version -notmatch '\A(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})\z') {
        throw 'Use a three-part version without a v prefix, such as 0.1.0.'
    }
    foreach ($part in $Version.Split('.')) {
        if ([int]$part -gt 65535) { throw 'Windows version components must be 0–65535.' }
    }
    return $Version
}

function New-ReleaseArchive {
    param([string]$Binary, [string]$Version, [string]$OutputDirectory)
    $Version = ConvertTo-ReleaseVersion $Version
    if (-not (Test-Path -LiteralPath $Binary -PathType Leaf)) { throw 'Release binary is missing.' }
    if (Test-Path -LiteralPath $OutputDirectory) { throw 'Release output already exists; choose a fresh output directory.' }
    New-Item -ItemType Directory -Path $OutputDirectory | Out-Null
    $staging = Join-Path $OutputDirectory 'Hostlight'
    New-Item -ItemType Directory -Path $staging | Out-Null
    Copy-Item -LiteralPath $Binary -Destination (Join-Path $staging 'hostlight.exe')
    $license = Join-Path (Split-Path -Parent $PSScriptRoot) 'LICENSE'
    Copy-Item -LiteralPath $license -Destination (Join-Path $staging 'LICENSE')
    $readme = @"
Hostlight $Version — Windows amd64 standalone build

Extract this entire folder and run hostlight.exe.
Microsoft Edge WebView2 Runtime is required. The application may prompt to install it.
This build is unsigned; Windows SmartScreen may display a warning.
Download only from the project's trusted release page and verify SHA256SUMS.txt.

Settings/history are saved under %APPDATA%\Hostlight, not alongside the executable.
Webhook credentials are protected for the current Windows user.
This is a standalone desktop app, not a Windows service.
Monitoring stops when you quit. Close-to-tray is opt-in under Desktop behavior.

https://github.com/DavFount/Hostlight

License: GNU GPL version 3 (GPL-3.0-only). See LICENSE.
Matching source: https://github.com/DavFount/Hostlight/tree/v$Version
"@
    [IO.File]::WriteAllText((Join-Path $staging 'README.txt'), $readme)
    $archive = Join-Path $OutputDirectory "hostlight-$Version-windows-amd64.zip"
    Compress-Archive -LiteralPath $staging -DestinationPath $archive
    $hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText((Join-Path $OutputDirectory 'SHA256SUMS.txt'), "$hash  $([IO.Path]::GetFileName($archive))`n")
    return $archive
}
