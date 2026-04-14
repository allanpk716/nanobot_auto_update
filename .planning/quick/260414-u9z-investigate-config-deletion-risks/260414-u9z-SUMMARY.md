---
quick_id: 260414-u9z
date: 2026-04-14
status: complete
---

# 调查报告: nanobot 实例配置文件/目录被清空或删除的可能性

## 已发现的风险点

---

### RISK-1: CleanupConfig 目录级删除可能误删共享目录中的其他实例配置 (HIGH)

**文件:** `internal/nanobot/config_manager.go:315-319`

**场景:**
- 实例 A: `--config ~/.nanobot-shared/config-a.json`
- 实例 B: `--config ~/.nanobot-shared/config-b.json`（同一目录不同文件名）
- 删除实例 A → `CleanupConfig` 执行 `os.RemoveAll(filepath.Dir(configPath))`
- 删除了 `~/.nanobot-shared/` 整个目录，实例 B 的配置也被删

**根因:** `shouldSkipConfigCleanup` 只检查**精确文件路径**是否匹配，但 `CleanupConfig` 删除的是**整个父目录**。如果两个实例在同一目录下有不同文件名的配置，保护检查不会触发。

**缓解因素:** 自动生成的配置路径格式为 `~/.nanobot-{name}/config.json`，每个实例默认有独立目录。只有手动配置自定义路径时才会触发此问题。

---

### RISK-2: HandlePut 可以用空/不完整数据覆盖 nanobot 配置 (MEDIUM-HIGH)

**文件:** `internal/api/nanobot_config_handler.go:104-145`

**场景:**
- API 调用 `PUT /api/v1/instances/{name}/nanobot-config`
- Body 为 `{}` → 写入空 JSON 对象，清空所有 nanobot 设置（providers, agents, gateway 等）
- Body 为部分数据如 `{"gateway": {"port": 0}}` → 覆盖整个配置，丢失 providers、API keys 等

**根因:** HandlePut 端点接受**任何合法 JSON map**，没有对 config body 做结构校验。前端虽然通常会发送完整配置，但 API 层没有防护。

---

### RISK-3: HandleUpdate 不同步 nanobot 配置 — 可能导致配置孤岛 (MEDIUM)

**文件:** `internal/api/instance_config_handler.go:313-369`

**场景:** 通过 `PUT /api/v1/instance-configs/{name}` 修改实例的 port 或 startCommand：
- config.yaml 更新成功
- **nanobot config.json 未更新**（HandleUpdate 没有 onUpdateInstance 回调）
- 端口变更后：nanobot 监听旧端口，健康检查检查新端口 → 实例显示不健康
- startCommand 变更（新 --config 路径）：旧配置变成孤岛，新路径可能没有配置文件

**根因:** 只有 create/copy/delete 有回调（onCreateInstance/onCopyInstance/onDeleteInstance），**update 操作没有回调**来同步 nanobot 配置。

---

### RISK-4: 热重载不清理被删除实例的配置目录 (LOW)

**文件:** `internal/config/hotreload.go:209-218` + `cmd/nanobot-auto-updater/main.go:382-399`

**场景:** 通过手动编辑 config.yaml 删除某个实例后：
- 热重载检测到 instances 变更
- `OnInstancesChange` 触发：停止所有进程 → 重建 InstanceManager → 启动剩余实例
- **不会触发 `onDeleteInstance` 回调**
- 被删除实例的 nanobot 配置目录保留在磁盘上成为孤岛

**影响:** 不是直接的删除风险，但孤岛配置会随时间累积。

---

### RISK-5: HandlePut 通过路径操控可覆盖其他实例的配置 (LOW)

**文件:** `internal/api/nanobot_config_handler.go:104-145`

**场景:** 如果实例 A 的 startCommand 被改为指向实例 B 的配置路径（通过 HandleUpdate），那么调用 HandlePut 写实例 A 的 nanobot 配置时，实际会覆盖实例 B 的配置文件。

**根因:** HandlePut 从实例当前的 startCommand 解析配置路径，不检查是否与其他实例的路径冲突。

---

## 已修复的问题

**复制操作配置损坏 (commit 7e7e283):** 之前复制实例时如果 --config 路径相同，会静默覆盖源实例配置。现已修复，包含：
- 预防措施：HandleCopy 中自动检测路径冲突并生成唯一路径
- 防御措施：CloneConfig 中路径相同时跳过克隆

---

## 安全确认：不会导致配置丢失的代码路径

| 操作 | 是否安全 | 说明 |
|------|---------|------|
| 自更新流程 | ✅ 安全 | 只替换应用二进制文件，不触碰配置文件 |
| 停止/启动实例 | ✅ 安全 | 只管理进程，不修改配置文件 |
| 健康检查 | ✅ 安全 | 只读取状态，无写入操作 |
| 日志记录 | ✅ 安全 | 写入日志文件，不触碰配置 |
| config.yaml 写入 | ✅ 安全 | UpdateConfig 使用 mutex 序列化，deep copy 隔离，viper 原子写入 |
| 热重载 | ✅ 安全 | 只停止/启动进程，不删除配置文件 |
| ConfigManager.WriteConfig | ✅ 安全 | 使用 mutex 保护并发，os.MkdirAll 确保目录存在 |
