@echo off
setlocal

cd /d "%~dp0"

set "DB_PATH=data\xhs.db"
set "COLLECTOR_CMD=go run ./cmd/xhs-native-collector"

if not "%~1"=="" (
  set "PORT=%~1"
) else (
  for /f %%i in ('powershell -NoProfile -Command "$preferred=18080; if (-not (Get-NetTCPConnection -LocalPort $preferred -ErrorAction SilentlyContinue)) { $preferred } else { $listener=[System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback,0); $listener.Start(); $port=$listener.LocalEndpoint.Port; $listener.Stop(); $port }"') do set "PORT=%%i"
)

set "ADDR=:%PORT%"
set "URL=http://localhost:%PORT%"

echo Starting xiaohongshu-tool web workspace...
echo URL: %URL%
echo DB: %DB_PATH%
echo.

start "" powershell -NoProfile -WindowStyle Hidden -Command "Start-Sleep -Seconds 2; Start-Process '%URL%'"
go run ./cmd/xhs-web --addr %ADDR% --db "%DB_PATH%" --collector-command "%COLLECTOR_CMD%"

endlocal
