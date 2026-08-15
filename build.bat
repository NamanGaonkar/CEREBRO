@echo off
REM ============================================================
REM  CEREBRO - build script (Windows)
REM  Produces a standalone cerebro.exe with no DLL dependencies.
REM  Requires: Go 1.22+   (winget install GoLang.Go)
REM ============================================================
where go >nul 2>nul
if errorlevel 1 (
  echo [ERROR] Go is not installed or not on PATH.
  echo         Install it with:  winget install GoLang.Go
  exit /b 1
)
echo [1/2] Fetching dependencies...
go mod tidy
if errorlevel 1 (
  echo [ERROR] go mod tidy failed.
  exit /b 1
)
echo [2/2] Building cerebro.exe ...
set CGO_ENABLED=0
go build -trimpath -ldflags "-s -w" -o cerebro.exe ./cmd/app
if errorlevel 1 (
  echo [ERROR] Build failed.
  exit /b 1
)
echo Done! Run it with:  .\cerebro.exe
