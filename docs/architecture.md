# Architecture

> This content was extracted from README.md for better organization.

## v0.3 架构说明 (v0.3)

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
