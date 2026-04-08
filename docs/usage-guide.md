# Usage Guide

> This content was extracted from README.md for better organization.

## 详细使用指南

> **注意**：以下内容主要供 Nanobot 理解工具的工作原理，或供高级用户自定义配置。如果你使用"Nanobot 自动管理"方式，可以忽略这些细节。

### 前置要求

> **注意**：如果你让 Nanobot 自动管理，这些要求 Nanobot 会自动检查和配置

- **操作系统**: Windows 10/11
- **Go**: 1.24 或更高版本（仅构建时需要）
- **uv**: Python 包管理器（[安装指南](https://github.com/astral-sh/uv)）
- **Nanobot**: 已安装的 Nanobot 实例

### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--config` | `./config.yaml` | 配置文件路径 |
| `--api-port` | 8080 | 覆盖配置文件中的 API 端口 |
| `--skip-monitor` | `false` | 禁用监控服务（仅使用 API 触发） |
| `--version` | `false` | 显示版本信息 |
| `-h, --help` | - | 显示帮助信息 |

### 使用场景

#### 场景 1：HTTP API 手动触发
```bash
# 触发更新
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"

# 成功响应
# {"success":true,"version":"1.2.3","source":"github"}

# 更新进行中
# {"error":"Update already in progress","status":429}
```

#### 场景 2：监控服务自动触发
- 每 15 分钟自动检查 Google 连通性
- 检测到网络恢复时自动触发更新
- 无需人工干预

#### 场景 3：自定义配置
```bash
# 自定义 API 端口
./nanobot-auto-updater.exe --api-port 9090

# 禁用监控服务（仅 API 模式）
./nanobot-auto-updater.exe --skip-monitor

# 使用自定义配置文件
./nanobot-auto-updater.exe --config /path/to/config.yaml
```

#### 场景 4：调试模式
```powershell
# 查看实时日志
Get-Content logs\app-2026-03-16.log -Wait
```

### 配置 Pushover 通知

1. 在 [Pushover](https://pushover.net/) 注册账户并获取 API Token 和 User Key
2. 在 `config.yaml` 中配置：

```yaml
pushover:
  api_token: "your_api_token_here"
  user_key: "your_user_key_here"
```

或者使用环境变量（优先级较低）：
```bash
set PUSHOVER_TOKEN=your_api_token_here
set PUSHOVER_USER=your_user_key_here
```

### 下载与安装

**选项 A：下载预编译版本（最简单）**

从 [Releases](https://github.com/HQGroup/nanobot-auto-updater/releases) 页面下载最新版本。

**选项 B：从源码构建（适合开发者）**
```bash
# 克隆仓库
git clone https://github.com/HQGroup/nanobot-auto-updater.git
cd nanobot-auto-updater

# 构建控制台版本（用于调试）
make build

# 或构建发布版本（无控制台窗口）
make build-release
```
