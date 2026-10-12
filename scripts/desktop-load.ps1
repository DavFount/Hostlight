param(
    [ValidateRange(1, 24)][int]$Hours = 1
)

$ErrorActionPreference = 'Stop'
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw 'This desktop workload is for Windows.'
}
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskPreviousData = $env:HOSTLIGHT_DEV_DATA_DIR
$taskFixture = $null
$taskTemp = Join-Path ([IO.Path]::GetTempPath()) ('hostlight-desktop-load-' + [guid]::NewGuid().ToString('N'))
$taskData = Join-Path $taskTemp 'data'
Push-Location -LiteralPath $taskRoot
try {
    Get-Command go, wails, npm.cmd -ErrorAction Stop | Out-Null
    New-Item -ItemType Directory -Path $taskTemp | Out-Null
    $taskExe = Join-Path $taskTemp 'fixture.exe'
    & go build -o $taskExe ./cmd/desktop-load-fixture
    if ($LASTEXITCODE -ne 0) { throw 'Could not build the local fixture.' }
    $taskFixture = Start-Process -FilePath $taskExe -ArgumentList @('-dir', ('"' + $taskData + '"'), '-duration', ($Hours.ToString() + 'h')) -WindowStyle Hidden -RedirectStandardOutput (Join-Path $taskTemp 'fixture.stdout.log') -RedirectStandardError (Join-Path $taskTemp 'fixture.stderr.log') -PassThru
    $taskReady = Join-Path $taskData 'ready.json'
    $taskDeadline = [DateTime]::UtcNow.AddSeconds(30)
    while (-not (Test-Path -LiteralPath $taskReady)) {
        if ($taskFixture.HasExited) { throw "Fixture exited; inspect $taskTemp\fixture.stderr.log" }
        if ([DateTime]::UtcNow -gt $taskDeadline) { throw 'Local fixture startup timed out.' }
        Start-Sleep -Milliseconds 100
    }
    $env:HOSTLIGHT_DEV_DATA_DIR = $taskData
    Write-Host "Development workspace: $taskData"
    Write-Host 'The app window must say Hostlight — Development workspace.'
    Write-Host '200 loopback TCP monitors run every second. User configuration, history, and WebView preferences are separate.'
    Write-Host 'In another PowerShell terminal, fail the first 20 endpoints:'
    Write-Host "Set-Content -LiteralPath '$taskData\control.txt' -Value down"
    Write-Host 'Recover them:'
    Write-Host "Set-Content -LiteralPath '$taskData\control.txt' -Value up"
    Write-Host "Fixture lifetime: $Hours hour(s). Exit Wails dev when finished; temporary data is kept for inspection."
    & wails dev
    if ($LASTEXITCODE -ne 0) { throw "Wails dev exited with code $LASTEXITCODE" }
} finally {
    $env:HOSTLIGHT_DEV_DATA_DIR = $taskPreviousData
    if ($null -ne $taskFixture -and -not $taskFixture.HasExited) {
        Stop-Process -InputObject $taskFixture -Force
    }
    Pop-Location
    if (Test-Path -LiteralPath $taskTemp) { Write-Host "Disposable test files retained at $taskTemp" }
}
