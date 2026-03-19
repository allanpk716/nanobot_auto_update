# Requirements: Nanobot Auto Updater v0.4

**Defined:** 2026-03-16
**Core Value:** 自动保持 nanobot 处于最新版本，无需用户手动干预

## v0.4 Requirements

v0.4 里程碑为 nanobot 实例添加实时日志查看功能，通过 SSE 流式传输和 Web UI 访问。

### Log Capture

- [ ] **CAPT-01**: 系统捕获 nanobot 进程的 stdout 输出流
- [ ] **CAPT-02**: 系统捕获 nanobot 进程的 stderr 输出流
- [ ] **CAPT-03**: 系统并发读取 stdout 和 stderr 管道，防止管道缓冲区满导致死锁
- [x] **CAPT-04**: 系统在 nanobot 进程启动时自动开始捕获输出
- [x] **CAPT-05**: 系统在 nanobot 进程停止时自动停止捕获输出

### Log Buffering

- [x] **BUFF-01**: 系统为每个 nanobot 实例维护独立的环形缓冲区 (Circular Buffer)
- [x] **BUFF-02**: 系统限制每个实例的缓冲区大小为 5000 行日志
- [x] **BUFF-03**: 系统使用线程安全的环形缓冲区实现，支持并发读写
- [x] **BUFF-04**: 系统在缓冲区满时自动覆盖最旧的日志 (FIFO)
- [x] **BUFF-05**: 系统为每条日志保留时间戳、来源 (stdout/stderr) 和内容

### SSE Streaming

- [x] **SSE-01**: 系统提供 `/api/v1/logs/:instance/stream` SSE 端点用于实时日志流
- [x] **SSE-02**: 系统使用 Server-Sent Events 协议推送日志到客户端
- [x] **SSE-03**: 系统每 30 秒发送 SSE 心跳注释防止连接超时
- [x] **SSE-04**: 系统检测客户端断开连接并停止发送事件
- [x] **SSE-05**: 系统在客户端连接时发送缓冲区中的历史日志
- [x] **SSE-06**: 系统将 stdout 和 stderr 分别标记为不同事件类型 (便于客户端区分)
- [x] **SSE-07**: 系统设置 HTTP WriteTimeout 为 0 以支持长连接

### Web UI

- [x] **UI-01**: 系统提供 `/logs/:instance` HTML 页面查看日志
- [ ] **UI-02**: 系统在 Web 页面自动滚动到最新日志 (类似 tail -f)
- [ ] **UI-03**: 用户可以通过按钮暂停和恢复自动滚动
- [ ] **UI-04**: 系统使用不同颜色区分 stdout 和 stderr 输出
- [x] **UI-05**: 系统在页面上显示 SSE 连接状态 (连接中/已连接/已断开)
- [x] **UI-06**: 系统将静态 HTML/CSS/JS 文件嵌入到 Go 二进制中 (单文件部署)
- [ ] **UI-07**: 系统提供实例选择下拉菜单，用于切换查看不同实例的日志

### Instance Management Integration

- [x] **INST-01**: 系统将 LogBuffer 集成到 InstanceLifecycle 结构中
- [x] **INST-02**: 系统在 InstanceManager 中管理所有实例的 LogBuffer
- [x] **INST-03**: 系统在实例启动时创建对应的 LogBuffer
- [x] **INST-04**: 系统在实例停止时保留 LogBuffer (可查看历史日志)
- [x] **INST-05**: 系统在实例重启时清空 LogBuffer (重新开始缓冲)

### Error Handling

- [ ] **ERR-01**: 系统在进程管道读取失败时记录错误日志并继续运行
- [ ] **ERR-02**: 系统在 SSE 客户端连接失败时记录警告日志并继续运行
- [ ] **ERR-03**: 系统在 LogBuffer 写入失败时记录错误日志并丢弃日志行
- [x] **ERR-04**: 系统在请求不存在的实例日志时返回 HTTP 404 Not Found

## v0.5 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Enhanced Features

- **SEAR-01**: 用户可以通过文本搜索过滤日志
- **SEAR-02**: 系统高亮显示搜索匹配的日志行
- **SEAR-03**: 系统支持正则表达式搜索

### Advanced Features

- **CONF-10**: 用户可以在配置文件中配置每个实例的缓冲区大小
- **UI-10**: 用户可以下载日志文件 (导出功能)
- **UI-11**: 系统提供暗色主题 UI
- **UI-12**: 用户可以同时查看多个实例的日志 (分屏视图)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| 无限日志历史 | 固定 5000 行环形缓冲防止 OOM，不需要持久化存储 |
| WebSocket 双向通信 | SSE 更简单，足够单向流式传输，WebSocket 增加不必要的复杂度 |
| 日志持久化到磁盘 | 实时查看器不负责存储，nanobot 自己管理日志文件 |
| 复杂查询语言 | 超出 MVP 范围，简单文本搜索足够，SQL-like 查询延迟到 v0.5+ |
| 日志认证 | 依赖现有 API Bearer Token 或 localhost 访问，无需额外认证层 |
| 多实例合并视图 | 单实例视图更清晰，合并视图导致日志交错难以调试 |
| 二进制日志格式 | SSE 仅支持文本，二进制格式增加序列化复杂度 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| BUFF-01 | Phase 19 | Complete |
| BUFF-02 | Phase 19 | Complete |
| BUFF-03 | Phase 19 | Complete |
| BUFF-04 | Phase 19 | Complete |
| BUFF-05 | Phase 19 | Complete |
| CAPT-01 | Phase 20 | Pending |
| CAPT-02 | Phase 20 | Pending |
| CAPT-03 | Phase 20 | Pending |
| CAPT-04 | Phase 20 | Complete |
| CAPT-05 | Phase 20 | Complete |
| INST-01 | Phase 21 | Complete |
| INST-02 | Phase 21 | Complete |
| INST-03 | Phase 21 | Complete |
| INST-04 | Phase 21 | Complete |
| INST-05 | Phase 21 | Complete |
| SSE-01 | Phase 22 | Complete |
| SSE-02 | Phase 22 | Complete |
| SSE-03 | Phase 22 | Complete |
| SSE-04 | Phase 22 | Complete |
| SSE-05 | Phase 22 | Complete |
| SSE-06 | Phase 22 | Complete |
| SSE-07 | Phase 22 | Complete |
| UI-01 | Phase 23 | Complete |
| UI-02 | Phase 23 | Pending |
| UI-03 | Phase 23 | Pending |
| UI-04 | Phase 23 | Pending |
| UI-05 | Phase 23 | Complete |
| UI-06 | Phase 23 | Complete |
| UI-07 | Phase 23 | Pending |
| ERR-01 | Phase 23 | Pending |
| ERR-02 | Phase 23 | Pending |
| ERR-03 | Phase 23 | Pending |
| ERR-04 | Phase 23 | Complete |

**Coverage:**
- v0.4 requirements: 33 total
- Mapped to phases: 33
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-16*
*Last updated: 2026-03-16 after roadmap creation*
