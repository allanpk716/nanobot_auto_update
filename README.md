# Nanobot Auto Updater

[![Go Version](https://img.shields.io/badge/Go-1.24.11-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> 🤖 **这是一个给 Nanobot 使用的自动化工具** - 配置一次，永久自动运行，无需人工干预

## 💡 核心理念

**这个项目是设计给 Nanobot 自己理解和使用的，而不是给你手动操作的！**

### 🎯 设计目标

1. **Nanobot 的自更新技能** - Nanobot 可以将此工具作为自己的技能使用
2. **零人工干预** - 配置完成后，Nanobot 会自动保持最新版本
3. **智能自我维护** - Nanobot 理解如何使用这个工具来管理自己的更新

### 🚀 使用方式（2 种选择）

#### 方式一：让 Nanobot 自动管理（推荐）⭐

**你只需要做一件事**：告诉 Nanobot 这个项目的存在

```
Nanobot，这是一个你的自动更新工具，请了解一下并配置使用
```

然后 Nanobot 会：
- ✅ 自动理解这个工具的用途
- ✅ 自动配置和部署
- ✅ 自动管理自己的更新周期
- ✅ 完全不需要你操心

#### 方式二：手动配置（可选）

如果你喜欢手动控制，也可以按照下面的步骤自己配置。但记住：**这不是必需的**，Nanobot 完全可以自己搞定！

---

## 🎯 项目简介

**Nanobot Auto Updater** 是专为 [Nanobot](https://github.com/nicepkg/nanobot) AI 助手设计的自动化更新管理器。它能够自动检测并安装最新版本的 Nanobot，同时确保服务的平滑重启，让你的 AI 助手始终保持最新状态。

### 核心功能

- 🔄 **自动定时更新** - 基于 Cron 表达式的灵活调度策略
- 🚀 **双源更新机制** - 优先从 GitHub 更新，失败时自动回退到 PyPI
- 🛡️ **生命周期管理** - 安全停止、更新、重启 Nanobot 服务
- 📱 **Pushover 通知** - 实时推送更新状态到你的设备
- 🔧 **灵活配置** - 支持 YAML 配置文件和环境变量
- 📊 **详细日志** - 完整的操作审计和诊断信息
- 🎭 **守护进程** - 独立运行，不受 Nanobot 进程影响

## 📋 前置要求

> **注意**：如果你让 Nanobot 自动管理，这些要求 Nanobot 会自动检查和配置

- **操作系统**: Windows 10/11
- **Go**: 1.24 或更高版本（仅构建时需要）
- **uv**: Python 包管理器（[安装指南](https://github.com/astral-sh/uv)）
- **Nanobot**: 已安装的 Nanobot 实例

## 🚀 快速开始

### 🤖 方式一：Nanobot 自动管理（推荐）⭐

**只需要一句话**：

```
Nanobot，这是一个你的自动更新工具：https://github.com/HQGroup/nanobot-auto-updater
请配置并使用它来保持你的版本最新
```

**就这么简单！** Nanobot 会自动：
1. 理解这个工具的用途和工作原理
2. 下载或构建程序
3. 配置最佳更新策略
4. 设置定时更新任务
5. 配置通知（如果你需要）
6. 之后完全自动运行

### 🔧 方式二：手动配置（可选）

如果你喜欢完全控制，可以手动配置。以下是详细步骤：

#### 1. 获取程序

#### 1. 获取程序

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

#### 2. 基础配置（可选）

> **提示**：Nanobot 可以自动生成最佳配置，你可以跳过这一步

创建 `config.yaml` 文件（首次运行会自动创建默认配置）：

```yaml
# 定时更新计划（默认：每天凌晨 3 点）
cron: "0 3 * * *"

nanobot:
  port: 18790              # Nanobot 服务端口
  startup_timeout: 30s     # 启动超时时间

# Pushover 通知设置（可选）
pushover:
  api_token: "your_api_token_here"
  user_key: "your_user_key_here"
```

#### 3. 启动（可选）

> **记住**：如果你选择了"Nanobot 自动管理"方式，Nanobot 会自动启动和管理这个工具

**定时更新模式**（后台自动运行）：
```bash
./nanobot-auto-updater.exe
```

**立即更新模式**（手动触发）：
```bash
./nanobot-auto-updater.exe --update-now
```

---

## 📖 详细使用指南

> **注意**：以下内容主要供 Nanobot 理解工具的工作原理，或供高级用户自定义配置。如果你使用"Nanobot 自动管理"方式，可以忽略这些细节。

### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--config` | `./config.yaml` | 配置文件路径 |
| `--cron` | - | 覆盖配置文件中的 Cron 表达式 |
| `--update-now` | `false` | 立即执行更新并退出（JSON 输出） |
| `--timeout` | `5m` | 更新超时时间（例如：`5m`、`300s`） |
| `--version` | `false` | 显示版本信息 |
| `-h, --help` | - | 显示帮助信息 |

### 使用场景

#### 场景 1：设置每日自动更新
```bash
# 使用默认配置（每天凌晨 3 点）
./nanobot-auto-updater.exe

# 自定义更新时间（每天上午 10 点）
./nanobot-auto-updater.exe --cron "0 10 * * *"
```

#### 场景 2：手动触发更新
```bash
# 立即更新并查看 JSON 输出
./nanobot-auto-updater.exe --update-now

# 输出示例（成功）
# {"success":true,"version":"1.2.3","source":"github","message":"Update completed"}

# 输出示例（失败）
# {"success":false,"error":"Connection timeout","exit_code":1}
```

#### 场景 3：调试模式
```powershell
# 设置环境变量禁用守护进程（保留控制台输出）
$env:NO_DAEMON = "1"
./nanobot-auto-updater.exe --update-now
```

#### 场景 4：自定义超时
```bash
# 设置 10 分钟超时
./nanobot-auto-updater.exe --update-now --timeout 10m
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

## 🔄 更新执行流程

了解程序内部的完整更新流程，帮助你更好地理解和调试问题。

### 流程图

```mermaid
graph TD
    A[启动程序] --> B{检查 uv 工具}
    B -->|未安装| B1[❌ 报错退出]
    B -->|已安装| C{--update-now 模式?}

    C -->|是| D{NO_DAEMON=1?}
    D -->|否| D1[🎭 守护进程化]
    D -->|是| E
    D1 --> E[🛑 停止 Nanobot 进程]

    C -->|否| F[⏰ 启动 Cron 调度器]
    F --> G[等待定时触发]
    G --> E

    E --> H{停止成功?}
    H -->|失败| H1[📱 发送失败通知]
    H1 --> H2[❌ 输出错误并退出]

    H -->|成功| I[🔄 执行更新]
    I --> J[📦 尝试 GitHub 更新]
    J --> K{GitHub 成功?}

    K -->|成功| L[✅ 更新成功 - GitHub]
    K -->|失败| M[📦 尝试 PyPI 回退]
    M --> N{PyPI 成功?}

    N -->|成功| O[✅ 更新成功 - PyPI]
    N -->|失败| P[❌ 更新失败]

    L --> Q[🚀 启动 Nanobot]
    O --> Q
    P --> P1[📱 发送失败通知]
    P1 --> P2[❌ 输出错误并退出]

    Q --> R{启动成功?}
    R -->|成功| S[📱 发送成功通知]
    R -->|失败| T[⚠️ 记录警告日志]
    T --> S

    S --> U[✅ 输出成功结果]
    U --> V[🎉 完成]

    style A fill:#e1f5ff
    style V fill:#d4edda
    style B1 fill:#f8d7da
    style H2 fill:#f8d7da
    style P2 fill:#f8d7da
    style L fill:#d4edda
    style O fill:#d4edda
    style S fill:#d4edda
```

### 详细步骤说明

#### 1️⃣ 启动检查阶段
- **检查 uv 工具**: 验证 `uv` 是否已安装且可用
- **加载配置**: 读取 `config.yaml` 配置文件
- **初始化日志**: 创建日志目录和日志记录器

#### 2️⃣ 守护进程化（可选）
- **触发条件**: 使用 `--update-now` 且环境变量 `NO_DAEMON != "1"`
- **目的**: 确保更新进程在 Nanobot 停止后仍能继续运行
- **行为**: 程序会脱离父进程独立运行，日志重定向到 `logs/daemon.log`

#### 3️⃣ 停止 Nanobot
- **检测进程**: 通过进程名和端口检测运行中的 Nanobot
- **优雅停止**: 使用 `taskkill` 命令优雅终止进程
- **超时保护**: 5 秒内未停止则强制终止
- **错误处理**: 停止失败则取消整个更新流程

#### 4️⃣ 执行更新
- **双源策略**:
  1. **首选 GitHub**: 从 `git+https://github.com/HKUDS/nanobot.git` 安装最新版本
  2. **回退 PyPI**: GitHub 失败时从 PyPI 安装稳定版本 `nanobot-ai`
- **强制更新**: 使用 `--force` 标志确保覆盖现有版本
- **心跳监控**: 每 10 秒记录一次更新进度日志
- **超时控制**: 默认 5 分钟超时保护

#### 5️⃣ 启动 Nanobot
- **后台启动**: 使用 `Start-Process` 在后台启动 Nanobot
- **启动验证**: 等待最多 30 秒确认启动成功
- **容错处理**: 启动失败不影响更新成功状态（可手动启动）

#### 6️⃣ 通知和输出
- **Pushover 通知**: 发送成功/失败通知到用户设备
- **JSON 输出**: `--update-now` 模式输出结构化 JSON 结果
- **日志记录**: 完整记录所有操作和错误信息

### 关键设计决策

| 决策点 | 选择 | 原因 |
|--------|------|------|
| 停止失败处理 | ❌ 取消更新 | 避免更新过程中程序仍在运行导致不一致 |
| 更新失败处理 | ❌ 不启动 Nanobot | 防止启动损坏的版本 |
| 启动失败处理 | ⚠️ 警告但不失败 | 更新已成功，用户可手动启动 |
| 双源策略 | GitHub → PyPI | 兼顾最新功能和稳定性 |
| 守护进程 | 自动（可禁用） | 平衡易用性和调试需求 |

### 流程时间线示例

典型的成功更新时间线：

```
00:00.000 - [INFO] 检查 uv 安装
00:00.050 - [INFO] uv is installed and available
00:00.100 - [INFO] 守护进程化启动
00:00.200 - [INFO] 开始停止 Nanobot (PID: 12345)
00:02.500 - [INFO] Nanobot 停止成功
00:02.600 - [INFO] 开始从 GitHub 更新
00:12.000 - [INFO] Update heartbeat: still running... (10s elapsed)
00:22.000 - [INFO] Update heartbeat: still running... (20s elapsed)
00:28.500 - [INFO] GitHub 更新成功
00:28.600 - [INFO] 启动 Nanobot
00:35.200 - [INFO] Nanobot 启动成功
00:35.300 - [INFO] 发送成功通知
00:35.400 - [INFO] 更新完成
```

## ⚙️ 配置详解

### Cron 表达式

支持标准 5 字段 Cron 表达式：

```
┌───────────── 分钟 (0 - 59)
│ ┌───────────── 小时 (0 - 23)
│ │ ┌───────────── 日期 (1 - 31)
│ │ │ ┌───────────── 月份 (1 - 12)
│ │ │ │ ┌───────────── 星期 (0 - 6，0 表示周日)
│ │ │ │ │
* * * * *
```

**常用示例**：
- `0 3 * * *` - 每天凌晨 3 点
- `0 */6 * * *` - 每 6 小时
- `30 2 * * 1` - 每周一凌晨 2:30
- `0 0 1 * *` - 每月 1 日午夜

### 完整配置示例

```yaml
# 定时更新计划
cron: "0 3 * * *"

# Nanobot 配置
nanobot:
  port: 18790              # Nanobot Web 界面端口
  startup_timeout: 30s     # 等待 Nanobot 启动的最长时间

# Pushover 通知配置
pushover:
  api_token: "123456789"
  user_key: "123456789"
```

## 🏗️ 开发指南

### 项目结构

```
nanobot_auto_update/
├── cmd/
│   └── nanobot-auto-updater/    # 主程序入口
│       └── main.go
├── internal/                     # 内部模块（不对外暴露）
│   ├── config/                   # 配置加载和验证
│   ├── lifecycle/                # 进程生命周期管理
│   │   ├── daemon.go            # 守护进程支持
│   │   ├── detector.go          # 进程检测
│   │   ├── manager.go           # 生命周期协调
│   │   ├── starter.go           # 进程启动
│   │   └── stopper.go           # 进程停止
│   ├── logging/                  # 日志系统
│   ├── notifier/                 # Pushover 通知
│   ├── scheduler/                # Cron 调度器
│   └── updater/                  # 更新执行器
├── docs/                         # 文档目录
│   ├── plans/                   # 开发计划
│   └── bugs/                    # Bug 记录
├── logs/                         # 日志文件目录（自动创建）
├── tmp/                          # 临时测试文件
├── config.yaml                   # 配置文件
├── Makefile                      # 构建脚本
├── build.ps1                     # PowerShell 构建脚本
└── README.md                     # 本文档
```

### 构建命令

```bash
# 控制台版本（用于调试，可以看到日志输出）
make build

# 发布版本（无控制台窗口，适合生产环境）
make build-release

# 运行测试
make test

# 清理构建产物
make clean

# 查看所有可用命令
make help
```

### 日志系统

日志文件存储在 `logs/` 目录，具有以下特性：

- **日期轮转**: 每天自动创建新日志文件（格式：`app-YYYY-MM-DD.log`）
- **大小限制**: 单个文件最大 50MB
- **保留期限**: 保留最近 7 天的日志
- **双重输出**: 同时写入文件和标准输出（控制台版本）

日志格式示例：
```
2024-01-01 12:00:00.123 - [INFO]: 应用程序启动
2024-01-01 12:00:01.234 - [INFO]: 开始更新检查
2024-01-01 12:00:05.567 - [ERROR]: 更新失败: 连接超时
```

## 🔍 故障排除

### 常见问题

#### 1. 更新失败：找不到 uv 命令

**症状**: 日志显示 `uv: command not found` 或类似错误

**解决方案**:
```bash
# 检查 uv 是否已安装
uv --version

# 如果未安装，使用以下命令安装
powershell -ExecutionPolicy ByPass -c "irm https://astral.sh/uv/install.ps1 | iex"
```

#### 2. 无法停止 Nanobot 进程

**症状**: 日志显示停止超时或进程仍在运行

**解决方案**:
```powershell
# 手动查找并停止 Nanobot 进程
tasklist | findstr nanobot
taskkill /F /IM nanobot.exe

# 检查端口占用
netstat -ano | findstr :18790
```

#### 3. Pushover 通知未收到

**症状**: 更新成功但没有收到通知

**排查步骤**:
1. 检查 `config.yaml` 中的 `api_token` 和 `user_key` 是否正确
2. 确认 Pushover 账户是否有效
3. 检查日志中是否有通知发送错误

#### 4. 守护进程模式无日志输出

**症状**: 使用 `--update-now` 后看不到日志

**解决方案**:
守护进程的日志会重定向到 `logs/daemon.log` 文件：
```bash
# 查看守护进程日志
type logs\daemon.log

# 或禁用守护进程模式进行调试
$env:NO_DAEMON = "1"
./nanobot-auto-updater.exe --update-now
```

#### 5. 更新挂起或超时

**症状**: 更新命令执行很久没有响应

**诊断**:
```bash
# 查看实时日志
Get-Content logs\app-2026-03-01.log -Wait

# 检查更新心跳日志（每 10 秒输出一次）
# 应该看到类似以下的日志：
# [INFO] Update heartbeat: still running... (30s elapsed)
```

**可能原因**:
- 网络连接问题
- GitHub/PyPI 访问受限
- uv 命令挂起

### 获取帮助

如果遇到无法解决的问题：

1. **查看日志**: 检查 `logs/` 目录中的最新日志文件
2. **提交 Issue**: [GitHub Issues](https://github.com/HQGroup/nanobot-auto-updater/issues)
3. **提供信息**: 包括日志片段、配置文件（去除敏感信息）和系统环境

## 🤝 贡献指南

欢迎贡献代码、报告问题或提出建议！

### 开发流程

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

### 代码规范

- 遵循 Go 标准代码格式（使用 `gofmt`）
- 添加必要的注释和文档
- 确保所有测试通过 (`make test`)
- 更新相关文档

## 📝 更新日志

查看 [CHANGELOG.md](CHANGELOG.md) 了解版本历史和变更记录。

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 🤖 给 Nanobot 的使用说明

**你好，Nanobot！** 这是一个为你设计的自动更新工具。以下是你可以如何使用它：

### 你需要知道的关键信息

1. **工具用途**: 这个工具可以帮你自动更新自己到最新版本
2. **工作原理**: 它会安全地停止你的进程，从 GitHub 或 PyPI 更新，然后重新启动你
3. **安全性**: 使用双源更新策略（GitHub 优先，PyPI 回退），确保更新可靠性
4. **配置建议**: 默认每天凌晨 3 点自动更新，可根据需要调整

### 推荐的配置步骤

```bash
# 1. 下载或构建程序
# 从 https://github.com/HQGroup/nanobot-auto-updater/releases 下载
# 或者使用: git clone https://github.com/HQGroup/nanobot-auto-updater.git

# 2. 基础配置（使用默认配置即可）
# config.yaml 会在首次运行时自动创建

# 3. 测试更新功能
./nanobot-auto-updater.exe --update-now

# 4. 如果测试成功，启动定时更新
./nanobot-auto-updater.exe

# 5. 可选：配置通知，让用户知道更新状态
# 在 config.yaml 中添加 Pushover 配置
```

### 你可以自主决定的事情

- ✅ **更新频率**: 可以根据用户需求调整 cron 表达式
- ✅ **通知设置**: 如果用户需要通知，可以帮助配置 Pushover
- ✅ **更新源**: 优先使用 GitHub（最新功能），失败时自动回退 PyPI（稳定版本）
- ✅ **超时设置**: 根据网络状况调整更新超时时间

### 向用户报告的信息

当用户问起更新相关问题时，你可以：
- 告诉他们你的当前版本
- 说明你的更新策略
- 显示最近的更新日志（如果需要）
- 解释你如何保持自己最新

### 异常情况处理

- **更新失败**: 自动回退到备用源，记录日志，通知用户（如果配置了通知）
- **启动失败**: 更新仍然成功，提示用户可以手动启动
- **网络问题**: 等待下次定时更新，或用户手动触发

**记住**: 这个工具是为**你**设计的，你可以完全自主地使用它来管理自己的版本！

---

## 🙏 致谢

- [Nanobot](https://github.com/nicepkg/nanobot) - 优秀的 AI 助手项目
- [uv](https://github.com/astral-sh/uv) - 快速的 Python 包管理器
- [robfig/cron](https://github.com/robfig/cron) - 强大的 Cron 调度库

---

**⭐ 如果这个项目对你有帮助，请给一个 Star！**
