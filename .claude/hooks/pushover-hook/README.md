# Claude Code 通知 Hook

支持 Pushover 和 Windows 10/11 原生通知，默认双通道同时发送。

## 禁用通知

您可以独立控制每个通知通道：

### 禁用 Pushover 通知

在项目根目录创建 `.no-pushover` 文件：

```bash
# Windows
type nul > .no-pushover

# Linux/Mac
touch .no-pushover
```

### 禁用 Windows 通知

在项目根目录创建 `.no-windows` 文件：

```bash
# Windows
type nul > .no-windows

# Linux/Mac
touch .no-windows
```

### 恢复通知

删除对应的文件即可恢复该通道的通知：

```bash
# Windows
del .no-pushover
del .no-windows

# Linux/Mac
rm .no-pushover
rm .no-windows
```

**注意**：
- 每个禁用文件对当前项目单独生效
- 同时存在 `.no-pushover` 和 `.no-windows` 将禁用所有通知
- Windows 原生通知仅支持 Windows 10/11，其他平台会自动跳过

## 问题诊断

如果通知未收到，按以下步骤排查：

### 快速诊断

在项目目录中运行：

```bash
python .claude/hooks/pushover-hook/diagnose.py
```

### 常见问题

#### 1. 环境变量未设置 ❌

**症状**: `debug.log` 显示 `ERROR: Missing env vars`

**解决方法**:

```bash
# Windows CMD
set PUSHOVER_TOKEN=your_app_token
set PUSHOVER_USER=your_user_key

# Windows PowerShell
$env:PUSHOVER_TOKEN="your_app_token"
$env:PUSHOVER_USER="your_user_key"

# Linux/Mac
export PUSHOVER_TOKEN=your_app_token
export PUSHOVER_USER=your_user_key
```

#### 2. Hook 未触发

**症状**: `debug.log` 不存在或为空

**检查**:
- `.claude/settings.json` 是否在项目根目录
- 脚本路径是否正确
- 运行诊断脚本验证配置

#### 3. API 调用失败

**症状**: `debug.log` 显示 HTTP 400/401 错误

**检查**:
- Token 是否有效: https://pushover.net/apps
- User Key 是否正确: https://pushover.net/
- API 是否启用（确保是 API Token，不是仅 SDK）

## 测试通知

### 测试 Pushover 通知

```bash
python .claude/hooks/pushover-hook/test-pushover.py
```

### 测试 Windows 通知 (Windows 10/11)

```bash
python .claude/hooks/pushover-hook/test-windows-notification.py
```

## 部署到新项目

1. 复制整个 `.claude` 文件夹到目标项目
2. 设置环境变量 `PUSHOVER_TOKEN` 和 `PUSHOVER_USER`
3. 运行诊断脚本验证配置
4. 触发一个 Claude Code 任务测试

## 升级说明

如果您从旧版本升级：
- 直接运行 `python install.py` 即可
- 安装脚本会自动：
  - ✅ 复制新文件到 pushover-hook/ 子目录
  - ✅ 删除旧位置的脚本文件
  - ✅ 更新 settings.json 中的路径
  - ✅ 备份现有配置（settings.json.backup_*）

## 日志轮转 (Log Rotation)

调试日志会自动轮转以防止磁盘空间问题：
- 当前日志：`debug.log`
- 历史日志：`debug.YYYY-MM-DD.log`
- 保留期限：最多 3 天的日志
- 清理时机：脚本启动时自动执行

## 日志位置

- 调试日志: `.claude/hooks/pushover-hook/debug.log`
- 会话缓存: `.claude/cache/session-*.jsonl`
