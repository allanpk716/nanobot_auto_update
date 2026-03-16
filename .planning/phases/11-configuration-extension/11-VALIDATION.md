---
phase: 11
slug: configuration-extension
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-03-16
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) |
| **Config file** | none — standard `go test` |
| **Quick run command** | `go test -v ./internal/config/...` |
| **Full suite command** | `go test -v ./... -race -cover` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/config/...`
- **After every plan wave:** Run `go test -v ./... -race -cover`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | Created in Wave |
|--------|----------|-----------|-------------------|-----------------|
| CONF-01 | Pushover credentials in YAML | unit | `go test -v ./internal/config -run TestLoadPushoverFromYAML` | Wave 0 |
| CONF-02 | API port in YAML | unit | `go test -v ./internal/config -run TestAPIConfigValidate` | Wave 0 |
| CONF-03 | Bearer Token in YAML | unit | `go test -v ./internal/config -run TestBearerToken` | Wave 0 |
| CONF-04 | Monitor interval in YAML | unit | `go test -v ./internal/config -run TestMonitorConfigValidate` | Wave 0 |
| CONF-05 | HTTP timeout in YAML | unit | `go test -v ./internal/config -run TestMonitorTimeout` | Wave 0 |
| CONF-06 | Startup validation rejects invalid config | unit | `go test -v ./internal/config -run TestConfigValidationWithNew` | Wave 0 |
| SEC-03 | Bearer Token length ≥ 32 chars | unit | `go test -v ./internal/config -run TestBearerTokenLength` | Wave 0 |

---

## Wave 0 Requirements

Before implementing configuration validation (Wave 1), create test infrastructure:

### Test Files to Create

| File | Purpose | Tests Included |
|------|---------|----------------|
| `internal/config/api_test.go` | APIConfig validation tests | 5 tests (valid, invalid port, token too short, timeout too short, empty token) |
| `internal/config/monitor_test.go` | MonitorConfig validation tests | 3 tests (valid, interval too short, timeout too short) |
| `testutil/testdata/config/api_valid.yaml` | Valid API config test data | Valid values for all fields |
| `testutil/testdata/config/api_invalid_token.yaml` | Invalid token test data | Token with 9 chars (violates SEC-03) |
| `testutil/testdata/config/api_invalid_port.yaml` | Invalid port test data | Port 70000 (out of range) |
| `testutil/testdata/config/monitor_valid.yaml` | Valid Monitor config test data | Valid Interval (15m) and Timeout (10s) |
| `testutil/testdata/config/monitor_invalid_interval.yaml` | Invalid interval test data | Interval 30s (below 1m minimum) |
| `internal/config/config_test.go` (extended) | Integration tests | 3 new tests for API/Monitor config loading |

### Wave 0 Gap Checklist

- [ ] `internal/config/api_test.go` exists with test stubs (all skip initially)
- [ ] `internal/config/monitor_test.go` exists with test stubs (all skip initially)
- [ ] `testutil/testdata/config/api_valid.yaml` exists and is valid YAML
- [ ] `testutil/testdata/config/api_invalid_token.yaml` exists and is valid YAML
- [ ] `testutil/testdata/config/api_invalid_port.yaml` exists and is valid YAML
- [ ] `testutil/testdata/config/monitor_valid.yaml` exists and is valid YAML
- [ ] `testutil/testdata/config/monitor_invalid_interval.yaml` exists and is valid YAML
- [ ] `internal/config/config_test.go` contains 3 new integration test stubs
- [ ] All test stubs can run: `go test -v ./internal/config/` (even if skipping)

---

## Per-Task Verification Map

| Plan | Task | Requirement | Test Type | Automated Command | File Created in Wave |
|------|------|-------------|-----------|-------------------|----------------------|
| 11-01a | 1 | CONF-02, CONF-03, SEC-03 | unit | `go test -v ./internal/config -run TestAPIConfigValidate` | Wave 0 |
| 11-01a | 2 | CONF-04, CONF-05 | unit | `go test -v ./internal/config -run TestMonitorConfigValidate` | Wave 0 |
| 11-01b | 3 | CONF-02, CONF-03 | integration | `ls testutil/testdata/config/api_*.yaml \| wc -l` | Wave 0 |
| 11-01b | 4 | CONF-04, CONF-05 | integration | `ls testutil/testdata/config/monitor_*.yaml \| wc -l` | Wave 0 |
| 11-01b | 5 | CONF-01~06 | integration | `go test -v ./internal/config -run "TestLoadAPI\|TestLoadMonitor\|TestConfigValidationWithNew"` | Wave 0 |
| 11-02 | 1 | CONF-02, CONF-03, SEC-03 | unit | `go test -v ./internal/config -run TestAPIConfigValidate` | Wave 1 |
| 11-02 | 2 | CONF-04, CONF-05 | unit | `go test -v ./internal/config -run TestMonitorConfigValidate` | Wave 1 |
| 11-03 | 1 | CONF-01~06 | integration | `go test -v ./internal/config` | Wave 2 |
| 11-03 | 2 | CONF-06 | manual | `grep -n "Configuration error" cmd/nanobot-auto-updater/main.go` | Wave 2 |
| 11-03 | 3 | CONF-01~06, SEC-03 | integration | `go test -v ./internal/config 2>&1 \| grep -E "PASS\|FAIL" \| grep -c PASS` | Wave 2 |

---

## Test Data Validation

All test data YAML files must be valid YAML parseable by Viper:

### Example: api_valid.yaml
```yaml
cron: "0 3 * * *"

api:
  port: 8080
  bearer_token: "this-is-a-secure-token-with-at-least-32-characters"
  timeout: "30s"
```

### Example: api_invalid_token.yaml
```yaml
cron: "0 3 * * *"

api:
  port: 8080
  bearer_token: "too-short"  # Only 9 chars, violates SEC-03
  timeout: "30s"
```

### Example: monitor_valid.yaml
```yaml
cron: "0 3 * * *"

monitor:
  interval: "15m"
  timeout: "10s"
```

---

## Existing Test Infrastructure

Current project uses standard Go testing patterns:
- Test files colocated with source files (`*_test.go`)
- Subtests using `t.Run()`
- Table-driven tests for multiple cases
- Error validation using `strings.Contains()`

Reference existing tests:
- `internal/config/config_test.go` — Configuration loading tests
- `internal/config/instance_test.go` — Instance validation tests

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Config file loading at startup | CONF-06 | Requires full application startup with real config.yaml | 1. Create config.yaml with all settings 2. Run `nanobot-auto-updater` 3. Verify startup logs show loaded config |

*Only startup integration requires manual verification. All config parsing/validation has automated tests.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ready for execution
