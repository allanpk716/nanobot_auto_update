# Phase 46: Service Configuration & Mode Detection - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-10
**Phase:** 46-service-configuration-mode-detection
**Areas discussed:** 配置结构设计, 模式检测策略, 配置解析验证

---

## 配置结构设计

| Option | Description | Selected |
|--------|-------------|----------|
| 顶层字段 auto_start: true | 简单直接，但与 per-instance auto_start 同名易混淆 | |
| service 子段 service.auto_start: true | 类似 self_update 结构，未来可扩展，语义清晰 | ✓ |
| 顶层字段 service_mode: auto | 明确不混淆，但名称较长 | |

**User's choice:** service 子段（推荐选项）
**Notes:** 预留 service_name 和 display_name 字段，为 Phase 48 服务注册做准备

### 默认值

| Option | Description | Selected |
|--------|-------------|----------|
| false (推荐) | 不配置时行为与当前一致，安全无回归 | ✓ |
| true | 默认开启服务模式 | |

**User's choice:** 默认 false

### 预留字段

| Option | Description | Selected |
|--------|-------------|----------|
| 只加 auto_start (YAGNI) | Phase 48/49 再按需扩展 | |
| 预留 service_name / display_name | 服务注册需要这些值，提前定义 | ✓ |

**User's choice:** 预留字段
**Notes:** 默认值 service_name: "NanobotAutoUpdater", display_name: "Nanobot Auto Updater"

---

## 模式检测策略

### 检测时机

| Option | Description | Selected |
|--------|-------------|----------|
| 先检测环境 (推荐) | svc.IsWindowsService() 先于配置加载 | ✓ |
| 先加载配置 | 加载 config → 检查 auto_start → 检测环境 | |

**User's choice:** 先检测环境

### auto_start: false + SCM 启动

| Option | Description | Selected |
|--------|-------------|----------|
| 运行但记录警告 (推荐) | 不崩溃，日志提醒，Phase 48 自动卸载 | ✓ |
| 拒绝启动并退出 | 严格匹配，但可能触发 SCM 恢复策略 | |

**User's choice:** 运行但记录警告

### auto_start: true + 控制台运行

| Option | Description | Selected |
|--------|-------------|----------|
| 自动注册服务后退出 | Phase 48 行为提前，一次搞定 | ✓ |
| 控制台运行 + 日志提示 | 用户可能只是测试 | |
| 控制台运行 + 交互提示 | 介于两者之间 | |

**User's choice:** 自动注册服务后退出
**Notes:** 最初选择交互提示，后改为自动注册。退出码 2 区分正常退出。

### 退出码

| Option | Description | Selected |
|--------|-------------|----------|
| 退出码 0 | 注册成功，正常退出 | |
| 退出码 2 | 区分"注册后退出"和"正常退出" | ✓ |

**User's choice:** 退出码 2

---

## 配置解析验证

### 验证规则

| Option | Description | Selected |
|--------|-------------|----------|
| service_name 格式校验 | 仅允许字母数字，无空格（SCM 要求） | ✓ |
| display_name 长度限制 | 最大 256 字符（SCM 限制） | ✓ |
| 仅 auto_start: true 时校验 | false 时 name/display_name 无意义 | ✓ |

**User's choice:** 全部验证都要

### 测试范围

| Option | Description | Selected |
|--------|-------------|----------|
| 只测配置解析+验证 (推荐) | svc 检测留给 Phase 47 集成测试 | ✓ |
| 抽象 isWindowsService 接口 | 可测试接口包装 | |

**User's choice:** 只测配置部分

---

## Claude's Discretion

- ServiceConfig 具体 Go struct 定义细节
- 验证错误消息的具体措辞
- WARN 日志的格式和内容

## Deferred Ideas

None — discussion stayed within phase scope
