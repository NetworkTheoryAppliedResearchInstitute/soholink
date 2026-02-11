@echo off
REM SoHoLINK Test Runner for Windows
REM Runs tests with various configurations and outputs results

setlocal enabledelayedexpansion

REM Configuration
set TEST_TIMEOUT=120s
set SHORT_MODE=false
set RACE_DETECTION=true
set COVERAGE=true

REM Parse arguments
:parse_args
if "%~1"=="" goto end_parse
if /i "%~1"=="--short" (
	set SHORT_MODE=true
	shift
	goto parse_args
)
if /i "%~1"=="--no-race" (
	set RACE_DETECTION=false
	shift
	goto parse_args
)
if /i "%~1"=="--no-coverage" (
	set COVERAGE=false
	shift
	goto parse_args
)
if /i "%~1"=="--help" (
	echo Usage: %~nx0 [OPTIONS]
	echo.
	echo Options:
	echo   --short        Run tests in short mode (skip slow tests^)
	echo   --no-race      Disable race detection
	echo   --no-coverage  Disable coverage report
	echo   --help         Show this help message
	echo.
	exit /b 0
)
echo Unknown option: %~1
echo Run '%~nx0 --help' for usage information
exit /b 1

:end_parse

REM Header
echo ═══════════════════════════════════════════
echo   SoHoLINK Test Suite
echo ═══════════════════════════════════════════
echo.

REM Build flags
set "TEST_FLAGS=-v -timeout %TEST_TIMEOUT%"
if "%SHORT_MODE%"=="true" (
	set "TEST_FLAGS=!TEST_FLAGS! -short"
	echo Mode: Short (skipping slow tests^)
) else (
	echo Mode: Full
)

if "%RACE_DETECTION%"=="true" (
	set "TEST_FLAGS=!TEST_FLAGS! -race"
	echo Race detection: Enabled
) else (
	echo Race detection: Disabled
)

if "%COVERAGE%"=="true" (
	set "TEST_FLAGS=!TEST_FLAGS! -coverprofile=coverage.out"
	echo Coverage: Enabled
) else (
	echo Coverage: Disabled
)

echo.
echo ───────────────────────────────────────────
echo Running tests...
echo ───────────────────────────────────────────
echo.

REM Run tests
go test !TEST_FLAGS! ./internal/...
set EXIT_CODE=!ERRORLEVEL!

if !EXIT_CODE! equ 0 (
	echo.
	echo ✓ All tests passed!
) else (
	echo.
	echo ✗ Some tests failed!
)

REM Generate coverage report if enabled
if "%COVERAGE%"=="true" (
	if exist coverage.out (
		echo.
		echo ───────────────────────────────────────────
		echo Coverage Report
		echo ───────────────────────────────────────────
		echo.

		REM Generate HTML report
		go tool cover -html=coverage.out -o coverage.html
		echo HTML report: coverage.html
		echo.

		REM Show coverage summary
		go tool cover -func=coverage.out | findstr /C:"total:"
		echo.
	)
)

REM Test summary
echo.
echo ═══════════════════════════════════════════
echo Test Summary
echo ═══════════════════════════════════════════

if !EXIT_CODE! equ 0 (
	echo Status: PASSED ✓
) else (
	echo Status: FAILED ✗
)

if "%COVERAGE%"=="true" (
	if exist coverage.out (
		for /f "tokens=3" %%a in ('go tool cover -func^=coverage.out ^| findstr /C:"total:"') do (
			echo Coverage: %%a
		)
	)
)

echo.
exit /b !EXIT_CODE!
