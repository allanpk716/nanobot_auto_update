---
status: resolved
trigger: "nanobot 实例启动后立即退出（几秒内），今天刚出现的新问题"
created: 2026-03-26T00:00:00Z
updated: 2026-03-26T18:05:00Z
---

## Current Focus
hypothesis: 已应用修复 - 1) 使用 detachedCtx 创建命令；2) 使用 detachedCtx 启动 captureLogs goroutines
test: 用户重新运行 tmp/nanobot-auto-updater.exe，观察实例是否持续运行
expecting: 实例启动后不再立即退出，能够持续运行，日志中能够看到 nanobot 的输出
next_action: 等待用户验证结果

## Symptoms

expected: 实例启动后保持运行状态，持续提供服务
actual: 启动后立即退出（几秒内），无法持续运行
errors: 进程以 exit status 1 退出
reproduction: 启动 nanobot-auto-updater 程序，观察实例启动行为
started: 今天（2026-03-26）首次出现，之前可能有过修复（昨天修复了进程检测优先级问题）

## Eliminated

## Evidence

- timestamp: 2026-03-26T00:00:00Z
  checked: 今天的日志文件最后100行
  found: 进程启动后立即退出（2-4秒内），错误码为 exit status 1
  implication: 进程启动成功但运行时遇到错误

- timestamp: 2026-03-26T00:00:00Z
  checked: git diff 查看最近5个提交的代码变更
  found: starter.go 进行了重大重构，从 "cmd /c" 方式改为直接执行命令，并添加了 splitCommand 函数解析命令
  implication: 命令解析逻辑可能存在问题，特别是处理带空格或特殊字符的路径参数时

- timestamp: 2026-03-26T00:00:00Z
  checked: 旧版本（d7b11b4提交）的 starter.go
  found: 旧版本包含 containsPortFlag 函数和自动添加 --port 参数的逻辑
  implication: 新版本可能移除了这个重要的参数处理逻辑

- timestamp: 2026-03-26T00:00:00Z
  checked: 新版本的 starter.go
  found: 确认 containsPortFlag 函数和自动添加 port 的逻辑被完全移除
  implication: 配置中 start_command 未包含 --port 参数的实例将无法正确启动

- timestamp: 2026-03-26T00:00:00Z
  checked: config.yaml 中的 start_command 配置
  found: nanobot-me 实例的 start_command 是 "nanobot gateway"（无 --port），nanobot-work-helper 包含 "--port 18792"
  implication: nanobot-me 实例会因缺少 port 参数而启动失败

- timestamp: 2026-03-26T00:00:00Z
  checked: git diff HEAD internal/lifecycle/starter.go
  found: 工作目录中的 starter.go 有未提交的重大修改，移除了 containsPortFlag 等函数
  implication: 这个未提交的重构导致了问题 - 移除了自动添加 --port 参数的功能

- timestamp: 2026-03-26T10:00:00Z
  checked: --port 参数修复后的行为
  found: 两个实例都成功启动（--port 参数修复有效），但启动后立即以 exit status 1 退出
  implication: --port 问题已解决，但存在新的启动方式问题

- timestamp: 2026-03-26T10:00:00Z
  checked: 手动运行 `nanobot gateway` 的行为
  found: 手动运行成功，显示正常启动日志（Telegram enabled, Cron started等）
  implication: nanobot 本身正常，问题在于 nanobot-auto-updater 的启动方式（工作目录、环境变量或输出重定向）

- timestamp: 2026-03-26T10:05:00Z
  checked: starter.go 的管道处理逻辑（第116-119行）
  found: 代码创建了 stdout/stderr 管道，传给 cmd，但第118-119行的 `_ = stdoutWriter` 和 `_ = stderrWriter` 没有关闭写入端
  implication: 经典的管道死锁问题 - 当缓冲区满时，nanobot 写入 stdout/stderr 会阻塞，导致进程无法继续运行

- timestamp: 2026-03-26T10:05:00Z
  checked: starter.go 的工作目录设置
  found: cmd 结构未设置 Dir 字段，nanobot 继承 nanobot-auto-updater 的当前目录
  implication: 如果 nanobot 需要从特定目录读取配置文件，可能会启动失败

- timestamp: 2026-03-26T10:10:00Z
  checked: 修复后的编译结果
  found: go build 成功，无编译错误
  implication: 修复代码语法正确，可以测试

- timestamp: 2026-03-26T16:35:00Z
  checked: 管道修复后的用户验证结果
  found: --port 参数修复有效，管道修复已应用，两个实例都成功启动但 0.003 秒后立即以 exit status 1 退出。手动运行 `nanobot gateway` 仍然成功。
  implication: 管道死锁不是根本原因。问题在于通过 nanobot-auto-updater 启动时，nanobot 无法看到自己的日志输出（被管道捕获），因此无法判断具体的启动失败原因

- timestamp: 2026-03-26T16:40:00Z
  checked: starter.go 的 cmd 结构配置（第64行）
  found: cmd := exec.CommandContext(ctx, executable, args...) 创建了命令，但**未设置 cmd.Dir 字段**
  implication: nanobot 进程将继承 nanobot-auto-updater 的当前工作目录。如果 nanobot 需要从特定目录读取配置文件（如 config.json），将无法找到配置，导致启动失败

- timestamp: 2026-03-26T16:40:00Z
  checked: config.yaml 中的实例配置
  found: nanobot-me 实例的 start_command 是 "nanobot gateway"（无 --config 参数），nanobot-work-helper 包含 "--config C:/Users/allan716/.nanobot-work-helper/config.json"
  implication: nanobot-me 实例可能依赖从当前工作目录查找默认配置文件（如 ./config.json 或 ~/.nanobot/config.json）。如果当前目录不正确，将无法找到配置文件

- timestamp: 2026-03-26T16:45:00Z
  checked: 今天的日志文件（16:32时间段）
  found: 两个实例都成功启动（进程验证通过），但 0.003 秒后立即以 exit status 1 退出。**关键：日志中没有看到任何 nanobot 的 stdout/stderr 输出**
  implication: captureLogs goroutine 可能没有捕获到 nanobot 的输出，可能是因为：1）nanobot 在启动后立即输出错误信息并退出；2）goroutine 的异步读取在进程退出时还未完成；3）管道读取策略有问题

- timestamp: 2026-03-26T16:50:00Z
  checked: 创建测试程序直接运行 `nanobot gateway --port 18790`
  found: nanobot 成功启动并正常运行！输出显示 Telegram bot 连接成功、Cron 服务启动、Heartbeat 启动、Agent loop 启动。nanobot 完全正常，没有退出
  implication: nanobot 本身没有问题。问题在于通过 nanobot-auto-updater 启动时的环境差异：1）CREATE_NO_WINDOW 标志；2）CREATE_NEW_PROCESS_GROUP 标志；3）stdout/stderr 重定向到管道的方式；4）HideWindow 设置

- timestamp: 2026-03-26T16:55:00Z
  checked: 创建测试程序，使用与 starter.go 完全相同的启动条件
  found: nanobot 正常运行了 10 秒直到测试程序主动杀死！这证明 CREATE_NO_WINDOW、CREATE_NEW_PROCESS_GROUP、HideWindow、管道处理都没有问题
  implication: **问题不在 starter.go**！问题在于 nanobot-auto-updater 的其他代码在启动后立即杀死了进程。可能是：1）detector.go 错误检测；2）manager.go 的启动后逻辑；3）context 被取消

- timestamp: 2026-03-26T17:00:00Z
  checked: 分析日志时间线和代码流程
  found: 两个 nanobot 进程在同一毫秒（16:32:27.159）退出！追踪代码发现：1）main.go 第 172-173 行创建了 autoStartCtx 并使用 `defer cancel()`；2）第 180 行调用 StartAllInstances(autoStartCtx)；3）StartAllInstances -> StartAfterUpdate -> StartNanobotWithCapture -> exec.CommandContext(ctx, ...)
  implication: **ROOT CAUSE FOUND**！当 goroutine 在第 180 行执行完 `instanceManager.StartAllInstances(autoStartCtx)` 后返回，第 173 行的 `defer cancel()` 会立即执行，取消 context。由于 starter.go 使用了 `exec.CommandContext(ctx, ...)`，当 context 被取消时，Go 会杀死所有通过这个 context 启动的进程！

- timestamp: 2026-03-26T17:30:00Z
  checked: Context 修复（detached context）后的用户验证结果
  found: Context 修复有效！退出时间从 0.003 秒变为 1-3 秒，实例启动成功。但仍然退出，exit status 从 1 变为 120。nanobot-me 在 1.4 秒后退出，nanobot-work-helper 在 3.2 秒后退出
  implication: Context 问题已解决，进程不再被立即杀死。但存在新问题导致进程在运行一小段时间后以 exit status 120 退出。Exit status 120 是一个新的错误码，需要调查其含义

- timestamp: 2026-03-26T17:45:00Z
  checked: Windows error code 120 含义
  found: Exit code 120 (0x78) 对应 ERROR_CALL_NOT_IMPLEMENTED（此函数在此系统上不受支持）。可能是 CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP 标志组合问题，或者是 nanobot 本身的退出码
  implication: 需要验证是 Windows API 问题还是 nanobot 逻辑问题

- timestamp: 2026-03-26T17:50:00Z
  checked: 创建测试程序使用完全相同的启动配置（CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP、管道、detached context）
  found: nanobot 成功启动并正常运行 10+ 秒，捕获到了大量输出，直到被主动杀死。这证明启动配置、标志组合、管道处理**全部正确**
  implication: **问题不在 starter.go 的启动逻辑**！问题在于实际运行时的某些环境差异或 nanobot-auto-updater 的其他代码

- timestamp: 2026-03-26T17:55:00Z
  checked: starter.go 中是否设置了 cmd.Dir（工作目录）
  found: **starter.go 没有设置 cmd.Dir**！nanobot 进程会继承 nanobot-auto-updater 的当前工作目录
  implication: 如果 nanobot 需要从特定目录读取配置或文件（如当前目录下的 ./config.json 或其他资源文件），可能会因为工作目录不正确而失败，导致退出

- timestamp: 2026-03-26T18:00:00Z
  checked: captureLogs 函数的 context 使用
  found: captureLogs 在第 125-126 行被传入**原始的 ctx**（来自调用方），而不是 detachedCtx。当 main.go 中的 `defer cancel()` 执行时，captureLogs 会立即退出
  implication: 虽然这不应该导致进程退出，但可能导致日志输出没有被正确捕获。修复：使用 detachedCtx 启动 captureLogs goroutines

## Resolution

root_cause: main.go 第 172-173 行的 `autoStartCtx` 使用了 `defer cancel()`，导致当启动 goroutine 返回时，context 被立即取消。由于 starter.go 使用 `exec.CommandContext(ctx, ...)` 启动进程，当 context 被取消时，Go 运行时会杀死所有通过这个 context 启动的子进程。这导致两个 nanobot 实例在启动成功后立即被杀死（几乎同时，在同一毫秒内）。
fix: 1) 修改 starter.go，使用 `context.Background()` (detachedCtx) 而不是传入的 context 来创建命令，使进程生命周期独立于启动 context；2) 同时使用 detachedCtx 启动 captureLogs goroutines，确保日志捕获不会被父 context 取消影响
verification: ✅ 已通过用户验证 - nanobot-auto-updater.exe 和两个 nanobot.exe 实例都持续运行 55+ 秒，健康检查正常工作，日志显示"实例已恢复运行"，所有进程稳定运行无退出
files_changed: [internal/lifecycle/starter.go]
