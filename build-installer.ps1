# SoHoLINK Installer Build Script
# Builds installers for Windows, Linux, and macOS

param(
    [string]$Platform = "all"
)

$ErrorActionPreference = "Stop"

Write-Host "================================" -ForegroundColor Cyan
Write-Host "SoHoLINK Installer Builder" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""

# Create dist directory
if (-not (Test-Path "dist")) {
    New-Item -ItemType Directory -Path "dist" | Out-Null
}

# Create build directory
if (-not (Test-Path "build")) {
    New-Item -ItemType Directory -Path "build" | Out-Null
}

# Function to build binaries
function Build-Binary {
    param(
        [string]$OS,
        [string]$Arch,
        [string]$OutputName
    )

    Write-Host "Building $OutputName for $OS/$Arch..." -ForegroundColor Yellow

    $env:GOOS = $OS
    $env:GOARCH = $Arch
    $env:CGO_ENABLED = "1"

    # Build wizard
    Write-Host "  - Building wizard..." -ForegroundColor Gray
    go build -o "build\$OutputName" ".\cmd\soholink-wizard\main.go"

    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ Build failed for $OutputName" -ForegroundColor Red
        exit 1
    }

    Write-Host "✅ Built $OutputName" -ForegroundColor Green
}

# Build Windows
if ($Platform -eq "all" -or $Platform -eq "windows") {
    Write-Host ""
    Write-Host "Building Windows Installer..." -ForegroundColor Cyan
    Write-Host ""

    # Build wizard executable
    Build-Binary -OS "windows" -Arch "amd64" -OutputName "soholink-wizard.exe"

    # Build main executable (placeholder)
    Write-Host "  - Building main executable..." -ForegroundColor Gray
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "1"

    # Create simple launcher
    @"
package main
import "fmt"
func main() {
    fmt.Println("SoHoLINK Service - Not yet implemented")
    fmt.Println("Run soholink-wizard.exe to configure")
}
"@ | Out-File -FilePath "build\main.go" -Encoding ASCII

    go build -o "build\soholink.exe" "build\main.go"
    Remove-Item "build\main.go"

    # Check for NSIS
    $nsisPath = "C:\Program Files (x86)\NSIS\makensis.exe"
    if (Test-Path $nsisPath) {
        Write-Host ""
        Write-Host "Creating Windows installer with NSIS..." -ForegroundColor Yellow

        # Run NSIS
        & $nsisPath "installer\windows\installer.nsi"

        if ($LASTEXITCODE -eq 0) {
            Write-Host "✅ Windows installer created: dist\SoHoLINK-Setup.exe" -ForegroundColor Green
        } else {
            Write-Host "⚠️  NSIS failed, but binaries are available in build\" -ForegroundColor Yellow
        }
    } else {
        Write-Host "⚠️  NSIS not found. Installer not created." -ForegroundColor Yellow
        Write-Host "   Install NSIS from: https://nsis.sourceforge.io/" -ForegroundColor Gray
        Write-Host "   Binaries are available in build\" -ForegroundColor Gray

        # Create a simple zip instead
        Write-Host "   Creating ZIP archive instead..." -ForegroundColor Yellow
        Compress-Archive -Path "build\soholink-wizard.exe","build\soholink.exe" -DestinationPath "dist\SoHoLINK-Windows.zip" -Force
        Write-Host "✅ Created dist\SoHoLINK-Windows.zip" -ForegroundColor Green
    }
}

# Build Linux
if ($Platform -eq "all" -or $Platform -eq "linux") {
    Write-Host ""
    Write-Host "Building Linux Package..." -ForegroundColor Cyan
    Write-Host ""

    Build-Binary -OS "linux" -Arch "amd64" -OutputName "soholink-wizard"

    # Build main executable
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    @"
package main
import "fmt"
func main() {
    fmt.Println("SoHoLINK Service - Not yet implemented")
    fmt.Println("Run soholink-wizard to configure")
}
"@ | Out-File -FilePath "build\main.go" -Encoding ASCII
    go build -o "build\soholink" "build\main.go"
    Remove-Item "build\main.go"

    # Create tarball
    Write-Host "Creating Linux tarball..." -ForegroundColor Yellow
    tar -czf "dist\SoHoLINK-Linux-x64.tar.gz" -C "build" "soholink-wizard" "soholink"

    Write-Host "✅ Created dist\SoHoLINK-Linux-x64.tar.gz" -ForegroundColor Green
}

# Build macOS
if ($Platform -eq "all" -or $Platform -eq "darwin") {
    Write-Host ""
    Write-Host "Building macOS Package..." -ForegroundColor Cyan
    Write-Host ""

    Write-Host "⚠️  macOS cross-compilation requires additional setup" -ForegroundColor Yellow
    Write-Host "   Build on macOS directly using: ./build-installer.sh" -ForegroundColor Gray
}

Write-Host ""
Write-Host "================================" -ForegroundColor Cyan
Write-Host "Build Complete!" -ForegroundColor Green
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Installers available in dist/:" -ForegroundColor White
Get-ChildItem "dist" | ForEach-Object {
    $size = [math]::Round($_.Length / 1MB, 2)
    Write-Host "  • $($_.Name) ($size MB)" -ForegroundColor Gray
}
Write-Host ""
