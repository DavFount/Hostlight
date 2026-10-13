$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'release-tools.ps1')
foreach ($valid in @('0.1.0', '1.2.3', '65535.0.0')) {
    if ((ConvertTo-ReleaseVersion $valid) -ne $valid) { throw 'Valid version changed.' }
}
foreach ($invalid in @('', 'v1.2.3', '01.2.3', '1.2', '1.2.3.4', '1.2.3-rc.1', '65536.0.0', '1.2.3;whoami', '../1.2.3', "1.2.3`n", ' 1.2.3', '1.2.3 ')) {
    $rejected = $false
    try { ConvertTo-ReleaseVersion $invalid | Out-Null } catch { $rejected = $true }
    if (-not $rejected) { throw "Unsafe/invalid version accepted: $invalid" }
}
$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("hostlight-package-test-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $testRoot | Out-Null
$binary = Join-Path $testRoot 'fixture.exe'
[IO.File]::WriteAllBytes($binary, [byte[]](77, 90, 0, 1))
$output = Join-Path $testRoot 'output'
$archive = New-ReleaseArchive -Binary $binary -Version '0.1.0' -OutputDirectory $output
$hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
if ((Get-Content -LiteralPath (Join-Path $output 'SHA256SUMS.txt') -Raw).Trim() -ne "$hash  hostlight-0.1.0-windows-amd64.zip") { throw 'Checksum mismatch.' }
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [IO.Compression.ZipFile]::OpenRead($archive)
try {
    $entries = @($zip.Entries | ForEach-Object { $_.FullName.Replace('\','/') })
    if ($entries.Count -ne 3 -or 'Hostlight/hostlight.exe' -notin $entries -or 'Hostlight/README.txt' -notin $entries -or 'Hostlight/LICENSE' -notin $entries) { throw "Unexpected archive entries: $entries" }
    $licenseReader = [IO.StreamReader]::new($zip.GetEntry('Hostlight/LICENSE').Open())
    try {
        if ($licenseReader.ReadToEnd() -ne [IO.File]::ReadAllText((Join-Path (Split-Path -Parent $PSScriptRoot) 'LICENSE'))) { throw 'Packaged license differs from source.' }
    } finally { $licenseReader.Dispose() }
} finally { $zip.Dispose() }
$rejected = $false
try { New-ReleaseArchive -Binary $binary -Version '0.1.0' -OutputDirectory $output | Out-Null } catch { $rejected = $true }
if (-not $rejected) { throw 'Existing package was overwritten.' }
Write-Output "Release-tool checks passed. Disposable fixtures: $testRoot"
