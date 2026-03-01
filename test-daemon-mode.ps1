# Test script to simulate update trigger from nanobot
# This will call nanobot-auto-updater with --update-now flag

Write-Host "Simulating update trigger from nanobot (daemon mode)..." -ForegroundColor Green
Write-Host "This process (PID: $PID) will act as the parent nanobot process" -ForegroundColor Yellow

# Run updater without NO_DAEMON to test daemon mode
& .\nanobot-auto-updater.exe --update-now --timeout 2m

Write-Host "`nParent process exiting (daemon should continue)..." -ForegroundColor Yellow
Write-Host "Check logs\app-2026-03-01.log for daemon process logs" -ForegroundColor Cyan
