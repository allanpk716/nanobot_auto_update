---
status: partial
phase: 53-instance-management-ui
source: [53-VERIFICATION.md]
started: 2026-04-12T18:30:00Z
updated: 2026-04-12T18:30:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. 视觉布局验证
expected: 实例卡片显示所有配置详情（name, port, command, timeout, auto_start），绿色状态点表示运行中，灰色表示已停止，所有文本为中文
result: [pending]

### 2. CRUD 对话框交互
expected: 各对话框打开时有正确的中文标签，编辑/复制表单正确预填充，删除显示运行中警告
result: [pending]

### 3. Nanobot 配置双向同步
expected: 结构化表单字段与 JSON textarea 双向更新，无效 JSON 显示红色中文错误提示
result: [pending]

### 4. 完整 CRUD 工作流
expected: 创建 -> 编辑 -> 复制 -> 删除端到端流程成功，Toast 通知显示，实例列表刷新
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
