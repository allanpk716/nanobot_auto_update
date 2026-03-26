---
status: awaiting_human_verify
trigger: "修复实例管理：使用保存的PID而不是端口检测"
created: 2026-03-26T13:45:00+08:00
updated: 2026-03-26T14:45:00+08:00
---

## Current Focus

hypothesis: 已完成PID管理重构，移除所有端口检测逻辑
test: 编译项目验证语法正确性
expecting: 编译成功，所有端口检测代码已移除
next_action: 通知用户验证完整的PID管理方案

## Symptoms

expected: 启动进程时保存PID，停止时使用保存的PID，不依赖端口检测
actual: 当前代码使用端口检测来识别进程（不可靠，因为nanobot不监听HTTP端口）
errors: |
  - nanobot gateway 命令不启动HTTP服务器
  - --port 参数只打印在日志中，没有实际用途
  - 端口检测注定失败
started: 2026-03-26T13:45:00+08:00
user_request: |
  用户指出了核心问题：
  1. 启动进程时我们已经有了PID（`cmd.Process.Pid`）
  2. 应该保存这个PID，用PID来管理进程
  3. 不需要端口检测，因为我们已经知道PID了

  用户的新理解：
  - nanobot gateway 命令**不启动HTTP服务器**
  - --port 参数只打印在日志中，没有实际用途
  - nanobot通过Telegram、Cron、Heartbeat等服务工作，不提供HTTP API
  - 端口检测注定失败，因为根本没有服务监听端口

  用户的清晰方案：
  1. 启动时杀掉所有nanobot进程
  2. 启动需要的实例，保存PID
  3. 等待几秒让进程稳定
  4. 检查PID是否还活着（进程存在=成功）
  5. 不需要端口检测（nanobot不监听HTTP端口）

## Eliminated

## Evidence

- timestamp: 2026-03-26T13:50:00+08:00
  checked: 分析当前代码的进程管理方式
  found: |
    **当前代码的问题：**
    1. `lifecycle.StartNanobotWithCapture` 返回 `error`，没有返回PID
    2. `InstanceLifecycle` 结构没有 `pid` 字段来保存PID
    3. `InstanceLifecycle.StartAfterUpdate` 没有保存PID
    4. `InstanceLifecycle.StopForUpdate` 使用 `lifecycle.IsNanobotRunning` 检测进程（不可靠）
    5. 健康检查使用端口检测（不可靠，因为nanobot gateway不监听HTTP端口）
  implication: 需要全面修改为PID管理方案

- timestamp: 2026-03-26T13:55:00+08:00
  checked: 实施PID管理方案
  found: |
    **已完成的修改（第一阶段）：**

    1. **internal/lifecycle/starter.go**
       - 修改 `StartNanobotWithCapture` 返回 `(int, error)` 而不是 `error`
       - 在成功启动后返回 `cmd.Process.Pid`
       - 所有错误返回路径都返回 `0` 作为PID

    2. **internal/instance/lifecycle.go**
       - 在 `InstanceLifecycle` 结构中添加 `pid int32` 字段
       - 修改 `StartAfterUpdate` 保存返回的PID（转换为int32）
       - 修改 `StopForUpdate`：
         - 如果pid == 0（未启动过），跳过停止
         - 如果pid > 0，直接使用保存的PID停止进程
         - 移除 `lifecycle.IsNanobotRunning` 调用（不再需要）
       - 成功停止后清除pid字段（设为0）

    3. **internal/lifecycle/capture_test.go**
       - 更新所有测试用例以接收新的返回值 `(int, error)`
       - 使用 `_` 忽略PID返回值（测试中不需要）

    **编译结果：**
    - 编译成功，无错误
  implication: PID管理方案已成功实施，代码可以编译

- timestamp: 2026-03-26T14:30:00+08:00
  checked: 完全移除端口检测逻辑
  found: |
    **已完成的修改（第二阶段）：**

    1. **internal/lifecycle/starter.go**
       - **删除了 `StartNanobot` 函数**（使用端口检测的旧函数）
       - **删除了 `waitForPortListening` 函数**（端口检测逻辑）
       - **删除了 `waitForProcessRunning` 函数**（不再需要）
       - 修改 `StartNanobotWithCapture`：
         - 移除 `waitForPortListening` 调用
         - 添加2秒的进程稳定等待时间
         - 使用 `process.NewProcess(pid)` 检查进程是否存在
         - 使用 `proc.Status()` 检查进程状态
         - 完全基于PID验证启动成功
       - 添加 `github.com/shirou/gopsutil/v3/process` 导入
       - 移除 `net` 包导入（不再需要）

    2. **internal/instance/lifecycle.go**
       - 添加 `IsRunning() bool` 方法：基于PID检查进程状态
       - 添加 `GetPID() int32` 方法：返回实例的PID

    3. **internal/lifecycle/detector.go**
       - 添加 `FindProcessByPID(pid int32, logger)` 函数：
         - 检查指定PID的进程是否存在
         - 验证进程状态（不是僵尸进程或停止状态）
         - 返回 `*process.Process` 对象

    4. **internal/lifecycle/manager.go**
       - 标记 `StartAfterUpdate` 为 DEPRECATED
       - 不再使用已删除的 `StartNanobot` 函数

    **编译结果：**
    - 编译成功，无错误
    - 所有端口检测代码已移除
  implication: 完全基于PID的进程管理方案已实施完成

## Resolution

root_cause: |
  **根本问题：**
  - 启动进程时没有保存PID，导致停止时无法精确定位进程
  - 依赖端口检测来识别进程，但nanobot不监听HTTP端口
  - 端口检测注定失败，无法正确管理进程

fix: |
  **修复方案（已完成）：**

  **第一阶段：基础PID管理**
  1. 修改 `StartNanobotWithCapture` 返回 `(int, error)` 而不是 `error`
  2. 在 `InstanceLifecycle` 中添加 `pid int32` 字段保存PID
  3. `StartAfterUpdate` 保存返回的PID
  4. `StopForUpdate` 直接使用保存的PID停止进程

  **第二阶段：完全移除端口检测**
  1. 删除 `StartNanobot` 函数（使用端口检测）
  2. 删除 `waitForPortListening` 函数（端口检测逻辑）
  3. 删除 `waitForProcessRunning` 函数（不再需要）
  4. 修改 `StartNanobotWithCapture`：
     - 移除 `waitForPortListening` 调用
     - 添加2秒进程稳定等待时间
     - 使用 `process.NewProcess(pid)` 验证进程存在
     - 使用 `proc.Status()` 检查进程状态
  5. 添加 `IsRunning()` 方法：基于PID检查进程状态
  6. 添加 `GetPID()` 方法：返回实例PID
  7. 添加 `FindProcessByPID()` 辅助函数

  **核心改进：**
  - 从"端口检测"切换到"PID管理"
  - 启动时保存PID，停止时使用PID
  - 使用进程稳定等待（2秒）而不是端口检测
  - 使用gopsutil库检查进程状态
  - 完全移除所有端口检测逻辑

verification: |
  **已验证：**
  - 代码编译成功，无语法错误
  - 所有函数签名已更新
  - 测试文件已更新
  - 所有端口检测代码已删除
  - 只使用PID进行进程管理

  **需要用户验证：**
  1. 启动修复后的程序
  2. 观察进程是否能正确启动和停止
  3. 确认进程数量不会累积（应该只有配置数量的进程）
  4. 查看日志确认PID被正确保存和使用
  5. 验证启动时等待2秒而不是检测端口
  6. 确认nanobot进程正常运行（通过Telegram等服务验证）

files_changed: [
  internal/lifecycle/starter.go,
  internal/instance/lifecycle.go,
  internal/lifecycle/detector.go,
  internal/lifecycle/manager.go,
  internal/lifecycle/capture_test.go
]
