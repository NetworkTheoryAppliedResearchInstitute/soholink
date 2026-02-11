@echo off
REM SoHoLINK Installer Creation Script

echo ================================
echo Building SoHoLINK Installer
echo ================================
echo.

REM Create directories
if not exist "dist" mkdir dist
if not exist "build" mkdir build

echo Building wizard executable...
set CGO_ENABLED=0
set GOFLAGS=-mod=mod
go build -o build\soholink-wizard.exe .\cmd\soholink-wizard-cli\main.go

if %ERRORLEVEL% NEQ 0 (
    echo Build failed!
    pause
    exit /b 1
)

echo.
echo Build successful!
echo.

REM Create README
echo SoHoLINK Setup Wizard > build\README.txt
echo. >> build\README.txt
echo Double-click soholink-wizard.exe to start setup >> build\README.txt
echo. >> build\README.txt
echo This wizard will: >> build\README.txt
echo - Detect your system hardware >> build\README.txt
echo - Calculate operating costs >> build\README.txt
echo - Suggest competitive pricing >> build\README.txt
echo - Generate complete configuration >> build\README.txt
echo. >> build\README.txt
echo (c) 2023 Network Theory Applied Research Institute >> build\README.txt

echo.
echo ================================
echo Installer ready!
echo ================================
echo.
echo Files in build\ folder:
dir /B build\
echo.
echo To distribute:
echo 1. Copy the build\ folder to your laptop
echo 2. Or create a ZIP of build\ folder
echo 3. Double-click soholink-wizard.exe
echo.
echo Ready to test!
echo.
pause
