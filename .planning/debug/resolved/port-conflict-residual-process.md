---
status: resolved
trigger: "端口占用问题 - nanobot-me 实例启动时端口 18790 一直不可用"
created: 2026-03-24T12:50:00Z
updated: 2026-03-24T15:30:00Z
---

## Current Focus

hypothesis: 修复已完成 - (1) StopForUpdate 清理残留进程；(2) 自动添加 --port 参数
test: 编译成功，需要用户测试验证两个修复都生效
expecting: (1) 停止残留进程；(2) 启动命令包含正确端口；(3) 端口可用
next_action: 请求用户验证修复效果

## Symptoms

expected: 先停止旧进程再启动新进程
actual: 新进程尝试启动但端口被残留进程占用
errors: "Port not yet available, retrying" 持续重试
reproduction: 偶尔发生，当程序崩溃留下残留进程时
started: 启动时发生

**Detailed logs:**
```
[INFO]: 通知管理器已启动 component=notification-manager check_interval=15m0s
2026-03-24 12:47:39.762 - [INFO]: 开始自动启动所有实例 instance_count=2 timeout=5m0s
2026-03-24 12:47:39.762 - [INFO]: 开始自动启动阶段 component=instance-manager instance_count=2
2026-03-24 12:47:39.762 - [INFO]: 正在启动实例 component=instance-manager instance=nanobot-me port=18790
2026-03-24 12:47:39.762 - [INFO]: Starting instance after update instance=nanobot-me component=instance-lifecycle
2026-03-24 12:47:39.763 - [DEBUG]: Buffer cleared instance=nanobot-me component=instance-lifecycle component=logbuffer
2026-03-24 12:47:39.763 - [INFO]: Starting nanobot with log capture instance=nanobot-me component=instance-lifecycle command=nanobot gateway port=18790
2026-03-24 12:47:39.779 - [INFO]: Nanobot process started instance=nanobot-me component=instance-lifecycle pid=131096
2026-03-24 12:47:39.780 - [DEBUG]: Waiting for port to become available instance=nanobot-me component=instance-lifecycle address=127.0.0.1:18790 timeout=30s
2026-03-24 12:47:39.890 - [INFO]: Found nanobot by process name pid=138532 process_name=nanobot.exe
2026-03-24 12:47:39.890 - [INFO]: 初始状态检查 instance=nanobot-me is_running=true pid=138532 detection_method=process_name
2026-03-24 12:47:39.890 - [INFO]: Detecting nanobot process port=18792
2026-03-24 12:47:39.891 - [DEBUG]: Searching for process by name process_name=nanobot.exe
2026-03-24 12:47:39.965 - [INFO]: Found nanobot by process name pid=138532 process_name=nanobot.exe
2026-03-24 12:47:39.965 - [INFO]: 初始状态检查 instance=nanobot-work-helper is_running=true pid=138532 detection_method=process_name
2026-03-24 12:47:40.101 - [INFO]: Google 连通性检查成功 component=network-monitor duration=339 status_code=200
2026-03-24 12:47:40.101 - [INFO]: 初始连通性状态 component=network-monitor is_connected=true
2026-03-24 12:47:41.283 - [DEBUG]: Port not yet available, retrying instance=nanobot-me component=instance-lifecycle port=18790 attempt=4
2026-03-24 12:47:43.286 - [DEBUG]: Port not yet available, retrying instance=nanobot-me component=instance-lifecycle port=18790 attempt=8
2026-03-24 12:47:45.288 - [DEBUG]: Port not yet available, retrying instance=nanobot-me component=instance-lifecycle port=18790 attempt=12
```

**Key observations:**
1. 两个实例配置：nanobot-me (port=18790) 和 nanobot-work-helper (port=18792)
2. nanobot-me 启动进程 PID=131096
3. 但检测到已运行的 nanobot 进程 PID=138532 (残留进程)
4. 端口 18790 被残留进程占用，新进程无法绑定
5. 两个实例的初始状态检查都发现了同一个 PID=138532

## Eliminated

## Evidence

- timestamp: 2026-03-24T12:50:00Z
  checked: internal/instance/manager.go 的 StartAllInstances 方法
  found: StartAllInstances 直接调用 inst.StartAfterUpdate(ctx)，没有先检查或停止残留进程
  implication: 自动启动流程缺少清理残留进程的步骤

- timestamp: 2026-03-24T12:50:00Z
  checked: internal/instance/lifecycle.go 的 StartAfterUpdate 方法
  found: StartAfterUpdate 只清除日志缓冲区并调用 StartNanobotWithCapture，没有任何停止逻辑
  implication: 启动逻辑假设端口是空闲的，不处理端口冲突情况

- timestamp: 2026-03-24T12:50:00Z
  checked: internal/instance/lifecycle.go 的 StopForUpdate 方法
  found: StopForUpdate 包含完整的进程检测和停止逻辑，但这个方法在自动启动流程中没有被调用
  implication: 停止逻辑存在但没有被自动启动流程使用

- timestamp: 2026-03-24T12:51:00Z
  checked: internal/lifecycle/starter.go 的 StartNanobotWithCapture 和 waitForPortListening 方法
  found: waitForPortListening 只是被动等待端口可用，不会主动清理占用端口的进程。它会持续重试直到超时
  implication: 底层启动逻辑不负责清理残留进程，需要上层调用者在启动前确保端口空闲

- timestamp: 2026-03-24T12:51:00Z
  checked: 对比 UpdateAll 和 StartAllInstances 的实现
  found: UpdateAll 有完整的 stopAll → update → startAll 流程，而 StartAllInstances 只有启动流程，缺少停止阶段
  implication: StartAllInstances 应该复用相同的停止-启动模式，在启动前先停止该实例端口的残留进程

- timestamp: 2026-03-24T12:53:00Z
  checked: 修复实施后的 internal/instance/manager.go
  found: 在 StartAllInstances 中，启动前添加了 inst.StopForUpdate(ctx) 调用，会检测并停止残留进程
  implication: 自动启动现在会先清理残留进程再启动，避免端口冲突

- timestamp: 2026-03-24T14:05:00Z
  checked: 用户测试验证结果
  found: 残留进程停止功能正常工作（成功停止 PID 154488），但启动后端口仍然不可用。日志显示 "command=nanobot gateway port=18790"，实际执行命令缺少 --port 18790 参数
  implication: 原始根本原因（残留进程）已修复，但暴露了第二个问题：启动命令缺少端口参数

- timestamp: 2026-03-24T14:08:00Z
  checked: internal/lifecycle/starter.go:148 和 internal/instance/lifecycle.go:101
  found: StartNanobotWithCapture 直接使用传入的 command 参数执行，不会自动添加 --port。port 参数仅用于 waitForPortListening 检查端口可用性
  implication: 代码设计要求 start_command 配置包含完整启动命令（包括 --port 参数）

- timestamp: 2026-03-24T14:08:00Z
  checked: config.yaml 实例配置
  found: nanobot-me 的 start_command 是 "nanobot gateway"（缺少 --port），而 nanobot-work-helper 的 start_command 是完整命令 "nanobot gateway --config ... --port 18792"
  implication: 配置不一致导致 nanobot-me 启动时使用了错误的端口

- timestamp: 2026-03-24T14:15:00Z
  checked: 修复方案实施
  found: 在 StartNanobotWithCapture 中添加了 containsPortFlag 检查，如果命令不包含 --port 则自动添加 "--port <port>"
  implication: 代码现在会自动确保启动命令包含正确的端口参数，无需手动配置

- timestamp: 2026-03-24T14:20:00Z
  checked: 编译测试
  found: go build 成功，修复代码编译通过
  implication: 代码语法正确，可以进行功能测试

## Resolution

root_cause: 两个问题：(1) 自动启动流程缺少停止阶段导致残留进程占用端口；(2) start_command 配置缺少 --port 参数导致 nanobot 使用错误的端口
fix: (1) 在 StartAllInstances 启动前调用 StopForUpdate 清理残留进程；(2) 在 StartNanobotWithCapture 中自动检查并补充 --port 参数（如果命令中未包含）
verification: 用户验证通过 - (1) 成功检测并停止残留进程 PID 151868；(2) 成功自动添加 --port 18790 参数到启动命令。nanobot 启动失败（网络问题）与修复无关。
files_changed: [internal/instance/manager.go, internal/lifecycle/starter.go]
