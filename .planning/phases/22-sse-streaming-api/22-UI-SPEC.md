---
phase: 22
slug: sse-streaming-api
status: draft
shadcn_initialized: false
preset: none
created: 2026-03-17
---

# Phase 22 — UI Design Contract (API 接口契约)

> 本阶段为后端 SSE API 实现,无可视化 UI 组件。此契约定义 API 接口规范、SSE 事件格式、错误响应格式,供 Phase 23 Web UI 集成使用。

---

## 设计系统

| Property | Value |
|----------|-------|
| Tool | none (后端 API 阶段) |
| Preset | not applicable |
| Component library | none |
| Icon library | none |
| Font | none |

**说明:** Phase 22 实现后端 SSE 端点,不涉及前端可视化组件。设计契约聚焦于 API 接口规范和数据格式约定。

---

## API 端点规范

### SSE 流式端点

| Property | Value |
|----------|-------|
| 路径 | `GET /api/v1/logs/:instance/stream` |
| 协议 | Server-Sent Events (SSE) |
| 内容类型 | `text/event-stream` |
| 超时 | WriteTimeout = 0 (无限时长连接) |
| 认证 | 无 (Out of Scope) |

### URL 路径参数

| 参数 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `instance` | string | nanobot 实例名称 | `nanobot-me`, `nanobot-dev` |

### HTTP 请求头

| Header | Value | Required |
|--------|-------|----------|
| `Accept` | `text/event-stream` | 否 (推荐) |
| `Cache-Control` | `no-cache` | 否 (服务器强制设置) |

### HTTP 响应头

| Header | Value | Purpose |
|--------|-------|---------|
| `Content-Type` | `text/event-stream` | SSE 协议标识 |
| `Cache-Control` | `no-cache` | 禁用缓存 |
| `Connection` | `keep-alive` | 保持长连接 |

---

## SSE 事件格式

### 事件类型定义

| 事件类型 | 用途 | 数据字段 |
|---------|------|---------|
| `connected` | 连接确认 | `{"instance": "实例名称"}` |
| `stdout` | 标准输出日志 | 日志内容 (纯文本) |
| `stderr` | 错误输出日志 | 日志内容 (纯文本) |

### SSE 事件格式规范

```
event: <事件类型>\n
data: <数据内容>\n
\n
```

**格式说明:**
- 每个事件必须以双换行符 `\n\n` 结束
- `event:` 字段用于区分 stdout 和 stderr (SSE-06)
- `data:` 字段包含日志内容或 JSON 数据

### 示例事件

**1. 连接确认事件**
```
event: connected
data: {"instance":"nanobot-me"}

```

**2. 标准输出日志**
```
event: stdout
data: 2026-03-17 14:30:00.123 [INFO] Application started

```

**3. 错误输出日志**
```
event: stderr
data: 2026-03-17 14:30:05.456 [ERROR] Connection failed

```

**4. 心跳注释 (SSE-03)**
```
: ping

```

**说明:** 心跳使用 SSE 注释格式 (`:` 开头),浏览器 EventSource 自动忽略,仅用于保持连接活跃。

---

## 间隔规范

### 心跳间隔

| 间隔类型 | 值 | Purpose |
|---------|---|---------|
| SSE 心跳 | 30 秒 | 防止代理和 load balancer 超时关闭连接 |

**实现:** 使用 `time.Ticker` 每 30 秒发送 `: ping\n\n` 注释。

---

## 数据格式

### LogEntry 数据结构 (来自 Phase 19)

```go
type LogEntry struct {
    Timestamp time.Time // 时间戳
    Source    string    // "stdout" 或 "stderr"
    Content   string    // 日志内容
}
```

### SSE 数据传输格式

| 字段 | SSE 传输方式 | 说明 |
|------|-------------|------|
| `Timestamp` | 不传输 | 客户端使用接收时间 |
| `Source` | `event:` 字段 | 映射为事件类型 (`stdout`/`stderr`) |
| `Content` | `data:` 字段 | 日志纯文本内容 |

**说明:** SSE 仅传输必要字段,客户端通过 `event:` 字段区分日志来源。

---

## 错误响应规范

### HTTP 错误码

| 状态码 | 场景 | 响应体 |
|--------|------|--------|
| 400 Bad Request | 缺少 `instance` 参数 | `Instance name required` |
| 404 Not Found | 实例不存在 (SSE-01, ERR-04) | `Instance {name} not found` |
| 500 Internal Server Error | 流式传输不支持 | `Streaming not supported` |

### 错误响应格式

**Content-Type:** `text/plain; charset=utf-8`

**示例:**
```
HTTP/1.1 404 Not Found
Content-Type: text/plain; charset=utf-8

Instance nanobot-unknown not found
```

---

## 客户端集成契约

### EventSource API 使用示例

```javascript
// 连接 SSE 端点
const instanceName = 'nanobot-me';
const eventSource = new EventSource(`/api/v1/logs/${instanceName}/stream`);

// 监听连接确认
eventSource.addEventListener('connected', (e) => {
    const data = JSON.parse(e.data);
    console.log('Connected to instance:', data.instance);
});

// 监听 stdout 日志
eventSource.addEventListener('stdout', (e) => {
    console.log('[STDOUT]', e.data);
    // Phase 23: 显示为绿色
});

// 监听 stderr 日志
eventSource.addEventListener('stderr', (e) => {
    console.error('[STDERR]', e.data);
    // Phase 23: 显示为红色
});

// 错误处理
eventSource.onerror = (e) => {
    if (eventSource.readyState === EventSource.CLOSED) {
        console.log('Connection closed');
    } else if (eventSource.readyState === EventSource.CONNECTING) {
        console.log('Reconnecting...');
    }
};
```

### 客户端实现约定

| 约定项 | 要求 |
|--------|------|
| 事件监听 | 必须分别监听 `stdout` 和 `stderr` 事件 |
| 自动重连 | EventSource 自动重连,无需手动实现 |
| 连接状态 | 使用 `EventSource.readyState` 检测状态 |
| 历史日志 | 服务器自动发送历史日志,客户端无需额外请求 |

---

## 行为契约

### 连接生命周期

| 阶段 | 服务器行为 | 客户端行为 |
|------|-----------|-----------|
| 连接建立 | 发送 `connected` 事件 + 历史日志 | 开始接收事件流 |
| 实时传输 | 新日志立即推送 (Flush) | 实时显示日志 |
| 心跳维持 | 每 30 秒发送 `: ping\n\n` | 自动忽略注释 |
| 客户端断开 | 检测 `ctx.Done()`,调用 `Unsubscribe()` | 关闭 EventSource |

### 并发连接

| 约定 | 说明 |
|------|------|
| 多客户端支持 | 同一实例支持多个并发 SSE 连接 |
| 资源清理 | 客户端断开时自动清理 LogBuffer 订阅 |
| Goroutine 管理 | 每个连接独立 goroutine,断开时退出 |

---

## 排版规范 (API 响应)

由于本阶段无可视化 UI,以下规范用于 API 响应文本:

| 属性 | 值 |
|------|---|
| 响应编码 | UTF-8 |
| 换行符 | `\n` (LF,Unix 风格) |
| JSON 缩进 | 无 (紧凑格式,减少传输量) |
| 时间格式 | RFC3339 (如 `2026-03-17T14:30:00Z`) |

---

## 注册表安全

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| 无外部注册表 | none | not applicable |

**说明:** Phase 22 使用 Go 标准库 `net/http` 实现,无需引入第三方 UI 组件库。

---

## 检查清单

### Phase 22 完成标准

- [ ] **SSE-01:** 实现 `/api/v1/logs/:instance/stream` 端点
- [ ] **SSE-02:** 正确设置 `Content-Type: text/event-stream`
- [ ] **SSE-03:** 每 30 秒发送心跳注释 `: ping\n\n`
- [ ] **SSE-04:** 检测客户端断开并清理资源 (Unsubscribe)
- [ ] **SSE-05:** 连接时发送 LogBuffer 历史日志
- [ ] **SSE-06:** stdout/stderr 使用不同事件类型
- [ ] **SSE-07:** HTTP 服务器 WriteTimeout = 0
- [ ] **ERR-04:** 实例不存在时返回 HTTP 404

### 集成验证

- [ ] LogBuffer.Subscribe() channel 正确转发到 SSE
- [ ] 客户端断开时无 goroutine 泄漏 (`runtime.NumGoroutine()` 不增长)
- [ ] 客户端能立即接收事件 (延迟 < 100ms)
- [ ] 长连接能持续 > 5 分钟
- [ ] EventSource 客户端能区分 stdout 和 stderr 事件

---

## Checker Sign-Off

- [x] Dimension 1 Copywriting: PASS (API 错误消息明确)
- [x] Dimension 2 Visuals: N/A (后端 API 阶段)
- [x] Dimension 3 Color: N/A (后端 API 阶段)
- [x] Dimension 4 Typography: N/A (后端 API 阶段)
- [x] Dimension 5 Spacing: N/A (后端 API 阶段)
- [x] Dimension 6 Registry Safety: PASS (无第三方依赖)

**Approval:** pending
