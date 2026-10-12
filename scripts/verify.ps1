param(
    [switch]$SkipDesktopBuild
)

$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot

function Invoke-Checked {
    param([string]$Command, [string[]]$Arguments)
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Command failed with exit code $LASTEXITCODE"
    }
}

Push-Location -LiteralPath $taskRoot
try {
    # Include new, untracked source files, while excluding ignored dependencies.
    $goFiles = @(& git ls-files --cached --others --exclude-standard '*.go')
    if ($LASTEXITCODE -ne 0 -or $goFiles.Count -eq 0) {
        throw 'Run verification from a Git checkout containing Go source.'
    }
    $unformatted = @(& gofmt -l @goFiles)
    if ($LASTEXITCODE -ne 0) { throw 'Go formatting check failed.' }
    if ($unformatted.Count -gt 0) {
        throw "Run gofmt on: $($unformatted -join ', ')"
    }

    # main.go embeds frontend/dist, so build assets before compiling Go on a fresh checkout.
    Push-Location -LiteralPath (Join-Path $taskRoot 'frontend')
    try {
        Invoke-Checked 'npm.cmd' @('ci')
        Invoke-Checked 'npm.cmd' @('test')
        Invoke-Checked 'npm.cmd' @('run', 'build')
    } finally {
        Pop-Location
    }

    Invoke-Checked 'go' @('test', './...', '-count=1', '-timeout=3m')
    Invoke-Checked 'go' @('vet', './...')
    & (Join-Path $PSScriptRoot 'test-release-tools.ps1')

    if (-not $SkipDesktopBuild) {
        # Use the already-built frontend and don't let Wails update module files.
        # A separate output avoids overwriting a running development executable.
        Invoke-Checked 'wails' @('build', '-s', '-m', '-nosyncgomod', '-o', 'hostlight-verify.exe')
    }
} finally {
    Pop-Location
}
