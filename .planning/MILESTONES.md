# Milestones

## v1.0 - Single Instance Auto-Update

**Completed:** 2026-02-18

**Features shipped:**
- 基础日志系统配置
- YAML 配置文件加载
- 单个 nanobot 实例的停止和启动
- UV 包管理器检测
- GitHub main 分支更新（带回退到 PyPI 稳定版）
- Pushover 失败通知
- Cron 定时调度
- Makefile 和 build.ps1 构建脚本

**Phases completed:** 4 phases (Phase 01-04)
- Phase 01: 基础配置和日志
- Phase 01.1: Nanobot 生命周期管理
- Phase 02: UV 检测和更新逻辑
- Phase 03: 调度和通知
- Phase 04: 运行时集成

**Lessons learned:**
- Windows 特定实现需要 `go:build windows` 约束
- 使用 `taskkill` 命令终止 Windows 进程
- 使用 `CREATE_NO_WINDOW` 标志隐藏控制台窗口
- Cron 调度需要 `SkipIfStillRunning` 防止重叠执行

---

## v0.2 - Multi-Instance Support

**Started:** 2026-03-09

**Goal:** 支持同时管理多个 nanobot 实例的升级和启动

**Status:** In Progress - Defining requirements
