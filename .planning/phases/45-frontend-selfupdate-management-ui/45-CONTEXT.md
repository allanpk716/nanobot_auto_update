# Phase 45: 前端 — 自更新管理 UI - Context

**Gathered:** 2026-04-08
**Status:** Ready for planning

<domain>
## Phase Boundary

home.html 顶部新增自更新管理区域（header 下方独立区块），包含当前版本显示、检测更新（版本号+日期+release notes）、触发更新与实时进度展示（蓝色进度条+下载百分比）。前端通过 web-config API 自动获取认证 token。

Requirements: UI-01, UI-02, UI-03, UI-04, UI-05
Depends on: Phase 44 (后端 ProgressState + web-config API)

</domain>

<decisions>
## Implementation Decisions

### 区块布局
- **D-01:** Header 下方独立区块 — 在 `<header>` 和 `<main>` 之间新增 `<section>` 元素，宽度撑满，与实例列表明确分隔
- **D-02:** 区块始终展开 — 不做折叠/展开功能，当前版本、按钮、检测结果/进度始终可见

### 版本信息展示
- **D-03:** 版本号以标签样式展示（如 `v0.9.0`），与现有 UI 按钮风格一致
- **D-04:** Release notes 截断显示（前 3-5 行），点击"展开"查看全部内容
- **D-05:** 发布日期以易读格式展示（如 "2026-04-07"）

### 进度展示与状态反馈
- **D-06:** 下载进度使用蓝色进度条（#2563eb，项目主色）+ 百分比文字，进度条约 300px 宽
- **D-07:** 状态阶段文字显示：检查中 → 下载中 XX% → 安装中 → 完成/失败
- **D-08:** 成功反馈：绿色文字提示"更新完成，服务即将重启"，区块内显示
- **D-09:** 失败反馈：红色文字提示 + 错误信息，区块内显示。不使用 alert 弹窗

### 轮询与更新流程
- **D-10:** 点击"立即更新"后开始 500ms 间隔轮询 `GET /api/v1/self-update/check`
- **D-11:** 轮询在 progress.stage 为 complete/failed 时停止，或超时 60 秒后停止
- **D-12:** 更新成功后 3 秒自动刷新页面（`setTimeout(() => location.reload(), 3000)`）
- **D-13:** 更新进行中禁用所有操作按钮（检测更新、立即更新），防止重复触发

### 认证流程
- **D-14:** 页面加载时调用 `GET /api/v1/web-config`（localhost-only，无需 auth）获取 auth_token
- **D-15:** auth_token 存储在 JS 变量中，后续 API 调用使用 `Authorization: Bearer <token>` 头
- **D-16:** web-config 获取失败时（非 localhost 访问），显示"请在本地访问"提示，自更新区域不可用

### Claude's Discretion
- 进度条 CSS 动画细节（是否带过渡效果）
- 版本号标签的具体 CSS 样式
- 区块内元素的间距和排列细节
- 截断 release notes 的具体行数（3-5 行范围内）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 44 后端产出 (Phase 45 依赖)
- `internal/api/webconfig_handler.go` — WebConfigHandler + localhostOnly 中间件，返回 auth_token
- `internal/api/selfupdate_handler.go` — HandleCheck (版本检查+进度) + HandleUpdate (触发更新)，SelfUpdateCheckResponse 结构含 progress 字段
- `internal/selfupdate/selfupdate.go` — ProgressState struct (stage/download_percent/error)，Updater.GetProgress() 方法

### 前端现有代码 (修改目标)
- `internal/web/static/home.html` — 主页面 HTML，header + instances-grid 结构
- `internal/web/static/style.css` — 所有 CSS 样式，spacing 变量、颜色、按钮样式
- `internal/web/static/home.js` — Home 页 JS，fetch API 调用模式、DOM 操作模式、polling 模式
- `internal/web/handler.go` — `//go:embed static/*` 静态文件嵌入

### API 路由注册
- `internal/api/server.go` — 路由注册，self-update 和 web-config 端点位置

### 认证
- `internal/api/auth.go` — authMiddleware + validateBearerToken

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `style.css` spacing 变量 (`--spacing-xs/sm/md/lg/xl`) — 自更新区块复用
- `style.css` 按钮样式 (`padding`, `border`, `hover/active/disabled` 状态) — 复用按钮模式
- `home.js` fetch + error handling 模式 — 自更新 API 调用复用
- `home.js` `loadInstances()` polling 模式 (`setInterval`) — 轮询模式参考
- `home.js` `restartInstance()` 按钮状态管理模式 (disable/restore) — 更新按钮复用

### Established Patterns
- 原生 HTML/CSS/JS，无框架 — 保持一致
- BEM-ish 命名：`.instance-card`, `.instance-name`, `.info-row`
- 蓝色主色 `#2563eb`，成功绿 `#16a34a`，错误红 `#dc2626`
- 按钮样式：白底、#ccc 边框、hover 变蓝边、disabled 时 opacity 0.6
- 卡片样式：白底、8px 圆角、hover 阴影
- fetch API 调用：`response.ok && data.success` 判断模式
- 错误处理：try-catch + console.error + 用户可见提示

### Integration Points
- `home.html <header>` 后新增 `<section>` — 自更新区块 HTML
- `style.css` 底部追加 — 自更新区块 CSS
- `home.js` DOMContentLoaded 初始化 — 新增 web-config token 获取 + 自更新按钮绑定
- `GET /api/v1/web-config` → 获取 auth_token → 存储为 JS 变量
- `GET /api/v1/self-update/check` (Bearer auth) → 检测版本 + 获取进度
- `POST /api/v1/self-update` (Bearer auth) → 触发更新 → 202 Accepted → 开始轮询

</code_context>

<specifics>
## Specific Ideas

- 区块始终展开、不做折叠 — 保持简洁，减少交互步骤
- 进度条颜色与项目主色 #2563eb 一致 — 视觉统一
- 更新成功后自动刷新页面 — 因为自更新后 exe 被替换，需要重启服务
- Release notes 截断 + 展开 — 平衡信息量和界面空间

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 45-frontend-selfupdate-management-ui*
*Context gathered: 2026-04-08*
