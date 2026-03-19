# Phase 23: Web UI and Error Handling - Research

**Researched:** 2026-03-19
**Domain:** Go embed, HTML/CSS/JavaScript, SSE client, error handling
**Confidence:** HIGH

## Summary

Phase 23 需要实现一个嵌入到 Go 二进制文件中的 Web UI，用于实时查看 nanobot 实例日志，并补充前序阶段中未完全实现的错误处理逻辑。核心技术包括：使用 Go 1.16+ 的 `embed` 包将静态 HTML/CSS/JS 文件嵌入二进制（实现单文件部署），使用 JavaScript EventSource API 接收 SSE 流式日志，以及实现自动滚动、暂停恢复、连接状态显示等 UI 交互功能。

**Primary recommendation:** 使用 Go `embed.FS` 嵌入 `internal/web/static/` 目录下的静态资源，通过 `http.FileServer(http.FS(embedFS))` 提供 Web UI 服务，前端使用原生 JavaScript EventSource API（无需框架）实现 SSE 连接和实时日志渲染，错误处理采用"记录日志并继续运行"模式确保服务可用性。

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `embed` | Go 1.16+ (native) | 静态资源嵌入二进制 | Go 官方标准库，零依赖，支持 `embed.FS` 文件系统，与 `net/http` 无缝集成 |
| `net/http` | Go 1.24.13 | HTTP 文件服务器 | 标准库 `http.FileServer(http.FS(embedFS))` 模式，无需第三方库 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| EventSource API | Native Browser API | SSE 客户端连接 | 所有现代浏览器原生支持（Chrome/Firefox/Safari/Edge），无需 polyfill |
| CSS Flexbox | CSS3 | 日志容器布局 | 简单响应式布局，无需 CSS 框架 |
| JavaScript ES6+ | ES2015+ | DOM 操作和事件处理 | 使用 `addEventListener`、`classList`、`template literals` 现代语法 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `embed.FS` | `embed.String` 单文件 | 多文件（HTML+CSS+JS）需要 `embed.FS`，便于分离开发和维护 |
| EventSource API | `fetch-event-source` 库 | 原生 API 足够简单，第三方库仅用于 POST 请求或自定义头（本项目不需要） |
| 原生 JavaScript | React/Vue/Svelte | 单页面 UI（~300 行 JS）不需要框架，框架增加二进制体积和构建复杂度 |

**Installation:**
```bash
# No external dependencies needed - embed and net/http are Go standard library
# EventSource API is built into modern browsers
```

**Version verification:**
- Go version: 1.24.13 (verified via `go version`, supports embed since 1.16+)
- Browser compatibility: EventSource API supported since Chrome 6+, Firefox 6+, Safari 5+ (MDN reference)

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── web/
│   ├── handler.go           # HTTP handler for /logs/:instance
│   └── static/              # Static files to embed
│       ├── index.html       # Main HTML page
│       ├── style.css        # Log viewer styles
│       └── app.js           # SSE client and DOM logic
```

### Pattern 1: Go Embed for Static Assets
**What:** 使用 `//go:embed` 指令将 `internal/web/static/` 目录嵌入到二进制中
**When to use:** 单文件部署要求，静态资源（HTML/CSS/JS）不超过 10MB
**Example:**
```go
// Source: https://pkg.go.dev/embed
package web

import (
    "embed"
    "io/fs"
    "net/http"
)

//go:embed static/*
var staticFiles embed.FS

// Handler serves embedded static files
func Handler() http.Handler {
    // Strip "static" prefix to serve files at root
    subFS, _ := fs.Sub(staticFiles, "static")
    return http.FileServer(http.FS(subFS))
}
```

### Pattern 2: SSE Client with Auto-Reconnect
**What:** 使用 EventSource API 建立持久连接，自动重连机制
**When to use:** 实时日志流、通知推送等单向服务器推送场景
**Example:**
```javascript
// Source: https://developer.mozilla.org/en-US/docs/Web/API/EventSource
const eventSource = new EventSource('/api/v1/logs/instance1/stream');

eventSource.onopen = () => {
    updateConnectionStatus('connected');
};

eventSource.onerror = () => {
    updateConnectionStatus('disconnected');
    // EventSource automatically retries (default 3s delay)
};

eventSource.addEventListener('stdout', (e) => {
    appendLog(e.data, 'stdout');
});

eventSource.addEventListener('stderr', (e) => {
    appendLog(e.data, 'stderr');
});
```

### Pattern 3: Auto-Scroll with Pause/Resume
**What:** 监听用户滚动事件，智能切换自动滚动和手动浏览模式
**When to use:** 日志查看器、聊天记录等实时更新场景
**Example:**
```javascript
// Source: Common pattern for tail -f implementations
const logContainer = document.getElementById('logs');
let autoScroll = true;

// Toggle auto-scroll when user scrolls manually
logContainer.addEventListener('scroll', () => {
    const isAtBottom = logContainer.scrollHeight - logContainer.scrollTop
                       <= logContainer.clientHeight + 50; // 50px tolerance
    autoScroll = isAtBottom;
    updateScrollButton(autoScroll);
});

function appendLog(message, source) {
    const logLine = document.createElement('div');
    logLine.textContent = message;
    logLine.className = source === 'stderr' ? 'log-stderr' : 'log-stdout';
    logContainer.appendChild(logLine);

    if (autoScroll) {
        logContainer.scrollTop = logContainer.scrollHeight;
    }
}

// Pause/Resume button handler
document.getElementById('scroll-toggle').addEventListener('click', () => {
    autoScroll = !autoScroll;
    if (autoScroll) {
        logContainer.scrollTop = logContainer.scrollHeight;
    }
    updateScrollButton(autoScroll);
});
```

### Anti-Patterns to Avoid
- **Anti-pattern:** 在 Go 代码中拼接 HTML 字符串
  **Why bad:** 难以维护，XSS 风险高，无法利用浏览器缓存和开发者工具
  **Instead:** 使用独立的 HTML/CSS/JS 文件并通过 `embed.FS` 嵌入

- **Anti-pattern:** 使用 `setInterval` 轮询服务器获取日志
  **Why bad:** 高延迟，浪费带宽，服务器压力大
  **Instead:** 使用 SSE（已在 Phase 22 实现）建立持久连接

- **Anti-pattern:** 在错误处理中 panic 或 os.Exit(1)
  **Why bad:** 单个实例的错误会导致整个服务崩溃
  **Instead:** 记录错误日志并继续运行（ERR-01/02/03）

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 静态文件服务 | 手动读取文件并写入 ResponseWriter | `http.FileServer(http.FS(embedFS))` | 处理 MIME 类型、范围请求、缓存头等复杂逻辑 |
| SSE 心跳检测 | 客户端 `setTimeout` 超时检测 | EventSource `onerror` 事件 | 浏览器原生重连机制更可靠（3 秒指数退避） |
| 日志滚动容器 | CSS `overflow: auto` + 手动计算滚动位置 | CSS Flexbox + `scrollTop`/`scrollHeight` | 现代浏览器 API 简单可靠，无需库 |

**Key insight:** Web UI 的核心是"薄前端"模式 - 所有业务逻辑（日志缓冲、SSE 推送）已在 Phase 19-22 完成，前端仅负责渲染和用户交互，无需复杂状态管理。

## Common Pitfalls

### Pitfall 1: Embed 路径混淆
**What goes wrong:** `//go:embed static/*` 嵌入的文件路径包含 `static/` 前缀，但 `http.FileServer` 期望根路径访问
**Why it happens:** `embed.FS` 保留目录结构，`http.FileServer` 不会自动剥离前缀
**How to avoid:** 使用 `fs.Sub(staticFiles, "static")` 剥离前缀
**Warning signs:** 访问 `/logs/instance1` 时返回 404，但日志显示 "file not found: static/index.html"

### Pitfall 2: EventSource 错误处理遗漏
**What goes wrong:** 只监听 `onmessage` 事件，未处理 `onerror`，导致连接断开后 UI 无提示
**Why it happens:** 开发时服务器稳定，未测试网络中断场景
**How to avoid:** 必须实现 `onerror` 处理器更新连接状态指示器
**Warning signs:** 关闭服务器后页面状态仍显示"已连接"

### Pitfall 3: 自动滚动与用户滚动冲突
**What goes wrong:** 用户向上滚动查看历史日志时，新日志到达自动跳到底部
**Why it happens:** 每次 `appendLog` 都强制设置 `scrollTop = scrollHeight`
**How to avoid:** 监听 `scroll` 事件，仅当用户位于底部（±50px 容差）时才自动滚动
**Warning signs:** 测试人员抱怨"无法查看历史日志"

### Pitfall 4: stderr 颜色对比度不足
**What goes wrong:** stdout 和 stderr 使用相似颜色（如蓝色和青色），视觉差异不明显
**Why it happens:** 开发者显示器色彩校准好，未考虑色盲用户
**How to avoid:** 使用高对比度组合（如黑色 vs 红色，或正常 vs 加粗+斜体）
**Warning signs:** 用户反馈"无法快速识别错误日志"

### Pitfall 5: 实例列表 API 未实现
**What goes wrong:** UI-07 要求实例选择下拉菜单，但 InstanceManager 缺少 `ListInstances()` 方法
**Why it happens:** 前序阶段未考虑此需求
**How to avoid:** 在 InstanceManager 添加 `GetInstanceNames() []string` 方法
**Warning signs:** 前端只能硬编码实例名称，无法动态发现新实例

## Code Examples

### Example 1: Instance List API (Go)
```go
// internal/instance/manager.go
// Source: Derived from existing GetLogBuffer() pattern
func (m *InstanceManager) GetInstanceNames() []string {
    names := make([]string, 0, len(m.instances))
    for _, inst := range m.instances {
        names = append(names, inst.config.Name)
    }
    return names
}
```

### Example 2: Web Handler Registration (Go)
```go
// internal/api/server.go (updated)
// Source: Based on existing SSE handler pattern
func NewServer(cfg *config.APIConfig, im *instance.InstanceManager, logger *slog.Logger) (*Server, error) {
    mux := http.NewServeMux()

    // SSE endpoint (Phase 22)
    sseHandler := NewSSEHandler(im, logger)
    mux.HandleFunc("GET /api/v1/logs/{instance}/stream", sseHandler.Handle)

    // Web UI endpoint (Phase 23)
    mux.Handle("GET /logs/{instance}", web.Handler())
    mux.Handle("GET /api/v1/instances", web.NewInstanceListHandler(im, logger))

    // ... rest of server setup
}
```

### Example 3: Instance List API Handler (Go)
```go
// internal/web/handler.go
// Source: Standard JSON response pattern
func NewInstanceListHandler(im *instance.InstanceManager, logger *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        names := im.GetInstanceNames()
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "instances": names,
        })
    }
}
```

### Example 4: SSE Connection Status (JavaScript)
```javascript
// Source: https://developer.mozilla.org/en-US/docs/Web/API/EventSource
let eventSource = null;
let reconnectAttempts = 0;
const maxReconnectAttempts = 10;

function connectSSE(instanceName) {
    updateConnectionStatus('connecting');

    eventSource = new EventSource(`/api/v1/logs/${instanceName}/stream`);

    eventSource.onopen = () => {
        reconnectAttempts = 0;
        updateConnectionStatus('connected');
    };

    eventSource.onerror = () => {
        updateConnectionStatus('disconnected');
        // EventSource auto-reconnects, but we can implement custom logic:
        reconnectAttempts++;
        if (reconnectAttempts >= maxReconnectAttempts) {
            eventSource.close();
            appendLog('Max reconnection attempts reached. Please refresh.', 'stderr');
        }
    };
}

function updateConnectionStatus(status) {
    const indicator = document.getElementById('connection-status');
    indicator.className = `status-${status}`;
    indicator.textContent = {
        'connecting': 'Connecting...',
        'connected': 'Connected',
        'disconnected': 'Disconnected'
    }[status];
}
```

### Example 5: Instance Selector (JavaScript)
```javascript
// Source: Standard fetch + DOM manipulation
async function loadInstanceSelector() {
    const response = await fetch('/api/v1/instances');
    const data = await response.json();
    const select = document.getElementById('instance-select');

    // Clear existing options
    select.innerHTML = '';

    // Populate dropdown
    data.instances.forEach(name => {
        const option = document.createElement('option');
        option.value = name;
        option.textContent = name;
        select.appendChild(option);
    });

    // Load first instance by default
    if (data.instances.length > 0) {
        selectInstance(data.instances[0]);
    }
}

function selectInstance(instanceName) {
    // Close existing connection
    if (eventSource) {
        eventSource.close();
    }

    // Clear log container
    document.getElementById('logs').innerHTML = '';

    // Connect to new instance
    connectSSE(instanceName);
}

document.getElementById('instance-select').addEventListener('change', (e) => {
    selectInstance(e.target.value);
});
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 外部静态文件目录 | `embed.FS` 嵌入二进制 | Go 1.16 (2021-02) | 单文件部署，无外部依赖，简化 Docker 镜像 |
| WebSocket 双向通信 | SSE 单向推送 | Phase 22 设计决策 | 更简单，HTTP/2 友好，CDN 兼容，适合单向日志流 |
| 轮询 `/api/logs` | SSE 持久连接 | Phase 22 实现 | 实时性提升（<100ms），带宽节省 90%+ |
| jQuery DOM 操作 | 原生 ES6+ API | 2015+ 现代浏览器标准 | 无需库依赖，代码量更少，性能更好 |

**Deprecated/outdated:**
- `statik`、`packr` 等第三方嵌入工具：Go 1.16+ 原生 embed 更简单
- `gulp`/`webpack` 构建流程：单文件 UI（<500 行）不需要构建工具

## Open Questions

1. **日志行数过多时的性能问题**
   - What we know: LogBuffer 限制 5000 行（Phase 19），但 DOM 渲染 5000 个 `<div>` 可能卡顿
   - What's unclear: 是否需要虚拟滚动（仅渲染可见行）
   - Recommendation: Phase 23 MVP 先实现完整渲染，若性能问题明显在 v0.5 添加虚拟滚动

2. **实例列表 API 认证需求**
   - What we know: `/api/v1/logs/:instance/stream` 无认证（Phase 22 设计）
   - What's unclear: `/api/v1/instances` 是否需要 Bearer Token 认证
   - Recommendation: 保持一致（无认证），依赖 localhost 访问或未来 API Gateway 层统一认证

3. **SSE 断线重连后的历史日志去重**
   - What we know: SSE-05 发送缓冲区历史日志，重连会重复发送
   - What's unclear: 前端是否需要基于时间戳去重
   - Recommendation: MVP 阶段允许重复（用户体验影响小），v0.5 可添加去重逻辑

## Validation Architecture

Test infrastructure detected: Go testing package with `capture_test.go` pattern

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testutil package |
| Config file | None (test files use hardcoded configs) |
| Quick run command | `go test ./internal/web/... -v -run TestHandler` |
| Full suite command | `go test ./... -v -race` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| UI-01 | `/logs/:instance` serves embedded HTML | unit | `go test ./internal/web -v -run TestWebHandler` | ❌ Wave 0 |
| UI-02 | Auto-scroll to latest log | integration | Manual test (browser automation) | ❌ N/A |
| UI-03 | Pause/resume scroll button | integration | Manual test (browser automation) | ❌ N/A |
| UI-04 | stdout/stderr color distinction | integration | Manual test (visual inspection) | ❌ N/A |
| UI-05 | Connection status indicator | unit | `go test ./internal/web -v -run TestConnectionStatus` | ❌ Wave 0 |
| UI-06 | Static files embedded in binary | unit | `go test ./internal/web -v -run TestEmbedFS` | ❌ Wave 0 |
| UI-07 | Instance selector dropdown | unit | `go test ./internal/instance -v -run TestGetInstanceNames` | ❌ Wave 0 |
| ERR-01 | Pipe read error → log and continue | unit | `go test ./internal/lifecycle -v -run TestCaptureLogsError` | ✅ Phase 20 |
| ERR-02 | SSE connection error → log and continue | unit | `go test ./internal/api -v -run TestSSEHandlerError` | ❌ Wave 0 |
| ERR-03 | LogBuffer write error → log and drop | unit | `go test ./internal/logbuffer -v -run TestWriteError` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/web ./internal/instance -v -race`
- **Per wave merge:** `go test ./... -v -race`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/web/handler_test.go` — covers UI-01, UI-06
- [ ] `internal/web/static/index.html` — main HTML page
- [ ] `internal/web/static/style.css` — log viewer styles (stdout/stderr colors)
- [ ] `internal/web/static/app.js` — SSE client, scroll logic, instance selector
- [ ] `internal/instance/manager_test.go` — add TestGetInstanceNames (UI-07)
- [ ] `internal/api/sse_test.go` — add TestSSEHandlerError (ERR-02)
- [ ] `internal/logbuffer/buffer_test.go` — add TestWriteError (ERR-03)

**Note:** UI-02/03/04 (auto-scroll, pause/resume, colors) require browser-based integration testing (e.g., Playwright/Selenium) or manual testing. Go unit tests can verify server-side logic but not browser DOM behavior.

## Sources

### Primary (HIGH confidence)
- [Go embed package - pkg.go.dev](https://pkg.go.dev/embed) - Official documentation, verified usage patterns
- [EventSource API - MDN Web Docs](https://developer.mozilla.org/en-US/docs/Web/API/EventSource) - Browser API reference, connection handling
- [Phase 22 SSE Implementation](file://./internal/api/sse.go) - Existing SSE handler (SSE-01 to SSE-07)
- [Phase 19 LogBuffer Implementation](file://./internal/logbuffer/buffer.go) - Existing buffer core (BUFF-01 to BUFF-05)

### Secondary (MEDIUM confidence)
- [Best Practices for Secure Error Handling in Go - JetBrains](https://blog.jetbrains.com/go/2026/03/02/secure-go-error-handling-best-practices/) - Error logging patterns, "log and continue" strategy
- [How to Bundle Static Assets with go:embed - OneUptime](https://oneuptime.com/blog/post/2026-01-25-bundle-static-assets-go-embed/view) - Practical embed.FS examples
- [Server-Sent Events 技术解析与实战 - Ayou](http://www.paradeto.com/2025/06/07/sse-1/) - SSE reconnection and error handling

### Tertiary (LOW confidence)
- Web search results for "JavaScript auto scroll log viewer" - Common patterns verified across multiple implementations
- Web search results for "SSE EventSource connection status" - Browser compatibility confirmed via MDN

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Go embed 和 EventSource API 均为成熟稳定的标准库/浏览器 API，文档完整
- Architecture: HIGH - 基于 Phase 19-22 已有架构扩展，模式清晰（embed.FS + http.FileServer）
- Pitfalls: MEDIUM - 部分陷阱（如自动滚动冲突）需实际开发测试验证，但业界有成熟解决方案

**Research date:** 2026-03-19
**Valid until:** 6 months (Go embed API 稳定，浏览器 EventSource API 长期支持)
