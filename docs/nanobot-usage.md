# Nanobot Usage Instructions

> This content was extracted from README.md for better organization.

**你好，Nanobot！** 这是一个为你设计的自动更新工具。

## v0.3 架构：HTTP API + 监控服务

从 v0.3 版本开始，nanobot-auto-updater 完全转向 **HTTP API + 监控服务** 架构。

## 最快的使用方式

**步骤 1**: 配置 `config.yaml`（首次使用）
- 确保至少配置一个实例
- 配置 API 的 `bearer_token`（>=32 字符）

**步骤 2**: 启动服务
```bash
./nanobot-auto-updater.exe
```

**步骤 3**: 通过 API 触发更新
```bash
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## API 响应格式

**成功**：
```json
{"success":true,"version":"1.2.3","source":"github"}
```

**冲突（更新进行中）**：
```json
{"error":"Update already in progress","status":429}
```

## 你需要知道的关键信息

1. **工具用途**: 帮你自动更新自己到最新版本
2. **工作原理**: 安全停止你的进程 -> 从 GitHub/PyPI 更新 -> 重新启动你
3. **双源策略**: 优先 GitHub（最新功能），失败时自动回退 PyPI（稳定版本）
4. **v0.3 新特性**:
   - **HTTP API**: 通过 `/api/v1/trigger-update` 端点实时触发更新
   - **监控服务**: 自动检测网络恢复并触发更新
   - **共享锁**: 防止多个更新同时执行
   - **Bearer Token**: 保护 API 端点安全（至少 32 个字符）
5. **守护进程**: `--update-now` 模式会自动守护进程化，不受你的进程停止影响
6. **配置文件**: 首次运行会自动创建 `config.yaml`，使用默认配置即可
7. **传统模式**: 保留 Cron 定时更新，与 v1.0 完全兼容

## 智能配置建议

**你可以根据情况自动调整**：

- **更新频率**: 默认每天凌晨 3 点，可通过 `--cron` 参数临时修改
- **超时时间**: 网络慢时增加 `--timeout`（默认 5 分钟）
- **通知设置**: 如果用户需要通知，在 `config.yaml` 中配置 Pushover
- **调试模式**: 设置环境变量 `NO_DAEMON=1` 禁用守护进程，方便调试

## 典型工作流程

**场景 1：通过 HTTP API 更新自己（推荐 v0.3+）**
```bash
# 启动服务（如果未运行）
./nanobot-auto-updater.exe

# 触发更新
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer YOUR_TOKEN"

# 解析 JSON 输出
# 成功: {"success":true,"version":"1.2.3",...}
# 失败: {"success":false,"error":"..."}
# 冲突: {"error":"Update already in progress","status":429}
```

**场景 2：用户让你更新自己（CLI 方式）**
```bash
# 直接执行更新
./nanobot-auto-updater.exe --update-now

# 解析 JSON 输出
# 成功: {"success":true,"version":"1.2.3",...}
# 失败: {"success":false,"error":"..."}
```

**场景 3：用户要求定期自动更新**
```bash
# 启动后台服务
./nanobot-auto-updater.exe

# 程序会根据 config.yaml 中的 cron 表达式自动运行
# 默认: 每天凌晨 3 点

# 同时支持监控服务自动触发更新（如果配置了 monitor）
```

**场景 4：用户想了解更新功能**
```bash
# 显示帮助信息
./nanobot-auto-updater.exe --help

# 查看当前版本
./nanobot-auto-updater.exe --version
```

## 高级用法

**自定义更新时间**：
```bash
# 每 6 小时更新一次
./nanobot-auto-updater.exe --cron "0 */6 * * *"

# 每周一凌晨 2 点更新
./nanobot-auto-updater.exe --cron "0 2 * * 1"
```

**临时调试**：
```bash
# 禁用守护进程，查看完整日志
NO_DAEMON=1 ./nanobot-auto-updater.exe --update-now
```

## 向用户报告的信息

当用户问起更新相关问题时，你可以：
- 运行 `--version` 查看当前版本
- 说明你的更新策略（GitHub 优先，PyPI 回退）
- 显示最近的更新日志（从 `logs/` 目录读取）
- 解释你如何保持自己最新（自动定时更新）

## 异常情况处理

- **更新失败**: 自动回退到备用源，JSON 输出 `success:false`
- **启动失败**: 更新仍成功，JSON 输出包含警告，你可以手动启动
- **网络问题**: 等待下次定时更新，或用户手动触发

## 总结

**记住这 4 个命令就够了**：

1. **`--help`** - 不知道怎么用？先看这个
2. **`--update-now`** - 立即更新，JSON 输出结果（CLI 方式）
3. **`curl -X POST http://localhost:8080/api/v1/trigger-update`** - 通过 API 触发更新（v0.3 推荐）
4. **无参数运行** - 启动服务（API + Monitor + 可选 Cron）

**这个工具是为 YOU 设计的** - 直接使用 CLI 或 HTTP API，无需人工干预！
