---
phase: 07-lifecycle-extension
plan: 01
subsystem: instance-management
tags: [lifecycle, error-handling, context-logging, tdd]

requires:
  - phase: 06-configuration-extension
    provides: InstanceConfig with name/port/start_command/startup_timeout fields
provides:
  - InstanceError custom error type with Chinese messages and error chain support
  - InstanceLifecycle wrapper with context-aware logging
  - Refactored StartNanobot function with dynamic command and port parameters
affects: [instance-supervisor, multi-instance-coordinator]

tech-stack:
  added: []
  patterns:
    - context-aware logging via logger.With() pre-injection
    - error wrapping with InstanceError for structured error reporting
    - shell command execution via cmd /c for Windows compatibility

key-files:
  created:
    - internal/instance/errors.go
    - internal/instance/errors_test.go
    - internal/instance/lifecycle.go
    - internal/instance/lifecycle_test.go
  modified:
    - internal/lifecycle/starter.go
    - internal/lifecycle/manager.go

key-decisions:
  - "使用中文错误消息(停止实例/启动实例)提升用户友好性"
  - "InstanceError 实现 Unwrap() 方法支持 errors.Is/As 错误链遍历"
  - "StartNanobot 使用 cmd /c 执行命令,支持管道和重定向等复杂命令"
  - "使用端口监听验证替代进程名验证,提高精确度"
  - "停止超时固定为 5 秒,启动超时默认 30 秒(配置为 0 时)"
  - "所有日志通过 logger.With() 预注入 instance 和 component 字段"

patterns-established:
  - "Pattern 1: 上下文感知日志 - 构造时注入 logger.With(\"instance\", name).With(\"component\", \"instance-lifecycle\")"
  - "Pattern 2: 结构化错误包装 - 所有底层错误通过 InstanceError 包装,包含实例名、操作类型、端口信息"

requirements-completed:
  - LIFECYCLE-01
  - LIFECYCLE-02

duration: 4min
completed: 2026-03-11
---

# Phase 07 Plan 01: 实例生命周期包装器 Summary

**实现 InstanceError 自定义错误类型和 InstanceLifecycle 包装器,为每个实例提供独立的上下文感知生命周期管理,支持定制化启动命令和结构化错误报告**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-11T00:28:06Z
- **Completed:** 2026-03-11T00:32:44Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- InstanceError 自定义错误类型,支持中文错误消息和错误链遍历
- InstanceLifecycle 包装器,每个实例拥有独立的上下文感知 logger
- 重构 StartNanobot 函数支持动态命令和端口参数,使用 cmd /c 执行 Shell 命令

## Task Commits

Each task was committed atomically:

1. **Task 1: 实现 InstanceError 自定义错误类型** - `297f606` (test)
   - RED: 创建 errors_test.go 编写失败测试
   - GREEN: 实现 InstanceError 结构体和 Error/Unwrap/operationText 方法
   - 所有测试通过
2. **Task 2: 重构 lifecycle.StartNanobot 支持动态命令参数** - `1590cae` (feat)
   - 修改函数签名添加 command 和 port 参数
   - 使用 cmd /c 执行 Shell 命令
   - 使用端口验证替代进程名验证
   - 更新 manager.go 调用点
3. **Task 3: 实现 InstanceLifecycle 包装器** - `295c674` (test)
   - RED: 创建 lifecycle_test.go 编写失败测试
   - GREEN: 实现 InstanceLifecycle 和 NewInstanceLifecycle
   - 实现 StopForUpdate 和 StartAfterUpdate 方法
   - 所有测试通过,60.6% 覆盖率

**Plan metadata:** 待提交 (docs: complete plan)

_Note: TDD tasks may have multiple commits (test → feat → refactor)_

## Files Created/Modified
- `internal/instance/errors.go` - InstanceError 自定义错误类型,支持中文消息和错误链
- `internal/instance/errors_test.go` - InstanceError 单元测试,覆盖错误格式、Unwrap、operationText
- `internal/instance/lifecycle.go` - InstanceLifecycle 包装器,上下文感知日志,调用 lifecycle 包函数
- `internal/instance/lifecycle_test.go` - InstanceLifecycle 单元测试,验证日志注入和错误包装
- `internal/lifecycle/starter.go` - 重构 StartNanobot 支持 command 和 port 参数,使用 cmd /c
- `internal/lifecycle/manager.go` - 更新调用 StartNanobot 传入 "nanobot gateway" 和 port

## Decisions Made
- 使用中文错误消息("停止实例"/"启动实例")提升用户友好性
- InstanceError 实现 Unwrap() 方法支持 errors.Is/As 错误链遍历
- StartNanobot 使用 cmd /c 执行命令,支持管道和重定向等复杂命令
- 使用端口监听验证替代进程名验证,提高精确度
- 停止超时固定为 5 秒,启动超时默认 30 秒(配置为 0 时)
- 所有日志通过 logger.With() 预注入 instance 和 component 字段

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- 测试覆盖率 60.6% 低于计划的 80%,但符合预期 - 进程管理功能需要集成测试而非单元测试(计划中已说明)

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- 实例生命周期包装器就绪,可以为监督者模式提供实例级管理能力
- 错误包装机制就绪,支持错误聚合模式
- 日志追踪机制就绪,每个实例的日志都包含实例名称

## Self-Check: PASSED
- All created/modified files verified to exist
- All task commit hashes verified in git history
- SUMMARY.md metadata accurate

---
*Phase: 07-lifecycle-extension*
*Completed: 2026-03-11*
