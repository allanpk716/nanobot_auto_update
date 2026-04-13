# Phase 53: Instance Management UI - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-12
**Phase:** 53-Instance Management UI
**Areas discussed:** 实例卡片布局, 对话框/模态系统, Nanobot 配置编辑器, 操作反馈与状态刷新

---

## 实例卡片布局

| Option | Description | Selected |
|--------|-------------|----------|
| 全部展示 | 名称、端口、启动命令、auto_start 标签、运行状态指示灯 | ✓ |
| 精简+展开 | 名称、端口、状态 + "详细信息"展开区域 | |
| 仅核心信息 | 名称、端口、状态 — 其余在编辑时才看到 | |

**User's choice:** 全部展示
**Notes:** 所有配置详情一目了然

| Option | Description | Selected |
|--------|-------------|----------|
| 单行水平排列 | 所有按钮排一行，简单直接 | |
| 下拉菜单收纳 | 主按钮显示，其他收入"..."下拉 | |
| 主要/次要分层 | 启停大按钮突出，其他小按钮一行 | ✓ |

**User's choice:** 主要/次要分层
**Notes:** 视觉层级分明

| Option | Description | Selected |
|--------|-------------|----------|
| 保持现有 Grid | CSS Grid auto-fill + minmax(280px, 1fr) | ✓ |
| 列表/表格形式 | 单列列表，信息密度更高 | |

**User's choice:** 保持现有 Grid

| Option | Description | Selected |
|--------|-------------|----------|
| 页面级按钮 | 网格上方，始终可见 | ✓ |
| 浮动按钮 | 右下角 FAB 样式 | |

**User's choice:** 页面级按钮

---

## 对话框/模态系统

| Option | Description | Selected |
|--------|-------------|----------|
| 居中模态弹窗 | 居中弹窗 + 半透明遮罩，最常见的管理界面模式 | ✓ |
| 右侧抽屉 | 从右侧滑入，不遮挡主列表 | |
| 独立页面 | 新建/编辑跳转新页面 | |

**User's choice:** 居中模态弹窗

| Option | Description | Selected |
|--------|-------------|----------|
| 单列纵向表单 | 每字段一行 | |
| 两列网格表单 | 字段分左右两列，缩短高度 | ✓ |
| Claude 决定 | Claude 根据字段类型自行决定 | |

**User's choice:** 两列网格表单
**Notes:** 5 个字段正好适合两列

| Option | Description | Selected |
|--------|-------------|----------|
| 简单确认对话框 | "确定删除？" + 运行中警告 | ✓ |
| 输入名称确认 | 输入实例名确认删除，更安全 | |

**User's choice:** 简单确认对话框

---

## Nanobot 配置编辑器

| Option | Description | Selected |
|--------|-------------|----------|
| 左右分栏（表单+JSON） | 左侧表单 + 右侧 JSON 预览，实时同步 | ✓ |
| Tab 切换（表单/JSON） | Tab 切换两个视图 | |
| Claude 决定 | Claude 决定最佳布局 | |

**User's choice:** 左右分栏

| Option | Description | Selected |
|--------|-------------|----------|
| 所有可配置字段 | 约 10 个常用字段全部入表单 | |
| 核心字段精简 | model/provider/gateway port/apiKey/telegram token，其余 JSON | ✓ |
| Claude 决定 | Claude 根据使用频率决定 | |

**User's choice:** 核心字段精简

| Option | Description | Selected |
|--------|-------------|----------|
| 原生 textarea | 简单轻量，无第三方库 | ✓ |
| JSON 编辑库 | 引入轻量库提供语法高亮、折叠、验证 | |

**User's choice:** 原生 textarea
**Notes:** 保持与项目原生 HTML/CSS/JS 技术栈一致

| Option | Description | Selected |
|--------|-------------|----------|
| 卡片按钮触发 | 卡片上"配置"按钮，独立模态窗 | ✓ |
| 编辑对话框内嵌 | 编辑实例对话框内 Tab 切换 | |

**User's choice:** 卡片按钮触发

---

## 操作反馈与状态刷新

| Option | Description | Selected |
|--------|-------------|----------|
| Toast 通知 | 右上角弹出通知条，3 秒自动消失 | ✓ |
| 内联状态提示 | 按钮/卡片内直接显示文字状态 | |
| Claude 决定 | Claude 决定最佳反馈方式 | |

**User's choice:** Toast 通知

| Option | Description | Selected |
|--------|-------------|----------|
| 保持轮询 | 保持 5 秒轮询，操作后立即额外刷新 | ✓ |
| 操作后延迟刷新 | 等待后端处理再刷新 | |
| Claude 决定 | Claude 决定 | |

**User's choice:** 保持轮询

| Option | Description | Selected |
|--------|-------------|----------|
| 按钮 loading 状态 | 按钮禁用+旋转图标，API 返回后刷新 | ✓ |
| 卡片 loading 遮罩 | 卡片整体显示 loading 遮罩 | |

**User's choice:** 按钮 loading 状态

---

## Claude's Discretion

- 模态弹窗的具体 CSS 样式和动画效果
- Toast 通知组件的具体实现（位置、堆叠、动画）
- 表单字段的输入验证规则和错误提示位置
- JSON textarea 的格式化和验证策略
- 新建实例时 nanobot 配置编辑器的集成方式
- 卡片上操作按钮的具体排列顺序和间距
- 两列表单中字段的分组方式

## Deferred Ideas

- JSON 编辑器语法高亮 — 可引入轻量库但当前不优先
- Nanobot 配置 schema 验证（ENC-01）— 未来里程碑
- 配置模板库（ENC-02）— 未来里程碑
- 批量操作 UI（AIM-02）— 未来里程碑
- 实例拖拽排序（AIM-03）— 未来里程碑
- 配置版本历史 UI（AIM-04）— 未来里程碑
