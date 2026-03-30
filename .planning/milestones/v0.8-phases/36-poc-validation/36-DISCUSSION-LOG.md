# Phase 36: PoC Validation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-29
**Phase:** 36-poc-validation
**Areas discussed:** PoC 程序形式和范围, 验证和测试方法, PoC 代码保留策略

---

## PoC 程序形式和范围

| Option | Description | Selected |
|--------|-------------|----------|
| 最小程序 | 一个 main.go：打印版本号 → selfupdate.Apply() → self-spawn → 打印新版本号。约 50-80 行 | ✓ |
| 模拟真实结构 | 模拟 nanobot-auto-updater：logger、config、HTTP server。约 200 行 | |
| 两步法 | 先最小程序，再增加真实场景验证 | |

**User's choice:** 最小程序
**Notes:** 最小可行性验证优先，不引入不必要的复杂度

### 新版本来源

| Option | Description | Selected |
|--------|-------------|----------|
| 本地构建两个版本 | 构建 v1 和 v2 两个 exe，v1 用 selfupdate.Apply() 应用 v2 | ✓ |
| 从 GitHub Release 下载 | 从测试 GitHub Release 下载新版本 | |

**User's choice:** 本地构建两个版本
**Notes:** 最简单、最快、无网络依赖。PoC 阶段不需要验证下载流程

---

## 验证和测试方法

| Option | Description | Selected |
|--------|-------------|----------|
| 手动观察验证 | 手动运行，观察版本号变化、.old 文件、进程重启 | |
| 自动化测试脚本 | Go 测试脚本：构建 → 运行 → 等待 → 文件检查验证 | ✓ |

**User's choice:** 自动化测试脚本
**Notes:** 可重复执行，适合 PoC 阶段严谨验证

### 新版本启动检测

| Option | Description | Selected |
|--------|-------------|----------|
| 文件输出验证 | PoC 将版本号写入文件，测试脚本读取验证 | ✓ |
| 进程检测 | 检查新进程 PID 和命令行 | |
| HTTP 端口验证 | 新版本启动 HTTP 服务，通过请求验证 | |

**User's choice:** 文件输出验证
**Notes:** 最简单可靠，不引入网络依赖

---

## PoC 代码保留策略

| Option | Description | Selected |
|--------|-------------|----------|
| 验证后删除 | Phase 38 从零开始写 | |
| 保留在 tmp/ | 保留作为参考，正式实现独立编写 | ✓ |
| 保留并复用 | 复用测试逻辑作为 E2E 测试基础 | |

**User's choice:** 保留在 tmp/
**Notes:** 符合项目约定（临时文件放 tmp/），正式实现可参考但不依赖 PoC 代码

---

## Claude's Discretion

- PoC 程序的具体实现细节
- 自动化测试脚本的结构和错误处理
- 文件路径约定、等待超时等实现参数

## Deferred Ideas

None — discussion stayed within phase scope
