#!/usr/bin/env python3
"""
Test script for Pushover notification configuration.

Run this script to verify:
1. Environment variables are set
2. Pushover API is accessible
3. Notifications can be sent successfully

Usage:
    python test-pushover.py
"""

import json
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
    """Print success message in green."""
    print(f"[OK] {text}")


def print_error(text: str) -> None:
    """Print error message in red."""
    print(f"[ERROR] {text}")


def print_warning(text: str) -> None:
    """Print warning message in yellow."""
    print(f"[WARN] {text}")


def print_info(text: str) -> None:
    """Print info message."""
    print(f"[INFO] {text}")


def check_environment() -> dict:
    """Check if required environment variables are set."""
    print_header("Step 1: Checking Environment Variables")

    token = os.environ.get("PUSHOVER_TOKEN")
    user = os.environ.get("PUSHOVER_USER")

    results = {
        "token_valid": bool(token),
        "user_valid": bool(user),
        "token": token,
        "user": user
    }

    if token:
        # Show first 8 characters only
        masked = token[:8] + "..." if len(token) > 8 else token
        print_success(f"PUSHOVER_TOKEN is set: {masked}")
        print_info(f"Token length: {len(token)} characters")
    else:
        print_error("PUSHOVER_TOKEN is not set")

    if user:
        masked = user[:8] + "..." if len(user) > 8 else user
        print_success(f"PUSHOVER_USER is set: {masked}")
        print_info(f"User key length: {len(user)} characters")
    else:
        print_error("PUSHOVER_USER is not set")

    return results


def check_curl() -> bool:
    """Check if curl is available."""
    print_header("Step 2: Checking curl Availability")

    try:
        result = subprocess.run(
            ["curl", "--version"],
            capture_output=True,
            text=True,
            timeout=5
        )
        if result.returncode == 0:
            # Extract version line
            version_line = result.stdout.split('\n')[0]
            print_success(f"curl is available: {version_line}")
            return True
        else:
            print_error("curl command failed")
            return False
    except FileNotFoundError:
        print_error("curl is not installed or not in PATH")
        return False
    except Exception as e:
        print_error(f"Error checking curl: {e}")
        return False


def send_test_notification(token: str, user: str) -> bool:
    """Send a test notification to Pushover."""
    print_header("Step 3: Sending Test Notification")

    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    title = "Pushover Test"
    message = f"Test notification from Claude Code hook\\nTime: {timestamp}"

    print_info(f"Title: {title}")
    print_info(f"Message: {message}")

    # Windows-compatible output file
    null_path = "NUL" if sys.platform == "win32" else "/dev/null"

    # Create a temp file to capture response body
    response_file = Path("pushover_response.txt")

    try:
        cmd = [
            "curl",
            "-s",
            "-o", str(response_file),
            "-w", "%{http_code}",
            "https://api.pushover.net/1/messages.json",
            "--data-urlencode", f"token={token}",
            "--data-urlencode", f"user={user}",
            "--data-urlencode", f"title={title}",
            "--data-urlencode", f"message={message}",
            "-d", "priority=0",
        ]

        print_info("Executing curl request...")
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)

        http_code = result.stdout.strip()
        print_info(f"HTTP Status Code: {http_code}")

        # Read response body
        if response_file.exists():
            response_body = response_file.read_text(encoding="utf-8")
            response_file.unlink()  # Clean up
            print_info(f"Response body: {response_body}")

            # Parse JSON for detailed error
            try:
                response_json = json.loads(response_body)
                if response_json.get("status") == 1:
                    print_success("Notification sent successfully!")
                    print_info(f"Request ID: {response_json.get('request', 'N/A')}")
                    return True
                else:
                    print_error("Notification failed")
                    if "errors" in response_json:
                        for error in response_json["errors"]:
                            print_error(f"API Error: {error}")
                    return False
            except json.JSONDecodeError:
                print_warning("Could not parse response as JSON")
        else:
            print_warning("No response body captured")

        # Fallback: check HTTP code
        if http_code == "200":
            print_success("Notification sent (HTTP 200)")
            return True
        elif http_code == "400":
            print_error("Bad Request (HTTP 400) - Check token and user key")
            return False
        else:
            print_error(f"Unexpected HTTP code: {http_code}")
            return False

    except subprocess.TimeoutExpired:
        print_error("Request timed out")
        response_file.unlink(missing_ok=True)
        return False
    except Exception as e:
        print_error(f"Exception: {e}")
        response_file.unlink(missing_ok=True)
        return False


def test_chinese_encoding(token: str, user: str) -> bool:
    """Test Chinese character encoding in notifications."""
    print_header("Step 3b: Testing Chinese Character Encoding")

    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    title = "编码测试 (Encoding Test)"
    message = f"中文测试：你来测试最新开发的 rustdesk 服务器协议的扫描功能\\nTime: {timestamp}\\nIP: 116.62.8.4"

    print_info(f"Title: {title}")
    print_info(f"Message: {message}")
    print_info(f"Expected: Chinese characters should display correctly")

    response_file = Path("pushover_response_chinese.txt")

    try:
        cmd = [
            "curl",
            "-s",
            "-o", str(response_file),
            "-w", "%{http_code}",
            "https://api.pushover.net/1/messages.json",
            "--data-urlencode", f"token={token}",
            "--data-urlencode", f"user={user}",
            "--data-urlencode", f"title={title}",
            "--data-urlencode", f"message={message}",
            "-d", "priority=0",
        ]

        print_info("Sending test notification with Chinese characters...")
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)

        http_code = result.stdout.strip()
        print_info(f"HTTP Status Code: {http_code}")

        if response_file.exists():
            response_body = response_file.read_text(encoding="utf-8")
            response_file.unlink()
            print_info(f"Response body: {response_body}")

            try:
                response_json = json.loads(response_body)
                if response_json.get("status") == 1:
                    print_success("Chinese encoding test passed!")
                    print_info("Please verify on your device that characters display correctly")
                    return True
                else:
                    print_error("Notification failed")
                    if "errors" in response_json:
                        for error in response_json["errors"]:
                            print_error(f"API Error: {error}")
                    return False
            except json.JSONDecodeError:
                print_warning("Could not parse response as JSON")
        else:
            print_warning("No response body captured")

        if http_code == "200":
            print_success("Notification sent (HTTP 200)")
            return True
        else:
            print_error(f"Unexpected HTTP code: {http_code}")
            return False

    except subprocess.TimeoutExpired:
        print_error("Request timed out")
        response_file.unlink(missing_ok=True)
        return False
    except Exception as e:
        print_error(f"Exception: {e}")
        response_file.unlink(missing_ok=True)
        return False


def test_empty_notification_body(token: str, user: str) -> bool:
    """Test empty notification body handling."""
    print_header("Step 3c: Testing Empty Notification Body")

    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    title = "Empty Body Test"
    message = f"Session: test-session-{timestamp}\\nType: test_notification\\nNo additional details provided"

    print_info(f"Title: {title}")
    print_info(f"Message: {message}")
    print_info("Expected: Should NOT show literal '{}'")

    response_file = Path("pushover_response_empty.txt")

    try:
        cmd = [
            "curl",
            "-s",
            "-o", str(response_file),
            "-w", "%{http_code}",
            "https://api.pushover.net/1/messages.json",
            "--data-urlencode", f"token={token}",
            "--data-urlencode", f"user={user}",
            "--data-urlencode", f"title={title}",
            "--data-urlencode", f"message={message}",
            "-d", "priority=0",
        ]

        print_info("Sending test notification simulating empty body...")
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)

        http_code = result.stdout.strip()
        print_info(f"HTTP Status Code: {http_code}")

        if response_file.exists():
            response_body = response_file.read_text(encoding="utf-8")
            response_file.unlink()

            try:
                response_json = json.loads(response_body)
                if response_json.get("status") == 1:
                    print_success("Empty body test passed!")
                    return True
                else:
                    print_error("Notification failed")
                    if "errors" in response_json:
                        for error in response_json["errors"]:
                            print_error(f"API Error: {error}")
                    return False
            except json.JSONDecodeError:
                print_warning("Could not parse response as JSON")
        else:
            print_warning("No response body captured")

        if http_code == "200":
            print_success("Notification sent (HTTP 200)")
            return True
        else:
            print_error(f"Unexpected HTTP code: {http_code}")
            return False

    except subprocess.TimeoutExpired:
        print_error("Request timed out")
        response_file.unlink(missing_ok=True)
        return False
    except Exception as e:
        print_error(f"Exception: {e}")
        response_file.unlink(missing_ok=True)
        return False


def show_hook_config() -> None:
    """Show current hook configuration."""
    print_header("Step 4: Hook Configuration")

    settings_path = Path(__file__).parent.parent / "settings.json"
    if settings_path.exists():
        print_success(f"Found settings.json: {settings_path}")
        try:
            content = settings_path.read_text(encoding="utf-8")
            settings = json.loads(content)
            hooks = settings.get("hooks", {})

            print_info("\nConfigured hooks:")
            for event_name in ["Stop", "Notification", "UserPromptSubmit"]:
                if event_name in hooks:
                    hook_list = hooks[event_name]
                    if hook_list and "hooks" in hook_list[0]:
                        for hook in hook_list[0]["hooks"]:
                            if hook.get("type") == "command":
                                cmd = hook.get("command", "")
                                print_info(f"  - {event_name}: {cmd}")

        except Exception as e:
            print_warning(f"Could not read settings.json: {e}")
    else:
        print_warning(f"settings.json not found at {settings_path}")


def show_debug_log() -> None:
    """Show recent debug log entries."""
    print_header("Step 5: Recent Debug Log")

    log_path = Path(__file__).parent / "debug.log"
    if log_path.exists():
        print_success(f"Found debug.log: {log_path}")
        try:
            content = log_path.read_text(encoding="utf-8")
            lines = content.strip().split("\n")
            # Show last 20 lines
            recent_lines = lines[-20:] if len(lines) > 20 else lines
            print_info(f"\nShowing last {len(recent_lines)} lines:\n")
            for line in recent_lines:
                print(f"  {line}")
        except Exception as e:
            print_warning(f"Could not read debug.log: {e}")
    else:
        print_warning("debug.log not found (no hooks triggered yet)")


def main() -> None:
    """Main test runner."""
    print(r"""
     ____  _____ ____     ___   _   _  ____ _____ ___ ___  _   _
    |  _ \| ____|  _ \   / _ \ / \ | |/ ___|_   _|_ _/ _ \| \ | |
    | | | |  _| | |_) | | | | / _ \| | |  _  | |  | | | | |  \| |
    | |_| | |___|  _ <  | |_| / ___ \ | |_| | | |  | | |_| | |\  |
    |____/|_____|_| \_\  \___/_/   \_\_|\____| |_| |___\___/|_| \_|

          Claude Code Pushover Hook - Configuration Test
    """)

    # Run checks
    env_results = check_environment()
    print()

    curl_ok = check_curl()
    print()

    if not all([env_results["token_valid"], env_results["user_valid"]]):
        print_header("Result: Configuration Incomplete")
        print_error("Required environment variables are missing.")
        print("\nTo fix, set the following environment variables:")
        print("  set PUSHOVER_TOKEN=your_app_token")
        print("  set PUSHOVER_USER=your_user_key")
        print("\nThen run this test again.")
        return

    if not curl_ok:
        print_header("Result: Dependencies Missing")
        print_error("curl is required but not available.")
        return

    # Send test notification
    notification_ok = send_test_notification(env_results["token"], env_results["user"])
    print()

    # Additional encoding tests
    if notification_ok:
        chinese_ok = test_chinese_encoding(env_results["token"], env_results["user"])
        print()
        empty_ok = test_empty_notification_body(env_results["token"], env_results["user"])
        print()

        # Update final result
        notification_ok = chinese_ok and empty_ok

    # Show configuration
    show_hook_config()
    print()

    show_debug_log()
    print()

    # Final result
    print_header("Test Summary")
    if notification_ok:
        print_success("All checks passed! Pushover notifications are working.")
        print_info("\nNext steps:")
        print_info("  1. Copy .claude folder to your project")
        print_info("  2. Ensure PUSHOVER_TOKEN and PUSHOVER_USER are set in your environment")
        print_info("  3. Trigger a Claude Code task and wait for Stop event")
        print_info("  4. Check debug.log for details")
    else:
        print_error("Test notification failed.")
        print_info("\nTroubleshooting:")
        print_info("  1. Verify your token at: https://pushover.net/apps")
        print_info("  2. Verify your user key at: https://pushover.net/")
        print_info("  3. Check that the token has 'API' enabled (not just SDK)")
        print_info("  4. Ensure message content is not too long")

    print()


if __name__ == "__main__":
    main()
