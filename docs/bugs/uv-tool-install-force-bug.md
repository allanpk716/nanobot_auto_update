# uv tool install 升级 Bug 修复记录

**日期:** 2026-02-19
**严重程度:** 高 (导致自动更新功能失效)
**状态:** 已修复

## 问题描述

### 现象
使用 `uv tool install` 命令更新 nanobot 时,如果工具已安装,命令只会提示 `is already installed`,不会执行升级操作,导致自动更新功能完全失效。

### 影响
- 定时更新任务无法完成升级
- 用户无法获取最新版本
- 修复和改进无法自动推送

## 根本原因

### 代码位置
- **文件:** `internal/updater/updater.go`
- **行号:** 79 (GitHub 升级), 92 (PyPI 回退)

### 问题代码
```go
// GitHub 升级 (第 79 行)
output, err := u.runCommand(ctx, "uv", "tool", "install", u.githubURL)

// PyPI 回退 (第 92 行)
output, err = u.runCommand(ctx, "uv", "tool", "install", u.pypiPackage)
```

### 技术原因
根据 uv 工具的设计:
- `uv tool install <package>` - 首次安装,已存在时不升级
- `uv tool install --force <package>` - 强制重新安装,覆盖已有版本
- `uv tool upgrade <package>` - 升级已安装的工具(但不支持 Git URL)

原代码缺少 `--force` 标志,导致更新命令无效。

## 修复方案

### 方案选择
选择 **方案 1: 使用 `--force` 标志**,理由:
- ✅ 简单可靠,最小改动
- ✅ 同时支持 GitHub 和 PyPI 两种源
- ✅ 保持原有的双源更新策略
- ✅ uv 官方支持的参数

### 修复实施

#### 1. 修改 updater.go

**文件:** `internal/updater/updater.go`

**修改 1 - 更新注释 (第 71-73 行):**
```go
// Update attempts to update nanobot from GitHub main branch first,
// falling back to PyPI stable version if GitHub fails.
// Uses --force flag to ensure updates work even when already installed.
```

**修改 2 - GitHub 升级 (第 79-80 行):**
```go
u.logger.Info("Starting forced update from GitHub main branch")
output, err := u.runCommand(ctx, "uv", "tool", "install", "--force", u.githubURL)
```

**修改 3 - 日志优化 (第 88 行):**
```go
u.logger.Warn("GitHub forced update failed, attempting PyPI fallback",
```

**修改 4 - PyPI 回退 (第 93 行):**
```go
output, err = u.runCommand(ctx, "uv", "tool", "install", "--force", u.pypiPackage)
```

#### 2. 更新相关文档

**更新文件:**
- `.planning/research/STACK.md` - 更新 uv 命令说明
- `.planning/phases/02-core-update-logic/02-RESEARCH.md` - 更新技术方案和示例代码

**添加内容:**
- Pitfall 6: Missing --force Flag - 新增常见陷阱说明
- 更新所有命令示例以包含 `--force` 标志

## 验证结果

### 单元测试
```bash
go test ./internal/updater/... -v
```

**结果:**
```
✓ TestCheckUvInstalled
✓ TestCheckUvInstalledErrorMessage
✓ TestNewUpdater
✓ TestTruncateOutput (所有子测试)
✓ TestUpdateResultConstants (所有子测试)
PASS
```

### 构建验证
```bash
go build -o nanobot-auto-updater.exe ./cmd
```

**结果:** 构建成功,生成 12MB 可执行文件

### 功能验证

**当前状态:**
```
uv tool list
nanobot-ai v0.1.4
- nanobot
```

**验证命令:**
```bash
.\nanobot-auto-updater.exe -update-now
```

**预期结果:**
- 日志显示 "Starting forced update from GitHub main branch"
- 如果升级失败,显示 "GitHub forced update failed, attempting PyPI fallback"
- 更新成功完成,nanobot 版本更新

## 技术细节

### uv 命令对比

| 命令 | 已安装时行为 | 是否支持 Git URL |
|------|------------|----------------|
| `uv tool install <pkg>` | 提示 "already installed" | ✅ |
| `uv tool install --force <pkg>` | 强制重新安装/升级 | ✅ |
| `uv tool upgrade <pkg>` | 升级到最新版本 | ❌ |

### 为什么不用 `uv tool upgrade`?
1. 不支持 Git URL 格式 (如 `git+https://github.com/...`)
2. 我们的主要更新源是 GitHub main 分支,需要 Git URL 支持
3. `--force` 标志提供统一的方式处理两种源

## 影响评估

### 风险
- ✅ **低风险** - 只添加必需的参数,不改变核心逻辑
- ✅ **向后兼容** - `--force` 对首次安装也有效
- ✅ **性能影响** - 可忽略 (定时任务每天一次)

### 改进
- ✅ **可靠性提升** - 更新操作真正生效
- ✅ **日志清晰** - 明确显示 "forced update"
- ✅ **文档完善** - 添加陷阱说明,防止未来再犯

## 相关文件

### 修改文件
1. `internal/updater/updater.go` - 主要修复
2. `.planning/research/STACK.md` - uv 命令文档更新
3. `.planning/phases/02-core-update-logic/02-RESEARCH.md` - 技术方案更新

### 新增文件
1. `docs/bugs/uv-tool-install-force-bug.md` - 本文档

## 教训总结

### 根本问题
1. **文档理解不完整** - 未充分阅读 uv 工具文档中关于已安装包的处理
2. **测试覆盖不足** - 缺少"已安装状态下的升级"场景测试
3. **依赖外部工具行为** - 未验证工具在实际场景中的行为

### 改进措施
1. **增强文档** - 在 RESEARCH.md 中添加 Pitfall 6
2. **改进测试** - 计划添加集成测试验证升级场景
3. **代码审查** - 对外部命令的使用进行更严格的审查

## 后续行动

### 立即执行
- [x] 修复代码添加 `--force` 标志
- [x] 运行单元测试验证
- [x] 构建可执行文件
- [x] 更新相关文档

### 短期计划
- [ ] 执行端到端测试,验证真实升级场景
- [ ] 观察生产环境升级日志
- [ ] 添加集成测试覆盖升级场景

### 长期改进
- [ ] 考虑添加版本检查,避免不必要的强制重装
- [ ] 评估 `uv tool upgrade` + PyPI 的替代方案
- [ ] 增强错误报告和诊断信息

## 参考资料

- [uv 官方文档 - Tools Guide](https://docs.astral.sh/uv/guides/tools/)
- [uv CLI Reference](https://docs.astral.sh/uv/reference/cli/)
- 项目文件: `.planning/research/STACK.md`
- 项目文件: `.planning/phases/02-core-update-logic/02-RESEARCH.md`

---

**修复完成时间:** 2026-02-19 12:00
**验证状态:** 单元测试通过,待生产验证
**负责人:** Claude Code Assistant
