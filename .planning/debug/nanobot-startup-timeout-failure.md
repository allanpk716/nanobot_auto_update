---
status: verifying
trigger: "调查问题: nanobot-auto-updater 启动实例超时失败"
created: 2026-03-25T19:50:00+08:00
updated: 2026-03-25T20:20:00+08:00
---

## Current Focus

hypothesis: 修复已应用，需要验证修复效果
test: 编译并运行修复后的程序，观察启动行为
expecting: 两个实例都能正常启动，没有进程混淆和超时错误
next_action: 编译程序并验证修复

## Symptoms

expected: 所有实例应该正常启动，无任何错误
actual: 19:40:48 之后日志出现错误提示，最终两个实例都成功启动了，实例可以正常使用
errors:
  - [ERROR]: Port not listening after timeout instance=nanobot-me port=18790 attempts=60
  - [ERROR]: Failed to start instance instance=nanobot-me
  - [ERROR]: Port not listening after timeout instance=nanobot-work-helper port=18792 attempts=60
  - [ERROR]: Failed to start instance instance=nanobot-work-helper
  - [WARN]: 自动启动完成(部分失败) started=0 failed=2
  - [WARN]: Nanobot process exited with error instance=nanobot-work-helper pid=175012 error=exit status 1
  - [WARN]: Nanobot process exited with error instance=nanobot-me pid=174944 error=exit status 1
timeline: 今天（2026-03-25）第一次出现
reproduction: 启动 nanobot-auto-updater 时自动启动实例

## Eliminated

## Evidence

- timestamp: 2026-03-25T19:50:00+08:00
  checked: 用户提供的日志摘要
  found: |
    关键时间点：
    - 19:40:48.451 - 发现旧进程 PID 171056
    - 19:40:50.021 - 启动新进程 PID 174944 (nanobot-me)
    - 19:40:50.489 - 发现另一个进程 PID 172880
    - 19:41:20.069 - nanobot-me 启动超时失败（30秒超时）
    - 19:41:20.070 - 启动新进程 PID 175012 (nanobot-work-helper)
    - 19:41:48.437 - 健康检查发现 PID 174700
    - 19:41:51.656 - nanobot-work-helper 启动超时失败（30秒超时）
    - 19:41:51.657 - 进程 174944 和 175012 退出（exit status 1）
  implication: |
    1. 自动启动的进程在30秒内未监听端口，导致超时
    2. 但后来健康检查发现了新的进程 PID 174700
    3. 最终实例能正常使用，说明实例确实启动成功了

- timestamp: 2026-03-25T20:00:00+08:00
  checked: starter.go 的 waitForPortListening 函数（第94-126行）
  found: |
    端口检测逻辑：
    - 每500ms检测一次端口是否监听
    - 使用 net.DialTimeout("tcp", address, 1*time.Second) 尝试连接
    - 如果超时（30秒），返回错误 "port X not listening after 30s"
  implication: |
    端口检测逻辑本身是正常的，问题在于为什么启动的进程（174944, 175012）
    在30秒内没有监听端口

- timestamp: 2026-03-25T20:02:00+08:00
  checked: detector.go 的 FindPIDByProcessName 函数（第37-63行）
  found: |
    进程检测逻辑：
    - 使用 process.Processes() 获取所有进程
    - 遍历所有进程，查找名称匹配的进程
    - 只返回第一个匹配的进程 PID
  implication: |
    **这是问题的关键**：FindPIDByProcessName 返回第一个匹配的 nanobot.exe 进程，
    无法区分不同端口的不同实例。当系统中有多个 nanobot.exe 进程时，会返回
    错误的 PID。

- timestamp: 2026-03-25T20:05:00+08:00
  checked: 详细日志分析
  found: |
    进程 PID 变化序列：
    1. 19:40:48.451 - 检测到旧进程 PID 171056（nanobot.exe）
    2. 19:40:48.733 - 强制终止 PID 171056
    3. 19:40:50.021 - 启动新进程 PID 174944 (nanobot-me, port 18790)
    4. 19:40:50.489 - 检测到另一个进程 PID 172880（这是从哪来的？）
    5. 19:41:20.069 - PID 174944 启动超时失败
    6. 19:41:20.110 - 尝试停止 nanobot-work-helper 时发现 PID 172880
    7. 19:41:20.344 - 强制终止 PID 172880
    8. 19:41:21.593 - 启动新进程 PID 175012 (nanobot-work-helper, port 18792)
    9. 19:41:48.437 - 健康检查发现 PID 174700（新的进程）
    10. 19:41:51.657 - PID 174944 和 175012 退出（exit status 1）
  implication: |
    **关键发现**：
    1. PID 172880 在启动 nanobot-me 之后立即出现，但不是我们启动的
    2. PID 172880 被误认为是 nanobot-work-helper 的进程并被停止了
    3. 实际启动的进程（174944, 175012）在超时后退出（exit status 1）
    4. 健康检查后来发现了新的进程 PID 174700，说明实例最终还是启动了

    **推测**：PID 172880 可能是用户手动启动的另一个实例，或者之前遗留的实例。
    由于进程检测器无法区分实例，导致误杀了其他实例的进程。

- timestamp: 2026-03-25T20:10:00+08:00
  checked: 19:40:48 时刻的初始状态检查
  found: |
    19:40:48.451 - 初始状态检查 instance=nanobot-me is_running=true pid=171056 detection_method=process_name
    19:40:48.583 - 初始状态检查 instance=nanobot-work-helper is_running=true pid=171056 detection_method=process_name
  implication: |
    **根本原因找到了**！
    1. nanobot-me 和 nanobot-work-helper 都检测到同一个进程 PID 171056
    2. 这说明当时只有一个 nanobot.exe 进程在运行
    3. nanobot-me 停止了 PID 171056 后，启动了自己的进程 PID 174944
    4. 但是 nanobot-work-helper 在初始检查时仍然检测到了旧的 PID 171056
    5. 当 nanobot-work-helper 开始停止操作时，PID 171056 已经被 nanobot-me 杀死了
    6. 此时系统中已经有一个新的进程 PID 172880（可能是 nanobot-me 的子进程或者其他来源）
    7. nanobot-work-helper 误以为 PID 172880 是自己的进程并杀死了它

    **结论**：进程检测器使用 `FindPIDByProcessName("nanobot.exe")` 无法区分不同端口的不同实例，
    导致多实例场景下的进程混淆。

- timestamp: 2026-03-25T20:15:00+08:00
  checked: detector.go 的 IsNanobotRunning 函数（第69-94行）
  found: |
    检测逻辑优先级：
    1. 首先使用 FindPIDByProcessName("nanobot.exe") 查找进程
    2. 如果找不到，才使用 FindPIDByPort(port) 查找

    这个逻辑有严重缺陷：当系统中有多个 nanobot.exe 进程时，进程名检测总是返回第一个
    匹配的进程，不管这个进程是否监听在指定的端口上。
  implication: |
    **修复方向**：
    1. 应该优先使用端口检测 FindPIDByPort(port)，因为它更精确
    2. 只有在端口检测失败时，才回退到进程名检测
    3. 或者在多实例场景下，完全放弃进程名检测，只使用端口检测

- timestamp: 2026-03-25T20:20:00+08:00
  checked: 应用修复到 detector.go
  found: |
    修改内容：
    - 调整 IsNanobotRunning 函数中的检测优先级
    - 首先使用端口检测 FindPIDByPort(port)（精确匹配）
    - 只有在端口检测失败时，才回退到进程名检测 FindPIDByProcessName（模糊匹配）
    - 更新了注释，说明端口检测更精确，进程名检测在多实例场景下不够精确
  implication: |
    修复已应用，现在需要验证：
    1. 程序已编译成功
    2. 需要停止当前运行的 nanobot-auto-updater
    3. 启动新编译的程序
    4. 观察是否还有进程混淆和超时错误

## Resolution

root_cause: |
  **进程检测器的检测优先级错误导致多实例混淆**

  在 `detector.go` 的 `IsNanobotRunning` 函数中，检测逻辑的优先级是：
  1. 首先使用 `FindPIDByProcessName("nanobot.exe")` 查找进程
  2. 只有在进程名检测失败时，才使用 `FindPIDByPort(port)` 查找

  这个逻辑在多实例场景下有严重缺陷：
  - 当系统中有多个 nanobot.exe 进程时，`FindPIDByProcessName` 总是返回第一个匹配的进程
  - 无法区分不同端口的不同实例
  - 导致实例 A 可能误判实例 B 的进程为自己的进程
  - 在停止操作时，可能误杀其他实例的进程

  **具体表现**：
  1. nanobot-me 和 nanobot-work-helper 在初始检查时都检测到同一个进程 PID 171056
  2. nanobot-me 停止了 PID 171056 并启动了自己的进程 PID 174944
  3. nanobot-work-helper 在停止操作时，检测到了一个新的进程 PID 172880（可能是 nanobot-me 的子进程）
  4. nanobot-work-helper 误以为 PID 172880 是自己的进程并杀死了它
  5. 实际启动的进程（174944, 175012）因为被干扰而无法正常启动，在超时后退出

fix: |
  **修改检测优先级：端口检测优先于进程名检测**

  在 `detector.go` 的 `IsNanobotRunning` 函数中，调整检测逻辑：
  1. 首先使用 `FindPIDByPort(port)` 查找进程（精确匹配）
  2. 只有在端口检测失败时，才回退到 `FindPIDByProcessName("nanobot.exe")`（模糊匹配）

  这样可以确保：
  - 每个实例只管理监听在自己端口上的进程
  - 避免多实例场景下的进程混淆
  - 误杀其他实例的进程

verification: |
  修复验证步骤：
  1. 编译修复后的程序
  2. 停止当前运行的 nanobot-auto-updater
  3. 启动新编译的程序
  4. 观察日志中是否还有进程混淆和超时错误
  5. 确认两个实例都能正常启动

files_changed: ["internal/lifecycle/detector.go"]
