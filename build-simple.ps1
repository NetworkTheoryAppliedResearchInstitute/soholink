# Simple SoHoLINK Build Script
# Creates a portable installer that works on any platform

$ErrorActionPreference = "Stop"

Write-Host "================================" -ForegroundColor Cyan
Write-Host "Building SoHoLINK Installer" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""

# Create directories
$null = New-Item -ItemType Directory -Path "dist" -Force
$null = New-Item -ItemType Directory -Path "build" -Force

# Build the wizard
Write-Host "Building SoHoLINK Setup Wizard..." -ForegroundColor Yellow
$env:CGO_ENABLED = "1"

go build -o "build\soholink-wizard.exe" ".\cmd\soholink-wizard\main.go"

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Build failed!" -ForegroundColor Red
    exit 1
}

Write-Host "✅ Build successful!" -ForegroundColor Green
Write-Host ""

# Create README for distribution
$readmeContent = @"
# SoHoLINK Setup

Welcome to SoHoLINK!

## Installation

1. **Extract this folder** to any location on your computer
2. **Double-click** ``soholink-wizard.exe`` to start the setup wizard
3. Follow the wizard steps to configure your system
4. Start earning from your spare compute resources!

## What is SoHoLINK?

SoHoLINK transforms your spare compute resources into income by connecting you to the federated cloud marketplace. Share your unused CPU, RAM, and storage with others and earn money automatically.

## System Requirements

- **Windows:** Windows 10/11 with Hyper-V
- **Linux:** Any modern distribution with KVM
- **macOS:** macOS 11+ (coming soon)

## Minimum Hardware

- 4+ CPU cores
- 8+ GB RAM
- 100+ GB free storage
- Internet connection

## Support

For help and documentation, visit: https://github.com/NetworkTheoryAppliedResearchInstitute/soholink

## License

© 2023 Network Theory Applied Research Institute
"@

$readmeContent | Out-File -FilePath "build\README.txt" -Encoding UTF8

# Copy logo (if exists)
if (Test-Path "assets\logo.png") {
    Copy-Item "assets\logo.png" "build\logo.png"
}

# Create the distribution package
Write-Host "Creating distribution package..." -ForegroundColor Yellow

$distName = "SoHoLINK-Setup-Windows"
$zipPath = "dist\$distName.zip"

# Remove old zip if exists
if (Test-Path $zipPath) {
    Remove-Item $zipPath
}

# Create zip
Compress-Archive -Path "build\soholink-wizard.exe","build\README.txt" -DestinationPath $zipPath -Force

if (Test-Path "build\logo.png") {
    Compress-Archive -Path "build\logo.png" -DestinationPath $zipPath -Update
}

$zipSize = [math]::Round((Get-Item $zipPath).Length / 1MB, 2)

Write-Host ""
Write-Host "================================" -ForegroundColor Cyan
Write-Host "✅ Build Complete!" -ForegroundColor Green
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Installer package created:" -ForegroundColor White
Write-Host "  📦 $zipPath ($zipSize MB)" -ForegroundColor Cyan
Write-Host ""
Write-Host "To install on your laptop:" -ForegroundColor Yellow
Write-Host "  1. Copy $zipPath to your laptop" -ForegroundColor Gray
Write-Host "  2. Extract the ZIP file" -ForegroundColor Gray
Write-Host "  3. Double-click soholink-wizard.exe" -ForegroundColor Gray
Write-Host "  4. Follow the setup wizard" -ForegroundColor Gray
Write-Host ""
