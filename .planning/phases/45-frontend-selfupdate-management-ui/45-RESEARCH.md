# Phase 45: 前端 — 自更新管理 UI - Research

**Researched:** 2026-04-08
**Domain:** Vanilla HTML/CSS/JS 前端开发 + REST API 集成
**Confidence:** HIGH

## Summary

本阶段在 `home.html` 的 `<header>` 和 `<main>` 之间新增自更新管理区块。项目使用纯原生 HTML/CSS/JS（无框架），通过 `go:embed` 嵌入静态文件。前端需要与 Phase 44 建立的三个后端 API 端点交互：`/api/v1/web-config`（获取 auth token）、`/api/v1/self-update/check`（检测版本+轮询进度）、`/api/v1/self-update`（触发更新）。

现有前端代码已有成熟的 fetch API 调用模式、按钮状态管理模式和 polling 模式，自更新功能可以直接复用这些模式。CSS 使用 CSS 变量体系（spacing scale）和统一颜色方案（蓝色主色 `#2563eb`、成功绿 `#16a34a`、错误红 `#dc2626`），自更新区块需要严格复用这些样式变量。

**Primary recommendation:** 在 `home.js` 的 DOMContentLoaded 事件中初始化自更新模块，遵循现有的 fetch + try-catch + DOM 操作模式，使用 `setInterval` 进行进度轮询（500ms），复用 `style.css` 的 spacing 变量和按钮样式。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Header 下方独立区块 — 在 `<header>` 和 `<main>` 之间新增 `<section>` 元素
- **D-02:** 区块始终展开 — 不做折叠/展开功能
- **D-03:** 版本号以标签样式展示（如 `v0.9.0`），与现有 UI 按钮风格一致
- **D-04:** Release notes 截断显示（前 3-5 行），点击"展开"查看全部内容
- **D-05:** 发布日期以易读格式展示（如 "2026-04-07"）
- **D-06:** 下载进度使用蓝色进度条（#2563eb）+ 百分比文字，进度条约 300px 宽
- **D-07:** 状态阶段文字：检查中 -> 下载中 XX% -> 安装中 -> 完成/失败
- **D-08:** 成功反馈：绿色文字提示"更新完成，服务即将重启"，区块内显示
- **D-09:** 失败反馈：红色文字提示 + 错误信息，区块内显示。不使用 alert 弹窗
- **D-10:** 点击"立即更新"后开始 500ms 间隔轮询 `GET /api/v1/self-update/check`
- **D-11:** 轮询在 progress.stage 为 complete/failed 时停止，或超时 60 秒后停止
- **D-12:** 更新成功后 3 秒自动刷新页面
- **D-13:** 更新进行中禁用所有操作按钮（检测更新、立即更新）
- **D-14:** 页面加载时调用 `GET /api/v1/web-config` 获取 auth_token
- **D-15:** auth_token 存储在 JS 变量中，后续 API 调用使用 Bearer token 头
- **D-16:** web-config 获取失败时显示"请在本地访问"提示

### Claude's Discretion
- 进度条 CSS 动画细节（是否带过渡效果）
- 版本号标签的具体 CSS 样式
- 区块内元素的间距和排列细节
- 截断 release notes 的具体行数（3-5 行范围内）

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UI-01 | 自更新管理区域布局 — home.html 顶部新增区块 | 现有 HTML 结构已验证，在 `<header>` 和 `<main>` 之间插入 `<section>` |
| UI-02 | 当前版本显示 — 从 API 获取版本号标签展示 | `GET /api/v1/self-update/check` 响应含 `current_version` 字段 |
| UI-03 | 检测更新 — 调用 check API 显示版本详情 | 后端 `HandleCheck` 返回 `latest_version`、`release_notes`、`published_at` |
| UI-04 | 触发更新与进度显示 — POST 触发 + 阶段状态 | 后端 `HandleUpdate` 返回 202，`Progress` 结构含 stage/download_percent/error |
| UI-05 | 下载进度百分比 — 进度条 + 轮询 | 500ms 轮询 check 端点获取 `progress.download_percent`，CSS 进度条实现 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Vanilla HTML/CSS/JS | N/A | 前端 UI | 项目既有约定，无框架，纯原生实现 [VERIFIED: 代码库] |
| go:embed | Go 1.22+ | 静态文件嵌入 | 项目使用 `//go:embed static/*` 模式 [VERIFIED: handler.go L15] |

### Supporting
无额外库 — 项目前端不使用任何第三方 JS/CSS 库。

### Alternatives Considered
不适用 — CONTEXT.md 已锁定使用原生 HTML/CSS/JS，不引入框架。

**Installation:**
```bash
# 无需安装任何包 — 修改现有的 3 个静态文件即可
# internal/web/static/home.html
# internal/web/static/style.css
# internal/web/static/home.js
```

## Architecture Patterns

### Recommended Project Structure
```
internal/web/static/
├── home.html      # [修改] 新增自更新区块 section HTML
├── style.css      # [修改] 底部追加自更新区块 CSS 样式
├── home.js        # [修改] 新增自更新初始化、API 调用、轮询逻辑
├── index.html     # [不修改] 日志查看页面
└── app.js         # [不修改] 日志页面 JS
```

### Pattern 1: Fetch API 调用模式
**What:** 统一的 fetch + response.ok + data.success 判断模式
**When to use:** 所有 API 调用
**Example:**
```javascript
// Source: [VERIFIED: home.js L6-27]
async function loadInstances() {
    try {
        const response = await fetch('/api/v1/instances/status');
        const data = await response.json();
        // 处理数据...
    } catch (error) {
        console.error('Failed to load instance status:', error);
        // 用户可见提示
    }
}
```

### Pattern 2: 带认证的 Fetch 调用（新增模式）
**What:** Bearer token 认证头用于受保护端点
**When to use:** 调用 self-update/check 和 self-update 端点
**Example:**
```javascript
// 自更新 API 需要 Bearer token 认证
const response = await fetch('/api/v1/self-update/check', {
    headers: {
        'Authorization': `Bearer ${authToken}`
    }
});
```

### Pattern 3: 按钮状态管理模式
**What:** 禁用按钮 + 改变文字 + setTimeout 恢复
**When to use:** 检测更新、立即更新按钮
**Example:**
```javascript
// Source: [VERIFIED: home.js L64-99]
async function restartInstance(instanceName, button) {
    const originalText = button.textContent;
    button.disabled = true;
    button.classList.add('loading');
    button.textContent = '重启中...';
    // ... API 调用后恢复
    setTimeout(() => {
        button.textContent = originalText;
        button.disabled = false;
        button.classList.remove('loading');
    }, 2000);
}
```

### Pattern 4: Polling 轮询模式
**What:** setInterval 定时轮询 + clearInterval 停止
**When to use:** 更新进度轮询
**Example:**
```javascript
// Source: [VERIFIED: home.js L108 — setInterval 模式]
// 现有: setInterval(loadInstances, 5000);
// 自更新轮询:
const pollTimer = setInterval(async () => {
    const resp = await fetch('/api/v1/self-update/check', { headers });
    const data = await resp.json();
    updateProgressUI(data.progress);
    if (data.progress.stage === 'complete' || data.progress.stage === 'failed') {
        clearInterval(pollTimer);
    }
}, 500);
```

### Anti-Patterns to Avoid
- **使用 alert/prompt/confirm:** 项目约定用区块内文字提示（D-08, D-09），不用弹窗
- **引入第三方 CSS/JS 框架:** 项目为纯原生实现，保持一致
- **直接操作 style 属性:** 应使用 CSS 类名切换（现有模式使用 className 赋值）
- **使用 var 声明变量:** 现有代码使用 const/let，保持一致
- **内联事件处理器:** 现有代码使用 addEventListener，不用 onclick 属性

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP 请求 | XMLHttpRequest | fetch API | 现有代码全部使用 fetch [VERIFIED: home.js, app.js] |
| 进度条动画 | requestAnimationFrame 手写 | CSS transition width | 更简洁、性能更好 |
| 日期格式化 | 手写日期解析 | 直接使用后端返回的 published_at 格式 | 后端已格式化为 ISO 格式 |
| 文本截断 | JS 计算 + substring | CSS line-clamp + 展开/收起切换 | CSS 方案更稳定 |

**Key insight:** 项目前端极度精简，所有交互通过原生 fetch + DOM 操作完成。自更新功能严格遵循这一模式即可。

## Common Pitfalls

### Pitfall 1: web-config API 仅限 localhost
**What goes wrong:** 远程访问时 web-config 返回 403，前端无法获取 token
**Why it happens:** `localhostOnly` 中间件检查 `r.RemoteAddr`，仅允许 127.0.0.1/::1
**How to avoid:** web-config 调用失败时（D-16），显示"请在本地访问"提示并禁用自更新区域
**Warning signs:** fetch 返回 403 状态码

### Pitfall 2: 轮询超时未处理
**What goes wrong:** 后端更新卡住或重启失败未设置 failed 状态，轮询无限进行
**Why it happens:** 更新是异步 goroutine，极端情况可能不会设置终态
**How to avoid:** 实现 60 秒超时机制（D-11），超时后停止轮询并显示超时提示
**Warning signs:** 轮询计时器持续运行超过 60 秒

### Pitfall 3: 更新期间服务重启导致请求失败
**What goes wrong:** 更新完成后后端立即执行 `os.Exit(0)` 重启，此时轮询请求可能失败
**Why it happens:** `HandleUpdate` goroutine 最后调用 `restartFn` 直接退出进程
**How to avoid:** 轮询中检测到 complete 状态后立即停止轮询，启动 3 秒刷新计时器（D-12），期间请求失败不影响用户体验
**Warning signs:** complete 状态后的轮询请求返回网络错误

### Pitfall 4: 并发触发更新
**What goes wrong:** 用户快速多次点击"立即更新"按钮
**Why it happens:** 两次 POST 请求几乎同时发出
**How to avoid:** 点击后立即禁用按钮（D-13），后端也有 `TryLockUpdate` 保护（返回 409 Conflict）
**Warning signs:** 后端返回 409 状态码

### Pitfall 5: go:embed 缓存
**What goes wrong:** 修改静态文件后未重新编译，看不到变更
**Why it happens:** `go:embed` 在编译时嵌入文件
**How to avoid:** 开发时每次修改后重新 `go build` 或 `go run`，不要依赖文件服务器的缓存刷新
**Warning signs:** 修改 CSS/JS/HTML 后浏览器看不到变化

### Pitfall 6: Release notes 中包含 Markdown 格式
**What goes wrong:** GitHub Release body 通常是 Markdown 格式，直接 innerHTML 渲染有 XSS 风险
**Why it happens:** 后端直接返回 GitHub API 的 body 字段
**How to avoid:** 使用 textContent 而非 innerHTML 渲染 release notes；如需格式化，使用简单的换行替换或第三方 Markdown 库（推荐 textContent + 换行处理，保持简洁）
**Warning signs:** release notes 包含 HTML 标签或特殊字符

## Code Examples

### 后端 API 响应结构（Phase 44 已实现）

#### GET /api/v1/web-config（无需认证，localhost-only）
```json
{
    "auth_token": "some-bearer-token-value"
}
```
Source: [VERIFIED: webconfig_handler.go L10-13]

#### GET /api/v1/self-update/check（需 Bearer 认证）
```json
{
    "current_version": "v0.9.0",
    "latest_version": "v0.10.0",
    "needs_update": true,
    "release_notes": "## What's Changed\n- Added feature X\n- Fixed bug Y",
    "published_at": "2026-04-07T12:00:00Z",
    "download_url": "https://github.com/...",
    "self_update_status": "idle",
    "self_update_error": "",
    "progress": {
        "stage": "downloading",
        "download_percent": 45,
        "error": ""
    }
}
```
Source: [VERIFIED: selfupdate_handler.go L42-52]

#### POST /api/v1/self-update（需 Bearer 认证）
```json
// 成功返回 202 Accepted
{
    "status": "accepted",
    "message": "Self-update started"
}
// 冲突返回 409 (更新已在进行)
{
    "error": "conflict",
    "message": "An update is already in progress. Please try again later."
}
```
Source: [VERIFIED: selfupdate_handler.go L135-157]

### ProgressState 结构
```go
type ProgressState struct {
    Stage           string `json:"stage"`            // "idle" / "checking" / "downloading" / "installing" / "complete" / "failed"
    DownloadPercent int    `json:"download_percent"` // 0-100
    Error           string `json:"error,omitempty"`  // 仅 failed 状态
}
```
Source: [VERIFIED: selfupdate.go L80-85]

### 自更新区块 HTML 结构建议
```html
<!-- 插入位置: home.html 的 </header> 和 <main> 之间 -->
<section id="selfupdate-section" class="selfupdate-section">
    <div class="selfupdate-header">
        <span class="selfupdate-title">自更新管理</span>
        <span id="current-version" class="version-badge">--</span>
    </div>
    <div class="selfupdate-actions">
        <button id="btn-check-update">检测更新</button>
        <button id="btn-start-update" disabled>立即更新</button>
    </div>
    <div id="update-result" class="update-result" style="display:none">
        <!-- 动态填充：版本详情 / 进度 / 成功/失败提示 -->
    </div>
</section>
```

### 进度条 CSS 建议
```css
/* 复用项目 CSS 变量和颜色 */
.progress-bar-container {
    width: 300px;
    height: 8px;
    background-color: #e5e7eb;
    border-radius: 4px;
    overflow: hidden;
}

.progress-bar-fill {
    height: 100%;
    background-color: #2563eb; /* 项目主色 */
    border-radius: 4px;
    transition: width 0.3s ease; /* Claude's Discretion: 过渡效果 */
}
```

### 轮询逻辑建议
```javascript
let authToken = '';
let pollTimer = null;
let pollStartTime = 0;

// 页面加载时获取 token
async function initSelfUpdate() {
    try {
        const resp = await fetch('/api/v1/web-config');
        if (!resp.ok) throw new Error('web-config 不可用');
        const data = await resp.json();
        authToken = data.auth_token;
        // 启用自更新 UI
        loadCurrentVersion();
    } catch (e) {
        // D-16: 显示"请在本地访问"提示
        document.getElementById('selfupdate-section').innerHTML =
            '<p class="selfupdate-warning">请在本地访问以使用自更新功能</p>';
    }
}

// 检测更新
async function checkUpdate() {
    const resp = await fetch('/api/v1/self-update/check', {
        headers: { 'Authorization': `Bearer ${authToken}` }
    });
    const data = await resp.json();
    // 渲染版本详情...
}

// 轮询进度
function startProgressPolling() {
    pollStartTime = Date.now();
    pollTimer = setInterval(async () => {
        // D-11: 60 秒超时
        if (Date.now() - pollStartTime > 60000) {
            clearInterval(pollTimer);
            showTimeoutError();
            return;
        }
        const resp = await fetch('/api/v1/self-update/check', {
            headers: { 'Authorization': `Bearer ${authToken}` }
        });
        const data = await resp.json();
        updateProgressUI(data.progress);
        if (data.progress.stage === 'complete' || data.progress.stage === 'failed') {
            clearInterval(pollTimer);
            handleTerminalState(data.progress);
        }
    }, 500); // D-10: 500ms 间隔
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| XMLHttpRequest | fetch API | 2015+ | 项目已使用 fetch [VERIFIED: home.js] |
| var 声明 | const/let | ES6+ | 项目已使用 [VERIFIED: home.js] |
| innerHTML 渲染用户内容 | textContent 防 XSS | Always | Release notes 必须用 textContent |

**Deprecated/outdated:**
- 无 — 项目前端技术栈稳定，无过时用法。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | 后端 API 路由路径不会在本阶段发生变化 | API 集成 | 需要同步修改前端调用路径 |
| A2 | published_at 格式为 ISO 8601 (2006-01-02T15:04:05Z) | UI-03 日期展示 | 需调整前端日期解析逻辑 |

**If this table is empty:** All claims in this research were verified or cited -- no user confirmation needed.

## Open Questions

1. **Release notes 中的 Markdown 渲染**
   - What we know: GitHub Release body 是 Markdown 格式
   - What's unclear: 是否需要前端渲染 Markdown（加粗、链接等）
   - Recommendation: 使用 textContent 纯文本渲染 + 换行处理。如需渲染 Markdown，可引入 marked.js，但与项目"不引入框架"原则冲突。建议 D-04 截断显示时用纯文本。

2. **进度条动画精确行为**
   - What we know: D-06 指定蓝色进度条 #2563eb，约 300px 宽
   - What's unclear: 进度条是否需要平滑过渡（transition）
   - Recommendation: 添加 `transition: width 0.3s ease` 使进度条平滑增长，属于 Claude's Discretion 范围。

## Environment Availability

Step 2.6: SKIPPED (no external dependencies identified) — 本阶段仅修改 3 个静态文件（HTML/CSS/JS），不依赖外部工具或服务。

## Validation Architecture

> config.json 中 `workflow.nyquist_validation` 未设置，视为启用。

### Test Framework
| Property | Value |
|----------|-------|
| Framework | 无前端测试框架 — 项目为 Go 后端 + 嵌入式原生前端 |
| Config file | 无 |
| Quick run command | N/A — 手动浏览器测试 |
| Full suite command | N/A |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| UI-01 | 自更新区块正确渲染在 header 和 main 之间 | manual-only | 浏览器访问 / | N/A |
| UI-02 | 当前版本号显示为标签样式 | manual-only | 浏览器验证 | N/A |
| UI-03 | 检测更新按钮调用 check API 并展示版本详情 | manual-only | 浏览器 + DevTools Network | N/A |
| UI-04 | 触发更新按钮 POST 后显示阶段状态 | manual-only | 浏览器 + DevTools Network | N/A |
| UI-05 | 下载进度实时更新进度条和百分比 | manual-only | 浏览器 + DevTools Network | N/A |

**说明:** 项目前端为纯静态 HTML/CSS/JS 嵌入在 Go 二进制中，无前端测试基础设施。所有前端验证需通过手动浏览器测试完成。Go 后端 API 的单元测试已在 Phase 44 完成。

### Sampling Rate
- **Per task commit:** 手动浏览器验证
- **Per wave merge:** 完整浏览器功能测试（版本显示、检测、更新触发、进度轮询）
- **Phase gate:** 全部 UI-01 ~ UI-05 通过浏览器验证

### Wave 0 Gaps
None -- 本阶段为纯前端手动测试，不需要测试框架。

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Bearer token via web-config API (localhost-only) |
| V3 Session Management | yes | Token 存储在 JS 变量中，页面关闭即清除 |
| V4 Access Control | yes | web-config 端点 localhost-only 限制 (Phase 44 已实现) |
| V5 Input Validation | yes | Release notes 使用 textContent 防 XSS |
| V6 Cryptography | no | 不涉及前端加密操作 |

### Known Threat Patterns for Vanilla JS + REST API

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| XSS via release notes | Tampering | textContent 渲染，不使用 innerHTML |
| Token 泄露到非 localhost | Information Disclosure | localhostOnly 中间件 + JS 变量作用域 |
| CSRF on update trigger | Tampering | Bearer token 认证（非 cookie-based） |
| Open redirect via download_url | Spoofing | 不在前端跳转到 download_url，仅展示 |

## Sources

### Primary (HIGH confidence)
- 代码库 `internal/web/static/home.html` — 现有 HTML 结构
- 代码库 `internal/web/static/style.css` — CSS 变量和样式模式
- 代码库 `internal/web/static/home.js` — JS 调用模式、按钮状态管理、polling
- 代码库 `internal/api/webconfig_handler.go` — web-config API 实现
- 代码库 `internal/api/selfupdate_handler.go` — self-update API 实现
- 代码库 `internal/selfupdate/selfupdate.go` — ProgressState 结构

### Secondary (MEDIUM confidence)
- CONTEXT.md — Phase 45 用户决策（锁定选择）

### Tertiary (LOW confidence)
无

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 项目不使用任何第三方前端库，已通过代码库验证
- Architecture: HIGH — HTML/CSS/JS 结构和 API 响应格式均已通过代码验证
- Pitfalls: HIGH — 基于代码库实际实现分析，非假设性风险
- API 集成: HIGH — 所有 API 端点实现已在 Phase 44 完成并验证

**Research date:** 2026-04-08
**Valid until:** 2026-05-08 (stable — 纯前端实现，技术栈不会变化)
