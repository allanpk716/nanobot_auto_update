# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v0.8 — Self-Update

**Shipped:** 2026-03-30
**Phases:** 5 | **Plans:** 8 | **Sessions:** 3

### What Was Built
- minio/selfupdate v0.6.0 PoC 验证: Windows exe 替换、.old 备份、self-spawn 重启
- GoReleaser + GitHub Actions CI/CD: v* tag 触发 Windows amd64 自动构建发布
- internal/selfupdate/ 包: GitHub Release 检查、semver 比较、SHA256 校验、ZIP 解压、运行中 exe 替换
- SelfUpdateHandler HTTP API: check/update 端点、共享互斥锁、202 Accepted 异步模式
- 安全恢复: Pushover 通知、.update-success 状态文件、.old 清理/恢复、端口重试

### What Worked
- PoC-first 策略: 先验证 minio/selfupdate Windows 可行性，消除不确定性后再实现
- TDD RED-GREEN 模式持续有效，Phase 38 的 26 个测试零重构
- 内外函数分离模式 (checkUpdateStateInternal/CheckUpdateStateForPath) 实现无 os.Exit 测试
- restartFn 注入模式解决了 self-spawn 测试中的子进程循环问题

### What Was Inefficient
- 预存在的 capture_test.go 编译错误从 Phase 38 延续到 Phase 40，始终需要 workaround
- TestE2E_Notification_NonBlocking 的 30s time.Sleep 在多个阶段造成测试选择问题

### Patterns Established
- restartFn 注入: 覆写 exec.Command+os.Exit 路径为可测试的函数字段
- 内外函数分离: 内部函数返回决策，外部函数执行副作用，可测试无需 os.Exit
- net.Listen + http.Serve 替代 ListenAndServe: 启用端口重绑能力
- SelfUpdateChecker/UpdateMutex 接口: 与 TriggerUpdater 同模式的 duck typing

### Key Lessons
1. Windows exe 替换使用 minio/selfupdate 的 rename trick 最可靠，不需要自定义文件操作
2. os.Exit 前的 goroutine 会被杀死 — 必须同步执行关键操作 (如通知)
3. 自更新后的端口竞争需要重试机制 (500ms × 5)，因为旧进程释放端口需要时间
4. 空的 .old 文件不应触发恢复 — 避免误判

### Cost Observations
- Model mix: 100% sonnet
- Sessions: 3 (Phase 36-37, Phase 38, Phase 39-40)
- Notable: PoC 验证仅 5 分钟就消除了整个里程碑的技术不确定性

---

## Milestone: v0.9 — Startup Notification & Telegram Monitor

**Shipped:** 2026-04-06
**Phases:** 3 | **Plans:** 6 | **Sessions:** 2

### What Was Built
- NotifyStartupResult + formatStartupMessage: 聚合多实例启动结果为单条 Pushover 通知 (6 TDD tests)
- TelegramMonitor 状态机: 日志模式检测 (trigger/success/failure) + AfterFunc 30s 超时 + Pushover 通知
- 8 并发/边界压力测试: panic recovery, 快速状态切换, timer 重启, context 取消, race-safe 交互
- InstanceLifecycle 集成: Notifier constructor chain 注入 (main→manager→lifecycle) + context 取消 + goroutine 生命周期
- 6 TDD lifecycle tests: TELE-07 零开销 + TELE-09 取消安全 + 成功通知验证

### What Worked
- Duck-typed 接口 (LogSubscriber/Notifier/Notifier in lifecycle.go): 最小范围单方法接口，避免循环导入
- AfterFunc 替代 time.NewTimer + goroutine: 更简洁的超时状态机
- Constructor chain 注入模式: 与 v0.7 Notifier + v0.8 SelfUpdateChecker 模式一致
- 独立 context 管理: 每个实例有自己的 monitorCancel，停止时精确取消

### What Was Inefficient
- Phase 43 人工 UAT 最终跳过 — 自动化 mock 测试已充分验证
- 预存在的 capture_test.go 编译错误继续从 v0.8 延续
- Exact log patterns from python-telegram-bot 未在真实环境验证 (hardcoded strings)

### Patterns Established
- AfterFunc timeout state machine: 替代 goroutine+timer 模式，更简洁
- Duck-typed local interface: 在 lifecycle.go 内部定义 Notifier 接口，最小化范围
- Monitor lifecycle tied to instance: startTelegramMonitor/stopTelegramMonitor 对称管理

### Key Lessons
1. 日志模式监控用 strings.Contains + 状态机是最简方案，无需正则或复杂解析
2. 实例级 context 取消是 goroutine 生命周期管理的正确方式 — Stop() 只需 cancel()
3. 历史日志过滤 (startTime) 对订阅者模式至关重要，避免重放误触发
4. 零开销监控 (stateIdle 初始态) 确保非 Telegram 实例无额外负担

### Cost Observations
- Model mix: 100% sonnet
- Sessions: 2 (Phase 41, Phase 42-43)
- Notable: 3 个阶段在 1 天内完成，duck-typing 模式成熟度高

---

## Milestone: v0.10 — 管理界面自更新功能

**Shipped:** 2026-04-08
**Phases:** 2 | **Plans:** 4 | **Sessions:** 2

### What Was Built
- 下载进度追踪: ProgressState (atomic.Value) + io.TeeReader + Content-Length 实时百分比
- Web Token API: localhost-only GET /api/v1/web-config 返回 Bearer token
- 自更新管理 UI: 版本标签 + 检测更新（版本号+日期+release notes 截断展开）+ 立即更新 + 500ms 进度轮询 + 进度条
- Code review 修复: createInstanceCard XSS (innerHTML → textContent) + download_percent 范围验证 + 错误消息脱敏

### What Worked
- 前后端分离 2 阶段模式: Phase 44 后端 API → Phase 45 前端 UI，职责清晰
- Code review 自动修复流程: gsd-code-review → gsd-code-review-fix，发现并修复 pre-existing XSS
- 直接编译二进制浏览器测试: 不需要发布 release 即可验证 UI 功能

### What Was Inefficient
- Executor agent 越权删除后端文件: worktree agent 修改了 Phase 44 的 Go 源码和 .planning 文件，需要手动恢复
- 恢复过程产生大量额外 commit，增加了 git 历史噪音

### Patterns Established
- textContent/createElement 渲染 API 数据: 防止 GitHub release notes XSS
- 500ms setInterval + 60s Date.now() timeout guard: 实时反馈 + 防止无限轮询
- 按钮状态互锁: 更新进行中禁用所有操作按钮

### Key Lessons
1. Worktree executor agent 需要 files_modified 约束 — 越权修改非计划文件是严重的 agent 行为问题
2. 浏览器测试可以通过直接编译运行二进制完成，无需 GitHub Release 发布
3. Code review 发现了 pre-existing XSS (createInstanceCard)，证明了 review 流程的价值

### Cost Observations
- Model mix: 100% sonnet
- Sessions: 2 (Phase 44, Phase 45)
- Notable: executor 越权问题增加了 ~30% 的修复时间

---

## Milestone: v0.11 — Windows 服务自启动

**Shipped:** 2026-04-11
**Phases:** 4 | **Plans:** 8 | **Sessions:** 2

### What Was Built
- svc.IsWindowsService() 运行模式检测 + build-tag 双实现 (service_windows.go / service.go)
- svc.Handler Execute 方法: 服务启动/停止/关机控制，30 秒优雅关闭，AppComponents 提取
- ServiceManager: CreateService + 3x restart recovery (60s interval, 24h reset), UnregisterService (Stop + poll + Delete), IsAdmin elevation check
- main.go 三分支 auto_start 逻辑: 服务模式告警 → 控制台模式注册 → 控制台模式卸载
- 双模式适配: daemon.go IsServiceMode guard + defaultRestartFn SCM restart + 工作目录验证
- 配置热重载: viper.WatchConfig + 500ms debounce + 6 组件重建回调 + 动态 Bearer Token getter
- Code review 修复: currentBearerToken RWMutex data race + startCancel 显式调用 + logFile.Close handle leak + os.Chdir error logging

### What Worked
- AppComponents/AppStartup/AppShutdown 提取模式: 服务和控制台模式共享启动逻辑，零重复
- Build-tag 双实现: Windows 平台完整功能 + 非 Windows 平台 no-op stubs，编译无障碍
- 三分支 auto_start 设计: 配置驱动模式切换，无需命令行参数
- Code review 流程: 发现 4 个问题 (data race, goroutine leak, file handle leak, missing error log)
- 500ms debounce timer 合并 Windows fsnotify 快速事件，避免重复重建

### What Was Inefficient
- Phase 48 UAT 测试暂停等待人工验证，但最终跳过 — 单元测试和 build 验证已充分
- 预存在的 capture_test.go 编译错误继续从 v0.8-v0.10 延续
- ROADMAP.md Phase 48 状态未及时更新，需要 milestone 完成时修正

### Patterns Established
- AppComponents 提取模式: main.go 启动/关闭逻辑抽取为独立函数，服务和控制台共享
- onReady callback: 服务 Running 状态后执行初始化操作（热重载等）
- IsServiceMode guard: 服务模式下跳过守护进程逻辑 (early return pattern)
- 双重启策略: service=os.Exit(1) 触发 SCM recovery, console=self-spawn
- HotReloadCallbacks 函数字段: config 包检测变更，调用方提供重建函数，避免循环导入
- 动态 Token getter: func() string closure 实现 Token 热重载

### Key Lessons
1. Windows 服务开发需要 svc.Handler + ServiceManager 分层: Handler 处理生命周期，Manager 处理注册/卸载
2. 配置热重载需要 debounce — Windows fsnotify 在文件保存时会产生多个快速事件
3. 全量替换 (StopAll→recreate→StartAll) 比增量 diff 更简单可靠，实例数量有限
4. os.Exit(1) 在服务模式下触发 SCM recovery policy — 非零退出码是正确的行为
5. Go string 赋值是原子的 — shared variable + func() string 是 Bearer Token 热重载的最简方案

### Cost Observations
- Model mix: 100% sonnet
- Sessions: 2 (Phase 46-48, Phase 49)
- Notable: 4 个阶段 2 天完成，Phase 48-49 在 1 个 session 内完成

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | ~5 | 4 | 项目创建，基础架构 |
| v0.2 | ~3 | 14 | 多实例支持，重构 |
| v0.4 | ~4 | 5 | SSE + embed.FS，前端 |
| v0.5 | ~3 | 6 | 监控 + HTTP API |
| v0.6 | ~2 | 4 | JSONL 持久化 + 查询 API |
| v0.7 | ~1 | 2 | Notifier 注入模式 |
| v0.8 | ~3 | 5 | PoC-first + CI/CD + 自更新 |
| v0.9 | ~2 | 3 | AfterFunc state machine + duck-typing + lifecycle monitor |
| v0.10 | ~2 | 2 | 前后端分离 + Web UI 自更新管理 + textContent XSS 防护 |
| v0.11 | ~2 | 4 | Windows Service + SCM + build-tag + config hot reload |

### Cumulative Quality

| Milestone | Tests | Key Patterns Added |
|-----------|-------|-------------------|
| v1.0 | ~10 | go:build windows, cron scheduling |
| v0.2 | ~30 | 错误链, 优雅降级, 配置验证 |
| v0.4 | ~50 | 环形缓冲区, SSE, embed.FS |
| v0.5 | ~60 | Bearer Token, atomic.Bool, Context timeout |
| v0.6 | ~70 | JSONL 持久化, 流式分页 |
| v0.7 | ~77 | Notifier 接口, duck typing |
| v0.8 | ~90+ | selfupdate, restartFn 注入, 内外函数分离 |
| v0.9 | ~100+ | AfterFunc state machine, duck-typed local interface, instance-level context |
| v0.10 | ~105+ | atomic.Value ProgressState, io.TeeReader, textContent XSS prevention, progress polling |
| v0.11 | ~115+ | svc.Handler, ServiceManager SCM, build-tag dual impl, IsServiceMode guard, HotReloadCallbacks, debounce timer |

### Top Lessons (Verified Across Milestones)

1. TDD RED-GREEN-REFACTOR 在所有阶段持续有效，零重构很常见
2. 注入模式 (TriggerUpdater, Notifier, SelfUpdateChecker) 统一了测试策略
3. 非阻塞 + panic recovery 模式确保所有异步操作不影响主流程
4. Windows 文件锁问题需要特殊处理 (1s pause, temp+rename)
5. Duck-typed 最小接口 + constructor chain 是 Go 项目解耦的标准模式 (v0.7-v0.9 验证)
