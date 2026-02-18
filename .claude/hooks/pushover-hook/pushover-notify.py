#!/usr/bin/env python3
"""
Pushover notification hook for Claude Code.

Sends notifications when:
- Task completes (Stop hook)
- Attention needed (Notification hook for permission/idle prompts)
"""

import json
import os
import subprocess
import sys
import urllib.request
import urllib.parse
from datetime import datetime, timedelta
import re
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor


# Setup logging
# Control debug logging with PUSHOVER_DEBUG env var (default: errors only)
DEBUG_MODE = os.environ.get("PUSHOVER_DEBUG", "").lower() in ("1", "true", "yes", "on")


# ORIGINAL VERSION (before PID isolation):
# def get_log_path() -> Path:
#     """Get the debug log file path with daily rotation."""
#     script_dir = Path(__file__).parent
#     today = datetime.now().strftime("%Y-%m-%d")
#     return script_dir / f"debug.{today}.log"

def get_log_path() -> Path:
    """
    Get the debug log file path with daily rotation and per-instance isolation.

    Each Claude Code instance (identified by PID) gets its own log file
    to prevent concurrent write conflicts in multi-instance scenarios.

    Returns:
        Path object for the log file: debug.YYYY-MM-DD-pid-{pid}.log
    """
    script_dir = Path(__file__).parent
    today = datetime.now().strftime("%Y-%m-%d")
    pid = os.getpid()
    return script_dir / f"debug.{today}-pid-{pid}.log"


def log(message: str, level: str = "info") -> None:
    """Write a message to the debug log with timestamp.

    Args:
        message: Message to log
        level: Log level - 'error', 'warn', or 'info' (default)
    """
    # Only log errors and warnings in production, unless DEBUG_MODE is enabled
    if level == "info" and not DEBUG_MODE:
        return

    try:
        log_path = get_log_path()
        timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        with open(log_path, "a", encoding="utf-8") as f:
            f.write(f"[{timestamp}] [{level.upper()}] {message}\n")
    except Exception:
        pass


def cleanup_old_logs(log_dir: Path, keep_days: int = 5) -> None:
    """
    Clean up old log files older than keep_days.

    Processes files matching both formats:
    - debug.YYYY-MM-DD.log (legacy)
    - debug.YYYY-MM-DD-pid-PID.log (multi-instance)
    Keeps today's logs and up to keep_days of historical logs.

    Args:
        log_dir: Directory containing log files
        keep_days: Number of days to keep logs (default: 5)
    """
    if not log_dir.exists():
        return

    try:
        today = datetime.now().date()
        cutoff_date = today - timedelta(days=keep_days)
        # Match both legacy and new PID-based formats
        log_pattern = re.compile(r'debug\.(\d{4}-\d{2}-\d{2})(?:-pid-\d+)?\.log')

        for log_file in log_dir.glob("debug*.log"):
            # Extract date from filename
            match = log_pattern.match(log_file.name)
            if not match:
                continue

            try:
                file_date = datetime.strptime(match.group(1), "%Y-%m-%d").date()
                if file_date < cutoff_date:
                    log_file.unlink(missing_ok=True)
                    log(f"Cleaned up old log: {log_file.name}")
            except ValueError:
                # Invalid date format, skip
                pass
            except Exception as e:
                # Log error but continue processing
                log(f"Error cleaning log file {log_file.name}: {e}")
    except Exception:
        # Silently fail - cleanup should never break the hook
        pass


def cleanup_expired_cache(cache_dir: Path, keep_days: int = 7) -> None:
    """
    Clean up cache files older than keep_days days.

    Args:
        cache_dir: Directory containing session cache files
        keep_days: Number of days to keep cache files (default: 7)

    Cleans:
        - session-*-pid-*.jsonl files older than keep_days
        - Uses file modification time (st_mtime) for age detection

    Note:
        Silently fails on errors to avoid breaking the hook
    """
    if not cache_dir.exists():
        log("Cache directory does not exist, skipping cleanup")
        return

    try:
        cutoff_time = datetime.now() - timedelta(days=keep_days)
        cleaned_count = 0

        # Find all cache files matching the new naming pattern
        for cache_file in cache_dir.glob("session-*-pid-*.jsonl"):
            try:
                # Check file modification time
                file_mtime = datetime.fromtimestamp(cache_file.stat().st_mtime)

                if file_mtime < cutoff_time:
                    cache_file.unlink(missing_ok=True)
                    log(f"Cleaned expired cache: {cache_file.name}")
                    cleaned_count += 1

            except Exception as e:
                log(f"Error cleaning cache file {cache_file.name}: {e}", level="error")

        if cleaned_count > 0:
            log(f"Cache cleanup completed: {cleaned_count} expired file(s) removed")
        else:
            log("No expired cache files to clean")

    except Exception as e:
        # Silently fail - cleanup should never break the hook
        log(f"Error during cache cleanup: {e}", level="error")


def is_notification_disabled(cwd: str) -> bool:
    """
    Check if notifications are disabled for the current project.

    Args:
        cwd: Current working directory (project root)

    Returns:
        True if .no-pushover file exists, False otherwise
    """
    silent_file = Path(cwd) / ".no-pushover"
    disabled = silent_file.exists()
    if disabled:
        log(f"Notifications disabled: {silent_file} exists")
    return disabled


def is_windows_notification_disabled(cwd: str) -> bool:
    """
    Check if Windows notifications are disabled for the current project.

    Args:
        cwd: Current working directory (project root)

    Returns:
        True if .no-windows file exists, False otherwise
    """
    silent_file = Path(cwd) / ".no-windows"
    disabled = silent_file.exists()
    if disabled:
        log(f"Windows notifications disabled: {silent_file} exists")
    return disabled


def send_windows_notification(title: str, message: str) -> bool:
    """
    Send a Windows 10/11 notification using PowerShell.

    Uses BurntToast module if available, falls back to Windows.UI.Notifications.
    Tries multiple methods for maximum compatibility.

    Args:
        title: Notification title
        message: Notification message body

    Returns:
        True if successful, False otherwise
    """
    log(f"send_windows_notification called: title='{title}'")

    # Convert literal \n to actual newlines
    message = message.replace("\\n", "\n")

    # Escape for PowerShell
    title_escaped = title.replace("'", "''").replace('"', '""')
    message_escaped = message.replace("'", "''").replace('"', '""').replace("`", "``")

    # Method 1: Try BurntToast module (most reliable)
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

    # Method 3: Use .NET ShellNotifyW (Windows classic balloon)
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
                log(f"Windows notification sent successfully using {method_name}")
                return True
            else:
                log(f"Method '{method_name}' failed, trying next...")
                continue

        except subprocess.TimeoutExpired:
            log(f"WARNING: {method_name} timed out, trying next...", level="warn")
            continue
        except Exception as e:
            log(f"WARNING: {method_name} error: {e}, trying next...", level="warn")
            continue

    log("WARNING: All Windows notification methods failed", level="warn")
    return False


def _send_pushover_internal(title: str, message: str, priority: int = 0, cwd: str = "") -> bool:
    """
    Internal: Send a notification via Pushover API using urllib.

    Args:
        title: Notification title
        message: Notification message body
        priority: Message priority (-2 to 2, default 0)
        cwd: Current working directory to check for .no-pushover file

    Returns:
        True if successful, False otherwise
    """
    # Check if notifications are disabled for this project
    if cwd and is_notification_disabled(cwd):
        log(f"Notification skipped due to .no-pushover file: {title}")
        return False

    log(f"send_pushover called: title='{title}', priority={priority}")

    token = os.environ.get("PUSHOVER_TOKEN")
    user = os.environ.get("PUSHOVER_USER")

    if not token or not user:
        log(f"ERROR: Missing env vars - TOKEN={bool(token)}, USER={bool(user)}", level="error")
        return False

    log(f"Environment variables found - TOKEN: {token[:10]}..., USER: {user[:10]}...")

    try:
        # Convert literal \n to actual newlines for the message
        message = message.replace("\\\\n", "\\n")

        # Build form data
        data = urllib.parse.urlencode({
            "token": token,
            "user": user,
            "title": title,
            "message": message,
            "priority": priority
        }).encode("utf-8")

        log(f"Sending POST request to Pushover API...")

        # Create request with proper headers
        request = urllib.request.Request(
            "https://api.pushover.net/1/messages.json",
            data=data,
            method="POST"
        )
        request.add_header("Content-Type", "application/x-www-form-urlencoded")
        request.add_header("User-Agent", "ClaudeCode-PushoverHook/1.0")

        # Send request with timeout
        with urllib.request.urlopen(request, timeout=10) as response:
            http_code = response.status
            response_body = response.read().decode("utf-8")

        log(f"HTTP Status Code: {http_code}")
        log(f"API Response: {response_body}")

        # Parse JSON response
        response_json = json.loads(response_body)

        if response_json.get("status") == 1:
            log(f"Request successful - ID: {response_json.get('request', 'N/A')}")
            return True
        else:
            log("ERROR: API returned status != 1", level="error")
            if "errors" in response_json:
                for error in response_json["errors"]:
                    log(f"API Error: {error}", level="error")
            return False

    except urllib.error.HTTPError as e:
        log(f"ERROR: HTTP {e.code} - {e.reason}", level="error")
        try:
            error_body = e.read().decode("utf-8")
            log(f"Error response: {error_body}")
        except Exception:
            pass
        return False
    except urllib.error.URLError as e:
        log(f"ERROR: URL error - {e.reason}", level="error")
        if isinstance(e.reason, Exception):
            log(f"Reason details: {e.reason}", level="error")
        return False
    except TimeoutError:
        log("ERROR: Request timed out", level="error")
        return False
    except json.JSONDecodeError as e:
        log(f"ERROR: Could not parse response as JSON: {e}", level="error")
        return False
    except Exception as e:
        log(f"ERROR: Exception in send_pushover: {e}", level="error")
        return False


def send_notifications(title: str, message: str, priority: int = 0, cwd: str = "") -> dict:
    """
    Send notifications via enabled channels in parallel.

    Windows local notifications display immediately without waiting for Pushover API.

    Args:
        title: Notification title
        message: Notification message body
        priority: Message priority (for Pushover, -2 to 2, default 0)
        cwd: Current working directory to check for disable files

    Returns:
        Dict with status of each channel: {"pushover": bool, "windows": bool}
    """
    results = {"pushover": False, "windows": False}

    # Check if both are disabled
    pushover_disabled = cwd and is_notification_disabled(cwd)
    windows_disabled = cwd and is_windows_notification_disabled(cwd)

    if pushover_disabled and windows_disabled:
        log("All notifications disabled (.no-pushover and .no-windows both exist)")
        return results

    futures = {}

    with ThreadPoolExecutor(max_workers=2) as executor:
        if not pushover_disabled:
            log("Starting Pushover notification thread")
            futures["pushover"] = executor.submit(_send_pushover_internal, title, message, priority, cwd)

        if not windows_disabled and sys.platform == "win32":
            log("Starting Windows notification thread")
            futures["windows"] = executor.submit(send_windows_notification, title, message)
        elif not windows_disabled and sys.platform != "win32":
            log("Windows native notification not supported on this platform")

        for name, future in futures.items():
            try:
                results[name] = future.result(timeout=10)
                log(f"{name.capitalize()} notification thread completed: {results[name]}")
            except Exception as e:
                log(f"ERROR: {name} notification thread failed: {e}", level="error")
                results[name] = False

    return results


def get_project_name(cwd: str) -> str:
    """
    Extract project name from working directory path.

    Args:
        cwd: Current working directory

    Returns:
        Project name or fallback string
    """
    try:
        name = os.path.basename(os.path.normpath(cwd))
        log(f"Extracted project name: {name} from {cwd}")
        return name
    except Exception as e:
        log(f"ERROR getting project name: {e}")
        return "Unknown Project"


def summarize_conversation(session_id: str, cwd: str) -> str:
    """
    Generate a summary of the conversation using Claude CLI.

    Args:
        session_id: The session identifier
        cwd: Current working directory

    Returns:
        Summary string or fallback message
    """
    log(f"summarize_conversation called for session {session_id}")

    cache_dir = Path(cwd) / ".claude" / "cache"
    pid = os.getpid()
    cache_file = cache_dir / f"session-{session_id}-pid-{pid}.jsonl"
    log(f"Cache file for PID {pid}: {cache_file}")

    # Fallback: extract last user message
    fallback_summary = "Task completed"

    if not cache_file.exists():
        log(f"Cache file not found: {cache_file}")
        return fallback_summary

    try:
        lines = cache_file.read_text(encoding="utf-8").strip().split("\n")
        log(f"Cache file has {len(lines)} lines")

        if not lines or lines == [""]:
            log("Cache file is empty")
            return fallback_summary

        # Get last user message as fallback
        for line in reversed(lines):
            try:
                data = json.loads(line)
                if data.get("type") == "user_prompt_submit":
                    content = data.get("prompt", "")
                    if content:
                        # Truncate to reasonable length
                        fallback_summary = (
                            content[:100] + "..." if len(content) > 100 else content
                        )
                        log(f"Using fallback summary from user message")
                        break
            except json.JSONDecodeError:
                continue

        # Try to use Claude CLI for summarization
        try:
            conversation_text = "\n".join(lines)
            prompt = f"""Summarize this conversation in one concise sentence (max 15 words):

{conversation_text}

Summary:"""

            log("Attempting Claude CLI summarization...")
            result = subprocess.run(
                ["claude", "-p", prompt],
                capture_output=True,
                text=True,
                timeout=30,
                cwd=cwd,
            )

            if result.returncode == 0 and result.stdout.strip():
                summary = result.stdout.strip()
                if len(summary) < 200:
                    log(f"Claude CLI summary: {summary}")
                    return summary
                else:
                    log(f"Claude CLI summary too long ({len(summary)} chars), using fallback")

            log(f"Claude CLI failed - return code: {result.returncode}")

        except subprocess.TimeoutExpired:
            log("Claude CLI timed out")
        except FileNotFoundError:
            log("Claude CLI not found")
        except Exception as e:
            log(f"Claude CLI exception: {e}")

        return fallback_summary

    except Exception as e:
        log(f"ERROR in summarize_conversation: {e}")
        return fallback_summary


def main() -> None:
    """Main hook handler."""
    log("=" * 60)

    # Force UTF-8 encoding for stdin on all platforms (Windows encoding fix)
    if hasattr(sys.stdin, 'reconfigure'):
        sys.stdin.reconfigure(encoding='utf-8')
        log(f"Stdin encoding configured: {sys.stdin.encoding}")
    else:
        log("WARNING: stdin.reconfigure not available (Python < 3.7)", level="warn")

    log(f"Hook script started - Event: Processing")

    # Clean up old log files (keep last 5 days)
    cleanup_old_logs(get_log_path().parent, keep_days=5)

    # Read hook event from stdin
    try:
        stdin_data = sys.stdin.read()
        log(f"Stdin read successfully, length: {len(stdin_data)}")
    except Exception as e:
        log(f"ERROR reading stdin: {e}")
        return

    if not stdin_data:
        log("ERROR: stdin is empty", level="error")
        return

    log(f"Stdin content: {stdin_data[:200]}...")

    # Fix Windows paths in JSON (backslashes need to be escaped)
    stdin_data = stdin_data.replace("\\", "\\\\")

    try:
        hook_input = json.loads(stdin_data)
        log(f"JSON parsed successfully")
    except json.JSONDecodeError as e:
        log(f"ERROR: JSON decode failed: {e}", level="error")
        return

    hook_event = hook_input.get("hook_event_name", "")
    session_id = hook_input.get("session_id", "")
    cwd = hook_input.get("cwd", os.getcwd())

    log(f"Event: {hook_event}, Session: {session_id}, CWD: {cwd}")

    if not session_id:
        log("ERROR: No session_id in input", level="error")
        return

    if hook_event == "UserPromptSubmit":
        log("Processing UserPromptSubmit event")
        # Record user input to cache with PID isolation
        cache_dir = Path(cwd) / ".claude" / "cache"
        cache_dir.mkdir(parents=True, exist_ok=True)

        pid = os.getpid()
        cache_file = cache_dir / f"session-{session_id}-pid-{pid}.jsonl"

        try:
            entry = {
                "type": "user_prompt_submit",
                "prompt": hook_input.get("prompt", ""),
                "timestamp": hook_input.get("timestamp", ""),
                "pid": pid,
            }

            with open(cache_file, "a", encoding="utf-8") as f:
                f.write(json.dumps(entry) + "\n")
            log(f"User prompt cached to {cache_file}")
        except (OSError, IOError) as e:
            log(f"ERROR caching user prompt: {e}")

    elif hook_event == "Stop":
        log("Processing Stop event")

        # Get cache file path for this session
        pid = os.getpid()
        cache_file = Path(cwd) / ".claude" / "cache" / f"session-{session_id}-pid-{pid}.jsonl"

        # Handle edge case: Stop before UserPromptSubmit
        if not cache_file.exists():
            log(f"No cache file found for session {session_id} (PID {pid})")
            summary = "Task completed (no user messages recorded)"
        else:
            summary = summarize_conversation(session_id, cwd)

        # Send task completion notification
        project_name = get_project_name(cwd)
        title = f"[{project_name}] Task Complete"
        message = f"Session: {session_id}\\nSummary: {summary}"

        log(f"Sending notification: {title}")
        results = send_notifications(title, message, priority=0, cwd=cwd)
        log(f"Notification results: Pushover={results['pushover']}, Windows={results['windows']}")

        log(f"Message stats: chars={len(message)}, bytes={len(message.encode('utf-8'))}")

        # Mark session as completed instead of deleting
        try:
            completed_entry = {
                "type": "session_complete",
                "timestamp": datetime.utcnow().isoformat(),
                "pid": pid
            }
            with open(cache_file, "a", encoding="utf-8") as f:
                f.write(json.dumps(completed_entry) + "\n")
            log(f"Session marked as completed: {cache_file}")
        except (OSError, IOError) as e:
            log(f"WARNING: Failed to mark session as completed: {e}", level="warn")

        # Clean up expired cache (7 days)
        cache_dir = Path(cwd) / ".claude" / "cache"
        cleanup_expired_cache(cache_dir, keep_days=7)

    elif hook_event == "Notification":
        log("Processing Notification event")
        # Log full input for debugging
        log(f"Full Notification input: {json.dumps(hook_input, ensure_ascii=False)}")
        # Get notification type (correct field name from docs)
        notification_type = hook_input.get("notification_type", "notification")
        log(f"Notification type: {notification_type}")

        # Skip idle_prompt notifications (CLI idle for 60+ seconds)
        if notification_type == "idle_prompt":
            log("Skipping idle_prompt notification - not sending pushover")
            return

        # Get notification message (correct field name from docs)
        notification_message = hook_input.get("message", "")

        project_name = get_project_name(cwd)

        title = f"[{project_name}] Attention Needed"

        # Build message from notification
        details = notification_message if notification_message else "No additional details provided"

        message = f"Session: {session_id}\\nType: {notification_type}\\n{details}"

        log(f"Sending attention notification: {title}")
        # Higher priority for attention needed
        results = send_notifications(title, message, priority=1, cwd=cwd)
        log(f"Notification results: Pushover={results['pushover']}, Windows={results['windows']}")

        log(f"Message stats: chars={len(message)}, bytes={len(message.encode('utf-8'))}")
    else:
        log(f"WARNING: Unknown hook event type: {hook_event}", level="warn")

    log(f"Hook script completed")
    log("=" * 60)


if __name__ == "__main__":
    main()
