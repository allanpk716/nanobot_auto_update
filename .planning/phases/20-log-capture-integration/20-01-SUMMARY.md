---
phase: 20-log-capture-integration
plan: 01
subsystem: log-capture
tags: [capture, stdout, stderr, bufio, scanner, context]

# Dependency graph
requires:
  - phase: 19
    provides: LogBuffer implementation with Write method and LogEntry struct
provides:
  - captureLogs function for reading logs from io.Reader
  - Context-aware goroutine lifecycle management
  - Line-by-line log capture using bufio.Scanner
affects: [20-02, 21]

# Tech tracking
tech-stack:
  added: []
  patterns: ["bufio.Scanner for line reading", "context cancellation pattern", "select+default non-blocking pattern"]

key-files:
  created: [internal/lifecycle/capture.go, internal/lifecycle/capture_test.go]
  modified: []

key-decisions:
  - "Use bufio.Scanner instead of bufio.Reader (Scanner handles line boundaries automatically)"
  - "Use select+default pattern for non-blocking scan with context cancellation check"
  - "Log and drop log lines on buffer write failure (don't block capture goroutine)"

patterns-established:
  - "Pattern: bufio.Scanner for line-by-line reading from io.Reader"
  - "Pattern: Context cancellation via select+default loop"
  - "Pattern: Error handling - log errors but continue running (ERR-01)"

requirements-completed: [CAPT-01, CAPT-02, CAPT-03]

# Metrics
duration: 6min
completed: 2026-03-17
---

# Phase 20 Plan 01: Capture Logs Function Summary

**实现 captureLogs 核心函数,使用 bufio.Scanner 从 io.Reader 逐行读取日志并写入 LogBuffer,支持 context 取消和并发安全**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-17T07:21:36Z
- **Completed:** 2026-03-17T07:27:33Z
- **Tasks:** 1
- **Files modified:** 2 (capture.go, capture_test.go)

## Accomplishments
- 实现 captureLogs 函数,支持从任意 io.Reader 逐行读取日志
- 使用 bufio.Scanner 自动处理行边界和缓冲区管理
- 支持 context 取消,确保 goroutine 可以正确退出
- LogEntry 正确设置 Timestamp, Source, Content 三个字段
- 错误处理完善:scanner 错误和 buffer 写入失败都记录日志但不中断流程

## Task Commits

Each task was committed atomically:

1. **Task 1: 创建 captureLogs 函数和测试** - `b81d127` (test), `0b8x084` (feat)
   - RED: 创建失败的测试 (3 个测试用例)
   - GREEN: 实现 captureLogs 函数,所有测试通过
   - REFACTOR: 代码简洁,无需重构

**Plan metadata:** 待提交 (docs: complete plan)

## Files Created/Modified
- `internal/lifecycle/capture.go` - captureLogs 函数实现,使用 bufio.Scanner 逐行读取
- `internal/lifecycle/capture_test.go` - 3 个测试用例:WritesToBuffer, ContextCancellation, LogEntryFields

## Decisions Made
- 使用 `bufio.Scanner` 而不是 `bufio.Reader`:Scanner 自动处理行边界和缓冲区管理,代码更简洁
- 使用 `select + default` 模式:在 default 中调用 scanner.Scan(),在 case 中检查 ctx.Done(),实现非阻塞扫描
- 错误处理策略:scanner 错误和 buffer 写入失败都记录日志但不中断流程,确保日志捕获不会阻塞进程运行

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None - TDD 流程顺利进行,RED-GREEN 阶段均一次成功,无需 REFACTOR。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
captureLogs 函数已实现并测试通过,可以用于 Phase 20-02 中的 StartNanobotWithCapture 函数,实现 nanobot 进程的 stdout/stderr 捕获。

---
*Phase: 20-log-capture-integration*
*Completed: 2026-03-17*

## Self-Check: PASSED
- [✓] internal/lifecycle/capture.go exists
- [✓] internal/lifecycle/capture_test.go exists
- [✓] Commit b81d127 (test) exists
- [✓] Commit 0b8b084 (feat) exists
- [✓] All tests pass (go test ./internal/lifecycle -run TestCaptureLogs)
