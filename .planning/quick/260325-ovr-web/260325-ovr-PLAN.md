---
phase: 260325-ovr-web
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/web/static/index.html
  - internal/web/static/app.js
  - internal/web/static/home.js
  - internal/web/static/style.css
  - internal/api/server.go
  - internal/web/handler.go
autonomous: true
requirements: []
user_setup: []

must_haves:
  truths:
    - "User can see a restart button for each instance"
    - "User can click restart button to restart that specific instance"
    - "Logs display without ANSI escape codes (readable text)"
  artifacts:
    - path: "internal/web/static/index.html"
      provides: "Restart button in log viewer header"
    - path: "internal/web/static/home.js"
      provides: "Restart button click handler"
    - path: "internal/api/server.go"
      provides: "Restart API endpoint registration"
    - path: "internal/web/handler.go"
      provides: "Restart endpoint handler"
  key_links:
    - from: "home.js / app.js"
      to: "/api/v1/instances/{name}/restart"
      via: "POST fetch"
      pattern: "fetch.*restart.*method.*POST"
---

<objective>
在 Web 界面添加实例重启功能和修复 ANSI 颜色转义码显示问题

Purpose: 用户发现实例启动后日志显示故障时，可以直接在 Web 界面点击重启按钮快速解决问题；同时修复日志中 ANSI 转义码显示问题，提升用户体验
Output: 带重启按钮的实例卡片和首页，干净的日志显示
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md

## 现有代码结构

### InstanceLifecycle (internal/instance/lifecycle.go)
```go
// 已有方法用于重启单个实例
func (il *InstanceLifecycle) StopForUpdate(ctx context.Context) error
func (il *InstanceLifecycle) StartAfterUpdate(ctx context.Context) error
```

### InstanceManager (internal/instance/manager.go)
```go
// 已有方法获取实例
func (m *InstanceManager) GetLogBuffer(instanceName string) (*logbuffer.LogBuffer, error)
// 实例列表存储在 m.instances ([]*InstanceLifecycle)
```

### API Server (internal/api/server.go)
- 使用 `http.ServeMux` 注册路由
- 已有模式: `mux.HandleFunc("POST /api/v1/trigger-update", ...)`

### Web Handler (internal/web/handler.go)
- 使用 `NewInstanceStatusHandler` 模式创建处理器

### 前端结构
- home.js: 首页实例卡片
- app.js: 日志查看页面
- style.css: 共享样式

### ANSI 转义码示例
日志中出现: `[32m2026-03-25 17:47:55.823[0m | [34m[1mDEBUG   [0m`
这是 Python logging 模块彩色输出的 ANSI 转义码，需要在前端过滤或转换
</context>

<tasks>

<task type="auto">
  <name>Task 1: 添加重启 API 端点</name>
  <files>internal/api/server.go, internal/web/handler.go</files>
  <action>
1. 在 handler.go 中添加 `NewInstanceRestartHandler` 函数:
   - 接收 instanceName 从 URL PathValue
   - 通过 InstanceManager 获取对应实例
   - 调用实例的 StopForUpdate 然后 StartAfterUpdate
   - 返回 JSON 响应 {success: true} 或 {error: string}

2. 在 server.go 中注册路由:
   - `POST /api/v1/instances/{name}/restart`
   - 不需要认证 (与现有 instances 端点保持一致)

3. 在 InstanceManager 中添加 `GetLifecycle(name string) (*InstanceLifecycle, error)` 方法用于获取单个实例

注意: 重启是阻塞操作，使用 context.Background() 作为上下文
  </action>
  <verify>
    <automated>go build ./...</automated>
  </verify>
  <done>
    - POST /api/v1/instances/{name}/restart 端点可用
    - 调用成功返回实例停止然后启动的结果
    - 实例不存在返回 404
  </done>
</task>

<task type="auto">
  <name>Task 2: 添加重启按钮到前端界面</name>
  <files>internal/web/static/home.js, internal/web/static/app.js, internal/web/static/index.html, internal/web/static/style.css</files>
  <action>
1. 在 style.css 中添加按钮样式:
   - `.btn-restart`: 重启按钮样式 (与现有 button 保持一致)
   - `.btn-restart:disabled`: 禁用状态
   - `.btn-restart.loading`: 加载中状态

2. 在 home.js 的 createInstanceCard 函数中:
   - 在实例卡片底部添加重启按钮
   - 按钮点击时调用 POST /api/v1/instances/{name}/restart
   - 点击后禁用按钮并显示"重启中..."，完成后恢复
   - 成功后刷新实例状态

3. 在 index.html 的 header controls 区域添加重启按钮:
   - 在 scroll-toggle 按钮旁边添加
   - 仅在查看日志页显示

4. 在 app.js 中添加重启功能:
   - 添加 restartInstance 函数
   - 调用 POST /api/v1/instances/{name}/restart
   - 重启后重新连接 SSE 流
  </action>
  <verify>
    <automated>go build ./...</automated>
  </verify>
  <done>
    - 首页每个实例卡片显示重启按钮
    - 日志页面 header 显示重启按钮
    - 点击按钮触发重启 API 调用
    - 按钮状态正确反映操作进度
  </done>
</task>

<task type="auto">
  <name>Task 3: 处理 ANSI 颜色转义码</name>
  <files>internal/web/static/app.js</files>
  <action>
在 app.js 的 appendLog 函数中添加 ANSI 转义码处理:

1. 添加 `stripAnsiCodes` 函数:
   - 使用正则表达式移除所有 ANSI 转义序列
   - 正则: `/\x1b\[[0-9;]*m/g` (匹配 ESC[...m 格式)

2. 在 appendLog 中调用 stripAnsiCodes 处理 message:
   - 在设置 textContent 之前清理日志文本

参考日志示例:
- 输入: `[32m2026-03-25 17:47:55.823[0m | [34m[1mDEBUG   [0m`
- 输出: `2026-03-25 17:47:55.823 | DEBUG`
  </action>
  <verify>
    <automated>go build ./...</automated>
  </verify>
  <done>
    - 日志中不再显示 ANSI 转义码
    - 日志文本干净可读
  </done>
</task>

</tasks>

<verification>
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
</verification>

<success_criteria>
- [ ] POST /api/v1/instances/{name}/restart 端点可用
- [ ] 首页实例卡片显示重启按钮
- [ ] 日志页面显示重启按钮
- [ ] 重启按钮可正常触发实例重启
- [ ] 日志中 ANSI 转义码被正确过滤
</success_criteria>

<output>
After completion, create `.planning/quick/260325-ovr-web/260325-ovr-SUMMARY.md`
</output>
