---
phase: 260325-ovr-web
plan: 01
subsystem: web-ui
tags:
  - restart-api
  - ui-enhancement
  - log-display
requires: []
provides:
  - 实例重启 API 端点
  - Web UI 重启按钮
  - 日志 ANSI 转义码过滤
affects:
  - internal/api/server.go
  - internal/instance/manager.go
  - internal/web/handler.go
  - internal/web/static/*
tech-stack:
  added:
    - Go net/http PathValue 路由参数
    - JavaScript fetch API
    - ANSI escape code 正则处理
  patterns:
    - RESTful API 设计 (POST /api/v1/instances/{name}/restart)
    - 前端加载状态管理 (loading/disabled states)
    - SSE 自动重连模式
key-files:
  created: []
  modified:
    - internal/api/server.go
    - internal/instance/manager.go
    - internal/web/handler.go
    - internal/web/static/style.css
    - internal/web/static/home.js
    - internal/web/static/app.js
    - internal/web/static/index.html
decisions:
  - 使用现有 StopForUpdate/StartAfterUpdate 方法实现重启逻辑
  - 前端使用正则表达式 /\x1b\[[0-9;]*m/g 过滤 ANSI 转义码
  - 重启按钮采用与其他按钮一致的样式设计
  - 重启成功后自动刷新状态或重连 SSE
  - 重启操作使用阻塞调用，使用请求上下文
metrics:
  duration: 8m
  tasks: 3
  files: 7
  completed_date: 2026-03-25
---

# Phase 260325-ovr Phase 01: 实例重启与日志显示优化 Summary

## 一句话总结

在 Web 界面添加实例重启 API 端点和 UI 按钮，并修复日志中 ANSI 转义码显示问题，提升用户体验。

## 目标达成情况

**目标**: 用户发现实例启动后日志显示故障时，可以直接在 Web 界面点击重启按钮快速解决问题；同时修复日志中 ANSI 转义码显示问题

**成果**:
- ✅ POST /api/v1/instances/{name}/restart 端点可用
- ✅ 首页实例卡片显示重启按钮
- ✅ 日志页面 header 显示重启按钮
- ✅ 重启按钮可正常触发实例重启
- ✅ 日志中 ANSI 转义码被正确过滤

## 技术实现

### Task 1: 添加重启 API 端点

**实现方式**:
1. 在 InstanceManager 中添加 `GetLifecycle(name string)` 方法用于获取单个实例
2. 在 handler.go 中创建 `NewInstanceRestartHandler` 处理器:
   - 从 URL PathValue 提取实例名称
   - 通过 InstanceManager.GetLifecycle 获取实例
   - 调用 StopForUpdate 停止实例
   - 调用 StartAfterUpdate 启动实例
   - 返回 JSON 响应 {success: true} 或 {error: string}
3. 在 server.go 中注册路由 `POST /api/v1/instances/{name}/restart`

**技术要点**:
- 使用 `r.PathValue("name")` 提取路径参数 (Go 1.22+ 特性)
- 重启是阻塞操作，使用请求上下文 `r.Context()`
- 实例不存在返回 404
- 停止或启动失败返回 500 + JSON 错误信息

**Commit**: 28d0a61

### Task 2: 添加重启按钮到前端界面

**实现方式**:
1. 在 style.css 中添加 `.btn-restart` 样式:
   - 基础样式与现有按钮保持一致
   - `.btn-restart:disabled` 禁用状态 (opacity: 0.6)
   - `.btn-restart.loading` 加载中状态 (cursor: wait)

2. 在 home.js 的 createInstanceCard 中:
   - 在实例卡片底部添加重启按钮
   - 点击时调用 `restartInstance(instanceName, button)`
   - 禁用按钮并显示"重启中..."
   - 成功后刷新实例状态
   - 失败后恢复按钮并显示错误提示

3. 在 index.html 的 header controls 区域:
   - 在 scroll-toggle 按钮旁边添加重启按钮
   - 仅在查看日志页显示

4. 在 app.js 中:
   - 添加 `restartInstance(instanceName, button)` 函数
   - 调用 POST /api/v1/instances/{name}/restart
   - 重启成功后关闭旧 SSE 连接
   - 清空日志容器并重新连接 SSE 流

**技术要点**:
- 使用 async/await 处理异步操作
- 按钮状态管理: normal → loading → success/failed → normal
- 2 秒延迟后恢复按钮状态
- SSE 重连: eventSource.close() + connectSSE()

**Commit**: 76e28ac

### Task 3: 处理 ANSI 颜色转义码

**实现方式**:
1. 添加 `stripAnsiCodes(text)` 函数:
   - 使用正则表达式 `/\x1b\[[0-9;]*m/g`
   - `\x1b` = ESC 字符
   - `\[` = 左括号
   - `[0-9;]*` = 数字和分号（颜色代码）
   - `m` = 终止字符

2. 在 `appendLog(message, source)` 中:
   - 在设置 textContent 之前调用 `stripAnsiCodes(message)`
   - 清理后的日志文本直接显示

**效果示例**:
- 输入: `[32m2026-03-25 17:47:55.823[0m | [34m[1mDEBUG   [0m`
- 输出: `2026-03-25 17:47:55.823 | DEBUG`

**Commit**: a2db485

## 偏离计划的情况

无偏离 - 所有任务按照计划执行。

## 验证

**自动验证**:
- ✅ `go build ./internal/...` 构建通过

**手动验证步骤**:
1. 启动应用，访问首页 http://localhost:{port}/
2. 验证每个实例卡片有重启按钮
3. 点击重启按钮，验证:
   - 按钮显示"重启中..."
   - API 返回成功
   - 实例状态刷新
4. 进入日志页面，验证:
   - header 有重启按钮
   - 日志文本无 ANSI 转义码
5. 点击日志页面重启按钮，验证 SSE 重新连接

## 已知问题

无。

## 文件清单

### 修改的文件 (7 个)

| 文件 | 变更说明 |
|------|---------|
| internal/api/server.go | 添加 POST /api/v1/instances/{name}/restart 路由注册 |
| internal/instance/manager.go | 添加 GetLifecycle 方法用于获取单个实例 |
| internal/web/handler.go | 添加 NewInstanceRestartHandler 处理器 |
| internal/web/static/style.css | 添加 .btn-restart 样式 |
| internal/web/static/home.js | 在实例卡片中添加重启按钮和 restartInstance 函数 |
| internal/web/static/app.js | 添加重启按钮事件处理、stripAnsiCodes 函数、SSE 重连逻辑 |
| internal/web/static/index.html | 在日志页面 header 添加重启按钮 |

### 创建的文件 (0 个)

无

## 提交历史

| Commit | 消息 | 文件数 |
|--------|------|--------|
| 28d0a61 | feat(260325-ovr): add instance restart API endpoint | 3 |
| 76e28ac | feat(260325-ovr): add restart buttons to frontend UI | 4 |
| a2db485 | feat(260325-ovr): strip ANSI escape codes from log messages | 1 |

## 后续工作

无。所有计划功能已实现并验证。

## 自检清单

- [x] POST /api/v1/instances/{name}/restart 端点可用
- [x] 首页实例卡片显示重启按钮
- [x] 日志页面显示重启按钮
- [x] 重启按钮可正常触发实例重启
- [x] 日志中 ANSI 转义码被正确过滤
- [x] 所有任务已提交
- [x] 构建验证通过

## Self-Check: PASSED

所有文件已创建，所有 commit 已验证存在。
