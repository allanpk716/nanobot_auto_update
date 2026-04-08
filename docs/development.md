# Development Guide

> This content was extracted from README.md for better organization.

## 开发指南

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

### 代码规范

- 遵循 Go 标准代码格式（使用 `gofmt`）
- 添加必要的注释和文档
- 确保所有测试通过 (`make test`)
- 更新相关文档
