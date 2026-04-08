# Nanobot Auto Updater

## What This Is

一个 Windows 后台监控服务，使用 Golang 开发，用于监控网络连通性并通过 HTTP API 触发 nanobot 工具的更新。**v0.9 启动通知与 Telegram 监控**：实例启动结果聚合通知、Telegram 连接状态监控（日志模式检测 + 30s 超时状态机）和 Pushover 通知。**v0.8 自更新**：通过 GitHub Releases 自动检测、下载并替换更新 nanobot-auto-updater 自身，包括 CI/CD 自动构建、HTTP API 触发自更新、安全恢复机制。**v0.7 更新生命周期通知**：HTTP API 触发更新时发送 Pushover 通知，包括更新开始/完成通知、非阻塞发送、优雅降级。**v0.6 更新日志记录和查询**：持久化记录每次更新操作的详细日志，提供分页查询 API 和 7 天自动清理。**v0.5 核心监控和自动化**：启动时自动启动实例、实例健康监控、Google 连通性监控、HTTP API 触发更新和 help 接口。**v0.4 实时日志查看**：通过 SSE 流式传输和嵌入式 Web UI 实时查看 nanobot 实例的 stdout/stderr 输出。多实例管理保持不变。通过配置文件和 HTTP API 控制行为。

## Core Value

自动保持 nanobot 处于最新版本，无需用户手动干预。

## Requirements

### Validated

**v0.8 Safety & Recovery** — 2026-03-30 (Phase 40):
- ✓ SAFE-01: 自更新后 self-spawn 重启 (daemon.go 标志) + 端口重试 (500ms × 5)
- ✓ SAFE-02: Pushover 通知 (开始/成功/失败) + Notifier 注入
- ✓ SAFE-03: .update-success 标记 + 启动时 .old 清理
- ✓ SAFE-04: .exe.old 异常检测 + 自动恢复旧版本

**v0.8 Self-Update HTTP API** — 2026-03-30 (Phase 39):
- ✓ API-01: POST /api/v1/self-update Bearer Token 认证 (401 Unauthorized)
- ✓ API-02: 自更新与 trigger-update 互斥锁 (409 Conflict)
- ✓ API-03: GET /api/v1/self-update/check 版本检查 + 状态查询
- ✓ API-04: Help 接口包含 self_update_check 和 self_update 端点说明

**v0.8 Self-Update Core** — 2026-03-30 (Phase 38):
- ✓ UPDATE-01: CheckLatest() 获取 GitHub 最新 Release
- ✓ UPDATE-02: NeedUpdate() semver 版本比较 (dev 版本始终更新)
- ✓ UPDATE-03: SHA256 校验下载 ZIP 完整性
- ✓ UPDATE-04: minio/selfupdate Apply 运行中 exe 替换
- ✓ UPDATE-05: .old 备份 + RollbackError 检查
- ✓ UPDATE-06: 1小时 Release 缓存
- ✓ UPDATE-07: self_update 配置段 (默认 HQGroup/nanobot-auto-updater)

**v0.8 CI/CD Pipeline** — 2026-03-29 (Phase 37):
- ✓ CICD-01: v* tag 触发 GitHub Actions 自动构建
- ✓ CICD-02: GoReleaser 编译 Windows amd64 + checksums 发布到 GitHub Releases
- ✓ CICD-03: ldflags 注入版本号 (-version 输出正确)

**v0.8 PoC Validation** — 2026-03-29 (Phase 36):
- ✓ VALID-01: minio/selfupdate v0.6.0 成功替换运行中 Windows exe
- ✓ VALID-02: 旧版本保存为 .old 备份文件
- ✓ VALID-03: 自更新后独立重启 (self-spawn via cmd.Start)

**v0.7 Update Lifecycle Notifications** — 2026-03-29:
- ✓ UNOTIF-01: 更新开始通知 (触发来源 + 实例数量)
- ✓ UNOTIF-02: 更新完成通知 (三态状态 + 耗时 + 实例详情)
- ✓ UNOTIF-03: 非阻塞通知 (goroutine + panic recovery)
- ✓ UNOTIF-04: 优雅降级 (Pushover 未配置时跳过)

**v0.6 Update Log Recording and Query System** — 2026-03-29:
- ✓ LOG-01: UpdateLog 数据模型 (UUID, 时间戳, 触发方式, 实例详情)
- ✓ LOG-02: 更新触发时记录日志 (TriggerHandler 集成)
- ✓ LOG-03: 非阻塞日志记录 (文件写入失败不影响更新)
- ✓ LOG-04: 更新 ID (UUID v4) 返回给客户端
- ✓ STORE-01: JSONL 文件持久化 (atomic write + fsync)
- ✓ STORE-02: 7天自动清理 (bufio.Scanner + atomic rename)
- ✓ QUERY-01: 查询 API (GET /api/v1/update-logs)
- ✓ QUERY-02: 分页参数 (limit/offset, 默认20/最大100)
- ✓ QUERY-03: 认证保护 (Bearer Token, 复用 Phase 28)

**v0.5 核心监控和自动化** — 2026-03-24:
- ✓ 启动时自动启动所有配置的实例 — Phase 24
- ✓ 实例健康监控（检测实例是否正常运行） — Phase 25
- ✓ Google 连通性监控服务（定时测试 google.com） — Phase 26
- ✓ 网络连通性变化通知（Pushover + 1分钟冷却） — Phase 27
- ✓ HTTP API 触发更新端点 (/api/v1/trigger-update) — Phase 28
- ✓ HTTP help 接口 (/api/v1/help) — Phase 29

**v0.4 实时日志查看** — 2026-03-20:

**v0.9 Startup Notification** — 2026-04-06 (Phase 41):
- ✓ STRT-01: 实例启动后通过 Pushover 通知启动结果（成功或失败）
- ✓ STRT-02: 通知在 auto-start goroutine 内执行（非阻塞）
- ✓ STRT-03: 未配置 Pushover 时优雅降级（返回 nil）

**v0.9 Telegram Monitor Core** — 2026-04-06 (Phase 42):
- ✓ TELE-01: "Starting Telegram bot" 触发监控窗口
- ✓ TELE-02: "Telegram bot commands registered" 检测连接成功
- ✓ TELE-03: "httpx.ConnectError" 检测连接失败
- ✓ TELE-04: 30 秒超时通知
- ✓ TELE-05: 成功 Pushover 通知
- ✓ TELE-06: 失败 Pushover 通知
- ✓ TELE-08: 历史日志条目过滤 (startTime 前忽略)

**v0.9 Telegram Monitor Integration** — 2026-04-06 (Phase 43):
- ✓ TELE-07: 未产生 trigger 日志的实例无监控开销 (TestMonitor_NoTriggerNoNotifications)
- ✓ TELE-09: 实例停止时取消监控,不发送虚假通知 (TestMonitor_StopCancelsMonitor)

### Active

**v0.10 管理界面自更新功能** — 2026-04-07:
- UI-01: 自更新管理区域布局 (home.html 顶部区块)
- UI-02: 当前版本显示 (标签样式)
- UI-03: 检测更新 (版本号+发布日期+更新说明)
- UI-04: 触发更新与进度显示 (checking→downloading%→installing→complete/failed)
- UI-05: 下载进度百分比 (轮询 + 进度条)
- API-01: 更新进度状态追踪 (ProgressState + io.TeeReader + atomic.Value)
- API-02: Web UI Token API (localhost-only GET /api/v1/web-config)

### Out of Scope

<!-- Explicit boundaries. Includes reasoning to prevent re-adding. -->

(None for this milestone)

## Context

**nanobot**: 一个 AI Agent 工具，托管在 GitHub (https://github.com/HKUDS/nanobot)，支持通过 uv 包管理器安装。

**uv**: Python 包管理器，用于安装和管理 Python 工具。

**Pushover**: 推送通知服务，用于在更新失败时通知用户。

**v0.10 管理界面自更新功能 (Active)** — 2026-04-07:
- 在 home.html 顶部新增自更新管理区域
- 显示当前版本、检测最新版本（版本号+日期+说明）、一键触发更新
- 更新过程实时显示阶段和下载进度百分比
- Web UI 自动从配置获取认证 Token，无需手动输入
- 2 个阶段 (44-45)，4 个计划

**v0.9 Shipped:** 2026-04-06 — 启动通知与 Telegram 监控里程碑完成，3 个阶段 (41-43)，6 个计划。Phase 41: NotifyStartupResult 聚合通知 + auto-start 集成。Phase 42: TelegramMonitor 状态机 (模式检测 + AfterFunc 超时 + duck-typing 接口) + 8 并发压力测试。Phase 43: InstanceLifecycle 集成 (constructor chain + context 取消 + goroutine 生命周期)。12/12 需求满足。

**v0.10 管理界面自更新功能** — 2026-04-07 (Active):
- 在 home.html 顶部新增自更新管理区域
- 显示当前版本、检测最新版本（版本号+日期+说明）、一键触发更新
- 更新过程实时显示阶段和下载进度百分比
- Web UI 自动从配置获取认证 Token，无需手动输入
- 2 个阶段 (44-45)，4 个计划

**v0.7 Shipped:** 2026-03-29 — 更新生命周期通知里程碑完成，2 个阶段 (34-35)，2 个计划，4 个任务。Phase 34: Notifier 注入 TriggerHandler + 异步通知 (开始/完成) + panic recovery。Phase 35: Notifier 接口重构 + recordingNotifier mock + 4 个 E2E 通知测试。审计结果: 4/4 需求满足，2/2 阶段通过，5/5 集成点连接，5/5 E2E 流程完整，77/77 测试通过。

**v0.6 Shipped:** 2026-03-29 — 更新日志记录和查询系统里程碑完成，4 个阶段 (30-33)，8 个计划，16 个任务。Phase 30: UpdateLog 数据模型 + UpdateLogger 组件。Phase 31: JSONL 文件持久化 + 7天自动清理。Phase 32: 查询 API + 分页 + Bearer Token 认证。Phase 33: E2E 集成测试 + 性能基准 (867ns-87us)。验证: 5/5 成功标准通过，50+ 测试全部通过。

**v0.5 Shipped:** 2026-03-24 — 核心监控和自动化里程碑完成，6 个阶段 (24-29)，16 个计划，22 个任务，~2,400 行新增代码。Phase 24: 启动时自动启动实例。Phase 25: 实例健康监控。Phase 26: Google 连通性监控。Phase 27: Pushover 通知（1 分钟冷却）。Phase 28: HTTP API 触发更新 (Bearer Token + 并发控制)。Phase 29: HTTP help 接口。审计结果: 20/20 需求满足，6/6 阶段通过，15/15 集成点连接，5/5 E2E 流程完整，2 技术债（非阻塞文档缺口）。

**v0.4 Shipped:** 2026-03-20 — 实时日志查看里程碑完成，5 个阶段，11 个计划，33 个需求，~12,000 行代码增加。测试覆盖: 单元测试 + 集成测试 + E2E 测试。审计结果: 33/33 需求满足，5/5 阶段通过，17/17 集成点连接，4/4 E2E 流程完整，0 缺口，0 技术债。

**v0.2 Shipped:** 2026-03-16 — 多实例支持里程碑完成，5 个阶段，7 个计划，8 个任务，~5000 LOC Go 代码。测试覆盖: 单元测试 + 集成测试 + E2E 测试。

**Tech Stack:** Golang, viper (YAML 配置), logrus (日志), cron (调度), Pushover API (通知), SSE (Server-Sent Events), embed.FS (静态资源嵌入), minio/selfupdate (exe 替换), GoReleaser (CI/CD), golang.org/x/mod/semver (版本比较)

**Key Patterns:**
- 上下文感知日志 (logger.With 预注入)
- 结构化错误包装 (InstanceError + 错误链)
- 优雅降级 (失败不中断整体流程)
- TDD 开发模式 (RED-GREEN-REFACTOR)
- 配置向后兼容 (legacy 模式自动检测)
- 环形缓冲区 (固定容量 5000 行)
- 非阻塞订阅者模式 (慢订阅者丢弃日志)

## Constraints

- **平台**: Windows 操作系统 — 目标用户在 Windows 环境使用
- **语言**: Golang — 用户指定
- **日志库**: github.com/WQGroup/logger — 用户指定，基于 logrus
- **日志格式**: `2024-01-01 12:00:00.123 - [INFO]: message` — 需要自定义 formatter
- **配置格式**: YAML — 可读性好，支持注释

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| 隐藏窗口运行 | 后台服务，无需用户交互 | ✓ Good |
| YAML 配置文件 | 可读性好，支持注释，Go 生态支持完善 | ✓ Good |
| Cron 定时触发 | 灵活的时间配置，用户熟悉 | ✓ Good |
| Pushover 失败通知 | 简单可靠的通知方式 | ✓ Good |
| GitHub main 分支优先 | 获取最新功能和修复 | ✓ Good |
| 稳定版回退机制 | 保证更新失败时仍可用 | ✓ Good |
| 多实例模式检测 (len(Instances) > 0) | 清晰的模式切换,向后兼容 | ✓ Good — Phase 10 |
| InstanceError 错误链支持 | 中文消息 + errors.Is/As 遍历 | ✓ Good — Phase 7 |
| 优雅降级 (继续处理其他实例) | 部分失败不影响整体流程 | ✓ Good — Phase 8 |
| 条件通知模式 (仅失败时发送) | 避免不必要的通知打扰 | ✓ Good — Phase 9 |
| mapstructure 标签 (而非 yaml) | viper 使用 mapstructure 解析 | ✓ Good — Phase 6 |
| O(n) map-based 唯一性验证 | 避免嵌套循环,性能更好 | ✓ Good — Phase 6 |
| errors.Join 聚合所有验证错误 | 用户一次性看到所有配置问题 | ✓ Good — Phase 6 |
| 上下文感知日志 (logger.With 预注入) | 所有日志自动包含实例名和组件 | ✓ Good — Phase 7 |
| cmd /c 执行启动命令 | 支持管道和重定向等复杂命令 | ✓ Good — Phase 7 |
| 端口监听验证替代进程名 | 更精确的启动验证 | ✓ Good — Phase 7 |
| Double error checking (UV + Instance) | 分类错误并路由到正确通知 | ✓ Good — Phase 10 |
| Context timeout (--update-now only) | 定时模式无超时,立即更新可配置超时 | ✓ Good — Phase 10 |
| 1-based 实例序号 | 符合用户直觉,避免 0-based 困惑 | ✓ Good — Phase 10-02 |
| 自实现环形缓冲区 ([5000]LogEntry) | 避免外部依赖和序列化开销 | ✓ Good — Phase 19 |
| sync.RWMutex 线程安全 | 读多写少场景,允许并发读取 | ✓ Good — Phase 19 |
| Channel 订阅模式 (容量 100) | 匹配 Go 并发习惯,集成 Phase 22 SSE | ✓ Good — Phase 19 |
| 非阻塞订阅者发送 (select+default) | 防止 Write 阻塞,确保系统稳定性 | ✓ Good — Phase 19 |
| bufio.Scanner 行读取 | 自动处理行边界,API 更简洁 | ✓ Good — Phase 20 |
| os.Pipe() 替代 cmd.StdoutPipe() | 避免 race condition | ✓ Good — Phase 20 |
| Context 取消 + select 非阻塞扫描 | 及时退出 goroutine | ✓ Good — Phase 20 |
| Clear() 清空数组 + 重置指针 | 线程安全状态重置 | ✓ Good — Phase 21 |
| 启动前 Clear, 停止后保留 | 调试友好,重启清空历史 | ✓ Good — Phase 21 |
| WriteTimeout=0 支持 SSE 长连接 | SSE 协议要求 | ✓ Good — Phase 22 |
| Graceful shutdown (10秒超时) | 优雅关闭服务器 | ✓ Good — Phase 22 |
| embed.FS 嵌入静态文件 | 单文件部署 | ✓ Good — Phase 23 |
| 原生 HTML/CSS/JS (无框架) | 简单日志查看器约 300 行 | ✓ Good — Phase 23 |
| 智能自动滚动 (50px 容差) | 检测手动滚动 | ✓ Good — Phase 23 |
| 高对比度 stderr 颜色 (#dc2626) | 确保可见性 | ✓ Good — Phase 23 |
| 实例名称按配置顺序返回 | 可预测的选择器顺序 | ✓ Good — Phase 23-02 |
| ERROR 级别记录管道读取错误 | 意外错误,服务继续 | ✓ Good — Phase 23-03 |
| WARN 级别记录 SSE 连接错误 | 预期错误,服务继续 | ✓ Good — Phase 23-03 |
| WARN 级别丢弃慢订阅者日志 | 非阻塞,不卡主流程 | ✓ Good — Phase 23-03 |
| 异步启动实例 (goroutine + panic recovery) | 非阻塞启动,应用稳定 | ✓ Good — Phase 24-03 |
| Context 5分钟超时控制 (auto-start) | 防止无限等待 | ✓ Good — Phase 24-03 |
| Nil-safe 组件管理 (healthMonitor 检查) | 零实例配置兼容 | ✓ Good — Phase 25-02 |
| 反向关闭顺序 (后启动先停止) | 确保依赖关系正确 | ✓ Good — Phase 25-02 |
| 网络监控独立启动 (不依赖实例数) | 监控 Google 无需实例 | ✓ Good — Phase 26-02 |
| 状态变化冷却确认 (1分钟 timer) | 避免频繁通知 | ✓ Good — Phase 27-01 |
| Pushover 异步发送 (goroutine) | 不阻塞监控主循环 | ✓ Good — Phase 27-01 |
| Bearer Token 认证 (RFC 6750) | 标准化认证机制 | ✓ Good — Phase 28-01 |
| Constant time string comparison | 防止时序攻击 | ✓ Good — Phase 28-01 |
| Atomic.Bool 并发更新控制 | 防止重复更新 | ✓ Good — Phase 28-02 |
| JSON 错误响应 (RFC 7807) | 标准化错误格式 | ✓ Good — Phase 28-03 |
| APIInstanceError 序列化适配 | JSON 兼容错误结构 | ✓ Good — Phase 28-03 |
| HTTP 状态码策略 (200/401/409/504) | 清晰的错误分类 | ✓ Good — Phase 28-03 |
| Help 接口无认证 (公开访问) | 第三方程序智能查询 | ✓ Good — Phase 29-02 |
| Version 注入 (main → server → handler) | 未来扩展性 | ✓ Good — Phase 29-02 |
| JSON Lines 日志格式 | 简单追加,无需全文件解析 | ✓ Good — Phase 30 |
| 三态分类 (success/partial_success/failed) | 精确表达多实例更新结果 | ✓ Good — Phase 30 |
| TriggerUpdater 接口 | mock 友好测试,解耦具体依赖 | ✓ Good — Phase 30 |
| Nil-safe UpdateLogger | handler 检查 nil,非阻塞错误日志 | ✓ Good — Phase 30 |
| UUID v4 入口生成 | 完整生命周期追踪 | ✓ Good — Phase 30 |
| Separate fileMu mutex | 文件 I/O 不阻塞 GetAll() | ✓ Good — Phase 31 |
| Lazy file open | 首次 Record() 时打开,支持纯内存模式 | ✓ Good — Phase 31 |
| Atomic cleanup (temp + rename) | Windows 安全的文件清理 | ✓ Good — Phase 31 |
| Non-blocking file write failure | 静默降级到纯内存模式 | ✓ Good — Phase 31 |
| UpdateLogger in main.go | 应用级生命周期控制 | ✓ Good — Phase 31 |
| bufio.Scanner 流式分页 | 避免 1000+ 记录内存问题 | ✓ Good — Phase 32 |
| Notifier 注入 TriggerHandler | 更新生命周期通知,与 UpdateLogger 同模式 | ✓ Good — Phase 34 |
| 异步通知 goroutine + panic recovery | 通知失败不影响更新流程 | ✓ Good — Phase 34 |
| instanceCount 构造时注入 | 通知内容含实例数,无需运行时查询 | ✓ Good — Phase 34 |
| statusToTitle/formatCompletionMessage | 三态通知标题和实例详情格式化 | ✓ Good — Phase 34 |
| Notifier 接口 (trigger.go 本地定义) | 最小范围,单方法接口,duck typing | ✓ Good — Phase 35 |
| TryLockUpdate/UnlockUpdate 共享互斥锁 | 自更新与 trigger-update 复用 atomic.Bool | ✓ Good — Phase 39 |
| SelfUpdateHandler (Check + Update 端点) | HandleCheck 版本信息 + HandleUpdate 异步更新 | ✓ Good — Phase 39 |
| atomic.Value 状态追踪 (idle/updating/updated/failed) | 无锁并发读取更新状态 | ✓ Good — Phase 39 |
| 202 Accepted 异步更新 (goroutine + panic recovery) | 非阻塞触发 + 容错 | ✓ Good — Phase 39 |
| selfupdate.Updater 注入 NewServer (第8参数) | 零配置自更新集成 | ✓ Good — Phase 39 |
| Help 端点自更新条目 (self_update_check + self_update) | API 可发现性 | ✓ Good — Phase 39 |
| minio/selfupdate v0.6.0 选择 | 战斗验证的 Windows exe rename trick | ✓ Good — Phase 36 |
| PoC-first 验证策略 | 消除 Windows exe 替换技术不确定性 | ✓ Good — Phase 36 |
| GoReleaser ZIP 格式 | Phase 38 下载 ZIP 并提取 exe | ✓ Good — Phase 37 |
| golang.org/x/mod/semver | 标准库扩展，成熟的 semver 比较 | ✓ Good — Phase 38 |
| struct-based 缓存 (cachedRelease + cacheTime) | 简洁且可测试 (操纵 cacheTime 测试过期) | ✓ Good — Phase 38 |
| In-memory ZIP 提取 (bytes.Reader) | 无临时文件，避免 Windows 文件锁问题 | ✓ Good — Phase 38 |
| GoReleaser checksums.txt 两空格分隔 | 匹配 GoReleaser 标准输出格式 | ✓ Good — Phase 38 |
| SelfUpdateChecker/UpdateMutex 接口 (duck typing) | 最小范围，单方法接口，与 TriggerUpdater 同模式 | ✓ Good — Phase 39 |
| restartFn 注入 SelfUpdateHandler | 生产用 defaultRestartFn，测试覆写 no-op，避免子进程循环 | ✓ Good — Phase 40 |
| Sync 完成通知 (os.Exit 前) | 避免 goroutine 被 os.Exit 杀死 (Pitfall 1) | ✓ Good — Phase 40 |
| CheckUpdateState 内外函数分离 | checkUpdateStateInternal 返回决策字符串，CheckUpdateStateForPath 执行，可测试无 os.Exit | ✓ Good — Phase 40 |
| net.Listen + http.Serve 替代 ListenAndServe | 启用端口绑定重试能力 | ✓ Good — Phase 40 |
| NotifyStartupResult 方法 + formatStartupMessage | 聚合多实例启动结果为单条通知 | ✓ Good — Phase 41 |
| TelegramMonitor 独立包 (internal/telegram) | 日志模式检测 + 30s AfterFunc 超时状态机 | ✓ Good — Phase 42 |
| Duck-typed LogSubscriber/Notifier 接口 | 最小范围,单方法接口,避免循环导入 | ✓ Good — Phase 42 |
| Notifier constructor chain (main→manager→lifecycle) | 注入式通知,与现有模式一致 | ✓ Good — Phase 43 |
| TelegramMonitor lifecycle tied to InstanceLifecycle | 启动后创建,停止前取消,goroutine 安全 | ✓ Good — Phase 43 |

## Configuration

**配置文件**: `./config.yaml`

**v1.0 Legacy 配置项** (向后兼容):
- cron: "0 3 * * *" (每天凌晨 3 点)
- pushover_token: "" (Pushover App Token)
- pushover_user: "" (Pushover User Key)
- nanobot.port: 18790 (nanobot 端口)
- nanobot.start_command: "启动命令" (nanobot 启动命令)
- nanobot.startup_timeout: 30 (启动超时秒数)

**v0.2 多实例配置** (新):
```yaml
instances:
  - name: "gateway"
    port: 18790
    start_command: "python -m nanobot.gateway"
    startup_timeout: 30
  - name: "worker"
    port: 18791
    start_command: "python -m nanobot.worker --port 18791"
    startup_timeout: 45
```

**v0.8 自更新配置** (新):
```yaml
self_update:
  github_owner: "HQGroup"       # 默认值
  github_repo: "nanobot-auto-updater"  # 默认值
```

**命令行参数**:
- `-cron`: 覆盖配置文件中的 cron 表达式
- `-config`: 指定配置文件路径
- `--update-now`: 立即执行一次更新并退出 (JSON 输出)
- `--timeout`: 配置立即更新超时时间 (默认 5 分钟)
- `-version`: 显示版本信息
- `help`: 显示帮助信息

**注意**: 旧的 `-run-once` 参数已在 v1.0 中移除,替换为 `--update-now`

---

## Current State

**Shipped:** v0.9 Startup Notification & Telegram Monitor (2026-04-06)
**Active:** v0.10 管理界面自更新功能 — Phase 45 complete (2026-04-08)
**Total:** 9 milestones shipped, 45 phases, ~19,158 LOC Go

*Last updated: 2026-04-08 after Phase 45 completion*

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state
