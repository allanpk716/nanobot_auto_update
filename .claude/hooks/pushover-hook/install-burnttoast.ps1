# Check PowerShell version and install BurntToast
$PSVersion = $PSVersionTable.PSVersion
Write-Host "PowerShell Version: $($PSVersion.Major).$($PSVersion.Minor).$($PSVersion.Build)"

# Try to import PowerShellGet
try {
    Import-Module PowerShellGet -ErrorAction Stop
    Write-Host "PowerShellGet is available"

    # Install BurntToast
    Write-Host "Installing BurntToast module..."
    Install-Module -Name BurntToast -Scope CurrentUser -Force -AllowClobber
    Write-Host "BurntToast installed successfully!"

    # Test it
    Import-Module BurntToast
    Write-Host "Testing BurntToast notification..."
    New-BurntToastNotification -Title "Claude Code" -Body "BurntToast is now installed!"
    Write-Host "Test notification sent!"
}
catch {
    Write-Host "ERROR: PowerShellGet not available or installation failed"
    Write-Host "Error: $($_.Exception.Message)"
    Write-Host ""
    Write-Host "Please manually install BurntToast:"
    Write-Host "1. Update PowerShell to latest version"
    Write-Host "2. Run: Install-Module -Name BurntToast -Scope CurrentUser"
}
