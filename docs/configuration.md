# Configuration Reference

> This content was extracted from README.md for better organization.

## 配置详解

### 完整配置示例

```yaml
# HTTP API 服务配置（必需）
api:
  port: 8080                    # API 服务端口
  bearer_token: "your-secret-token-at-least-32-characters-long"  # 认证令牌（必填，>=32字符）
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

### 配置说明

首次运行会自动创建默认配置文件 `config.yaml`。**必须编辑配置文件，至少添加一个实例**。

- **api** (必需) — HTTP API 服务配置，包含端口、Bearer Token 认证和请求超时
- **monitor** (必需) — 监控服务配置，定义 Google 连通性检查间隔和请求超时
- **instances** (必需) — 至少配置一个 Nanobot 实例，支持多实例
- **pushover** (可选) — Pushover 通知设置，用于推送更新状态到设备
