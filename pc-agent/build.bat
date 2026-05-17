@echo off
echo ══════════════════════════════════════════════════
echo   PC Command Agent — Go Build Script
echo ══════════════════════════════════════════════════

set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64

echo [1/3] Running go vet...
go vet ./...
if errorlevel 1 ( echo FAIL: go vet; exit /b 1 )

echo [2/3] Building release binary...
go build -ldflags="-s -w -H windowsgui" -o agent.exe ./cmd/agent
if errorlevel 1 ( echo FAIL: go build; exit /b 1 )

for %%I in (agent.exe) do set SIZE=%%~zI
set /a SIZEMB=%SIZE%/1048576
echo [3/3] Build complete!
echo   Binary: agent.exe (%SIZEMB% MB)
echo   CGO:    disabled (zero DLL dependencies)
echo   Flags:  -s -w -H windowsgui (stripped, no console)
echo ══════════════════════════════════════════════════
