# Project Research Summary

**Project:** Nanobot Auto Updater
**Domain:** Multi-instance process management for Windows service
**Researched:** 2026-03-09
**Confidence:** HIGH

## Executive Summary

这是一个将现有单实例自动更新器扩展为支持多实例管理的项目。核心挑战在于协调多个 nanobot 进程的停止→更新→启动流程,同时保持系统可靠性和可调试性。

研究建议采用监督者模式(Supervisor Pattern),通过新增 `internal/instance` 包实现 `InstanceManager` 协调器。关键技术选型包括使用 `golang.org/x/sync/errgroup` 进行并发协调,以及 `sync.Map` 进行线程安全的状态跟踪。架构设计强调从配置加载、进程管理到错误聚合和通知的完整流程,其中错误聚合和通知去重是多实例场景的核心难点。

主要风险包括进程停止/启动过程中的竞态条件、资源泄漏、以及通知风暴。通过严格的串行化停止→等待→更新→串行化启动流程、带超时的等待确认、以及聚合通知策略来规避这些陷阱。

## Key Findings

### Recommended Stack

研究推荐使用 Go 标准库和一个并发协调库来扩展多实例支持。核心原则是最小化新依赖,最大化利用现有代码基础。

**Core technologies:**
- **golang.org/x/sync/errgroup (v0.18.0)**: 并发 goroutine 协调与错误传播 — 提供内置的错误处理用于并行操作,适合管理多个 nanobot 停止/启动操作,支持错误收集和超时控制
- **sync.Map (Go 1.24+ stdlib)**: 线程安全的进程状态跟踪 — Go 1.24+ 使用 HashTrieMap 底层实现,读操作无锁,适合读多写少的状态检查场景
- **context (stdlib)**: 取消和超时传播 — 用于协调多个实例的停止/启动操作,支持共享截止时间和错误取消
- **log/slog (stdlib, 已在使用)**: 带实例上下文的结构化日志 — 用于多实例日志记录,在日志属性中嵌入实例 ID

**Critical version requirements:**
- Go 1.24+ (当前项目使用 1.24.11,满足要求)
- golang.org/x/sync@v0.18.0 (2025年10月发布,包含 panic trapping 特性)

### Expected Features

**Must have (table stakes):**
- **实例配置(YAML)** — 用户期望定义哪些实例存在,使用 YAML 数组结构,包含 name、port、command 字段
- **唯一实例名称验证** — 必需的标识符,用于日志和通知,启动时验证无重复,快速失败
- **停止所有实例** — 基本的编排操作,遍历实例列表,停止每个实例(复用现有停止逻辑)
- **启动所有实例** — 基本的编排操作,遍历实例列表,使用配置的命令启动每个实例
- **优雅降级** — 部分实例成功,部分失败时不中止全部,继续启动其他实例
- **失败通知(按实例)** — 继续现有 v0.1 功能,报告哪些实例失败,在消息中包含实例名称

**Should have (competitive):**
- **实例健康状态跟踪** — 了解哪些实例在运行 vs 失败,跟踪每个实例的状态,在日志和通知中报告
- **单个实例控制** — 按名称停止/启动特定实例,通过 CLI 标志定位特定实例

**Defer (v2+):**
- **可配置重试策略** — 自动重试失败的实例,需要退避逻辑,复杂度高
- **并行实例启动** — 并发启动实例以加快恢复速度,需要 goroutine 管理和错误聚合
- **实例分组** — 定义实例组进行部分更新,如"前端" vs "后端"组

### Architecture Approach

研究建议采用分层架构,新增 `internal/instance` 包作为协调层,最小化修改现有组件。核心模式是监督者(Orchestrator),InstanceManager 协调多实例生命周期操作,同时保持现有单实例逻辑的完整性。

**Major components:**
1. **internal/instance (NEW)** — 新增包,包含 `InstanceManager` 协调器,负责停止所有→更新→启动所有的工作流协调,收集结果和失败,聚合错误用于通知
2. **internal/config (EXTENDED)** — 扩展现有配置包,添加 `Instances []InstanceConfig` 字段,验证唯一名称和端口,保持向后兼容性
3. **internal/lifecycle (EXTENDED)** — 最小化扩展,添加 `InstanceLifecycle` 包装器,为每个实例提供上下文感知的日志,复用现有 detector/stopper/starter 逻辑
4. **internal/notifier (EXTENDED)** — 扩展通知包,添加 `NotifyInstanceFailures(results []InstanceResult)` 方法,支持聚合通知以避免通知风暴

**Architecture pattern:**
- Supervisor/Orchestrator 模式: 中央 InstanceManager 协调跨多个实例的生命周期操作
- Configuration Extension 模式: 扩展现有配置结构,维护向后兼容性
- Context-Aware Logging 模式: 每个实例生命周期操作在所有日志消息中包含实例名称

### Critical Pitfalls

多实例管理引入了新的陷阱类别,特别是并发和资源管理方面:

1. **停止/启动序列中的竞态条件** — 通过严格的串行化停止→等待→更新→串行化启动流程规避,每个进程停止后添加明确的等待确认,使用带超时的等待机制,在启动新进程前验证资源已释放
2. **失败进程句柄的资源泄漏** — 通过使用 defer 确保进程句柄始终被清理,实现进程健康检查机制定期清理僵尸进程,为每个进程维护生命周期状态机,使用 context.Context 管理进程超时和取消
3. **静默失败和不完整的错误聚合** — 通过实现错误聚合模式收集所有实例的错误而非遇到第一个失败就返回,结构化错误报告区分"哪些成功,哪些失败,失败原因是什么",日志和通知分离
4. **多实例失败的通知风暴** — 通过实现通知去重和聚合,同一批次操作的失败合并为一条通知,添加速率限制,智能分组按失败类型或时间窗口分组通知
5. **进程识别混淆** — 通过每个实例必须有唯一标识符,所有日志和错误消息包含实例标识符,维护实例 ID → 配置的映射便于故障排查

## Implications for Roadmap

基于研究发现,建议采用 5 阶段结构,每个阶段有明确的依赖关系和可测试的交付物:

### Phase 1: 配置扩展和多实例验证
**Rationale:** 配置是所有其他功能的基础,必须首先实现。无依赖项,可独立测试。
**Delivers:** 扩展的配置结构支持实例列表,完整的验证逻辑
**Addresses:** 实例配置(YAML)、实例名称验证
**Avoids:** 配置架构过度工程化(保持扁平化结构)
**Uses:** 现有 viper/yaml 基础设施
**Implements:** Configuration Extension 模式

### Phase 2: 生命周期扩展
**Rationale:** 在配置就绪后,需要包装现有生命周期逻辑以支持实例上下文。依赖 Phase 1 的 InstanceConfig。
**Delivers:** InstanceLifecycle 包装器,上下文感知日志
**Addresses:** 停止所有实例、启动所有实例(单个实例级别)
**Avoids:** 进程识别混淆(所有日志包含实例标识符)
**Uses:** Phase 1 的 InstanceConfig,现有 lifecycle.Manager
**Implements:** Context-Aware Logging 模式

### Phase 3: 实例协调器(核心编排)
**Rationale:** 核心协调逻辑,需要配置和生命周期包装器。这是多实例管理的核心价值所在。
**Delivers:** InstanceManager 实现,StopAllInstances(),StartAllInstances(),UpdateAll() 方法,错误聚合逻辑
**Addresses:** 停止所有实例、启动所有实例、优雅降级
**Avoids:** 静默失败和不完整的错误聚合、停止/启动序列中的竞态条件
**Uses:** Phase 1 配置,Phase 2 InstanceLifecycle,errgroup 进行协调
**Implements:** Supervisor/Orchestrator 模式

### Phase 4: 通知扩展
**Rationale:** 错误聚合完成后,需要扩展通知以支持多实例失败报告。依赖 Phase 3 的 InstanceResult 类型。
**Delivers:** NotifyInstanceFailures() 方法,聚合通知逻辑
**Addresses:** 失败通知(按实例)
**Avoids:** 多实例失败的通知风暴
**Uses:** Phase 3 InstanceResult,现有 Pushover 集成

### Phase 5: 主程序集成和端到端测试
**Rationale:** 最后阶段,集成所有组件并进行全面的端到端测试。依赖所有前序阶段。
**Delivers:** 修改后的 main.go,集成测试,文档更新
**Addresses:** 所有功能的端到端验证
**Avoids:** 所有陷阱的集成验证
**Uses:** 所有前序阶段的交付物

### Phase Ordering Rationale

- **Phase 1 → Phase 2:** 配置定义了实例结构,生命周期包装器需要知道如何解析和验证实例配置
- **Phase 2 → Phase 3:** InstanceManager 需要调用 InstanceLifecycle 方法,必须先有包装器
- **Phase 3 → Phase 4:** 通知需要 InstanceResult 类型,该类型在 InstanceManager 中定义
- **Phase 4 → Phase 5:** 所有组件就绪后才能进行主程序集成和端到端测试

**Grouping rationale:**
- Phase 1-2: 基础设施扩展(配置和生命周期)
- Phase 3-4: 业务逻辑实现(协调和通知)
- Phase 5: 集成和验证

**How this avoids pitfalls:**
- 串行化停止→启动避免竞态条件(Phase 3 实现严格序列)
- 错误聚合避免静默失败(Phase 3 收集所有错误)
- 通知聚合避免通知风暴(Phase 4 实现去重逻辑)
- 上下文日志避免进程识别混淆(Phase 2 注入实例 ID)

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3:** 复杂的并发协调,需要深入研究 Windows 特定的进程管理细节和 Go 的 CommandContext 在 Windows 上的行为,以及 nanobot 进程行为(是否支持优雅停止,停止时需要多长时间,是否会产生子进程)
- **Phase 4:** Pushover API 限制和去重机制需要查看官方文档,验证通知聚合策略的有效性

Phases with standard patterns (skip research-phase):
- **Phase 1:** 配置扩展有成熟的 viper/mapstructure 模式,验证逻辑简单明确
- **Phase 2:** 生命周期包装器是简单的适配器模式,日志上下文注入有标准做法
- **Phase 5:** 主程序集成是常规的依赖注入和函数调用修改

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | 基于官方 pkg.go.dev 文档和高置信度的社区资源,errgroup 和 sync.Map 都是成熟的 Go 标准库/扩展库,版本兼容性已验证 |
| Features | MEDIUM | 基于 Docker Compose/Supervisord/Systemd 的竞品分析和通用进程管理模式,部分模式(如错误聚合)需要实现验证,社区资源置信度为 MEDIUM |
| Architecture | HIGH | 基于清晰的现有架构分析和成熟的监督者模式,社区讨论和实践案例丰富,架构模式在 Go 生态中有广泛验证 |
| Pitfalls | MEDIUM | 所有发现基于 WebSearch,需要实现时验证。竞态条件、资源泄漏、通知风暴等陷阱在文档中有描述,但具体行为需要测试验证 |

**Overall confidence:** HIGH (架构和栈), MEDIUM (特性和陷阱需要实现验证)

### Gaps to Address

**需要实现时验证的领域:**

1. **Windows 特定的进程管理细节:** Go 在 Windows 上的信号处理与 Linux 不同,需要验证具体行为
   - 如何处理: Phase 3 实现时进行实际测试,验证 CommandContext 在 Windows 上的超时和取消行为

2. **Nanobot 进程行为:** nanobot 是否支持优雅停止,停止时需要多长时间,是否会产生子进程
   - 如何处理: Phase 2-3 实现时测试 nanobot 的停止行为,确定最佳的停止策略(优雅停止 vs 强制终止)

3. **Pushover API 限制:** 通知速率限制、去重机制需要查看官方文档
   - 如何处理: Phase 4 实现前查看 Pushover API 文档,实现相应的速率限制和去重逻辑

4. **长期运行稳定性:** 24x7 运行时的资源使用模式需要实际测试验证
   - 如何处理: Phase 5 端到端测试时进行压力测试(启动/停止 100 次,检查内存和句柄),以及长期运行的资源监控

5. **实际竞态条件验证:** 文档中描述的竞态条件是否真实存在
   - 如何处理: Phase 3 实现时进行并发测试,验证并行停止场景,确认是否需要额外的同步机制

## Sources

### Primary (HIGH confidence)
- pkg.go.dev/golang.org/x/sync — Version v0.18.0, errgroup panic trapping 特性已验证
- pkg.go.dev/golang.org/x/sync/errgroup — 官方 API 文档,并发协调模式
- victoriametrics.com/blog/go-sync-map — sync.Map 内部机制和使用场景,Go 1.24 HashTrieMap 优化
- github.com/puzpuzpuz/go-concurrent-map-bench — sync.Map 性能基准测试

### Secondary (MEDIUM confidence)
- dev.to — errgroup 使用模式用于并发任务管理
- reddit.com/r/golang — sync.Map 性能基准测试讨论
- Docker Compose Docs — 健康检查和依赖模式,竞品分析参考
- Icinga Blog — Systemd 模板服务,多实例管理模式
- AWS Well-Architected — 优雅降级哲学,部分失败处理
- Medium — Go 并发常见错误,进程管理模式
- OneUptime Blog — Alert fatigue 预防策略
- Level Up — 基于 Context 的 Goroutine 管理
- Medium — Goroutine 管理策略和模式

### Tertiary (LOW confidence)
- Stack Overflow — YAML 重复键检测,配置验证最佳实践
- Server Fault — 服务器命名约定,实例标识最佳实践
- Reddit (r/dotnet) — 错误聚合模式,社区讨论
- Oracle Docs — 进程实例监控,通用监控概念
- Process Identification Patterns — Celonis 文档,通用标识模式

**Source quality notes:**
- 官方 Go 文档(pkg.go.dev)和知名开源项目文档(Docker, Systemd)置信度最高
- 技术博客和社区讨论(Medium, dev.to, Reddit)提供实用模式但需要验证
- 通用概念文档(Oracle, Celonis)提供思路但非 Go 特定,需要适配

---
*Research completed: 2026-03-09*
*Ready for roadmap: yes*
