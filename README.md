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

### 🚀 使用方式

**推荐方式：HTTP API + 监控服务（v0.3+）**

**配置要求**：
1. 在 `config.yaml` 中配置 API、Monitor 和至少一个实例：
   ```yaml
   api:
     port: 8080
     bearer_token: "your-secret-token-at-least-32-characters-long"
   monitor:
     interval: 15m
   instances:
     - name: "nanobot-instance-1"
       port: 18790
       start_command: "nanobot gateway"
   ```
2. 启动服务：`./nanobot-auto-updater.exe`

**触发更新**：
```bash
# 手动触发更新
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer your-secret-token-at-least-32-characters-long"

# 响应示例（成功）
{"success":true,"version":"1.2.3","source":"github","message":"Update completed"}

# 响应示例（冲突 - 有其他更新正在进行）
{"error":"Update already in progress","status":429}
```

**优势**：
- ✅ 实时触发，无需等待定时
- ✅ 易于集成到外部系统
- ✅ 监控服务自动检测网络恢复并触发更新
- ✅ Bearer Token 保护 API 安全

如果你喜欢手动控制，也可以按照下面的步骤自己配置。但记住：**这不是必需的**，Nanobot 完全可以自己搞定！

---

## 🎯 项目简介

**Nanobot Auto Updater** 是专为 [Nanobot](https://github.com/nicepkg/nanobot) AI 助手设计的自动化更新管理器。它能够自动检测并安装最新版本的 Nanobot，同时确保服务的平滑重启，让你的 AI 助手始终保持最新状态。

### 核心功能

- 🚀 **双源更新机制** - 优先从 GitHub 更新，失败时自动回退到 PyPI
- 🛡️ **生命周期管理** - 安全停止、更新、重启 Nanobot 服务
- 📱 **Pushover 通知** - 实时推送更新状态到你的设备
- 🔧 **灵活配置** - 支持 YAML 配置文件和环境变量
- 📊 **详细日志** - 完整的操作审计和诊断信息
- 🎯 **实时日志查看** (v0.4+) - Web UI 和 SSE 流式传输实时查看实例日志

## 🏗️ 架构说明 (v0.3)

### v0.3 重大架构转型

从 v0.3 版本开始，nanobot-auto-updater 从**定时更新工具**转变为**监控服务 + HTTP API 触发更新**模式。

### 核心架构组件

#### 1. HTTP API 服务器
- **端点**: `/api/v1/trigger-update`
- **认证**: Bearer Token（必须至少 32 个字符）
- **功能**: 接收外部 HTTP 请求触发更新
- **端口**: 默认 8080（可配置）
- **超时**: 默认 30 秒（可配置）

#### 2. 监控服务
- **功能**: 定期检查 Google 连通性
- **检查间隔**: 默认 15 分钟（可配置）
- **触发条件**: 检测到网络从不可用恢复到可用时自动触发更新
- **请求超时**: 默认 10 秒（可配置）

#### 3. 共享更新锁
- **目的**: 防止多个更新请求同时执行
- **实现**: 基于文件的互斥锁
- **冲突处理**: 返回 HTTP 429 Too Many Requests

### 架构优势

| 特性 | v0.3 新架构 |
|------|------------|
| 触发方式 | HTTP API + 监控服务 |
| 外部集成 | 简单（REST API） |
| 实时性 | 即时触发 |
| 并发控制 | 共享锁机制 |
| 安全性 | Bearer Token |

## 📋 前置要求

> **注意**：如果你让 Nanobot 自动管理，这些要求 Nanobot 会自动检查和配置

- **操作系统**: Windows 10/11
- **Go**: 1.24 或更高版本（仅构建时需要）
- **uv**: Python 包管理器（[安装指南](https://github.com/astral-sh/uv)）
- **Nanobot**: 已安装的 Nanobot 实例

## 🚀 快速开始

### 📋 巻加配置（首次使用）

**步骤 1**: 配置 `config.yaml`

首次运行会自动创建默认配置文件，你需要编辑它，至少添加一个实例配置：

**步骤 2**: 启动服务

```bash
./nanobot-auto-updater.exe
```

**步骤 3**: 触发更新（需要时）

```bash
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer YOUR_TOKEN"
# 查看输出的 JSON，如果 success=true 则成功
```

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

### ⚙️ 配置文件

首次运行会自动创建默认配置文件 `config.yaml`。**必须编辑配置文件，至少添加一个实例**：

```yaml
# HTTP API 服务配置（必需）
api:
  port: 8080                    # API 服务端口
  bearer_token: "your-secret-token-at-least-32-characters"  # 认证令牌（必填，≥32字符）
  timeout: 30s                  # 请求超时时间

# 监控服务配置（必需）
monitor:
  interval: 15m                 # Google 连通性检查间隔
  timeout: 10s                  # HTTP 请求超时

# 实例配置（必需 - 至少配置一个实例）
instances:
  - name: "nanobot-instance-1"
    port: 18790
    start_command: "nanobot gateway"
    startup_timeout: 30s

  # 可以配置多个实例
  # - name: "nanobot-instance-2"
  #   port: 18791
  #   start_command: "nanobot gateway --port 18791"
  #   startup_timeout: 30s

# Pushover 通知设置（可选）
pushover:
  api_token: "your_api_token_here"
  user_key: "your_user_key_here"
```

### 🚀 启动服务

```bash
./nanobot-auto-updater.exe
```

服务启动后会：
- ✅ 启动 HTTP API 服务器（默认端口 8080）
- ✅ 启动监控服务（每 15 分钟检查一次 Google 连通性）
- ✅ 等待 API 触发或监控服务自动触发更新

---

## 📊 实时日志查看 (v0.4+)

v0.4 版本新增实时日志查看功能，可以通过 Web UI 或 SSE API 查看 Nanobot 实例的实时输出。

### 方式 1：Web UI（推荐）

访问浏览器查看实时日志：

```
http://localhost:8080/logs/{实例名称}
```

**示例：**

如果你的配置是：
```yaml
instances:
  - name: "nanobot-gateway"
    port: 18790
  - name: "nanobot-work-helper"
    port: 18792
```

则可以访问：
- http://localhost:8080/logs/nanobot-gateway
- http://localhost:8080/logs/nanobot-work-helper

**Web UI 功能：**
- ✅ **实时滚动** - 类似 `tail -f`，自动滚动到最新日志
- ✅ **实例选择器** - 下拉菜单快速切换实例
- ✅ **暂停/恢复** - 按钮控制自动滚动
- ✅ **颜色区分** - Stdout（蓝色）/ Stderr（红色）
- ✅ **连接状态** - 实时显示 SSE 连接状态
- ✅ **历史日志** - 保留最近 5000 行日志
- ✅ **单文件部署** - 静态资源嵌入二进制，无需外部文件

**使用截图示例：**

打开浏览器访问 `http://localhost:8080/logs/nanobot-gateway`：
1. 页面顶部显示实例选择下拉菜单
2. 左上角显示连接状态（连接中/已连接/已断开）
3. 右上角有暂停/恢复按钮
4. 日志实时滚动，蓝色为 stdout，红色为 stderr
5. 手动滚动时自动暂停，滚动到底部时自动恢复

### 方式 2：SSE API（程序化访问）

通过 Server-Sent Events API 实时接收日志流：

```bash
# 实时流式日志（Ctrl+C 退出）
curl -N http://localhost:8080/api/v1/logs/nanobot-gateway/stream

# 返回 SSE 格式：
# event: stdout
# data: {"timestamp":"2026-03-20T10:30:00Z","content":"Starting nanobot..."}

# event: stderr
# data: {"timestamp":"2026-03-20T10:30:01Z","content":"Warning: ..."}
```

**SSE 事件格式：**

```
event: stdout|stderr
data: {"timestamp":"RFC3339时间戳","content":"日志内容"}
```

**特性：**
- ✅ **历史回放** - 连接时自动发送最近 5000 行历史日志
- ✅ **实时推送** - 新日志实时推送到客户端
- ✅ **心跳保活** - 每 30 秒发送心跳注释防止超时
- ✅ **自动重连** - 客户端断开后可自动重连

### 方式 3：浏览器 EventSource API

在 JavaScript 中使用 EventSource API 接收日志：

```html
<script>
const eventSource = new EventSource('/api/v1/logs/nanobot-gateway/stream');

eventSource.addEventListener('stdout', (e) => {
  const data = JSON.parse(e.data);
  console.log(`[OUT] ${data.timestamp}: ${data.content}`);
});

eventSource.addEventListener('stderr', (e) => {
  const data = JSON.parse(e.data);
  console.error(`[ERR] ${data.timestamp}: ${data.content}`);
});

eventSource.onerror = (e) => {
  console.error('SSE connection error');
  // EventSource 会自动重连
};
</script>
```

### 技术细节

**架构：**
- **LogBuffer** - 环形缓冲区，固定 5000 行容量，FIFO 自动覆盖
- **Log Capture** - os.Pipe() 捕获 stdout/stderr，并发 goroutine 读取
- **SSE Streaming** - Server-Sent Events 协议，30 秒心跳保活
- **Instance Isolation** - 每个实例独立缓冲，互不影响

**性能：**
- **内存占用** - 每个实例约 1MB（5000 行 × 200 字节/行）
- **写入性能** - O(1) 非阻塞写入
- **并发安全** - sync.RWMutex 保护，支持多客户端同时访问
- **优雅降级** - 慢客户端自动丢弃日志，不影响主流程

---

## 📖 详细使用指南

> **注意**：以下内容主要供 Nanobot 理解工具的工作原理，或供高级用户自定义配置。如果你使用"Nanobot 自动管理"方式，可以忽略这些细节。

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

## 🔄 更新执行流程

了解程序内部的完整更新流程，帮助你更好地理解和调试问题。

### v0.3 架构流程图

```mermaid
graph TD
    A[启动程序] --> B{检查配置模式}

    B -->|API + Monitor| C[启动 HTTP API 服务器]
    C --> D[启动监控服务]
    D --> E[定期检查 Google 连通性]
    E -->|连通性恢复| E1{获取更新锁}
    E1 -->|失败| E2[⏳ 等待下次检查]
    E1 -->|成功| G[触发更新]
    E -->|仍然不通| E

    C -->|收到 API 请求| H{验证 Bearer Token}
    H -->|失败| H1[❌ 401 Unauthorized]
    H -->|成功| I{获取更新锁}

    I -->|失败| I1[⏳ 返回 429 Too Many Requests]
    I -->|成功| G

    B -->|传统 Cron| F[启动 Cron 调度器]
    F -->|定时触发| G

    G --> J[执行更新流程]
    J --> K[返回结果]

    K --> L[释放更新锁]
    L --> M[🎉 完成]

    style C fill:#e1f5ff
    style D fill:#e1f5ff
    style G fill:#fff4e6
    style M fill:#d4edda
```

### 详细更新流程图（核心流程）

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
- **注意**: v0.3 推荐使用 HTTP API 模式，守护进程化主要用于 CLI 模式

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

### 完整配置示例

```yaml
# HTTP API 服务配置（必需）
api:
  port: 8080                    # API 服务端口
  bearer_token: "your-secret-token-at-least-32-characters-long"  # 认证令牌（必填，≥32字符）
  timeout: 30s                  # 请求超时时间

# 监控服务配置（必需）
monitor:
  interval: 15m                 # Google 连通性检查间隔
  timeout: 10s                  # HTTP 请求超时

# 实例配置（必需 - 至少配置一个实例）
instances:
  - name: "nanobot-instance-1"
    port: 18790
    start_command: "nanobot gateway"
    startup_timeout: 30s

  # 可以配置多个实例
  # - name: "nanobot-instance-2"
  #   port: 18791
  #   start_command: "nanobot gateway --port 18791"
  #   startup_timeout: 30s
  #   repo_path: "C:\\path\\to\\nanobot-repo-2"  # 可选

# Pushover 通知配置（可选）
pushover:
  api_token: "your_api_token_here"
  user_key: "your_user_key_here"
```

## 🏗️ 开发指南

### 项目结构

```
nanobot_auto_update/
├── cmd/
│   └── nanobot-auto-updater/    # 主程序入口
│       └── main.go
├── internal/                     # 内部模块（不对外暴露）
│   ├── api/                      # HTTP API 服务器 + Bearer 认证（v0.3 新增）
│   │   ├── handler.go           # Web UI 处理器（v0.4 新增）
│   │   └── sse.go               # SSE 日志流式传输（v0.4 新增）
│   ├── monitor/                  # Google 连通性监控（v0.3 新增）
│   ├── lock/                     # 共享更新锁（v0.3 新增）
│   ├── logbuffer/                # 环形日志缓冲区（v0.4 新增）
│   │   ├── buffer.go            # 线程安全环形缓冲区
│   │   └── buffer_test.go       # 单元测试
│   ├── config/                   # 配置加载（扩展：API/Monitor 配置）
│   ├── instance/                 # 多实例管理（v0.2 引入）
│   ├── lifecycle/                # 进程生命周期管理
│   │   ├── detector.go          # 进程检测
│   │   ├── manager.go           # 生命周期协调 + LogBuffer 管理（v0.4 扩展）
│   │   ├── starter.go           # 进程启动 + 日志捕获（v0.4 扩展）
│   │   ├── capture.go           # Stdout/Stderr 捕获（v0.4 新增）
│   │   └── stopper.go           # 进程停止
│   ├── logging/                  # 日志系统
│   ├── notifier/                 # Pushover 通知
│   └── updater/                  # 更新执行器
├── web/                          # Web UI 静态文件（v0.4 新增）
│   └── logs.html                # 日志查看器单文件应用
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

#### 6. API 认证失败

**症状**: 返回 `401 Unauthorized`

**解决方案**:
```bash
# 检查 Bearer Token 是否正确配置
# 确保配置文件中的 token 与请求中的 token 一致
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer your-exact-token-here"

# 确认 token 长度至少 32 个字符
```

#### 7. 更新锁冲突

**症状**: 返回 `429 Too Many Requests` 或 `Update already in progress`

**原因**: 另一个更新正在进行中

**解决方案**:
```bash
# 等待当前更新完成（通常几分钟内）
# 查看日志确认更新进度
Get-Content logs\app-2026-03-16.log -Wait

# 如果确认没有更新在运行，可能是锁文件残留
# 锁文件位置: logs/update.lock（重启程序会自动清理）
```

#### 8. 监控服务未启动

**症状**: API 可用但监控不工作

**排查步骤**:
1. 检查 `config.yaml` 中的 `monitor` 配置段是否存在
2. 确认 `monitor.interval` 配置正确（至少 1 分钟）
3. 查看日志中是否有监控服务启动记录
4. 确认未使用 `--skip-monitor` 参数

#### 9. Bearer Token 配置错误

**症状**: 配置加载失败，提示 token 长度不足

**解决方案**:
```yaml
# 确保 bearer_token 至少 32 个字符
api:
  bearer_token: "this-token-must-be-at-least-32-characters-long"
```

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

**你好，Nanobot！** 这是一个为你设计的自动更新工具。

### 🎯 v0.3 架构：HTTP API + 监控服务

从 v0.3 版本开始，nanobot-auto-updater 完全转向 **HTTP API + 监控服务** 架构。

### ⚡ 最快的使用方式

**步骤 1**: 配置 `config.yaml`（首次使用）
- 确保至少配置一个实例
- 配置 API 的 `bearer_token`（≥32 字符）

**步骤 2**: 启动服务
```bash
./nanobot-auto-updater.exe
```

**步骤 3**: 通过 API 触发更新
```bash
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 📋 API 响应格式

**成功**：
```json
{"success":true,"version":"1.2.3","source":"github"}
```

**冲突（更新进行中）**：
```json
{"error":"Update already in progress","status":429}
```

### 🎯 你需要知道的关键信息

1. **工具用途**: 帮你自动更新自己到最新版本
2. **工作原理**: 安全停止你的进程 → 从 GitHub/PyPI 更新 → 重新启动你
3. **双源策略**: 优先 GitHub（最新功能），失败时自动回退 PyPI（稳定版本）
4. **v0.3 特性**:
   - **HTTP API**: 通过 `/api/v1/trigger-update` 实时触发更新
   - **监控服务**: 自动检测网络恢复并触发更新
   - **共享锁**: 防止并发更新
   - **Bearer Token**: API 安全认证（≥32 字符）
5. **自动触发**: 监控服务每 15 分钟检查一次网络，自动触发更新

### 🔄 典型工作流程

**场景 1：用户让你更新自己**
```bash
# 触发更新
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer YOUR_TOKEN"

# 解析 JSON 响应
# 成功: {"success":true,"version":"1.2.3",...}
# 失败: {"success":false,"error":"..."}
# 冲突: {"error":"Update already in progress","status":429}
```

**场景 2：监控服务自动更新**
- 服务每 15 分钟检查 Google 连通性
- 检测到网络恢复时自动触发更新
- 完全自动化，无需人工干预

### 🎉 总结

**记住这 2 个命令就够了**：

1. **`./nanobot-auto-updater.exe`** - 启动服务（API + Monitor）
2. **`curl -X POST http://localhost:8080/api/v1/trigger-update`** - 通过 API 触发更新

**这个工具是为 YOU 设计的** - 完全基于 HTTP API，易于集成！
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer YOUR_TOKEN"
# 查看输出的 JSON，如果 success=true 则成功
```

#### 使用 CLI（传统方式）

```bash
# 步骤 1: 获取帮助信息（可选，了解所有功能）
./nanobot-auto-updater.exe --help

# 步骤 2: 测试更新功能
./nanobot-auto-updater.exe --update-now
# 查看输出的 JSON，如果 success=true 则成功

# 步骤 3: 启动定时更新（可选，如果不手动触发）
./nanobot-auto-updater.exe
# 程序会根据 config.yaml 中的 cron 表达式自动运行
```

### 🎯 你需要知道的关键信息

1. **工具用途**: 帮你自动更新自己到最新版本
2. **工作原理**: 安全停止你的进程 → 从 GitHub/PyPI 更新 → 重新启动你
3. **双源策略**: 优先 GitHub（最新功能），失败时自动回退 PyPI（稳定版本）
4. **v0.3 新特性**:
   - **HTTP API**: 通过 `/api/v1/trigger-update` 端点实时触发更新
   - **监控服务**: 自动检测网络恢复并触发更新
   - **共享锁**: 防止多个更新同时执行
   - **Bearer Token**: 保护 API 端点安全（至少 32 个字符）
5. **守护进程**: `--update-now` 模式会自动守护进程化，不受你的进程停止影响
6. **配置文件**: 首次运行会自动创建 `config.yaml`，使用默认配置即可
7. **传统模式**: 保留 Cron 定时更新，与 v1.0 完全兼容

### 💡 智能配置建议

**你可以根据情况自动调整**：

- **更新频率**: 默认每天凌晨 3 点，可通过 `--cron` 参数临时修改
- **超时时间**: 网络慢时增加 `--timeout`（默认 5 分钟）
- **通知设置**: 如果用户需要通知，在 `config.yaml` 中配置 Pushover
- **调试模式**: 设置环境变量 `NO_DAEMON=1` 禁用守护进程，方便调试

### 🔄 典型工作流程

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

### 🛠️ 高级用法

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

### 📝 向用户报告的信息

当用户问起更新相关问题时，你可以：
- 运行 `--version` 查看当前版本
- 说明你的更新策略（GitHub 优先，PyPI 回退）
- 显示最近的更新日志（从 `logs/` 目录读取）
- 解释你如何保持自己最新（自动定时更新）

### ⚠️ 异常情况处理

- **更新失败**: 自动回退到备用源，JSON 输出 `success:false`
- **启动失败**: 更新仍成功，JSON 输出包含警告，你可以手动启动
- **网络问题**: 等待下次定时更新，或用户手动触发

### 🎉 总结

**记住这 4 个命令就够了**：

1. **`--help`** - 不知道怎么用？先看这个
2. **`--update-now`** - 立即更新，JSON 输出结果（CLI 方式）
3. **`curl -X POST http://localhost:8080/api/v1/trigger-update`** - 通过 API 触发更新（v0.3 推荐）
4. **无参数运行** - 启动服务（API + Monitor + 可选 Cron）

**这个工具是为 YOU 设计的** - 直接使用 CLI 或 HTTP API，无需人工干预！

---

## 🙏 致谢

- [Nanobot](https://github.com/nicepkg/nanobot) - 优秀的 AI 助手项目
- [uv](https://github.com/astral-sh/uv) - 快速的 Python 包管理器
- [robfig/cron](https://github.com/robfig/cron) - 强大的 Cron 调度库

---

**⭐ 如果这个项目对你有帮助，请给一个 Star！**
