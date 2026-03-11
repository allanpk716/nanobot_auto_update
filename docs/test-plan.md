# 多实例集成手动测试计划

## 测试目标

验证 Phase 10-01 多实例集成的完整功能,包括:
1. 多实例模式检测和加载
2. `--update-now` 立即更新模式
3. Legacy 单实例模式向后兼容
4. 日志追踪完整性
5. 错误通知正确性 (可选)
6. 资源管理和长期稳定性

## 前置条件

- Windows 操作系统
- Go 1.21+ 已安装
- UV 包管理器已安装 (`pip install uv`)
- Pushover 账号 (可选,用于通知测试)
- 项目已克隆到本地

## 测试环境准备

### 1. 编译程序

```bash
# 从项目根目录执行
go build -o nanobot-auto-updater.exe ./cmd/nanobot-auto-updater
```

预期结果: 编译成功,生成 `nanobot-auto-updater.exe` 文件

### 2. 准备测试配置文件

#### 多实例配置 (tmp/manual_test_multi.yaml)

```yaml
cron: "*/5 * * * *"
instances:
  - name: "test-gateway"
    port: 18790
    start_command: "echo test-gateway"
    startup_timeout: 5s
  - name: "test-worker"
    port: 18791
    start_command: "echo test-worker"
    startup_timeout: 5s

pushover:
  api_token: "YOUR_PUSHOVER_API_TOKEN"
  user_key: "YOUR_PUSHOVER_USER_KEY"
```

#### Legacy 配置 (tmp/manual_test_legacy.yaml)

```yaml
cron: "*/5 * * * *"
nanobot:
  port: 18790
  startup_timeout: 30s

pushover:
  api_token: "YOUR_PUSHOVER_API_TOKEN"
  user_key: "YOUR_PUSHOVER_USER_KEY"
```

## 测试用例

### 1. 多实例配置验证

**测试步骤:**

1. 使用多实例配置运行 `--update-now` 模式:
   ```bash
   ./nanobot-auto-updater.exe --config tmp/manual_test_multi.yaml --update-now --timeout 30s
   ```

2. 观察日志输出

**预期结果:**
- 日志显示: `Running in multi-instance mode`
- 日志显示: `instance_count: 2`
- 日志显示每个实例的停止和启动操作:
  - `instance=test-gateway component=instance-lifecycle`
  - `instance=test-worker component=instance-lifecycle`
- 输出 JSON 包含 `success` 字段
- 由于 `echo` 命令不会监听端口,启动会失败,但这是预期行为

**验证点:**
- [ ] 模式检测正确 (`Running in multi-instance mode`)
- [ ] 实例数量正确 (`instance_count: 2`)
- [ ] 每个实例的日志包含 `instance` 和 `component` 字段
- [ ] 双层错误检查工作正常 (UV 更新成功,实例启动失败)

### 2. Legacy 配置验证

**测试步骤:**

1. 使用 legacy 配置运行 `--update-now` 模式:
   ```bash
   ./nanobot-auto-updater.exe --config tmp/manual_test_legacy.yaml --update-now --timeout 30s
   ```

2. 观察日志输出

**预期结果:**
- 日志显示: `Running in legacy single-instance mode`
- 日志显示: `port: 18790`
- 使用旧的 `lifecycle.Manager` 逻辑

**验证点:**
- [ ] 模式检测正确 (`Running in legacy single-instance mode`)
- [ ] 端口配置正确 (`port: 18790`)
- [ ] Legacy 逻辑工作正常

### 3. 日志追踪验证

**测试步骤:**

1. 运行多实例更新 (同测试用例 1)
2. 检查日志输出的结构化字段

**预期结果:**
- 每个实例的日志包含:
  - `instance` 字段 (实例名称)
  - `component` 字段 (`instance-lifecycle` 或 `instance-manager`)
  - 停止和启动操作的详细状态

**验证点:**
- [ ] 所有实例日志包含 `instance` 字段
- [ ] 所有实例日志包含 `component` 字段
- [ ] 停止操作日志包含 `port` 字段
- [ ] 启动操作日志包含 `command` 和 `port` 字段

### 4. 错误通知验证 (可选)

**前置条件:**
- 配置了有效的 Pushover API Token 和 User Key
- 环境变量: `PUSHOVER_TOKEN` 和 `PUSHOVER_USER`

**测试步骤:**

1. 使用错误的 `start_command` 触发失败:
   ```yaml
   instances:
     - name: "test-fail"
       port: 18790
       start_command: "invalid-command-that-does-not-exist"
       startup_timeout: 5s
   ```

2. 运行 `--update-now` 模式
3. 检查 Pushover 通知

**预期结果:**
- 收到 Pushover 通知
- 通知消息包含失败实例的详细信息:
  - 实例名称
  - 端口
  - 错误原因

**验证点:**
- [ ] Pushover 通知成功发送
- [ ] 通知消息包含失败实例数量
- [ ] 通知消息包含失败实例名称
- [ ] 通知消息包含错误原因

### 5. 资源管理和长期稳定性验证

#### 快速验证 (15-20 分钟)

**测试步骤:**

1. 运行单元测试中的长期运行测试:
   ```bash
   go test -v ./cmd/nanobot-auto-updater -run TestMultiInstanceLongRunning -timeout 2h
   ```

2. 观察测试输出

**预期结果:**
- 测试完成 10 次更新周期
- 内存稳定 (增长率 < 1.5x)
- Goroutine 稳定 (差异 < 25)

**验证点:**
- [ ] 测试成功完成
- [ ] 内存增长率 < 1.5x
- [ ] Goroutine 差异 < 25
- [ ] 无 panic 或 fatal error

#### 完整验证 (24-48 小时)

**测试步骤:**

1. 创建 5 分钟定时周期的配置文件:
   ```yaml
   cron: "*/5 * * * *"  # 每 5 分钟执行一次
   instances:
     - name: "production"
       port: 18790
       start_command: "C:/path/to/nanobot.exe"
       startup_timeout: 30s
   ```

2. 启动定时任务模式:
   ```bash
   ./nanobot-auto-updater.exe --config tmp/long_running.yaml
   ```

3. 资源监控步骤:

   **使用 Windows 任务管理器:**
   - 打开任务管理器 (Ctrl+Shift+Esc)
   - 找到 `nanobot-auto-updater.exe` 进程
   - 记录以下指标:
     - 内存 (私有工作集)
     - 句柄数量 (在"详细信息"标签页中右键列标题 -> 选择列 -> 勾选"句柄")

   **记录时间表:**
   - 启动时: 记录初始值
   - 每 4-6 小时: 记录一次
   - 24 小时: 记录一次
   - 48 小时: 最终记录

4. 创建监控日志文件 (tmp/resource_monitor.csv):
   ```csv
   Timestamp,Memory_MB,Handles,UpdateCycles,Notes
   2026-03-11 14:00,15.2,150,0,Initial
   2026-03-11 18:00,16.1,152,48,4h check
   2026-03-11 22:00,15.8,151,96,8h check
   ...
   ```

**预期结果:**
- 24-48 小时稳定运行
- 内存趋势稳定 (无持续增长)
- 句柄数量稳定 (无持续增长)
- 日志模式一致

**验收标准:**

| 指标 | 正常范围 | 异常标志 |
|------|---------|---------|
| 内存 (私有工作集) | < 50 MB, 无持续增长趋势 | > 100 MB 或持续增长 |
| 句柄数量 | < 500, 波动 < 50 | > 1000 或持续增长 |
| 更新周期完成率 | 100% | < 95% |
| Panic/Fatal Error | 0 | > 0 |

**验证点:**
- [ ] 程序运行 24-48 小时无崩溃
- [ ] 内存使用稳定 (无持续增长)
- [ ] 句柄数量稳定 (无持续增长)
- [ ] 所有更新周期正常完成
- [ ] 日志模式一致,无异常错误

**正常/异常判断标准:**

✅ **正常:**
- 内存波动在 ±10 MB 范围内
- 句柄数量波动在 ±100 范围内
- 偶尔的启动失败 (如果实例配置正确,应该 100% 成功)

❌ **异常:**
- 内存每 4 小时增长 > 5 MB
- 句柄数量每 4 小时增长 > 50
- 频繁的 UV 更新失败
- Panic 或 Fatal Error

**停止测试:**
- 按 `Ctrl+C` 优雅停止
- 验证程序正确清理资源并退出

## 验收标准汇总

所有测试用例必须通过以下验收标准:

### 功能验收
- [x] 多实例模式检测正确
- [x] Legacy 模式向后兼容
- [x] 日志追踪完整 (包含 `instance` 和 `component` 字段)
- [x] 双层错误检查工作正常 (UV 更新失败 + 实例失败)

### 性能验收
- [x] 快速验证: 10 次更新周期,内存稳定,goroutine 稳定
- [ ] 完整验证: 24-48 小时运行,内存和句柄稳定 (可选)

### 通知验收 (可选)
- [ ] Pushover 通知正确发送
- [ ] 通知消息包含详细错误信息

## 测试报告模板

```
## Phase 10-01 手动测试报告

**测试日期:** YYYY-MM-DD
**测试人员:** [姓名]
**测试环境:** Windows [版本], Go [版本], UV [版本]

### 测试结果汇总

| 测试用例 | 状态 | 备注 |
|---------|------|------|
| 多实例配置验证 | PASS/FAIL | |
| Legacy 配置验证 | PASS/FAIL | |
| 日志追踪验证 | PASS/FAIL | |
| 错误通知验证 | PASS/FAIL/N/A | |
| 快速稳定性验证 | PASS/FAIL | |
| 完整稳定性验证 | PASS/FAIL/N/A | |

### 资源监控数据 (可选)

| 时间 | 内存 (MB) | 句柄数量 | 更新周期数 | 备注 |
|------|----------|----------|-----------|------|
| 启动 | | | | |
| 24h | | | | |
| 48h | | | | |

### 发现的问题

1. [问题描述]
   - 严重程度: Critical/Major/Minor
   - 重现步骤:
   - 预期结果:
   - 实际结果:

### 总体评估

[ ] 所有功能测试通过
[ ] 资源管理正常
[ ] 可以进入下一阶段

**签名:** _________________
```

## 附录: 常见问题排查

### 问题 1: UV 更新失败

**症状:** 日志显示 `UV update failed`

**可能原因:**
- UV 未安装或不在 PATH 中
- 网络连接问题 (无法访问 GitHub)
- UV 版本过旧

**解决方案:**
```bash
# 检查 UV 安装
uv --version

# 更新 UV
pip install --upgrade uv

# 手动测试 UV 更新
uv tool install --force git+https://github.com/HKUDS/nanobot.git
```

### 问题 2: 实例启动失败

**症状:** 日志显示 `Port not listening after timeout`

**可能原因:**
- `start_command` 配置错误
- 端口被占用
- 启动超时时间过短

**解决方案:**
```bash
# 检查端口占用
netstat -ano | findstr :18790

# 增加 startup_timeout
startup_timeout: 60s

# 验证 start_command
# 在命令行手动执行 start_command 查看是否有错误
```

### 问题 3: Pushover 通知未发送

**症状:** 无 Pushover 通知,但配置正确

**可能原因:**
- API Token 或 User Key 错误
- 网络连接问题
- Pushover 服务问题

**解决方案:**
```bash
# 检查环境变量
echo %PUSHOVER_TOKEN%
echo %PUSHOVER_USER%

# 测试 Pushover API
curl -X POST -d "token=YOUR_TOKEN&user=YOUR_USER&message=Test" https://api.pushover.net/1/messages.json
```

### 问题 4: 内存持续增长

**症状:** 任务管理器显示内存每 4 小时增长 > 5 MB

**可能原因:**
- Goroutine 泄漏
- 未关闭的资源 (文件句柄、网络连接)
- 日志文件过大

**排查步骤:**
1. 检查 goroutine 数量 (使用 pprof):
   ```bash
   # 在代码中添加 pprof endpoint
   go tool pprof http://localhost:6060/debug/pprof/goroutine
   ```

2. 检查日志文件大小:
   ```bash
   dir logs
   ```

3. 检查句柄数量 (任务管理器)

---

**文档版本:** 1.0
**创建日期:** 2026-03-11
**最后更新:** 2026-03-11
