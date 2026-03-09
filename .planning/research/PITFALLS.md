# Pitfalls Research

**Domain:** Multi-instance nanobot process management for existing auto-updater
**Researched:** 2026-03-09 (Updated for v0.2 Multi-Instance Support)
**Confidence:** MEDIUM (WebSearch-based, verified across multiple sources)

---

## Executive Summary

本文档记录了为现有单实例自动更新器添加多实例支持时的常见陷阱。v0.2 里程碑的核心挑战在于:从管理单个 nanobot 进程扩展到同时管理多个实例,同时保持系统的可靠性和可维护性。

**关键发现:**
- 多进程管理的竞态条件是最高危陷阱,可能导致资源冲突和启动失败
- 错误聚合和通知去重是用户体验的关键,处理不当会导致 alert fatigue
- 配置复杂度需严格控制,过度设计会导致维护困难
- 进程标识和日志关联是调试的基础,必须在 Phase 1 就正确实现

---

## Critical Pitfalls (v0.2 Multi-Instance Specific)

### Pitfall M1: Race Conditions During Stop/Start Sequences

**What goes wrong:**
当停止多个进程时,如果采用并行停止但未正确等待所有进程完全退出,可能导致:
- 旧进程仍占用端口/资源时新进程启动失败
- 进程状态不一致(部分停止,部分运行)
- 更新过程中文件被锁定导致升级失败

**Why it happens:**
开发者倾向于使用并行操作提高效率,但低估了进程完全退出所需的时间。Windows 进程可能需要额外时间清理资源,特别是在处理信号时。

**How to avoid:**
1. 实现严格的串行停止→等待→更新→串行启动流程
2. 每个进程停止后添加明确的等待确认(检查进程是否真正退出)
3. 使用带超时的等待机制(例如:等待最多 10 秒,超时后强制终止)
4. 在启动新进程前验证资源已释放(端口、文件锁等)

**Warning signs:**
- 日志中出现"地址已被占用"或"文件被锁定"错误
- 间歇性的进程启动失败(时好时坏)
- 更新后部分实例运行的是旧版本

**Phase to address:**
Phase 1 (Multi-Instance Process Management) - 核心进程管理逻辑实现阶段

**Sources:**
- [Golang Concurrency Mistakes](https://medium.com/@puneetpm/5-golang-concurrency-mistakes-that-are-silently-killing-your-performance-updated-for-go-1-25-5f54d88e71be) (MEDIUM)
- [Race Condition Scenarios](https://stackoverflow.com/questions/78934496/different-behaviors-in-go-race-condition-scenarios) (MEDIUM)

---

### Pitfall M2: Resource Leaks from Failed Process Handles

**What goes wrong:**
当进程启动失败或异常退出时,如果未正确清理:
- 进程句柄未释放导致内存泄漏
- 僵尸进程累积占用系统资源
- 长期运行后导致系统资源耗尽

**Why it happens:**
错误处理路径常被忽视。开发者专注于"快乐路径"(happy path),忘记在失败场景中释放资源。特别是在 Go 中,`os/exec` 进程对象需要显式管理。

**How to avoid:**
1. 使用 defer 确保进程句柄始终被清理
2. 实现进程健康检查机制,定期清理僵尸进程
3. 为每个进程维护生命周期状态机(running/stopped/failed)
4. 使用 context.Context 管理进程超时和取消

**Warning signs:**
- 系统运行一段时间后内存持续增长
- 任务管理器中发现多个已"停止"的 nanobot 进程
- 长期运行后新进程启动变慢或失败

**Phase to address:**
Phase 1 (Multi-Instance Process Management) - 进程生命周期管理

**Sources:**
- [Multiprocessing Memory Leak](https://stackoverflow.com/questions/55092139/gracefully-terminate-a-process-on-windows) (MEDIUM)
- [Zombie/Orphan Processes on Windows](https://github.com/mem0ai/mem0/issues/15423) (MEDIUM)

---

### Pitfall M3: Configuration Schema Over-Engineering

**What goes wrong:**
设计过于复杂的实例配置结构:
- 嵌套层次过深导致难以理解和维护
- 过多的可选参数导致验证逻辑复杂
- 配置文件变更时向后兼容性差

**Why it happens:**
试图一次性解决所有可能的未来需求,而非聚焦当前需求。多实例配置容易引入不必要的抽象和层次。

**How to avoid:**
1. 保持配置结构扁平化 - 实例列表包含必要的启动参数即可
2. 只添加当前里程碑所需字段,避免过度设计
3. 配置验证应简单明确:必填项检查 + 类型检查 + 基本逻辑验证
4. 为配置变更设计明确的迁移路径

**Example minimal config structure:**
```yaml
instances:
  - name: "bot-1"           # 唯一标识符
    command: "nanobot run"  # 启动命令
    workdir: "./bot1"       # 工作目录
  - name: "bot-2"
    command: "nanobot run --port 8081"
    workdir: "./bot2"
```

**Warning signs:**
- 单个配置项需要 3 层以上嵌套
- 配置验证代码比业务逻辑还长
- 用户需要文档才能理解配置结构

**Phase to address:**
Phase 1 (Multi-Instance Process Management) - 配置解析和验证

**Sources:**
- [YAML Configuration Mistakes](https://comate.baidu.com/zh/page/xe9q3bn4gmz) (LOW)
- [Go Config Silent Bug](https://buildsoftwaresystems.com/post/go-config-yaml-safer-mapstructure-fix/) (MEDIUM)

---

### Pitfall M4: Silent Failures and Incomplete Error Aggregation

**What goes wrong:**
在多实例场景中,当部分实例启动失败时:
- 只报告第一个错误,丢失其他实例的失败信息
- 错误被吞掉,用户不知道哪些实例失败
- 日志中有错误但未触发通知

**Why it happens:**
单实例思维模式的延续 - 遇到错误立即返回。在多实例场景中,应该收集所有错误后再报告,而不是遇到第一个失败就停止。

**How to avoid:**
1. 实现错误聚合模式 - 收集所有实例的错误,而非遇到第一个失败就返回
2. 结构化错误报告 - 区分"哪些成功,哪些失败,失败原因是什么"
3. 日志和通知分离 - 日志记录所有详细信息,通知只发送摘要
4. 失败实例列表明确包含实例名称和具体错误

**Example error aggregation pattern:**
```go
type UpdateResult struct {
    Successful []string     // 成功的实例名称
    Failed     []InstanceError // 失败实例及错误
}

type InstanceError struct {
    InstanceName string
    Error       error
}
```

**Warning signs:**
- 日志显示多个错误但通知只提到一个
- 用户无法定位具体哪个实例失败
- 重试时反复失败但不清楚失败范围

**Phase to address:**
Phase 2 (Error Handling and Notifications) - 错误收集和报告

**Sources:**
- [Silent Failures Detection](https://www.stacksync.com/blog/detect-silent-failures-mulesoft) (MEDIUM)
- [AggregateException Pattern](https://www.reddit.com/r/dotnet/comments/17huhdh/what_is_your_preferred_way_of_returning_multiple/) (LOW)

---

### Pitfall M5: Notification Spam from Multiple Instance Failures

**What goes wrong:**
当更新导致多个实例失败时:
- 每个实例失败都发送一条通知,造成通知风暴
- 用户收到大量重复或相似的通知,导致 alert fatigue
- 重要信息淹没在噪音中

**Why it happens:**
简单的"失败就通知"逻辑在单实例场景合理,但在多实例场景中,批量操作可能导致大量同时失败,触发大量通知。

**How to avoid:**
1. 实现通知去重和聚合 - 同一批次操作的失败合并为一条通知
2. 添加速率限制 - 限制单位时间内的通知数量
3. 智能分组 - 按失败类型或时间窗口分组通知
4. 通知分级 - 单实例失败 vs 批量失败使用不同通知策略

**Example notification strategy:**
```
# 聚合通知示例
Update completed: 3/5 instances successful
Failed instances:
- bot-2: Port 8081 already in use
- bot-5: Working directory not found
```

**Warning signs:**
- 用户开始忽略 Pushover 通知
- 短时间内收到 5+ 条相似通知
- 用户关闭通知功能以避免干扰

**Phase to address:**
Phase 2 (Error Handling and Notifications) - 通知策略

**Sources:**
- [Alert Fatigue Prevention](https://oneuptime.com/blog/post/2026-01-30-alert-fatigue-prevention/view) (MEDIUM)
- [Prometheus Alertmanager Patterns](https://last9.io/blog/prometheus-alertmanager/) (MEDIUM)

---

### Pitfall M6: Process Identification Confusion

**What goes wrong:**
当实例失败时无法准确识别:
- 错误日志中只有"实例启动失败",未指明是哪个实例
- 多个实例使用相同配置导致无法区分
- 进程名称相同,任务管理器中难以区分

**Why it happens:**
日志和错误消息设计时未考虑多实例场景。默认的日志格式缺少实例标识符。

**How to avoid:**
1. 每个实例必须有唯一标识符(名称或 ID)
2. 所有日志和错误消息包含实例标识符
3. 考虑在进程名称中包含实例标识(如果 nanobot 支持)
4. 维护实例 ID → 配置的映射,便于故障排查

**Example structured logging:**
```go
log.WithFields(log.Fields{
    "instance": instanceName,
    "phase": "startup",
}).Info("Starting nanobot process")
```

**Warning signs:**
- 用户报告"nanobot 启动失败",但无法指明哪个
- 日志中有多条错误但无法关联到具体实例
- 需要查看配置文件才能理解错误上下文

**Phase to address:**
Phase 1 (Multi-Instance Process Management) - 日志和错误消息设计

**Sources:**
- [Process Instance Monitoring](https://docs.oracle.com/cd/E13214_01/wli/docs81/manage/processmonitoring.html) (LOW)
- [Process Identification Patterns](https://docs.celonis.com/en/monitoring-running-process-instances.html) (LOW)

---

## Critical Pitfalls (v0.1 - Still Relevant)

以下陷阱来自 v0.1,在多实例场景中仍然适用,需要特别注意。

### Pitfall 1: Cannot Replace Running Binary on Windows

**What goes wrong:**
尝试替换正在运行的可执行文件导致"Access Denied"错误。Windows 锁定正在运行的可执行文件。

**How to avoid:**
1. 下载新二进制到临时位置
2. 移动/重命名旧二进制到备份位置(即使被锁定也可行)
3. 移动新二进制到目标位置
4. 下次启动时清理旧的备份文件

**Phase to address:**
Phase 1 (Core Update Logic)

**Multi-instance consideration:** 所有实例停止后才能执行二进制替换

---

### Pitfall 2: Service Control Manager (SCM) Recovery Actions Don't Work as Expected

**What goes wrong:**
Windows 服务恢复设置(失败时重启)不如预期工作,或导致系统挂起。

**How to avoid:**
1. 设置适当的 `WaitHint` 和 `CheckPoint` 值
2. 永远不要将重置周期设置为 0
3. 实现应用级健康检查而非仅依赖 SCM 恢复

**Phase to address:**
Phase 1 (Core Service Setup)

**Multi-instance consideration:** 服务级恢复与实例级恢复需分离设计

---

### Pitfall 3: Cron Scheduler Job Overlap and Pile-up

**What goes wrong:**
当定时任务执行时间超过间隔时,多个实例堆积并依次运行,导致资源耗尽。

**How to avoid:**
1. 实现互斥锁或标志防止并发任务执行
2. 使用 `SkipIfStillRunning` 选项(如果调度器支持)
3. 设计任务为幂等和自我限制

**Phase to address:**
Phase 1 (Scheduling Logic)

**Multi-instance consideration:** 多实例管理操作本身可能耗时,需防止重叠

---

### Pitfall 4: Command Prompt Window Flashes on Background Execution

**What goes wrong:**
即使作为后台服务运行,生成子进程(如 `uv` 命令)时仍会短暂显示命令提示符窗口。

**How to avoid:**
```go
cmd := exec.Command("uv", "pip", "install", "nanobot")
cmd.SysProcAttr = &syscall.SysProcAttr{
    HideWindow: true,
    CreationFlags: syscall.CREATE_NO_WINDOW,
}
```

**Phase to address:**
Phase 1 (Core Update Logic)

**Multi-instance consideration:** 所有子进程(包括多个 nanobot 实例)都需隐藏窗口

---

### Pitfall 5: Log Rotation Breaks Logging Mid-Flight

**What goes wrong:**
当日志文件轮转时,部分日志条目丢失,或 logger 继续写入旧的(已轮转的)文件句柄。

**How to avoid:**
1. 使用轮转感知的日志库(lumberjack,或 zap with lumberjack sink)
2. 配置适当的轮转阈值
3. 在负载下测试轮转

**Phase to address:**
Phase 1 (Logging Setup)

**Multi-instance consideration:** 多实例日志量大,轮转频率可能更高

---

### Pitfall 6: Configuration Zero-Value Bugs (Silent Failures)

**What goes wrong:**
配置解析静默接受无效值并使用 Go 的零值,导致难以调试的意外行为。

**How to avoid:**
1. 使用 `mapstructure` 的严格解析
2. 解析后实现显式验证
3. 添加"debug config"命令转储解析后的配置

**Phase to address:**
Phase 1 (Configuration)

**Multi-instance consideration:** 实例列表为空、实例名称重复等需要验证

---

### Pitfall 7: HTTP Client Default Timeout is None

**What goes wrong:**
HTTP 请求在服务器无响应时无限期挂起,阻塞更新过程。

**How to avoid:**
```go
client := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        ResponseHeaderTimeout: 10 * time.Second,
    },
}
```

**Phase to address:**
Phase 1 (Network Operations)

**Multi-instance consideration:** 多实例场景下网络操作更多,超时影响更大

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| 并行停止所有实例 | 更快的停止速度 | 竞态条件,资源冲突 | Never - 始终串行化 |
| 只报告第一个错误 | 简化错误处理 | 丢失重要失败信息 | Never - 必须聚合 |
| 跳过进程退出确认 | 简化代码 | 僵尸进程,资源泄漏 | Never - 必须等待确认 |
| 每个失败独立通知 | 实现简单 | 通知风暴,alert fatigue | MVP 阶段可接受,但需尽快重构 |
| 不验证配置唯一性 | 简化验证逻辑 | 运行时混淆,难以调试 | Never - 配置加载时验证 |
| 假设进程总是能优雅退出 | 简化清理逻辑 | 挂起进程,超时等待 | Never - 必须处理强制终止场景 |
| Skip verification after update download | Faster development | Corrupted updates cause mysterious failures | Never - always verify checksums/signatures |
| Use global singletons for scheduler/logger | Less parameter passing | Hard to test, hard to reset between tests | Never in production code |
| Ignore `uv` exit codes | Simpler error handling | Silent update failures, stale versions | Never |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| **Go os/exec** | 使用 `yaml` 标签配置进程结构体 | 使用 `mapstructure` 标签,Viper 内部使用 mapstructure |
| **Windows Process Kill** | 直接调用 `Process.Kill()` | 先发送优雅停止信号,超时后再强制终止 |
| **Context Cancellation** | 依赖默认 SIGKILL 行为 | 实现自定义信号处理,允许进程清理资源 |
| **Viper Config** | 假设 `AutomaticEnv` 总是工作 | 显式设置环境变量绑定,测试覆盖 |
| **Process Wait** | 不调用 `cmd.Wait()` 导致僵尸进程 | 始终在 goroutine 中等待进程退出 |
| **YAML Validation** | 只验证必填项 | 验证实例名称唯一性,启动命令有效性 |
| **uv package manager** | Assuming uv is in PATH | Use full path or verify `uv` in PATH at startup |
| **uv package manager** | Ignoring python version compatibility issues | Check `requires-python` before installing |
| **uv package manager** | Not handling network failures during install | Implement retry with backoff for `uv pip install` |
| **Windows Service** | Not handling Session 0 isolation | Ensure no UI dependencies; test in Session 0 |
| **Windows Service** | Using current user's environment variables | Services run as SYSTEM - use service-specific config paths |
| **File system** | Hardcoding paths like `C:\Users\...` | Use `%PROGRAMDATA%` or `%ALLUSERSPROFILE%` for shared data |
| **Notification webhooks** | No timeout on webhook calls | Set client timeout; use fire-and-forget with queue for reliability |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| 串行启动 N 个实例 | 启动时间随实例数线性增长 | 并行启动(但要验证资源无冲突) | 实例数 > 5 |
| 每次更新重新解析配置 | 配置解析重复执行 | 启动时解析一次,缓存在内存 | 配置文件 > 100KB |
| 全量进程健康检查 | CPU 占用高,日志量大 | 采样检查 + 异常时全量检查 | 实例数 > 10 |
| 同步等待所有实例启动 | 启动时间取决于最慢实例 | 设置启动超时,独立报告慢实例 | 单个实例启动 > 30 秒 |
| No connection pooling | Many TIME_WAIT sockets, slow requests | Use `http.Transport` with connection pooling | 10+ concurrent requests |
| Unbounded log file growth | Disk space exhaustion | Implement log rotation with size limits | Days/weeks of operation |
| Synchronous updates on startup | Slow service startup | Run updates in background; use cached version for startup | When updates take >30 seconds |
| No request queuing | Memory exhaustion under load | Implement bounded queue for operations | High update check frequency |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| 配置文件包含明文凭证 | 凭证泄露风险 | 使用环境变量或加密存储 |
| 未验证启动命令路径 | 任意代码执行 | 验证命令路径在允许目录内 |
| 实例间无隔离 | 一个实例故障影响其他 | 资源隔离(不同工作目录) |
| 配置文件权限过宽 | 配置被篡改 | 限制配置文件访问权限 |
| Downloading updates over HTTP | Man-in-the-middle attack delivers malicious binary | Always use HTTPS; verify TLS certificate |
| No signature verification on updates | Compromised CDN delivers malicious binary | Sign updates; verify signature before applying |
| Storing credentials in config file | Credential exposure if file is read | Use Windows Credential Manager or environment variables |
| Running service as SYSTEM unnecessarily | Privilege escalation if service is compromised | Use minimal privilege service account |
| Logging sensitive data | Credential/token exposure in logs | Redact sensitive fields before logging |
| No integrity check on downloaded binaries | Corrupted update breaks installation | Always verify checksum/hash before applying |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| "实例启动失败"无上下文 | 用户不知道哪个实例失败 | "实例 'bot-1' 启动失败: 端口 8080 已被占用" |
| 通知消息技术化 | 用户无法理解错误原因 | 提供人类可读的错误描述 + 建议操作 |
| 配置错误报 YAML 解析错误 | 用户需要懂 YAML 语法 | 提供配置示例 + 字段级错误提示 |
| 批量操作无进度反馈 | 用户不知道系统是否在工作 | 提供实时进度:"正在启动实例 2/5" |
| 错误日志分散 | 用户难以关联相关信息 | 结构化日志,包含操作 ID/批次 ID |
| Silent updates with no notification | Users unaware of changes; surprised by behavior changes | Log update events; optionally notify on major updates |
| No rollback mechanism | Stuck on broken version | Keep previous binary; provide rollback command |
| Blocking updates during critical work | Disruption to user workflow | Defer updates; apply during idle periods |
| Cryptic error messages | Users can't troubleshoot or report issues | Include actionable error messages with context |
| No "check for update" command | Users forced to wait for scheduled check | Provide manual update trigger |

---

## "Looks Done But Isn't" Checklist

**v0.2 Multi-Instance Checklist:**
- [ ] **Multi-Instance Stop:** 所有进程真正退出(检查任务管理器) — 验证无僵尸进程
- [ ] **Configuration Loading:** 配置文件格式错误时明确提示 — 验证错误消息包含行号和字段
- [ ] **Error Aggregation:** 批量操作中所有失败都被记录 — 验证错误日志数量与失败实例数匹配
- [ ] **Process Cleanup:** 进程启动失败后无资源泄漏 — 验证句柄和内存释放
- [ ] **Notification Dedup:** 批量失败时通知被聚合 — 验证只发送一条摘要通知
- [ ] **Instance Identification:** 日志中能识别具体实例 — 验证每条日志包含实例 ID
- [ ] **Graceful Shutdown:** 进程有机会清理资源 — 验证临时文件被删除
- [ ] **Config Validation:** 实例名称唯一性被验证 — 验证重复名称被拒绝

**v0.1 Core Checklist (Still Relevant):**
- [ ] **Update Process:** Often missing rollback capability - verify you can revert to previous version
- [ ] **Service Installation:** Often missing proper uninstall/cleanup - verify service can be cleanly removed
- [ ] **Log Rotation:** Often missing handling of log during rotation - verify no logs lost during rotation
- [ ] **Error Recovery:** Often missing retry after network failure - verify update retries on transient failures
- [ ] **Graceful Shutdown:** Often missing wait for in-flight operations - verify clean shutdown with pending operations
- [ ] **Configuration:** Often missing validation of required fields - verify startup fails on missing config
- [ ] **Windows Service:** Often missing testing in Session 0 - verify service works when started by SCM

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| 僵尸进程累积 | LOW | 1. 重启 auto-updater 服务<br>2. 如有残留,手动 taskkill |
| 竞态条件导致启动失败 | MEDIUM | 1. 停止所有实例<br>2. 等待 5 秒<br>3. 重新执行启动 |
| 配置错误 | LOW | 1. 修正配置文件<br>2. 重启服务 |
| 通知风暴 | LOW | 1. 服务内部有速率限制会自动恢复<br>2. 用户可暂时关闭 Pushover 通知 |
| 部分实例失败 | LOW | 1. 查看日志识别失败实例<br>2. 手动启动失败实例或下次 cron 自动重试 |
| 资源泄漏(内存/句柄) | MEDIUM | 1. 重启 auto-updater 服务<br>2. 长期需要修复代码 |
| Corrupted update blocks service | HIGH | Manual intervention: stop service, delete corrupted binary, restore backup or reinstall |
| Log rotation broke logging | LOW | Restart service; logs resume to new file |
| Config error causes crash loop | MEDIUM | Boot into safe mode / use alternate config path; fix config file |
| Scheduler stuck in overlap | LOW | Restart service; clears job queue |
| Network timeout blocks update | LOW | Automatic: retry with backoff; manual: check network/connectivity |

---

## Pitfall-to-Phase Mapping

**v0.2 Multi-Instance Phases:**

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 竞态条件(停止/启动) | Phase 1 (Process Management) | 集成测试:并行停止 3+ 实例,验证无端口冲突 |
| 资源泄漏 | Phase 1 (Process Management) | 压力测试:启动/停止 100 次,检查内存和句柄 |
| 配置过度复杂 | Phase 1 (Process Management) | 代码审查:配置结构体不超过 2 层嵌套 |
| 错误未聚合 | Phase 2 (Error Handling) | 集成测试:模拟 3 个实例失败,验证错误包含所有 3 个 |
| 通知风暴 | Phase 2 (Notifications) | 集成测试:5 个实例同时失败,验证只发送 1 条通知 |
| 进程识别混淆 | Phase 1 (Process Management) | 日志审查:每条日志包含实例标识符 |
| 配置验证不足 | Phase 1 (Process Management) | 单元测试:重复名称、缺失字段、无效命令都被拒绝 |
| 优雅退出处理 | Phase 1 (Process Management) | 集成测试:强制停止场景,验证资源清理 |

**v0.1 Core Phases (Still Relevant):**

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Cannot Replace Running Binary | Phase 1 | Test update while service is running; verify rename-then-replace works |
| SCM Recovery Actions | Phase 1 | Kill service process; verify SCM restarts it |
| Cron Scheduler Job Overlap | Phase 1 | Schedule job at 1-minute interval; make job take 2 minutes; verify no overlap |
| Command Prompt Window Flashes | Phase 1 | Run service; trigger update; verify no visible windows |
| Log Rotation Breaks Logging | Phase 1 | Fill log to rotation threshold; verify logging continues to new file |
| Configuration Zero-Value Bugs | Phase 1 | Provide config with typo; verify startup fails with clear error |
| HTTP Client Default Timeout | Phase 1 | Point at server that never responds; verify request times out |

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| 配置解析 | Viper mapstructure 标签错误 | 使用 mapstructure 标签而非 yaml 标签,测试嵌套结构 |
| 进程启动 | 未等待进程完全退出 | 添加 Wait() 调用,设置超时机制 |
| 错误处理 | 只记录第一个错误 | 实现错误切片,收集所有失败后再报告 |
| 通知发送 | 失败立即发送通知 | 实现通知队列,批量聚合后再发送 |
| 日志记录 | 缺少实例上下文 | 在 logger 中注入实例 ID 字段 |
| 单元测试 | 难以模拟多进程场景 | 使用接口抽象进程管理,便于 mock |
| Multi-instance stop/start | 竞态条件导致资源冲突 | 严格串行化,每个步骤验证完成再继续 |
| Error aggregation | 丢失部分实例的失败信息 | 使用错误切片,记录所有失败实例 |
| Notification design | 通知风暴导致用户忽略 | 实现聚合和速率限制 |

---

## Sources

### High Confidence Sources
- (None - all findings from WebSearch require validation)

### Medium Confidence Sources (v0.2 Multi-Instance)
- [5 Golang Concurrency Mistakes](https://medium.com/@puneetpm/5-golang-concurrency-mistakes-that-are-silently-killing-your-performance-updated-for-go-1-25-5f54d88e71be) - Go 并发常见错误
- [Go Context Process Management](https://github.com/golang/go/issues/22757) - Go 官方进程管理提案
- [Alert Fatigue Prevention](https://oneuptime.com/blog/post/2026-01-30-alert-fatigue-prevention/view) - 通知疲劳预防策略
- [Go Config mapstructure Tags](https://buildsoftwaresystems.com/post/go-config-yaml-safer-mapstructure-fix/) - Viper 配置最佳实践
- [Cascading Failure Resilience](https://www.sciencedirect.com/science/article/pii/S016740482400375X) - 级联失败研究

### Low Confidence Sources (v0.2 Multi-Instance)
- [Process Instance Monitoring](https://docs.oracle.com/cd/E13214_01/wli/docs81/manage/processmonitoring.html) - 通用进程监控概念
- [YAML Configuration Mistakes](https://comate.baidu.com/zh/page/xe9q3bn4gmz) - YAML 通用错误
- [Error Aggregation Patterns](https://www.reddit.com/r/dotnet/comments/17huhdh/what_is_your_preferred_way_of_returning_multiple/) - 社区讨论

### Medium Confidence Sources (v0.1 Core)
- Microsoft Learn: "Descriptions of some best practices when you create Windows Services" - https://support.microsoft.com/en-us/topic/descriptions-of-some-best-practices-when-you-create-windows-services-13ca508e-231d-43e6-b960-3b04ccf79064
- Microsoft Learn: "Guidelines for Services" - https://learn.microsoft.com/en-us/windows/win32/rstmgr/guidelines-for-services
- InfoQ: "The Service and the Beast: Building a Windows Service that Does Not Fail to Restart" - https://infoq.com/articles/windows-services-reliable-restart
- Stephen Cleary Blog: "Win32 Service Gotcha: Recovery Actions" - https://blog.stephencleary.com/2020/06/servicebase-gotcha-recovery-actions.html
- GitHub go-co-op/gocron Issue #385: "CPU usage 100% after system time change" - https://github.com/go-co-op/gocron/issues/385
- Stack Overflow: "How to hide command prompt window when using Exec in Golang" - https://stackoverflow.com/questions/42500570
- GitHub golang/go Issue #69939: "syscall: special case cmd.exe /c in StartProcess" - https://github.com/golang/go/issues/69939
- GitHub natefinch/lumberjack Issues: Log rotation problems - https://github.com/natefinch/lumberjack/issues
- Medium: "Implementing Log File Rotation in Go: Insights from logrus, zap, and slog" - https://leapcell.io/blog/log-rotation-and-file-splitting-in-go
- Build Software Systems: "Go Config: Stop the Silent YAML Bug (Use mapstructure for Safety)" - https://buildsoftwaresystems.com/post/go-config-yaml-safer-mapstructure-fix/
- DEV Community: "Mastering Network Timeouts and Retries in Go" - https://dev.to/jones_charles_ad50858dbc0/mastering-network-timeouts-and-retries-in-go
- Lokal.so: "Comprehensive Guide on Golang Self-upgrading binary" - https://lokal.so/blog/comprehensive-guide-on-golang-go-self-upgrading-binary/
- GitHub creativeprojects/go-selfupdate - https://github.com/creativeprojects/go-selfupdate
- GitHub fynelabs/selfupdate - https://github.com/fynelabs/selfupdate
- Microsoft Tech Community: "Application Compatibility - Session 0 Isolation" - https://techcommunity.microsoft.com/blog/askperf/application-compatibility---session-0-isolation/372361
- Core Technologies Blog: "Investigating OneDrive Failures in Session 0 on Windows Server" - https://www.coretechnologies.com/blog/alwaysup/onedrive-fails-in-session-0/
- GitHub astral-sh/uv Issues: Various compatibility and behavior issues - https://github.com/astral-sh/uv/issues

---

## Gaps to Address

**Topics needing phase-specific research:**
1. **Windows 特定的进程管理细节** - Go 在 Windows 上的信号处理与 Linux 不同,需要验证具体行为
2. **Nanobot 进程行为** - nanobot 是否支持优雅停止?停止时需要多长时间?是否会产生子进程?
3. **Pushover API 限制** - 通知速率限制、去重机制需要查看官方文档
4. **长期运行稳定性** - 24x7 运行时的资源使用模式需要实际测试验证

**Validation needed during implementation:**
- 实际测试并行停止场景,确认竞态条件是否真实存在
- 验证 Go 的 CommandContext 在 Windows 上的具体行为
- 测试通知聚合策略的有效性

---

*Pitfalls research for: Multi-instance nanobot auto-updater (v0.2)*
*Original research (v0.1): 2026-02-18*
*Updated for multi-instance: 2026-03-09*
