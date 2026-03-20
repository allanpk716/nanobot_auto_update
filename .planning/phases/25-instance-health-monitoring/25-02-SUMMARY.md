---
phase: 25-instance-health-monitoring
plan: 02
subsystem: health-monitoring
tags: [integration, lifecycle, goroutine, graceful-shutdown]
dependency_graph:
  requires:
    - 25-01 (HealthMonitor implementation)
    - config.HealthCheck.Interval field
  provides:
    - Health monitor lifecycle management
    - Automatic health monitoring on startup
    - Graceful health monitor shutdown
  affects:
    - Application startup sequence
    - Application shutdown sequence
tech_stack:
  added:
    - health.HealthMonitor integration
    - Non-blocking goroutine pattern
    - Graceful shutdown pattern
  patterns:
    - Lifecycle management with Start/Stop pattern
    - Nil-safe component management
    - Context-based shutdown coordination
key_files:
  created: []
  modified:
    - cmd/nanobot-auto-updater/main.go (integrated health monitor lifecycle)
decisions:
  - Start health monitor after API server (maintain startup order)
  - Stop health monitor before API server on shutdown (reverse order)
  - Use nil check for healthMonitor (handles config with zero instances)
  - Run health monitor in goroutine (non-blocking)
  - Chinese log for startup message (project standard)
metrics:
  duration: <1 minute
  tasks_completed: 1
  files_modified: 1
  commits: 1
  completed_date: 2026-03-20
---

# Phase 25 Plan 02: Health Monitor Integration in main.go Summary

集成 HealthMonitor 到 main.go 应用程序生命周期，实现应用启动时自动开始健康监控，并在关闭时优雅停止监控器。

## 一句话总结

成功集成健康监控器到应用程序生命周期，使用非阻塞 goroutine 启动监控，并在优雅关闭时优先停止监控器。

## 执行的任务

### Task 1: 集成 HealthMonitor 到 main.go

**目标:** 将健康监控器集成到应用程序的启动和关闭流程中

**实现内容:**

1. **添加 health 包导入**
   - 在导入列表中添加 `"github.com/HQGroup/nanobot-auto-updater/internal/health"`

2. **创建和启动健康监控器**
   - 在 API 服务器启动后创建 HealthMonitor
   - 使用 `cfg.HealthCheck.Interval` 配置健康检查间隔
   - 在单独的 goroutine 中启动监控器（非阻塞）
   - 添加中文日志 "健康监控已启动"
   - 仅在配置了实例时才启动监控器（nil 检查）

3. **优雅关闭健康监控器**
   - 在 API 服务器关闭之前调用 `healthMonitor.Stop()`
   - 添加 nil 检查以处理零实例配置场景
   - 保持关闭顺序：先停止健康监控器，再关闭 API 服务器

**验证结果:**
- ✅ 构建成功: `go build ./cmd/nanobot-auto-updater`
- ✅ 健康包导入存在
- ✅ `health.NewHealthMonitor` 调用存在
- ✅ `healthMonitor.Start()` 调用存在
- ✅ `healthMonitor.Stop()` 调用存在
- ✅ 使用 `cfg.HealthCheck.Interval` 配置
- ✅ 中文日志 "健康监控已启动" 存在

**Commit:** 2c2c2ee

## 实施的关键模式

1. **非阻塞启动模式**
   ```go
   go healthMonitor.Start()
   logger.Info("健康监控已启动", "interval", cfg.HealthCheck.Interval)
   ```
   - 使用 goroutine 确保不阻塞主流程
   - 立即记录启动日志

2. **Nil-安全组件管理**
   ```go
   var healthMonitor *health.HealthMonitor
   if len(cfg.Instances) > 0 {
       healthMonitor = health.NewHealthMonitor(...)
   }
   ```
   - 仅在有实例时创建监控器
   - 避免不必要的资源消耗

3. **优雅关闭顺序**
   ```go
   // Stop health monitor first
   if healthMonitor != nil {
       healthMonitor.Stop()
   }

   // Shutdown API server
   if apiServer != nil {
       apiServer.Shutdown(shutdownCtx)
   }
   ```
   - 反向关闭：后启动的先停止
   - 确保依赖关系的正确处理

## 技术亮点

1. **生命周期管理**
   - Start 在应用启动时自动开始监控
   - Stop 在应用关闭时优雅停止监控
   - 符合 HEALTH-01 和 HEALTH-04 需求

2. **配置驱动**
   - 使用 `cfg.HealthCheck.Interval` 控制监控频率
   - 支持灵活的监控间隔配置

3. **资源管理**
   - 仅在配置了实例时才启动监控器
   - 避免空监控器浪费资源

## 偏差和调整

无 - 计划完全按预期执行。

## 运行时验证建议

运行时测试步骤（手动验证）：

1. **准备测试配置**
   ```yaml
   health_check:
     interval: 10s
   instances:
     - name: "test-instance"
       port: 8080
   ```

2. **启动应用程序**
   ```bash
   ./nanobot-auto-updater.exe
   ```

3. **验证启动日志**
   - 应看到 "健康监控已启动 interval=10s"
   - 应看到 "初始状态检查 instance=test-instance"

4. **优雅关闭**
   - 按 Ctrl+C 停止应用
   - 应看到 "健康监控已停止" 在 API 服务器关闭之前

## 后续工作

此计划完成了健康监控器的生命周期集成。下一步：

- Phase 25 已完成 - 所有健康监控需求已实现
- 可以继续 Phase 26: Network Monitoring Core

## 自检结果

**文件检查:**
- ✅ cmd/nanobot-auto-updater/main.go 存在并已修改

**提交检查:**
- ✅ Commit 2c2c2ee 存在于 git log

**接受标准:**
- ✅ HealthMonitor 已导入
- ✅ 健康监控器在 goroutine 中启动
- ✅ 使用 cfg.HealthCheck.Interval
- ✅ 在关闭时调用 healthMonitor.Stop()
- ✅ 应用程序编译成功
- ✅ 中文日志消息存在
