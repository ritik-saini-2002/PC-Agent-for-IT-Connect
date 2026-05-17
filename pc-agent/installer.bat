@echo off
echo ══════════════════════════════════════════════════
echo   PC Command Agent — Installer
echo ══════════════════════════════════════════════════
echo.

:: Check admin
net session >nul 2>&1
if errorlevel 1 (
    echo ERROR: Run this as Administrator!
    pause
    exit /b 1
)

:: Copy binary
set INSTALL_DIR=%ProgramData%\PCCommandAgent
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
copy /Y agent.exe "%INSTALL_DIR%\agent.exe" >nul
echo [OK] Binary copied to %INSTALL_DIR%

:: Install service
sc stop PCCommandAgent >nul 2>&1
sc delete PCCommandAgent >nul 2>&1
sc create PCCommandAgent binPath= "\"%INSTALL_DIR%\agent.exe\"" start= auto obj= LocalSystem
sc description PCCommandAgent "PC Command Agent — Remote control service"
sc failure PCCommandAgent reset= 60 actions= restart/5000/restart/5000/restart/5000
echo [OK] Windows Service installed

:: Firewall rules
netsh advfirewall firewall delete rule name="PC Command Agent API" >nul 2>&1
netsh advfirewall firewall delete rule name="PC Command Agent Stream" >nul 2>&1
netsh advfirewall firewall add rule name="PC Command Agent API" dir=in action=allow protocol=TCP localport=5000
netsh advfirewall firewall add rule name="PC Command Agent Stream" dir=in action=allow protocol=TCP localport=5001
echo [OK] Firewall rules configured (ports 5000, 5001)

:: Start service
sc start PCCommandAgent
echo [OK] Service started

echo.
echo ══════════════════════════════════════════════════
echo   Installation complete!
echo   Service: PCCommandAgent (auto-start)
echo   Ports:   5000 (API) + 5001 (Stream)
echo ══════════════════════════════════════════════════
pause
