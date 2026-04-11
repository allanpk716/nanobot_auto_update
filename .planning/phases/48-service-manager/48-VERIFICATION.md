---
phase: 48-service-manager
verified: 2026-04-11T08:24:30+08:00
status: human_needed
score: 12/12 must-haves verified
overrides_applied: 0
---

# Phase 48: Service Manager Verification Report

**Phase Goal:** 用户设置 auto_start: true 后，程序自动完成服务注册和恢复策略配置，无需手动操作 sc.exe
**Verified:** 2026-04-11T08:24:30+08:00
**Status:** human_needed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

**Roadmap Success Criteria (contract-level):**

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | auto_start: true 时，程序以管理员权限运行后自动注册为 Windows 服务（SCM CreateService）并退出 | VERIFIED | servicemgr_windows.go:74 `scm.CreateService()` + main.go:129-149 Case 2: `lifecycle.IsAdmin()` check + `lifecycle.RegisterService()` + `os.Exit(2)` |
| 2 | auto_start: false 时，程序检测到已注册服务则自动卸载（SCM DeleteService）并退出 | VERIFIED | servicemgr_windows.go:164 `svcHandle.Delete()` + main.go:153-165 Case 3: `lifecycle.UnregisterService(context.Background(), ...)` + logs "switched to console mode" |
| 3 | 服务配置了 SCM 恢复策略：失败后自动重启，无需人工干预 | VERIFIED | servicemgr_windows.go:91-98 `SetRecoveryActions` with 3x ServiceRestart at 60s, resetPeriod=86400; line 101 `SetRecoveryActionsOnNonCrashFailures(true)` |
| 4 | 非 Windows 平台编译时 auto_start 配置被忽略，不影响其他功能 | VERIFIED | servicemgr.go (build tag `!windows`): no-op stubs for RegisterService/UnregisterService returning nil; IsAdmin returns false |

**Plan 01 Truths (ServiceManager core):**

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 5 | auto_start: true 时服务已注册，程序跳过注册并输出日志提示（幂等操作） | VERIFIED | servicemgr_windows.go:58-63 `scm.OpenService()` check, if succeeds: close handle + log "Service already registered, skipping" + return nil |
| 6 | 非管理员运行注册/卸载时输出错误提示并以退出码 1 退出 | VERIFIED | servicemgr_windows.go:32-38 `IsAdmin()` via `OpenCurrentProcessToken` + `IsElevated`; main.go:131-135 non-admin -> error log + `os.Exit(1)` |
| 7 | UnregisterService 支持 context.Context 取消，避免 30 秒不可中断阻塞 | VERIFIED | servicemgr_windows.go:115 `ctx context.Context` param; line 143 `case <-ctx.Done()` -> `goto deleteService`; poll loop with `pollTimeout` |
| 8 | 所有 SCM 错误带有操作上下文包装，便于诊断 | VERIFIED | 7 instances of `registerService:` prefix errors (lines 47,53,69,86,97) + 2 instances of `unregisterService:` prefix errors (lines 119,165) |

**Plan 02 Truths (main.go integration):**

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 9 | auto_start: true 且非管理员运行时，main.go 输出错误提示（含 Run as administrator 操作指引）并以退出码 1 退出 | VERIFIED | main.go:131 `lifecycle.IsAdmin()` check; line 133 hint "Right-click the executable and select 'Run as administrator'"; line 135 `os.Exit(1)` |
| 10 | auto_start: true 且管理员运行时，main.go 调用 RegisterService 后以退出码 2 退出 | VERIFIED | main.go:143 `lifecycle.RegisterService(cfg, logger)`; line 149 `os.Exit(2)` with comment |
| 11 | auto_start: false 且服务模式下，main.go 输出可操作警告（含如何卸载服务的具体步骤） | VERIFIED | main.go:121-126 Case 1: `slog.Warn` with "action" field containing "set auto_start: false in config.yaml, then run this program from a console (not as a service) to auto-uninstall" |
| 12 | auto_start: false 且控制台模式下，程序尝试卸载服务后输出 "switched to console mode" 日志并继续运行 | VERIFIED | main.go:154 `lifecycle.UnregisterService(context.Background(), cfg, logger)`; line 160 `slog.Info("Service uninstalled, switched to console mode", ...)` |

**Score:** 12/12 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/lifecycle/servicemgr_windows.go` | ServiceManager with RegisterService, UnregisterService, IsAdmin (Windows) | VERIFIED | 182 lines; exports ServiceManager, NewServiceManager, IsAdmin, RegisterService, UnregisterService; build tag `windows`; all SCM operations substantive |
| `internal/lifecycle/servicemgr.go` | Non-Windows stub for ServiceManager (no-op) | VERIFIED | 38 lines; build tag `!windows`; all 4 exports present as no-op stubs |
| `internal/lifecycle/servicemgr_test.go` | Unit tests for ServiceManager | VERIFIED | 5 tests (TestNewServiceManager, TestIsAdmin, TestRegisterService_EmptyServiceName, TestRegisterService_NonAdminOrNonWindows, TestUnregisterService_NonAdminOrNonWindows); all pass |
| `cmd/nanobot-auto-updater/main.go` | auto_start branching logic calling lifecycle.RegisterService/UnregisterService | VERIFIED | 3 cases wired: Case 1 (service mode + auto_start off -> warning), Case 2 (console + auto_start on -> register + exit 2), Case 3 (console + auto_start off -> unregister + continue) |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `servicemgr_windows.go` | `golang.org/x/sys/windows/svc/mgr` | `mgr.Connect`, `mgr.CreateService`, `mgr.OpenService`, `service.Delete`, `SetRecoveryActions` | WIRED | 2 matches for `mgr.(Connect|CreateService|OpenService)` pattern; SetRecoveryActions: 2 matches; svcHandle.Delete: 1 match |
| `servicemgr_windows.go` | `golang.org/x/sys/windows` | `OpenCurrentProcessToken`, `token.IsElevated` | WIRED | 2 matches for `OpenCurrentProcessToken|IsElevated` |
| `servicemgr_windows.go` | `internal/config` | `cfg.Service.ServiceName`, `cfg.Service.DisplayName` | WIRED | 15 references to `cfg.Service.(ServiceName|DisplayName)` across both methods |
| `main.go` | `internal/lifecycle` | `lifecycle.RegisterService`, `lifecycle.UnregisterService`, `lifecycle.IsAdmin` | WIRED | 1 call each: RegisterService (line 143), UnregisterService (line 154), IsAdmin (line 131) |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `servicemgr_windows.go:RegisterService` | `svcHandle` (from CreateService) | `mgr.Connect()` -> `scm.CreateService()` | Yes -- real SCM handle used for SetRecoveryActions + Close | FLOWING |
| `servicemgr_windows.go:UnregisterService` | `svcHandle` (from OpenService) | `mgr.Connect()` -> `scm.OpenService()` | Yes -- handle used for Control(Stop), Query loop, Delete | FLOWING |
| `servicemgr_windows.go:IsAdmin` | `token.IsElevated()` | `windows.OpenCurrentProcessToken()` | Yes -- real process token elevation check | FLOWING |
| `main.go` Case 2 | `cfg.Service.AutoStart` | `config.Load()` via pflag | Yes -- flows from validated config to RegisterService call | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Lifecycle package builds | `go build ./internal/lifecycle/` | Exit 0 | PASS |
| Main package builds | `go build ./cmd/nanobot-auto-updater/` | Exit 0 | PASS |
| Lifecycle vet passes | `go vet ./internal/lifecycle/` | Exit 0 | PASS |
| Main vet passes | `go vet ./cmd/nanobot-auto-updater/` | Exit 0 | PASS |
| ServiceManager tests pass | `go test ./internal/lifecycle/ -run "TestNewServiceManager|TestIsAdmin|TestRegisterService|TestUnregisterService" -v -count=1` | 5/5 PASS in 0.050s | PASS |
| Empty ServiceName defensive check | `TestRegisterService_EmptyServiceName` | Error contains "service_name is empty" | PASS |
| No Phase 48 placeholders remain | `grep "Phase 48" main.go` | 0 matches | PASS |
| RegisterService export check | `grep "func RegisterService" servicemgr_windows.go` | 1 match | PASS |
| UnregisterService ctx param check | `grep "context.Context" servicemgr_windows.go` | 7 matches | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| MGR-02 | 48-01, 48-02 | auto_start 为 true 时，程序以管理员权限注册自身为 Windows 服务并退出 | SATISFIED | servicemgr_windows.go:RegisterService (CreateService + SetRecoveryActions) + main.go Case 2 (IsAdmin + RegisterService + os.Exit(2)) |
| MGR-03 | 48-01, 48-02 | auto_start 为 false 时，程序检测到已注册服务则自动卸载 | SATISFIED | servicemgr_windows.go:UnregisterService (Stop + DeleteService) + main.go Case 3 (UnregisterService + "switched to console mode") |
| MGR-04 | 48-01, 48-02 | 服务配置 SCM 恢复策略（失败后自动重启） | SATISFIED | servicemgr_windows.go:91-101: 3x ServiceRestart at 60s + 24h reset + SetRecoveryActionsOnNonCrashFailures(true) |

**Orphaned requirements:** None. All REQUIREMENTS.md Phase 48 entries (MGR-02, MGR-03, MGR-04) are claimed by plans.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | - | - | - | No anti-patterns detected in Phase 48 files |

No TODO/FIXME/placeholder comments found. No stub returns (empty array/object/null) in service management code. No console.log-only implementations.

Note: 2 pre-existing test failures in `capture_test.go` (TestStartNanobotWithCapture_CapturesOutput, TestStartNanobotWithCapture_ProcessExit) are from Phase 20/23, not introduced by Phase 48. All 5 Phase 48-specific tests pass.

### Human Verification Required

### 1. Service Registration with Admin Privileges

**Test:** Set `auto_start: true` in config.yaml with valid `service_name` and `display_name`. Run the executable as Administrator from a console.
**Expected:** Program registers the Windows service via SCM, logs success message, and exits with code 2. Verify with `sc query <service_name>` that the service exists with recovery policy configured (`sc qfailure <service_name>`).
**Why human:** Requires admin privileges and real Windows SCM interaction. Unit tests verify the code paths but cannot test actual SCM CreateService/DeleteService without elevated process token.

### 2. Service Registration Idempotency

**Test:** Run the same executable as Administrator again after successful registration.
**Expected:** Program detects the service already exists, logs "Service already registered, skipping", and exits with code 2 without errors.
**Why human:** Requires admin privileges and a service that was registered in Test 1. Cannot be simulated in unit tests.

### 3. Service Unregistration (auto_start: false)

**Test:** Set `auto_start: false` in config.yaml (or remove the auto_start field). Run the executable from console (admin or non-admin).
**Expected:** Program attempts to unregister the service. If registered, logs "Service uninstalled, switched to console mode" and continues running in console mode. If not registered, logs "Service not registered, nothing to uninstall" and continues. Verify with `sc query <service_name>` that the service no longer exists.
**Why human:** Requires real SCM DeleteService interaction. The stop-wait-delete flow with 30s polling cannot be validated without a running service.

### 4. Non-Admin Error Message

**Test:** Set `auto_start: true` in config.yaml. Run the executable from console WITHOUT administrator privileges.
**Expected:** Program logs "Administrator privileges required" with hint "Right-click the executable and select 'Run as administrator'" and exits with code 1.
**Why human:** Requires running without admin token. CI and development environments typically run as admin, making this hard to automate.

### 5. Recovery Policy Verification

**Test:** After registering the service (Test 1), run `sc qfailure <service_name>` to inspect recovery policy.
**Expected:** Recovery actions show 3 restart attempts at 60-second intervals with 24-hour reset period. "Reboot action" flag should show recovery on non-crash failures is enabled.
**Why human:** SCM recovery policy is a Windows registry/SCM configuration that can only be verified through actual service registration and `sc.exe` queries.

### 6. Console Mode Non-Regression

**Test:** With `auto_start: false` and no service registered, run the program. Verify it starts normally in console mode, API server starts, instances are monitored, and Ctrl+C triggers graceful shutdown.
**Expected:** All Phase 47 console mode behavior preserved. No regression in signal handling, API startup, or component initialization.
**Why human:** Requires full application runtime verification. The main.go auto_start branch logic precedes all component startup, so any regression would block the entire application.

### Gaps Summary

No automated gaps found. All 12 must-have truths verified through code analysis, build checks, vet checks, and unit test execution. All 3 requirements (MGR-02, MGR-03, MGR-04) are satisfied by the implementation.

The phase requires 6 human verification items because the core functionality (SCM CreateService, DeleteService, recovery policy configuration) operates at the Windows system level and cannot be tested without admin privileges and a real Windows Service Control Manager. The unit tests verify code paths, error wrapping, defensive checks, and cross-platform stub behavior, but full end-to-end validation requires manual execution.

---

_Verified: 2026-04-11T08:24:30+08:00_
_Verifier: Claude (gsd-verifier)_
