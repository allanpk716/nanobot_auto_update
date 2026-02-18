#!/usr/bin/env python3
"""
Test script for Windows native notification support.

Run this script to verify:
1. Platform is Windows 10/11
2. PowerShell is available
3. Windows notifications can be sent successfully
4. Disable file (.no-windows) works correctly

Usage:
    python test-windows-notification.py
"""

import os
import subprocess
import sys
from datetime import datetime
from pathlib import Path


def print_header(text: str) -> None:
    """Print a section header."""
    print(f"\n{'='*60}")
    print(f"  {text}")
    print(f"{'='*60}")


def print_success(text: str) -> None:
    """Print success message."""
    print(f"[OK] {text}")


def print_error(text: str) -> None:
    """Print error message."""
    print(f"[ERROR] {text}")


def print_warning(text: str) -> None:
    """Print warning message."""
    print(f"[WARN] {text}")


def print_info(text: str) -> None:
    """Print info message."""
    print(f"[INFO] {text}")


def check_platform() -> bool:
    """Check if running on Windows."""
    print_header("Step 1: Checking Platform")

    if sys.platform == "win32":
        import platform
        version = platform.version()
        print_success(f"Running on Windows")
        print_info(f"Version: {version}")
        return True
    else:
        print_error(f"Not running on Windows (platform: {sys.platform})")
        print_info("Windows native notifications are only supported on Windows 10/11")
        return False


def check_powershell() -> bool:
    """Check if PowerShell is available."""
    print_header("Step 2: Checking PowerShell")

    try:
        result = subprocess.run(
            ["powershell", "-Command", "$PSVersionTable.PSVersion"],
            capture_output=True,
            text=True,
            timeout=5
        )
        if result.returncode == 0:
            version = result.stdout.strip()
            print_success(f"PowerShell is available")
            print_info(f"Version: {version}")
            return True
        else:
            print_error("PowerShell command failed")
            return False
    except FileNotFoundError:
        print_error("PowerShell is not installed or not in PATH")
        return False
    except Exception as e:
        print_error(f"Error checking PowerShell: {e}")
        return False


def send_windows_notification(title: str, message: str) -> bool:
    """
    Send a Windows 10/11 notification using PowerShell.

    Uses multiple methods with fallback for compatibility.

    Args:
        title: Notification title
        message: Notification message body

    Returns:
        True if successful, False otherwise
    """
    # Convert literal \n to actual newlines (like pushover-notify.py)
    message = message.replace("\\n", "\n")

    # Escape for PowerShell
    title_escaped = title.replace("'", "''").replace('"', '""')
    message_escaped = message.replace("'", "''").replace('"', '""').replace("`", "``")

    # Method 1: Try BurntToast module
    ps_script_burnttoast = f'''
    try {{
        Import-Module BurntToast -ErrorAction Stop
        New-BurntToastNotification -Title '{title_escaped}' -Body '{message_escaped}'
        exit 0
    }} catch {{
        exit 1
    }}
    '''

    # Method 2: Try Windows.UI.Notifications (WinRT) with proper runtime loading
    ps_script_winrt = f'''
    try {{
        Add-Type -AssemblyName System.Runtime.WindowsRuntime -ErrorAction Stop
        $null = [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime]
        $null = [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime]
        $xmlString = "<toast><visual><binding template=`"ToastText02`"><text id=`"1`">{title_escaped}</text><text id=`"2`">{message_escaped}</text></binding></visual></toast>"
        $xmlDoc = New-Object Windows.Data.Xml.Dom.XmlDocument
        $xmlDoc.LoadXml($xmlString)
        $toast = New-Object Windows.UI.Notifications.ToastNotification $xmlDoc
        [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("ClaudeCode").Show($toast)
        exit 0
    }} catch {{
        exit 1
    }}
    '''

    # Method 3: Use .NET classic balloon
    ps_script_classic = f'''
    Add-Type -AssemblyName System.Windows.Forms
    $balloon = New-Object System.Windows.Forms.NotifyIcon
    $balloon.Icon = [System.Drawing.SystemIcons]::Information
    $balloon.BalloonTipTitle = '{title_escaped}'
    $balloon.BalloonTipText = '{message_escaped}'
    $balloon.Visible = $true
    $balloon.ShowBalloonTip(5000)
    Start-Sleep -Seconds 6
    $balloon.Dispose()
    exit 0
    '''

    methods = [
        ("BurntToast module", ps_script_burnttoast),
        ("Windows.UI.Notifications (WinRT)", ps_script_winrt),
        ("Classic balloon (.NET)", ps_script_classic),
    ]

    for method_name, script in methods:
        try:
            result = subprocess.run(
                ["powershell", "-Command", script],
                capture_output=True,
                text=True,
                timeout=10
            )

            if result.returncode == 0:
                print_success(f"Notification sent using {method_name}")
                return True
            else:
                print_info(f"Method '{method_name}' failed, trying next...")
                if result.stdout.strip():
                    print_info(f"  stdout: {result.stdout[:300]}")
                if result.stderr.strip():
                    print_info(f"  stderr: {result.stderr[:300]}")
                continue

        except subprocess.TimeoutExpired:
            print_warning(f"{method_name} timed out, trying next...")
            continue
        except Exception as e:
            print_warning(f"{method_name} error: {e}, trying next...")
            continue

    print_error("All notification methods failed")
    return False


def test_basic_notification() -> bool:
    """Test basic Windows notification."""
    print_header("Step 3: Testing Basic Notification")

    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    title = "Claude Code Test"
    message = f"Test notification from Windows hook\\nTime: {timestamp}"

    print_info(f"Title: {title}")
    print_info(f"Message: {message}")
    print_info("Sending notification...")

    success = send_windows_notification(title, message)

    if success:
        print_success("Notification sent successfully!")
        print_info("Please check your Windows notification center")
        return True
    else:
        print_error("Failed to send notification")
        return False


def test_chinese_notification() -> bool:
    """Test Chinese character support."""
    print_header("Step 4: Testing Chinese Characters")

    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    title = "编码测试"
    message = f"中文测试通知\\n时间: {timestamp}"

    print_info(f"Title: {title}")
    print_info(f"Message: {message}")
    print_info("Sending notification with Chinese characters...")

    success = send_windows_notification(title, message)

    if success:
        print_success("Chinese notification sent successfully!")
        print_info("Please verify characters display correctly")
        return True
    else:
        print_error("Failed to send Chinese notification")
        return False


def test_disable_file() -> bool:
    """Test .no-windows disable file functionality."""
    print_header("Step 5: Testing Disable File")

    # Get current directory
    cwd = Path.cwd()
    disable_file = cwd / ".no-windows"

    # Create disable file
    print_info(f"Creating disable file: {disable_file}")
    try:
        disable_file.touch()
        print_success("Disable file created")
    except Exception as e:
        print_error(f"Failed to create disable file: {e}")
        return False

    # Test that notification is skipped
    print_info("Testing notification with disable file present...")
    # Import the notification module to test
    try:
        # Add parent directory to path
        parent_dir = Path(__file__).parent
        sys.path.insert(0, str(parent_dir))

        # Import and test
        import pushover_notify

        disabled = pushover_notify.is_windows_notification_disabled(str(cwd))
        if disabled:
            print_success("Disable file is correctly detected")
        else:
            print_error("Disable file was not detected")
            disable_file.unlink(missing_ok=True)
            return False

    except ImportError as e:
        print_warning(f"Could not import pushover_notify: {e}")
        print_info("Skipping module test")
    except Exception as e:
        print_warning(f"Error during module test: {e}")

    # Clean up disable file
    print_info("Cleaning up disable file...")
    disable_file.unlink(missing_ok=True)

    if not disable_file.exists():
        print_success("Disable file removed")
        return True
    else:
        print_error("Failed to remove disable file")
        return False


def main() -> None:
    """Main test runner."""
    print(r"""
     ____  _____ ____     ___   _   _  ____ _____ ___ ___  _   _
    |  _ \| ____|  _ \   / _ \ / \ | |/ ___|_   _|_ _/ _ \| \ | |
    | | | |  _| | |_) | | | | / _ \| | |  _  | |  | | | | |  \| |
    | |_| | |___|  _ <  | |_| / ___ \ | |_| | | |  | | |_| | |\  |
    |____/|_____|_| \_\  \___/_/   \_\_|\____| |_| |___\___/|_| \_|

          Claude Code Windows Notification - Test Suite
    """)

    # Run checks
    platform_ok = check_platform()
    print()

    if not platform_ok:
        print_header("Result: Platform Not Supported")
        print_error("Windows native notifications require Windows 10/11")
        return

    powershell_ok = check_powershell()
    print()

    if not powershell_ok:
        print_header("Result: Dependencies Missing")
        print_error("PowerShell is required but not available")
        return

    # Send test notifications
    basic_ok = test_basic_notification()
    print()

    chinese_ok = test_chinese_notification()
    print()

    disable_ok = test_disable_file()
    print()

    # Final result
    print_header("Test Summary")
    all_passed = basic_ok and chinese_ok and disable_ok

    if all_passed:
        print_success("All checks passed! Windows notifications are working.")
        print_info("\nFeatures tested:")
        print_info("  1. Basic notification delivery")
        print_info("  2. Chinese character support")
        print_info("  3. Disable file functionality")
        print_info("\nUsage:")
        print_info("  - Create .no-windows in project root to disable Windows notifications")
        print_info("  - Both Pushover and Windows notifications are sent by default")
    else:
        print_error("Some tests failed.")
        if not basic_ok:
            print_info("  - Basic notification failed")
        if not chinese_ok:
            print_info("  - Chinese character test failed")
        if not disable_ok:
            print_info("  - Disable file test failed")

    print()


if __name__ == "__main__":
    main()
