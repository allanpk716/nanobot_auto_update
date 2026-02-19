# build.ps1 - PowerShell build script for nanobot-auto-updater
# Provides both console (debug) and GUI (release) builds

param(
    [Parameter(Position=0)]
    [ValidateSet("build", "build-release", "clean", "test", "help")]
    [string]$Target = "build",

    [string]$Version
)

$ErrorActionPreference = "Stop"

function Get-Version {
    if ($Version) {
        return $Version
    }

    $gitVersion = git describe --tags --always --dirty 2>$null
    if ($LASTEXITCODE -eq 0 -and $gitVersion) {
        return $gitVersion
    }

    return "dev"
}

function Build-Console {
    Write-Host "Building console version..." -ForegroundColor Cyan
    go build -o nanobot-auto-updater.exe ./cmd/nanobot-auto-updater
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "Built console version: nanobot-auto-updater.exe" -ForegroundColor Green
}

function Build-Release {
    Write-Host "Building release version (no console)..." -ForegroundColor Cyan
    $ver = Get-Version
    $ldflags = "-H=windowsgui -X main.Version=$ver"
    go build -ldflags="$ldflags" -o nanobot-auto-updater.exe ./cmd/nanobot-auto-updater
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "Built release version (no console): nanobot-auto-updater.exe" -ForegroundColor Green
}

function Clean-Artifacts {
    Write-Host "Cleaning build artifacts..." -ForegroundColor Cyan
    if (Test-Path "nanobot-auto-updater.exe") {
        Remove-Item "nanobot-auto-updater.exe" -Force
    }
    Write-Host "Cleaned build artifacts" -ForegroundColor Green
}

function Run-Tests {
    Write-Host "Running tests..." -ForegroundColor Cyan
    go test ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Tests failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "All tests passed" -ForegroundColor Green
}

function Show-Help {
    Write-Host "Available targets:" -ForegroundColor Cyan
    Write-Host "  .\build.ps1 build         - Build console version (for debugging)"
    Write-Host "  .\build.ps1 build-release - Build GUI version (for distribution)"
    Write-Host "  .\build.ps1 test          - Run tests"
    Write-Host "  .\build.ps1 clean         - Remove build artifacts"
    Write-Host "  .\build.ps1 help          - Show this help"
    Write-Host ""
    Write-Host "Variables:" -ForegroundColor Cyan
    Write-Host "  -Version x.x.x            - Set version (default: git tag or 'dev')"
    Write-Host ""
    Write-Host "Examples:" -ForegroundColor Cyan
    Write-Host "  .\build.ps1 build"
    Write-Host "  .\build.ps1 build-release"
    Write-Host "  .\build.ps1 build-release -Version 1.0.0"
}

# Execute target
switch ($Target) {
    "build" { Build-Console }
    "build-release" { Build-Release }
    "clean" { Clean-Artifacts }
    "test" { Run-Tests }
    "help" { Show-Help }
}
