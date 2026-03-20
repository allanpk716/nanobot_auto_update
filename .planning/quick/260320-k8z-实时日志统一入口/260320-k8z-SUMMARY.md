---
phase: 260320-k8z
plan: 01
subsystem: web-ui
tags: [home-page, instance-list, navigation, status-api]
dependency_graph:
  requires:
    - "InstanceManager.GetInstanceConfigs()"
    - "lifecycle.IsNanobotRunning()"
  provides:
    - "GET /"
    - "GET /logs"
    - "GET /api/v1/instances/status"
  affects:
    - "Web UI navigation flow"
tech_stack:
  added:
    - "home.html template"
    - "home.js frontend logic"
    - "InstanceStatus API struct"
  patterns:
    - "Embed FS for static files"
    - "Auto-refresh polling (5s interval)"
key_files:
  created:
    - "internal/web/static/home.html"
    - "internal/web/static/home.js"
  modified:
    - "internal/web/handler.go"
    - "internal/api/server.go"
    - "internal/instance/manager.go"
    - "internal/web/static/index.html"
    - "internal/web/static/app.js"
    - "internal/web/static/style.css"
decisions:
  - "使用网格布局展示实例卡片，自动响应式排列"
  - "每 5 秒自动刷新实例状态"
  - "实例名称可点击，直接跳转到日志详情页"
  - "详情页返回按钮使用 JavaScript 跳转而非 <a> 标签"
metrics:
  duration: "5m"
  completed_date: "2026-03-20"
  tasks_completed: 3
  files_modified: 6
  files_created: 2
---

# Phase 260320-k8z Plan 01: 实时日志统一入口 Summary

## 一句话概述

创建统一日志入口页面，展示所有实例列表及运行状态，支持点击导航至详情页，并提供返回首页按钮。

## 完成的工作

### Task 1: 实例状态 API (Commit: 3ea0e3a)

**实现内容：**
- 在 `InstanceManager` 中添加 `GetInstanceConfigs()` 方法，返回实例配置列表
- 创建 `InstanceStatus` 结构体，包含 name、port、running 字段
- 实现 `NewInstanceStatusHandler` 处理器，调用 `lifecycle.IsNanobotRunning()` 检测运行状态
- 在 `internal/api/server.go` 中注册 `GET /api/v1/instances/status` 路由

**关键文件：**
- `internal/instance/manager.go` - 新增 GetInstanceConfigs()
- `internal/web/handler.go` - 新增 InstanceStatus 和 NewInstanceStatusHandler
- `internal/api/server.go` - 注册新路由

### Task 2: 首页实例列表 (Commit: aaeca07)

**实现内容：**
- 创建 `home.html` 模板，使用网格布局展示实例卡片
- 创建 `home.js` 前端逻辑：
  - 调用 `/api/v1/instances/status` 获取实例列表
  - 渲染实例卡片，显示名称、端口、运行状态
  - 每 5 秒自动刷新状态
- 更新 `style.css` 添加首页样式：
  - 实例卡片网格布局（自动填充）
  - 运行状态指示器（绿色"运行中" / 灰色"已停止"）
  - 悬停效果
- 实现 `NewHomePageHandler` 处理器
- 注册 `GET /` 和 `GET /logs` 路由指向首页

**关键文件：**
- `internal/web/static/home.html` - 首页模板
- `internal/web/static/home.js` - 首页逻辑
- `internal/web/static/style.css` - 首页样式
- `internal/web/handler.go` - NewHomePageHandler
- `internal/api/server.go` - 首页路由

### Task 3: 详情页返回按钮 (Commit: 47ce3bd)

**实现内容：**
- 在 `index.html` 的 header 控制区域添加"返回首页"按钮
- 在 `app.js` 中添加按钮点击事件处理，跳转到 `/`

**关键文件：**
- `internal/web/static/index.html` - 添加返回按钮
- `internal/web/static/app.js` - 添加点击处理

## 偏差说明

**无偏差** - 计划完全按预期执行，所有功能一次实现成功。

## 验证结果

**构建验证：**
- ✅ `go build ./cmd/nanobot-auto-updater` 成功

**功能验证（待测试）：**
- [ ] 访问 `/` 或 `/logs` 显示首页
- [ ] 首页显示所有配置的实例列表
- [ ] 每个实例显示名称、端口、运行状态
- [ ] 点击实例名称跳转到 `/logs/{instance-name}`
- [ ] 详情页左上角有"返回首页"按钮
- [ ] 点击返回按钮跳转回首页
- [ ] 原有 `/logs/{instance-name}` 日志流功能正常

## 关键决策

1. **网格布局** - 使用 `grid-template-columns: repeat(auto-fill, minmax(280px, 1fr))` 实现响应式卡片布局
2. **自动刷新** - 使用 `setInterval(loadInstances, 5000)` 每 5 秒轮询状态 API
3. **状态检测** - 复用 `lifecycle.IsNanobotRunning()` 进行进程检测
4. **导航设计** - 首页实例名称使用 `<a>` 标签，详情页返回按钮使用 JavaScript 跳转

## 下一步建议

1. 在实际环境中测试所有路由和跳转
2. 验证实例状态检测准确性
3. 测试多实例场景下的状态刷新
4. 考虑添加实例启动/停止按钮（未来功能）

## 文件清单

**新增文件（2 个）：**
- `internal/web/static/home.html`
- `internal/web/static/home.js`

**修改文件（6 个）：**
- `internal/instance/manager.go`
- `internal/web/handler.go`
- `internal/api/server.go`
- `internal/web/static/index.html`
- `internal/web/static/app.js`
- `internal/web/static/style.css`

## 提交记录

- **3ea0e3a**: feat(260320-k8z): add instance status API with running state detection
- **aaeca07**: feat(260320-k8z): create home page with instance list
- **47ce3bd**: feat(260320-k8z): add back-to-home button on detail page

## Self-Check: PASSED

所有文件和提交均已验证：
- ✅ home.html 存在
- ✅ home.js 存在
- ✅ commit 3ea0e3a 存在
- ✅ commit aaeca07 存在
- ✅ commit 47ce3bd 存在
