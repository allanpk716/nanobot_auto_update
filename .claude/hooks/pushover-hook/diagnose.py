#!/usr/bin/env python3
"""
Quick diagnostic script to verify hook configuration.
Run this in the target project to check if hooks will work.
"""

import json
import os
import sys
from pathlib import Path


def print_section(title: str) -> None:
    print(f"\n{'='*60}")
    print(f" {title}")
    print('='*60)


def check_env_vars() -> bool:
    print_section("1. Environment Variables")

    token = os.environ.get("PUSHOVER_TOKEN")
    user = os.environ.get("PUSHOVER_USER")

    if token:
        print(f"[OK] PUSHOVER_TOKEN: {token[:8]}... (length: {len(token)})")
    else:
        print("[MISSING] PUSHOVER_TOKEN - not set")

    if user:
        print(f"[OK] PUSHOVER_USER: {user[:8]}... (length: {len(user)})")
    else:
        print("[MISSING] PUSHOVER_USER - not set")

    return bool(token and user)


def check_python() -> bool:
    print_section("2. Python Availability")

    print(f"[OK] Python version: {sys.version}")
    print(f"[OK] Python executable: {sys.executable}")
    return True


def check_hook_script() -> bool:
    print_section("3. Hook Script")

    # Find the hook script relative to this file
    script_dir = Path(__file__).parent
    hook_script = script_dir / "pushover-notify.py"

    if hook_script.exists():
        print(f"[OK] Hook script exists: {hook_script}")
        print(f"[OK] File size: {hook_script.stat().st_size} bytes")
        return True
    else:
        print(f"[MISSING] Hook script not found: {hook_script}")
        return False


def check_settings() -> bool:
    print_section("4. Settings Configuration")

    settings_path = Path(__file__).parent.parent.parent / "settings.json"

    if not settings_path.exists():
        print(f"[MISSING] settings.json not found at: {settings_path}")
        return False

    print(f"[OK] settings.json exists: {settings_path}")

    try:
        content = settings_path.read_text(encoding="utf-8")
        settings = json.loads(content)
        hooks = settings.get("hooks", {})

        print(f"\nConfigured events:")
        for event in ["Stop", "Notification", "UserPromptSubmit"]:
            if event in hooks:
                print(f"  [OK] {event}")
                hook_config = hooks[event]
                if hook_config and "hooks" in hook_config[0]:
                    for h in hook_config[0]["hooks"]:
                        if h.get("type") == "command":
                            cmd = h.get("command", "")
                            print(f"       Command: {cmd}")
            else:
                print(f"  [MISSING] {event}")

        return True
    except Exception as e:
        print(f"[ERROR] Failed to read settings.json: {e}")
        return False


def check_cache_dir() -> bool:
    print_section("5. Cache Directory")

    cwd = os.getcwd()
    cache_dir = Path(cwd) / ".claude" / "cache"

    if cache_dir.exists():
        print(f"[OK] Cache directory exists: {cache_dir}")
    else:
        print(f"[INFO] Cache directory not found (will be created): {cache_dir}")

    return True


def show_current_config() -> None:
    print_section("6. Current Project Info")

    print(f"Current directory: {os.getcwd()}")
    print(f"Python executable: {sys.executable}")
    print(f"Platform: {sys.platform}")

    # Try to detect if we're in a Claude Code session
    session_id = os.environ.get("CLAUDE_SESSION_ID")
    if session_id:
        print(f"[OK] CLAUDE_SESSION_ID: {session_id}")
    else:
        print("[INFO] Not currently in a Claude Code session")


def check_notification_channels() -> bool:
    """Check available notification channels."""
    print_section("7. Notification Channels")

    channels = []

    # Check Pushover
    token = os.environ.get("PUSHOVER_TOKEN")
    user = os.environ.get("PUSHOVER_USER")
    if token and user:
        print("  [OK] Pushover (enabled)")
        channels.append("pushover")
    else:
        print("  [DISABLED] Pushover (missing env vars)")

    # Check Windows
    if sys.platform == "win32":
        print("  [OK] Windows Native (available)")
        channels.append("windows")
    else:
        print(f"  [N/A] Windows Native (not supported on {sys.platform})")

    # Check disable files
    cwd = os.getcwd()
    no_pushover = Path(cwd) / ".no-pushover"
    no_windows = Path(cwd) / ".no-windows"

    if no_pushover.exists():
        print("  [INFO] Pushover disabled by .no-pushover file")
    if no_windows.exists():
        print("  [INFO] Windows disabled by .no-windows file")

    if no_pushover.exists() and no_windows.exists():
        print("  [WARN] All notifications disabled!")

    return len(channels) > 0


def main() -> None:
    print("""
    ╔═══════════════════════════════════════════════════════════╗
    ║       Claude Code Notification Hook - Diagnostic Check    ║
    ╚═══════════════════════════════════════════════════════════╝
    """)

    checks = [
        ("Environment Variables", check_env_vars),
        ("Python Availability", check_python),
        ("Hook Script", check_hook_script),
        ("Settings Configuration", check_settings),
        ("Cache Directory", check_cache_dir),
        ("Notification Channels", check_notification_channels),
    ]

    results = {}
    for name, check_func in checks:
        try:
            results[name] = check_func()
        except Exception as e:
            print(f"\n[ERROR] Exception during {name}: {e}")
            results[name] = False

    show_current_config()

    print_section("Summary")
    all_ok = all(results.values())
    missing = [name for name, passed in results.items() if not passed]

    if all_ok:
        print("\n[SUCCESS] All checks passed!")
        print("\nNext steps:")
        print("  1. Trigger a Claude Code task")
        print("  2. Let it complete to trigger Stop event")
        print("  3. Check for notifications (Pushover and/or Windows)")
        print("  4. Check debug.log for details")
    else:
        print("\n[ISSUES FOUND] Some checks failed:")
        for name in missing:
            print(f"  - {name}")

        if "Environment Variables" in missing:
            print("\n[REQUIRED] Set environment variables:")
            print("  set PUSHOVER_TOKEN=your_app_token")
            print("  set PUSHOVER_USER=your_user_key")

    print()


if __name__ == "__main__":
    main()
