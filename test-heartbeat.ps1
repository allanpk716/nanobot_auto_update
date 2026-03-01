# Test script to verify heartbeat logging
# This simulates a slow update by using a longer timeout

Write-Host "Testing heartbeat logging functionality..." -ForegroundColor Green
Write-Host "This test will run for ~30 seconds to verify heartbeat logs" -ForegroundColor Yellow

# Run in NO_DAEMON mode to see logs in console
$env:NO_DAEMON = "1"
& .\nanobot-auto-updater.exe --update-now --timeout 1m

Write-Host "`nTest completed. Check logs above for heartbeat messages." -ForegroundColor Cyan
