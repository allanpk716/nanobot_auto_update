---
phase: 12
slug: monitoring-service
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-16
---

# Phase 12 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (标准库) |
| **Config file** | 无 - 使用 Go convention |
| **Quick run command** | `go test ./internal/monitor/... -v -short` |
| **Full suite command** | `go test ./internal/monitor/... -v` |

**Feedback Latency:**
- After every task commit: `go test ./internal/monitor/... -v -short`
- After every wave merge: `go test ./internal/monitor/... -v`
- Phase gate: `go test ./internal/monitor/... -v -race` (包含竞态检测)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 12-01-01 | 01 | 1 | MON-08 | unit | `go test ./internal/monitor/... -run TestCheckerTimeout -v` | ❌ W0 | ⬜ pending |
| 12-01-02 | 01 | 1 | MON-08 | unit | `go test ./internal/monitor/... -run TestCheckerSuccess -v` | ❌ W0 | ⬜ pending |
| 12-02-01 | 02 | 1 | MON-01 | unit | `go test ./internal/monitor/... -run TestServiceInterval -v` | ❌ W0 | ⬜ pending |
| 12-02-02 | 02 | 1 | MON-04 | unit | `go test ./internal/monitor/... -run TestServiceLogging -v` | ❌ W0 | ⬜ pending |
| 12-02-03 | 02 | 1 | MON-05 | unit | `go test ./internal/monitor/... -run TestServiceFailureContinue -v` | ❌ W0 | ⬜ pending |
| 12-02-04 | 02 | 1 | MON-05 | unit | `go test ./internal/monitor/... -run TestServiceGracefulShutdown -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending | ✅ passed | ❌ failed | ⚠️ flaky*

---

## Wave 0 Dependencies

| File | Purpose | Created By |
|------|---------|------------|
| `internal/monitor/service.go` | 监控服务主体 | Task 12-02 |
| `internal/monitor/checker.go` | HTTP 连通性检查器 | Task 12-01 |
| `internal/monitor/service_test.go` | 服务测试 | Task 12-02 |
| `internal/monitor/checker_test.go` | 检查器测试 | Task 12-01 |

---

## Manual Verifications

*All phase behaviors have automated verification.*

---

## Test Scenarios Detail

### 1. HTTP 超时处理测试 (MON-08)
```go
func TestCheckerTimeout(t *testing.T) {
    // 创建延迟响应的测试服务器
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(15 * time.Second) // 超过 10 秒超时
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()

    checker := NewChecker(server.URL, 10*time.Second)
    ctx := context.Background()

    _, err := checker.CheckConnectivity(ctx)
    if err == nil {
        t.Error("Expected timeout error, got nil")
    }
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Errorf("Expected DeadlineExceeded, got: %v", err)
    }
}
```

### 2. 15分钟间隔正确性测试 (MON-01)
```go
func TestServiceInterval(t *testing.T) {
    // 使用短间隔测试 (100ms 模拟 15 分钟)
    cfg := config.MonitorConfig{
        Interval: 100 * time.Millisecond,
        Timeout:  10 * time.Second,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
    defer cancel()

    svc := NewService(cfg, slog.Default())
    // 验证在 350ms 内执行 3 次检查
}
```

### 3. 连续失败场景测试 (MON-05)
```go
func TestServiceFailureContinue(t *testing.T) {
    // 创建始终失败的测试服务器
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
    }))
    defer server.Close()

    // Run 应该在 context 取消后正常返回，而不是因为检查失败而提前返回
    err := svc.Run(ctx)
    if !errors.Is(err, context.Canceled) {
        t.Errorf("Expected context.Canceled, got: %v", err)
    }
}
```

### 4. 优雅停机测试 (MON-05)
```go
func TestServiceGracefulShutdown(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    svc := NewService(cfg, slog.Default())

    done := make(chan error, 1)
    go func() {
        done <- svc.Run(ctx)
    }()

    // 等待第一次检查完成
    time.Sleep(100 * time.Millisecond)

    // 发送取消信号
    cancel()

    // 验证在合理时间内退出
    select {
    case err := <-done:
        if !errors.Is(err, context.Canceled) {
            t.Errorf("Expected context.Canceled, got: %v", err)
        }
    case <-time.After(1 * time.Second):
        t.Error("Service did not stop within timeout")
    }
}
```

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
