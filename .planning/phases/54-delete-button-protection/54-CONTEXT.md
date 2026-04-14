# Phase 54: Delete Button Protection - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning

<domain>
## Phase Boundary

运行中实例禁用删除按钮，防止误删。实例停止后恢复可删除。仅涉及前端按钮禁用状态控制，不涉及后端变更。
</domain>

<decisions>
## Implementation Decisions

### Button Disabled State
- **D-01:** 在 `createInstanceCard()` 中设置 `deleteBtn.disabled = isRunning`，运行时禁用删除按钮
- **D-02:** 禁用样式复用项目现有 `opacity: 0.6; cursor: not-allowed` 模式，不添加 tooltip 或文字变更

### CSS Strategy
- **D-03:** 为 `.btn-delete-danger:disabled` 添加 CSS 规则，复用现有 disabled 样式模式（参考 `.btn-action:disabled`, `.btn-form-danger:disabled`）

### Confirmation Dialog
- **D-04:** Phase 53 的删除确认对话框（UI-05）保持不变。运行中实例的删除按钮在卡片层已被禁用，不会触发确认对话框，因此确认对话框中的警告文本（"该实例正在运行中"）仅作为额外防护

### Claude's Discretion
- 具体选择添加 CSS 规则的位置（独立规则或合并到已有 disabled 规则组）
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Instance Card UI
- `internal/web/static/home.js` L874-1046 — `createInstanceCard()` 函数，deleteBtn 创建于 L1031-1034
- `internal/web/static/home.js` L443-530 — `showDeleteDialog()` 函数，已有 isRunning 警告逻辑

### CSS Patterns
- `internal/web/static/style.css` L526-528 — `.btn-action:disabled` 现有 disabled 样式 (`opacity: 0.6; cursor: wait`)
- `internal/web/static/style.css` L823-826 — `.btn-form-danger:disabled` 现有 disabled 样式 (`opacity: 0.6; cursor: not-allowed`)
- `internal/web/static/style.css` L531-548 — `.btn-secondary` 和 `.btn-delete-danger` 样式定义

### Requirements
- `.planning/REQUIREMENTS.md` DEL-01 — 删除按钮禁用需求定义
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `isRunning` 参数已传入 `createInstanceCard(config, isRunning)`，直接可用
- 项目已有统一的 `:disabled` CSS 模式 (`opacity: 0.6; cursor: not-allowed`)，多处使用
- 卡片在 `loadInstances()` 每次调用时完整重建，5 秒轮询自然刷新 disabled 状态

### Established Patterns
- 按钮 disabled 模式：JS 设置 `btn.disabled = condition`，CSS `:disabled` 伪类控制样式
- `.btn-action:disabled` 使用 `cursor: wait`（操作进行中），`.btn-form-*:disabled` 使用 `cursor: not-allowed`（不可操作）

### Integration Points
- `createInstanceCard()` L1034: `deleteBtn.addEventListener('click', ...)` — 在此行前添加 `deleteBtn.disabled = isRunning`
- `loadInstances()` 5 秒轮询完整重建卡片，disabled 状态随 `isRunning` 自动更新
</code_context>

<specifics>
## Specific Ideas

- 需求原文明确指出："现有 `isRunning` 参数已传入 `createInstanceCard()`，仅需设置 `deleteBtn.disabled = isRunning`"
</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope
</deferred>

---
*Phase: 54-delete-button-protection*
*Context gathered: 2026-04-14*
