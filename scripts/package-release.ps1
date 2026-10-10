param([Parameter(Mandatory)][string]$Version)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'release-tools.ps1')
$Version = ConvertTo-ReleaseVersion $Version
$taskRoot = Split-Path -Parent $PSScriptRoot
$configPath = Join-Path $taskRoot 'wails.json'
$originalConfig = [IO.File]::ReadAllBytes($configPath)
$output = Join-Path $taskRoot "build/bin/release/$Version"
if (Test-Path -LiteralPath $output) { throw 'This version is already packaged locally; existing artifacts were preserved.' }

Push-Location -LiteralPath $taskRoot
try {
    # The temporary resource version is restored even if compilation fails.
    $config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
    $config | Add-Member -Force NoteProperty info ([pscustomobject]@{
        companyName = 'David Fountain'
        productName = 'Hostlight'
        productVersion = $Version
        comments = 'Standalone desktop network monitoring'
    })
    [IO.File]::WriteAllText($configPath, ($config | ConvertTo-Json -Depth 20))
    & wails build -s -m -nosyncgomod -o hostlight-release.exe -ldflags "-X main.applicationVersion=$Version"
    if ($LASTEXITCODE -ne 0) { throw 'Release build failed; no archive was created.' }
    $binary = Join-Path $taskRoot 'build/bin/hostlight-release.exe'
    $metadata = [Diagnostics.FileVersionInfo]::GetVersionInfo($binary)
    $parts = @($Version.Split('.') | ForEach-Object { [int]$_ })
    if ($metadata.FileMajorPart -ne $parts[0] -or $metadata.FileMinorPart -ne $parts[1] -or
        $metadata.FileBuildPart -ne $parts[2] -or $metadata.ProductMajorPart -ne $parts[0] -or
        $metadata.ProductMinorPart -ne $parts[1] -or $metadata.ProductBuildPart -ne $parts[2] -or
        $metadata.ProductName -ne 'Hostlight') {
        throw 'Windows release metadata did not match the requested version.'
    }
    New-ReleaseArchive -Binary $binary -Version $Version -OutputDirectory $output
} finally {
    [IO.File]::WriteAllBytes($configPath, $originalConfig)
    Pop-Location
}
