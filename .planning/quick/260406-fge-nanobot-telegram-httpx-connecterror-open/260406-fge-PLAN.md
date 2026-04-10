---
phase: quick-nanobot-telegram-connecterror
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - docs/bugs/telegram-httpx-connecterror-openclash.md
autonomous: true
requirements: [DIAG-01, FIX-01]
must_haves:
  truths:
    - "Root cause of httpx.ConnectError when nanobot connects to Telegram is identified and documented"
    - "Working solution is verified by successfully connecting to Telegram API"
    - "Nanobot Telegram channel configuration is updated with correct proxy settings"
  artifacts:
    - path: "docs/bugs/telegram-httpx-connecterror-openclash.md"
      provides: "Diagnosis and fix documentation"
      contains: "CRYPT_E_REVOCATION_OFFLINE"
  key_links:
    - from: "nanobot config.json"
      to: "Telegram API (api.telegram.org)"
      via: "HTTPXRequest with proxy parameter"
      pattern: "proxy.*http"
---

<objective>
Diagnose and fix the httpx.ConnectError when nanobot connects to Telegram under OpenClash proxy environment.

Purpose: The nanobot gateway fails to start its Telegram channel because httpx (underlying HTTP library used by python-telegram-bot) cannot connect to api.telegram.org. This blocks all Telegram bot functionality.

Output: Documented diagnosis + verified fix that allows nanobot to connect to Telegram.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md

## Pre-investigation Results

The following diagnosis has already been completed by the planner:

### Environment
- OS: Windows 10, gateway router: 192.168.100.1 (OpenClash)
- Nanobot: v0.1.4.post6 (PyInstaller binary, installed via uv tool)
- Nanobot source: C:/Users/allan716/AppData/Roaming/uv/tools/nanobot-ai/Lib/site-packages/nanobot/

### Root Cause
OpenClash transparent proxy performs TLS MITM on `api.telegram.org`. Windows schannel fails certificate revocation check with `CRYPT_E_REVOCATION_OFFLINE (0x80092013)`. This causes httpx.ConnectError in python-telegram-bot.

Evidence:
1. `curl https://api.telegram.org` -> TLS error (exit 35, CRYPT_E_REVOCATION_OFFLINE)
2. `curl -k https://api.telegram.org` -> succeeds (HTTP 404 = normal for invalid token)
3. `curl https://www.google.com` -> succeeds (transparent proxy handles Google correctly)
4. OpenClash Clash API on 192.168.100.1:9090 responds (returns "Unauthorized")

### Key Code: Telegram Channel Proxy Support
File: nanobot/channels/telegram.py (lines 169-175, 252-268)
- `TelegramConfig` already has `proxy: str | None = None` field
- `HTTPXRequest(proxy=proxy)` is used in both api_request and poll_request
- The proxy parameter is passed through to httpx

### Key Code: Nanobot Config Schema
File: nanobot/config/schema.py
- Config stored at: ~/.nanobot/config.json (or --config path)
- Channel config is under `channels.telegram` in config.json
- Also: `tools.web.proxy` exists for web tools (line 119-121)

### OpenClash Proxy Ports (from testing)
- Port 7890: HTTP proxy active but returns 407 (auth required)
- Port 7891: SOCKS5 active but rejects connection (auth required)
- Port 9090: Clash API (returns "Unauthorized")

### Solution Options
**Option A (Recommended): Fix OpenClash configuration** - Ensure api.telegram.org is proxied correctly without TLS MITM. This is a router-side fix.
**Option B: Configure nanobot proxy** - Set `proxy` in Telegram config to use OpenClash's HTTP/SOCKS5 proxy. Requires knowing auth credentials or configuring OpenClash to allow LAN without auth.
**Option C: System-level proxy** - Set HTTPS_PROXY env var for the nanobot process.

### Nanobot Config File Location
The nanobot-auto-updater config.yaml references these nanobot instances:
1. nanobot-me: port 18790, command "nanobot gateway"
2. nanobot-work-helper: port 18792, command "nanobot gateway --config C:/Users/allan716/.nanobot-work-helper/config.json --port 18792"

Neither config file currently exists. The default config location is ~/.nanobot/config.json.
</context>

<tasks>

<task type="checkpoint:decision" gate="blocking">
  <name>Task 1: Decide fix approach for Telegram connectivity</name>
  <files>docs/bugs/telegram-httpx-connecterror-openclash.md</files>
  <what-built>Diagnosis is complete. A decision is needed on the fix approach.</what-built>
  <decision>How to fix nanobot Telegram connectivity under OpenClash proxy?</decision>
  <context>
    Root cause confirmed: OpenClash transparent proxy TLS MITM breaks Telegram API connections.
    Certificate revocation check fails (CRYPT_E_REVOCATION_OFFLINE).

    Three approaches available:
  </context>
  <options>
    <option id="option-a">
      <name>Fix OpenClash router config (Recommended)</name>
      <pros>
        - Fixes the root cause at the network level
        - No changes needed to nanobot configuration
        - All applications benefit, not just nanobot
        - OpenClash can be configured to proxy Telegram traffic without MITM
      </cons>
        - Requires access to OpenClash admin panel (192.168.100.1)
        - Need to add api.telegram.org to proxy rules or bypass TLS inspection
      </recommendation>
        1. Login to OpenClash admin panel (usually http://192.168.100.1)
        2. Navigate to OpenClash -> Config Subscribe or Rule Settings
        3. Ensure "Telegram" domain (api.telegram.org, *.t.me) uses "Proxy" mode, NOT "MITM/TLS-intercept"
        4. Alternatively, add api.telegram.org to the bypass/direct TLS list
        5. Save and restart OpenClash service
    </option>
    <option id="option-b">
      <name>Configure nanobot proxy with OpenClash proxy port</name>
      <pros>
        - Application-level fix, controllable
        - nanobot already supports proxy config
      </cons>
        - OpenClash HTTP proxy (7890) requires authentication (407 error observed)
        - Need to find/configure LAN access without auth in OpenClash
        - Steps: OpenClash -> Override -> Mixed Port -> enable "Allow LAN" and disable auth for LAN
        - Then set proxy: "http://192.168.100.1:7890" in nanobot config
      </recommendation>
        1. In OpenClash, enable "Allow LAN" and set "Authentication" to none for LAN devices
        2. Create/edit ~/.nanobot/config.json with:
           {
             "channels": {
               "telegram": {
                 "enabled": true,
                 "token": "YOUR_BOT_TOKEN",
                 "proxy": "http://192.168.100.1:7890"
               }
             }
           }
        3. Restart nanobot gateway
    </option>
    <option id="option-c">
      <name>Set HTTPS_PROXY environment variable</name>
      <pros>
        - Simple, no code/config changes
        - Works for any HTTP client (httpx, requests, etc.)
      </cons>
        - Same auth issue as Option B (OpenClash 7890 returns 407)
        - Affects all connections, not just Telegram
        - Need to update nanobot-auto-updater's start_command in config.yaml
      </recommendation>
        Update config.yaml instances to include environment variable:
        start_command: "HTTPS_PROXY=http://192.168.100.1:7890 nanobot gateway"
    </option>
  </options>
  <resume-signal>Select: option-a, option-b, option-c, or describe your preferred approach</resume-signal>
</task>

<task type="auto">
  <name>Task 2: Verify Telegram connectivity and document the fix</name>
  <files>docs/bugs/telegram-httpx-connecterror-openclash.md</files>
  <action>
    After the user selects and applies a fix approach (Task 1), verify that Telegram API is now accessible:

    1. Run connectivity test: `curl --connect-timeout 10 https://api.telegram.org/bot_test/getMe`
       - Expected: HTTP 401 (invalid token but TLS succeeds) instead of TLS error

    2. If nanobot config was modified (option B/C), also test with httpx directly:
       ```python
       python -c "import httpx; r = httpx.get('https://api.telegram.org/bot_test/getMe', proxy='http://192.168.100.1:7890'); print(r.status_code)"
       ```

    3. Start nanobot gateway and verify Telegram channel connects:
       `nanobot gateway` (check logs for "Telegram bot @xxx connected")

    4. Create bug documentation at docs/bugs/telegram-httpx-connecterror-openclash.md with:
       - Problem description (httpx.ConnectError, CRYPT_E_REVOCATION_OFFLINE)
       - Root cause (OpenClash TLS MITM on api.telegram.org)
       - Fix applied (whichever approach was chosen)
       - Verification steps and results
  </action>
  <verify>curl --connect-timeout 10 https://api.telegram.org/bot_test/getMe returns HTTP status (not TLS error)</verify>
  <done>
    - Telegram API accessible without TLS errors
    - Bug documentation created at docs/bugs/telegram-httpx-connecterror-openclash.md
    - (If applicable) nanobot config updated with proxy settings
  </done>
</task>

</tasks>

<verification>
1. `curl --connect-timeout 10 https://api.telegram.org/bot_test/getMe` returns a valid HTTP response (not exit code 35)
2. docs/bugs/telegram-httpx-connecterror-openclash.md exists and documents root cause + fix
3. (If option B chosen) nanobot config.json contains telegram.proxy field
</verification>

<success_criteria>
- Telegram API connectivity verified (curl returns HTTP status, not TLS error)
- Root cause documented (OpenClash TLS MITM -> CRYPT_E_REVOCATION_OFFLINE -> httpx.ConnectError)
- Fix applied and verified
</success_criteria>

<output>
After completion, create `.planning/quick/260406-fge-nanobot-telegram-httpx-connecterror-open/260406-fge-SUMMARY.md`
</output>
