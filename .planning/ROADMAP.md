# Roadmap: Nanobot Auto Updater v0.4

**Milestone:** v0.4 实时日志查看
**Created:** 2026-03-16
**Granularity:** Standard

## Overview

v0.4 里程碑为现有的 nanobot-auto-updater 应用添加实时日志查看功能。通过三层架构设计（日志捕获层、环形缓冲层、SSE 流式传输层），用户可以通过 HTTP API 和 Web UI 实时查看 nanobot 实例的 stdout/stderr 输出，保留最近 5000 行日志历史。

**Requirements Coverage:** 33/33 requirements mapped

## Milestones

- **v1.0 MVP** - Phases 1-4 (shipped 2026-02-18)
- **v0.2 Multi-Instance** - Phases 5-18 (shipped 2026-03-16)
- **v0.4 Real-time Logs** - Phases 19-23 (in progress)

## Phases

<details>
<summary>v1.0 MVP (Phases 1-4) - SHIPPED 2026-02-18</summary>

### Phase 1: 基础配置和日志
**Goal**: 建立项目基础结构和日志系统
**Plans**: 3 plans (completed)

### Phase 01.1: Nanobot 生命周期管理
**Goal**: 实现单个 nanobot 实例的启动和停止
**Plans**: 2 plans (completed)

### Phase 2: UV 检测和更新逻辑
**Goal**: 实现 UV 包管理器检测和 nanobot 更新流程
**Plans**: 2 plans (completed)

### Phase 3: 调度和通知
**Goal**: 实现 cron 定时调度和 Pushover 失败通知
**Plans**: 2 plans (completed)

### Phase 4: 运行时集成
**Goal**: 集成所有组件并实现后台运行模式
**Plans**: 1 plan (completed)

</details>

<details>
<summary>v0.2 Multi-Instance (Phases 5-18) - SHIPPED 2026-03-16</summary>

多实例支持里程碑，支持同时管理多个 nanobot 实例的升级和启动。包含 5 个阶段，7 个计划，8 个任务，约 5000 行 Go 代码。

</details>

### v0.4 Real-time Logs (In Progress)

**Milestone Goal:** 为 nanobot 实例提供实时日志查看功能，通过 HTTP API 和 Web UI 访问

- [x] **Phase 19: Log Buffer Core** - 实现线程安全的环形缓冲区和广播机制 (completed 2026-03-17)
- [x] **Phase 20: Log Capture Integration** - 捕获 nanobot 进程的 stdout/stderr 输出 (completed 2026-03-17)
- [x] **Phase 21: Instance Management Integration** - 将 LogBuffer 集成到实例生命周期管理 (completed 2026-03-17)
- [ ] **Phase 22: SSE Streaming API** - 实现 SSE 端点用于实时日志流式传输
- [ ] **Phase 23: Web UI and Error Handling** - 提供内置 Web UI 和错误处理机制

## Phase Details

### Phase 19: Log Buffer Core
**Goal**: 建立线程安全的环形缓冲区基础设施，支持日志存储和实时广播
**Depends on**: Phase 18 (v0.2 多实例支持)
**Requirements**: BUFF-01, BUFF-02, BUFF-03, BUFF-04, BUFF-05
**Success Criteria** (what must be TRUE):
  1. 系统可以为每个 nanobot 实例创建独立的环形缓冲区（5000 行容量）
  2. 系统的环形缓冲区支持并发读写操作，无数据竞态（通过 go test -race 验证）
  3. 系统在缓冲区满时自动覆盖最旧的日志行（FIFO 行为）
  4. 每条日志保留时间戳、来源（stdout/stderr）和内容三要素
  5. 系统提供订阅机制，支持多个客户端同时接收实时日志更新
**Plans**: 2 plans

Plans:
- [x] 19-01-PLAN.md — 实现环形缓冲区核心
- [x] 19-02-PLAN.md — 实现订阅机制

### Phase 20: Log Capture Integration
**Goal**: 修改进程启动逻辑，捕获 nanobot 进程的 stdout/stderr 输出并写入 LogBuffer
**Depends on**: Phase 19
**Requirements**: CAPT-01, CAPT-02, CAPT-03, CAPT-04, CAPT-05
**Success Criteria** (what must be TRUE):
  1. 系统在 nanobot 进程启动时自动开始捕获 stdout 和 stderr 输出流
  2. 系统并发读取 stdout 和 stderr 管道，无死锁风险（即使输出量超过 10MB）
  3. 捕获的日志行实时写入对应的 LogBuffer
  4. 系统在 nanobot 进程停止时自动停止捕获输出
  5. 进程捕获逻辑不影响 nanobot 进程的正常启动和运行
**Plans**: 2 plans

Plans:
- [x] 20-01-PLAN.md — 实现 captureLogs 核心函数 (CAPT-01, CAPT-02, CAPT-03)
- [x] 20-02-PLAN.md — 实现 StartNanobotWithCapture 函数 (CAPT-04, CAPT-05)

### Phase 21: Instance Management Integration
**Goal**: 将 LogBuffer 集成到现有的实例生命周期管理系统，为每个实例提供独立的日志缓冲
**Depends on**: Phase 19, Phase 20
**Requirements**: INST-01, INST-02, INST-03, INST-04, INST-05
**Success Criteria** (what must be TRUE):
  1. 每个 nanobot 实例在启动时自动创建对应的 LogBuffer
  2. 用户可以通过 InstanceManager 按名称访问任意实例的 LogBuffer
  3. 实例停止时 LogBuffer 保留（用户仍可查看历史日志）
  4. 实例重启时 LogBuffer 被清空（重新开始缓冲）
  5. 现有的实例更新流程（stop→update→start）工作不变，向后兼容
**Plans**: 2 plans

Plans:
- [ ] 21-01-PLAN.md — 添加 LogBuffer.Clear() 方法 (INST-05)
- [ ] 21-02-PLAN.md — 集成 LogBuffer 到 InstanceLifecycle 和 InstanceManager (INST-01, INST-02, INST-03, INST-04, INST-05)

### Phase 22: SSE Streaming API
**Goal**: 提供 HTTP 端点，通过 Server-Sent Events 协议实时推送日志流
**Depends on**: Phase 19, Phase 21, v0.3 HTTP API 服务器
**Requirements**: SSE-01, SSE-02, SSE-03, SSE-04, SSE-05, SSE-06, SSE-07
**Success Criteria** (what must be TRUE):
  1. 用户可以通过 `/api/v1/logs/:instance/stream` SSE 端点实时接收日志流
  2. SSE 连接建立后，客户端接收缓冲区中的历史日志，然后实时接收新日志
  3. 系统每 30 秒发送 SSE 心跳注释，防止连接超时
  4. 系统检测客户端断开连接并停止发送事件（无 goroutine 泄漏）
  5. stdout 和 stderr 分别标记为不同的事件类型（便于客户端区分）
  6. SSE 端点支持 HTTP 长连接（WriteTimeout 设置为 0）
  7. 请求不存在的实例日志时返回 HTTP 404 Not Found
**Plans**: TBD

Plans:
- [ ] TBD

### Phase 23: Web UI and Error Handling
**Goal**: 提供内置 Web UI 页面查看日志，并实现全面的错误处理机制
**Depends on**: Phase 22
**Requirements**: UI-01, UI-02, UI-03, UI-04, UI-05, UI-06, UI-07, ERR-01, ERR-02, ERR-03, ERR-04
**Success Criteria** (what must be TRUE):
  1. 用户可以通过 `/logs/:instance` HTML 页面查看日志（单文件部署，静态资源嵌入二进制）
  2. Web 页面自动滚动到最新日志（类似 tail -f 行为）
  3. 用户可以通过按钮暂停和恢复自动滚动
  4. stdout 和 stderr 使用不同颜色区分（视觉差异明显）
  5. 页面显示 SSE 连接状态（连接中/已连接/已断开）
  6. 用户可以通过实例选择下拉菜单切换查看不同实例的日志
  7. 系统在进程管道读取失败时记录错误日志并继续运行（不影响整体服务）
  8. 系统在 SSE 客户端连接失败时记录警告日志并继续运行
  9. 系统在 LogBuffer 写入失败时记录错误日志并丢弃日志行（不阻塞进程）
**Plans**: TBD

Plans:
- [ ] TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 19 → 20 → 21 → 22 → 23

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 19. Log Buffer Core | 2/2 | Complete    | 2026-03-17 |
| 20. Log Capture Integration | 2/2 | Complete    | 2026-03-17 |
| 21. Instance Management Integration | 2/2 | Complete    | 2026-03-17 |
| 22. SSE Streaming API | 0/TBD | Not started | - |
| 23. Web UI and Error Handling | 0/TBD | Not started | - |
