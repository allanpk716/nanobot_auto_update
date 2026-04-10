# Requirements: Nanobot Auto Updater -- v0.11

**Defined:** 2026-04-09
**Core Value:** 自动保持 nanobot 处于最新版本,无需用户手动干预。

## v0.11 Requirements

Requirements for Windows Service auto-start milestone. Each maps to roadmap phases.

### 服务核心

- [ ] **SVC-01**: 程序启动时通过 svc.IsWindowsService() 检测运行模式，自动选择服务模式或控制台模式
- [ ] **SVC-02**: 实现 svc.Handler 接口的 Execute 方法，处理服务启动/停止/关机请求
- [ ] **SVC-03**: 服务模式优雅关闭，响应 Stop 和 Shutdown 控制码，确保资源清理

### 服务管理

- [ ] **MGR-01**: config.yaml 新增 `auto_start: true/false` 配置项
- [ ] **MGR-02**: auto_start 为 true 时，程序以管理员权限注册自身为 Windows 服务并退出
- [ ] **MGR-03**: auto_start 为 false 时，程序检测到已注册服务则自动卸载
- [ ] **MGR-04**: 服务配置 SCM 恢复策略（失败后自动重启）

### 现有代码适配

- [ ] **ADPT-01**: daemon.go 在服务模式下跳过守护进程模式
- [ ] **ADPT-02**: restartFn 在服务模式下使用 SCM 重启（net stop + net start）而非 self-spawn
- [ ] **ADPT-03**: 服务模式设置工作目录为 exe 所在目录，避免 System32 默认路径问题
- [ ] **ADPT-04**: 服务模式下监听配置文件变更，自动重载配置无需重启服务

## v2 Requirements

Deferred to future milestone.

### 服务增强

- **SVC-04**: Windows Event Log 集成（日志输出到事件查看器）
- **MGR-05**: 命令行子命令 install-service / uninstall-service 独立管理服务

## Out of Scope

| Feature | Reason |
|---------|--------|
| 服务暂停/恢复功能 | 当前不需要，服务只需启动和停止 |
| 多服务实例 | 程序设计为单实例运行 |
| Linux/systemd 支持 | 项目限定 Windows 平台 |
| 服务依赖配置 | 无外部服务依赖 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| MGR-01 | Phase 46 | Pending |
| SVC-01 | Phase 46 | Pending |
| SVC-02 | Phase 47 | Pending |
| SVC-03 | Phase 47 | Pending |
| MGR-02 | Phase 48 | Pending |
| MGR-03 | Phase 48 | Pending |
| MGR-04 | Phase 48 | Pending |
| ADPT-01 | Phase 49 | Pending |
| ADPT-02 | Phase 49 | Pending |
| ADPT-03 | Phase 49 | Pending |
| ADPT-04 | Phase 49 | Pending |

**Coverage:**
- v0.11 requirements: 11 total
- Mapped to phases: 11
- Unmapped: 0

---
*Requirements defined: 2026-04-09*
*Last updated: 2026-04-10 after roadmap update (added ADPT-04)*
