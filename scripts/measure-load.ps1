param(
    [ValidateRange(60, 86400)][int]$Seconds = 60
)

$ErrorActionPreference = 'Stop'
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw 'This workload uses Windows process CPU counters and must run on Windows.'
}
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskPreviousSeconds = $env:HOSTLIGHT_LOAD_SECONDS
Push-Location -LiteralPath $taskRoot
try {
    $env:HOSTLIGHT_LOAD_SECONDS = [string]$Seconds
    & go test . -run '^TestWindowsLoad200$' -count=1 -v "-timeout=$($Seconds + 120)s"
    if ($LASTEXITCODE -ne 0) { throw "Load verification failed with exit code $LASTEXITCODE" }
} finally {
    $env:HOSTLIGHT_LOAD_SECONDS = $taskPreviousSeconds
    Pop-Location
}
