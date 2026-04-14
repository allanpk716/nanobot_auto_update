# Requirements: Nanobot Auto Updater — v0.18.0

**Defined:** 2026-04-14
**Core Value:** 自动保持 nanobot 处于最新版本，无需用户手动干预

## v1 Requirements

Requirements for v0.18.0 milestone. Each maps to roadmap phases.

### Delete Protection

- [ ] **DEL-01**: 实例运行中时删除按钮禁用（灰色不可点击），停止后才可点击删除。现有 `isRunning` 参数已传入 `createInstanceCard()`，仅需设置 `deleteBtn.disabled = isRunning`。

### JSON Config Editor

- [ ] **EDT-01**: 配置编辑控件支持 JSON 语法高亮显示。使用 Ace Editor v1.43.6 `src-min-noconflict` 构建，vendored 到 `internal/web/static/vendor/ace/`，通过 embed.FS 内嵌部署。Ace Editor JSON mode 提供内置语法高亮。
- [ ] **EDT-02**: 配置编辑控件实时自动校验 JSON 格式，语法错误即时高亮提示给用户。Ace Editor Web Worker 自动检测 JSON 语法错误并在编辑器内标注错误位置和行号。

### Config Directory Management

- [ ] **CFG-01**: 创建实例对话框中集成 nanobot 配置编辑区域，用户可在同一界面填写实例信息和 config 配置。在现有创建实例表单下方添加配置编辑面板（复用 Ace Editor 组件）。
- [ ] **CFG-02**: 创建实例时允许用户填写 config 保存目录路径。后端 `InstanceConfig` 新增 `config_dir` 字段，`ParseConfigPath()` 支持自定义路径。空字符串表示使用默认路径（向后兼容）。
- [ ] **CFG-03**: 启动实例时自动创建不存在的配置目录（`os.MkdirAll`）；若目录已存在且含 config 文件，则读取并展示给用户。创建实例对话框加载时检查目录状态，已有配置则预填充到编辑器。

## Future Requirements

Deferred to future milestones.

### Semantic Validation

- **SEM-01**: JSON 配置语义级校验（字段必填检查、类型检查、值范围校验）
- **SEM-02**: 配置 schema 版本管理和迁移

### UI Optimization

- **UI-OPT-01**: 实例状态轮询优化为 diff-based 更新（替代当前 5s 全量重渲染）
- **UI-OPT-02**: 删除/启动/停止操作后乐观 UI 更新（避免 1-5s 状态延迟）

## Out of Scope

| Feature | Reason |
|---------|--------|
| 删除确认对话框 | 已在 v0.12 (UI-05) 实现，零工作 |
| 语义级 JSON 校验（字段必填、类型检查） | 超出本次范围，仅做语法级校验 |
| CDN 加载编辑器库 | 破坏离线单文件部署约束（embed.FS 是核心架构决策） |
| 5s 轮询优化为 diff-based 更新 | 独立优化，非本里程碑重点 |
| Monaco Editor | ~5MB 太大，embed.FS 不友好 |
| CodeMirror 6 | 仅支持 ES modules，需要构建步骤，与零构建约束冲突 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| DEL-01 | — | Pending |
| EDT-01 | — | Pending |
| EDT-02 | — | Pending |
| CFG-01 | — | Pending |
| CFG-02 | — | Pending |
| CFG-03 | — | Pending |

**Coverage:**
- v1 requirements: 6 total
- Mapped to phases: 0
- Unmapped: 6 ⚠️

---
*Requirements defined: 2026-04-14*
*Last updated: 2026-04-14 after initial definition*
