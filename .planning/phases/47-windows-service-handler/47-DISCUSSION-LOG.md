# Phase 47: Windows Service Handler - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-10
**Phase:** 47-windows-service-handler
**Areas discussed:** Handler 接口设计, 关闭流程编排, 服务启动入口

---

## Handler 接口设计

| Option | Description | Selected |
|--------|-------------|----------|
| lifecycle/service_windows.go (推荐) | 在 internal/lifecycle/ 下新建 service_windows.go，与 IsServiceMode() 同包，生命周期代码内聚 | ✓ |
| 新包 internal/service/ | 完全独立于 lifecycle，更清晰的关注点分离，但增加包数量 | |
| 直接在 main.go 中定义 | 简单但不可测试 | |

**User's choice:** lifecycle/service_windows.go

| Option | Description | Selected |
|--------|-------------|----------|
| 标准状态机 (推荐) | StartPending → Running → StopPending → Stopped 完整状态转换 | ✓ |
| 简化状态 | 仅 Running/Stopped，服务管理器状态可能不准确 | |

**User's choice:** 标准状态机

**Notes:** 标准状态机确保 SCM 正确显示服务状态，符合 Windows 服务开发惯例。

---

## 关闭流程编排

| Option | Description | Selected |
|--------|-------------|----------|
| 抽取共用关闭函数 (推荐) | 从 main.go 抽取 AppShutdown，服务模式和控制台模式复用同一函数 | ✓ |
| 服务模式独立关闭逻辑 | Execute 内部独立实现，更解耦但有代码重复 | |

**User's choice:** 抽取共用关闭函数

| Option | Description | Selected |
|--------|-------------|----------|
| 统一超时，按模式区分 (推荐) | 控制台 10s，服务 30s，共用一个关闭函数，超时参数由调用方传入 | ✓ |
| 每组件独立超时 | 更精细但复杂度高 | |

**User's choice:** 统一超时，按模式区分

**Notes:** 保持与现有 main.go 关闭顺序一致（通知管理器 → 网络监控 → 健康监控 → cron → UpdateLogger → API 服务器）。

---

## 服务启动入口

| Option | Description | Selected |
|--------|-------------|----------|
| main.go 分支调用 svc.Run (推荐) | 服务模式初始化日志、配置后调用 svc.Run()，Execute 内启动业务组件，控制台模式不变 | ✓ |
| 早期拦截 + 包装整个 main | main.go 早期阶段检测到服务模式就调用 RunAsService()，Execute 会很臃肿 | |

**User's choice:** main.go 分支调用 svc.Run

**Notes:** 控制台模式完全不受影响，服务模式是新代码路径。

---

## Claude's Discretion

- ServiceHandler 结构体的具体字段设计
- AppComponents 容器结构体的字段组织方式
- Execute 内部 goroutine 的启动模式
- 非服务控制码的忽略方式
- 测试策略（单元测试模拟 ChangeRequest channel）

## Deferred Ideas

None — discussion stayed within phase scope
