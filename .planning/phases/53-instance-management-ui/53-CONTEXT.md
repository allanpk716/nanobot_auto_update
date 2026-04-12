# Phase 53: Instance Management UI - Context

**Gathered:** 2026-04-12
**Status:** Ready for planning

<domain>
## Phase Boundary

在 Web 后台界面完整管理 nanobot 实例的 CRUD、启停控制和 nanobot 配置编辑。所有后端 API 已由 Phase 50-52 实现，本 phase 只构建前端 UI。

具体交付：
1. 重新设计的实例列表页 — 卡片展示完整配置详情 + 分层操作按钮
2. 新建实例对话框 — 两列表单 + nanobot 配置编辑
3. 编辑实例对话框 — 修改 auto-updater 配置字段
4. 复制实例对话框 — 克隆配置到新实例
5. 删除实例确认对话框 — 运行中实例警告
6. Nanobot 配置混合编辑器 — 结构化表单 + JSON textarea 左右分栏
7. Toast 通知反馈系统 — 操作结果反馈
8. "新建实例"页面级按钮入口

不包括：后端 API（Phase 50-52 已完成）、nanobot 配置 schema 验证（未来）、批量操作（未来）。

</domain>

<decisions>
## Implementation Decisions

### 实例卡片布局
- **D-01:** 卡片全部展示配置信息 — 名称、端口、启动命令、auto_start 标签、运行状态指示灯
- **D-02:** 操作按钮使用主要/次要分层 — 启动/停止为大按钮突出显示，编辑/复制/删除/配置为小按钮一行排列
- **D-03:** 整体布局保持现有 CSS Grid auto-fill + minmax(280px, 1fr) 响应式网格
- **D-04:** "新建实例"按钮为页面级按钮 — 放在实例网格上方，始终可见

### 对话框/模态系统
- **D-05:** 所有 CRUD 操作使用居中模态弹窗 — 背景半透明遮罩，点外部关闭
- **D-06:** 新建/编辑实例使用两列网格表单 — 字段分左右两列缩短高度（name/port 左列，start_command/startup_timeout/auto_start 右列）
- **D-07:** 删除确认使用简单确认对话框 — "确定删除实例 X？" + 运行中实例红色警告文字
- **D-08:** 复制实例使用与新建类似的对话框 — 预填充源实例配置，用户修改 name/port

### Nanobot 配置编辑器
- **D-09:** 混合编辑器使用左右分栏布局 — 左侧结构化表单编辑常用字段，右侧 JSON textarea 显示完整配置
- **D-10:** 结构化表单仅包含核心字段 — model、provider、默认 apiKey（首个非空 provider 的 apiKey）、gateway port、telegram token。其余字段通过右侧 JSON 编辑器修改
- **D-11:** JSON 编辑使用原生 textarea — 简单轻量，无第三方依赖，保持与项目原生 HTML/CSS/JS 技术栈一致
- **D-12:** 表单修改实时同步到右侧 JSON textarea — 用户可通过任一侧编辑保存
- **D-13:** 配置编辑器入口为卡片上独立的"配置"按钮 — 与编辑实例配置按钮分开，打开独立的 nanobot 配置编辑模态窗

### 操作反馈与状态刷新
- **D-14:** 操作结果使用 Toast 通知 — 页面右上角弹出通知条（成功绿色/失败红色），3 秒自动消失
- **D-15:** 实例状态保持现有 5 秒轮询策略 — 操作后立即触发一次额外刷新
- **D-16:** 启停操作使用按钮 loading 状态 — 点击后按钮显示 loading（禁用+旋转图标），API 返回后刷新卡片

### Claude's Discretion
- 模态弹窗的具体 CSS 样式和动画效果
- Toast 通知组件的具体实现（位置、堆叠、动画）
- 表单字段的输入验证规则和错误提示位置
- JSON textarea 的格式化和验证策略
- 新建实例时 nanobot 配置编辑器的集成方式（同一步骤 vs 分步骤向导）
- 卡片上操作按钮的具体排列顺序和间距
- 两列表单中字段的分组方式

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 后端 API（已实现）
- `internal/api/server.go` — 所有路由注册，包含 Phase 50-52 的 CRUD/lifecycle/nanobot-config 端点
- `internal/api/instance_config_handler.go` — InstanceConfigHandler CRUD 端点（GET/POST/PUT/DELETE/COPY /api/v1/instance-configs）
- `internal/api/instance_lifecycle_handler.go` — InstanceLifecycleHandler 启停端点（POST /api/v1/instances/{name}/start|stop）
- `internal/api/nanobot_config_handler.go` — NanobotConfigHandler 读写端点（GET/PUT /api/v1/instances/{name}/nanobot-config）
- `internal/api/auth.go` — AuthMiddleware() Bearer Token 认证

### 现有前端（需修改）
- `internal/web/static/home.html` — 实例列表页面结构（需大幅重写）
- `internal/web/static/home.js` — 实例列表逻辑（需大幅重写，保留自更新模块）
- `internal/web/static/style.css` — 共享样式（需扩展，新增模态/Toast/编辑器样式）
- `internal/web/handler.go` — HomePageHandler 和 InstanceListHandler（可能需调整）

### 配置数据结构
- `internal/config/instance.go` — InstanceConfig 结构体（了解字段名和类型）
- `internal/api/instance_config_handler_test.go` — API 请求/响应格式参考

### 认证
- `internal/api/webconfig_handler.go` — GET /api/v1/web-config 获取 Bearer Token（localhost-only）

### 参考模板
- `internal/api/selfupdate_handler.go` — 复杂 UI 交互参考（进度轮询、状态管理）
- `internal/nanobot/config_manager.go` — NanobotConfigManager（了解 nanobot config.json 结构和路径）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `home.js` 的自更新模块（`initSelfUpdate`、`checkUpdate`、`startUpdate`、`startProgressPolling`）— 保持不变
- `home.js` 的 `loadHeaderVersion()` — 保持不变
- `style.css` 的 CSS 变量系统（`--spacing-*`）和基础样式 — 扩展使用
- `fetch()` API 调用模式（Bearer token from `/api/v1/web-config`）— 复用
- 现有卡片 CSS（`.instance-card`、`.instances-grid`）— 扩展增强

### Established Patterns
- 原生 HTML/CSS/JS 无框架 — `document.createElement()` 动态创建 DOM
- `fetch()` + `Authorization: Bearer {token}` 认证 API 调用
- 5 秒 `setInterval` 轮询刷新状态
- `textContent` 防 XSS（已在自更新模块中使用）
- embed.FS 嵌入静态文件 — 修改 `internal/web/static/` 下的文件即可

### Integration Points
- `GET /api/v1/instances/status` — 当前实例状态 API（返回 name/port/running）
- `GET /api/v1/instance-configs` — 完整实例配置列表（Phase 50，返回所有配置字段）
- `POST /api/v1/instance-configs` — 创建实例（Phase 50）
- `PUT /api/v1/instance-configs/{name}` — 更新实例（Phase 50）
- `DELETE /api/v1/instance-configs/{name}` — 删除实例（Phase 50）
- `POST /api/v1/instance-configs/{name}/copy` — 复制实例（Phase 50）
- `POST /api/v1/instances/{name}/start` — 启动实例（Phase 51）
- `POST /api/v1/instances/{name}/stop` — 停止实例（Phase 51）
- `GET /api/v1/instances/{name}/nanobot-config` — 读取 nanobot 配置（Phase 52）
- `PUT /api/v1/instances/{name}/nanobot-config` — 更新 nanobot 配置（Phase 52）

</code_context>

<specifics>
## Specific Ideas

- 实例状态 API (`/api/v1/instances/status`) 只返回 name/port/running，卡片需要展示更多信息（command/auto_start），应改用 `/api/v1/instance-configs` 获取完整配置，再结合 status API 获取运行状态
- 模态弹窗需要通用组件（overlay + container + close button），在 home.js 中实现一个 `showModal(title, content)` 工具函数
- Toast 通知需要通用组件（堆叠管理 + 自动消失），在 home.js 中实现一个 `showToast(message, type)` 工具函数
- Nanobot 配置编辑器的左右分栏在模态弹窗内可能空间有限，弹窗宽度应设置为较大值（如 80vw 或 min 800px）
- 表单到 JSON 的同步需要双向：表单修改更新 JSON，JSON 修改也更新表单字段（如果 JSON 中对应字段存在）
- 两列网格表单布局：左列 name + start_command，右列 port + startup_timeout + auto_start 开关

</specifics>

<deferred>
## Deferred Ideas

- JSON 编辑器语法高亮 — 可引入轻量库（如 highlight.js）但当前不优先
- Nanobot 配置 schema 验证（ENC-01）— 未来里程碑
- 配置模板库（ENC-02）— 未来里程碑
- 批量操作 UI（AIM-02）— 未来里程碑
- 实例拖拽排序（AIM-03）— 未来里程碑
- 配置版本历史 UI（AIM-04）— 未来里程碑

</deferred>

---

*Phase: 53-instance-management-ui*
*Context gathered: 2026-04-12*
