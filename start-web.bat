@echo off
setlocal

cd /d "%~dp0"

set "DB_PATH=data\xhs.db"
set "BIN_DIR=%CD%\bin"
set "WEB_EXE=%BIN_DIR%\xhs-web.exe"
set "COLLECTOR_EXE=%BIN_DIR%\xhs-native-collector.exe"
set "COLLECTOR_CMD=%COLLECTOR_EXE%"

set "GO_EXE=go"
set "GO_FOUND=1"
where go >nul 2>nul
if errorlevel 1 (
  if exist "C:\Program Files\Go\bin\go.exe" (
    set "GO_EXE=C:\Program Files\Go\bin\go.exe"
  ) else (
    set "GO_FOUND=0"
  )
)
if "%GO_FOUND%"=="0" (
  echo ERROR: Go was not found in PATH.
  echo.
  echo This project is a Go application and requires Go 1.24 or newer.
  echo Install Go from https://go.dev/dl/ or run:
  echo   winget install GoLang.Go
  echo.
  echo After installation, close this terminal, open a new PowerShell window,
  echo and run .\start-web.bat again.
  exit /b 1
)

if not "%~1"=="" (
  set "PORT=%~1"
) else (
  for /f %%i in ('powershell -NoProfile -Command "$preferred=18080; if (-not (Get-NetTCPConnection -LocalPort $preferred -ErrorAction SilentlyContinue)) { $preferred } else { $listener=[System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback,0); $listener.Start(); $port=$listener.LocalEndpoint.Port; $listener.Stop(); $port }"') do set "PORT=%%i"
)

set "ADDR=127.0.0.1:%PORT%"
set "URL=http://localhost:%PORT%"

echo Starting xiaohongshu-tool web workspace...
echo URL: %URL%
echo DB: %DB_PATH%
echo.

if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"
echo Building fixed local executables...
"%GO_EXE%" build -o "%WEB_EXE%" ./cmd/xhs-web
if errorlevel 1 exit /b 1
"%GO_EXE%" build -o "%COLLECTOR_EXE%" ./cmd/xhs-native-collector
if errorlevel 1 exit /b 1

start "" powershell -NoProfile -WindowStyle Hidden -Command "Start-Sleep -Seconds 2; Start-Process '%URL%'"
"%WEB_EXE%" --addr %ADDR% --db "%DB_PATH%" --collector-command "%COLLECTOR_CMD%"

endlocal
