# Roadmap: Nanobot Auto Updater

## Milestones

- **v1.0 Single Instance Auto-Update** - Phases 01-04 (shipped 2026-02-18)
- **v0.2 Multi-Instance Support** - Phases 05-18 (shipped 2026-03-16)
- **v0.4 Real-time Log Viewing** - Phases 19-23 (shipped 2026-03-20)
- **v0.5 Core Monitoring and Automation** - Phases 24-29 (shipped 2026-03-24)
- **v0.6 Update Log Recording and Query System** - Phases 30-33 (shipped 2026-03-29)
- **v0.7 Update Lifecycle Notifications** - Phases 34-35 (shipped 2026-03-29)
- **v0.8 Self-Update** - Phases 36-40 (shipped 2026-03-30)
- **v0.9 Startup Notification & Telegram Monitor** - Phases 41-43 (shipped 2026-04-06)
- **v0.10 管理界面自更新功能** - Phases 44-45 (shipped 2026-04-08)
- **v0.11 Windows 服务自启动** - Phases 46-49 (shipped 2026-04-11)
- **v0.12 实例管理与配置编辑** - Phases 50-53 (shipped 2026-04-13)
- **v0.18.0 实例管理增强** - Phases 54-57 (in progress)

## Phases

<details>
<summary>v1.0 Single Instance Auto-Update (Phases 01-04) - SHIPPED 2026-02-18</summary>

基础自动更新功能已交付。

</details>

<details>
<summary>v0.2 Multi-Instance Support (Phases 05-18) - SHIPPED 2026-03-16</summary>

多实例管理功能已交付。

</details>

<details>
<summary>v0.4 Real-time Log Viewing (Phases 19-23) - SHIPPED 2026-03-20</summary>

实时日志查看功能已交付。

</details>

<details>
<summary>v0.5 Core Monitoring and Automation (Phases 24-29) - SHIPPED 2026-03-24</summary>

核心监控和自动化功能已交付。

</details>

<details>
<summary>v0.6 Update Log Recording and Query System (Phases 30-33) - SHIPPED 2026-03-29</summary>

- [x] Phase 30: Log Structure and Recording (2/2 plans) - completed 2026-03-27
- [x] Phase 31: File Persistence (2/2 plans) - completed 2026-03-28
- [x] Phase 32: Query API (2/2 plans) - completed 2026-03-29
- [x] Phase 33: Integration and Testing (2/2 plans) - completed 2026-03-29

</details>

<details>
<summary>v0.7 Update Lifecycle Notifications (Phases 34-35) - SHIPPED 2026-03-29</summary>

- [x] Phase 34: Update Notification Integration (1/1 plan) - completed 2026-03-29
- [x] Phase 35: Notification Integration Testing (1/1 plan) - completed 2026-03-29

</details>

<details>
<summary>v0.8 Self-Update (Phases 36-40) - SHIPPED 2026-03-30</summary>

- [x] Phase 36: PoC Validation (1/1 plan) - completed 2026-03-29
- [x] Phase 37: CI/CD Pipeline (1/1 plan) - completed 2026-03-29
- [x] Phase 38: Self-Update Core (2/2 plans) - completed 2026-03-30
- [x] Phase 39: HTTP API Integration (2/2 plans) - completed 2026-03-30
- [x] Phase 40: Safety & Recovery (2/2 plans) - completed 2026-03-30

</details>

<details>
<summary>v0.9 Startup Notification & Telegram Monitor (Phases 41-43) - SHIPPED 2026-04-06</summary>

- [x] Phase 41: Startup Notification (2/2 plans) - completed 2026-04-06
- [x] Phase 42: Telegram Monitor Core (2/2 plans) - completed 2026-04-06
- [x] Phase 43: Telegram Monitor Integration (2/2 plans) - completed 2026-04-06

</details>

<details>
<summary>v0.10 管理界面自更新功能 (Phases 44-45) - SHIPPED 2026-04-08</summary>

- [x] Phase 44: 后端 -- 自更新进度追踪 + Web Token API (2/2 plans) - completed 2026-04-07
- [x] Phase 45: 前端 -- 自更新管理 UI (2/2 plans) - completed 2026-04-08

</details>

<details>
<summary>v0.11 Windows 服务自启动 (Phases 46-49) - SHIPPED 2026-04-11</summary>

- [x] Phase 46: Service Configuration & Mode Detection (2/2 plans) - completed 2026-04-10
- [x] Phase 47: Windows Service Handler (2/2 plans) - completed 2026-04-10
- [x] Phase 48: Service Manager (2/2 plans) - completed 2026-04-11
- [x] Phase 49: Existing Code Adaptation (2/2 plans) - completed 2026-04-11

</details>

<details>
<summary>v0.12 实例管理与配置编辑 (Phases 50-53) - SHIPPED 2026-04-13</summary>

- [x] Phase 50: Instance Config CRUD API (2/2 plans) - completed 2026-04-11
- [x] Phase 51: Instance Lifecycle Control API (2/2 plans) - completed 2026-04-12
- [x] Phase 52: Nanobot Config Management API (2/2 plans) - completed 2026-04-12
- [x] Phase 53: Instance Management UI (3/3 plans) - completed 2026-04-13

</details>

### v0.18.0 实例管理增强 (In Progress)

**Milestone Goal:** 增强 Web UI 实例管理的安全性、易用性和配置编辑体验

- [ ] **Phase 54: Delete Button Protection** - 运行中实例禁用删除按钮，防止误删
- [ ] **Phase 55: JSON Editor Integration** - Ace Editor 语法高亮与实时 JSON 校验
- [ ] **Phase 56: Config Directory Backend** - 自定义配置目录后端支持
- [ ] **Phase 57: Config Directory Frontend** - 配置目录前端集成与创建对话框增强

## Phase Details

### Phase 54: Delete Button Protection
**Goal**: 用户无法删除正在运行的实例，避免意外中断服务
**Depends on**: Nothing (first phase of milestone)
**Requirements**: DEL-01
**Success Criteria** (what must be TRUE):
  1. 实例处于运行状态时，卡片上的删除按钮显示为灰色且不可点击
  2. 实例停止后，删除按钮恢复为可点击状态
  3. 5 秒状态轮询刷新后，按钮状态与实例当前运行状态一致
**Plans**: TBD

Plans:
- [ ] 54-01: Delete button disabled state for running instances

### Phase 55: JSON Editor Integration
**Goal**: 用户在配置编辑器中获得 JSON 语法高亮和实时错误提示，提升编辑体验
**Depends on**: Nothing
**Requirements**: EDT-01, EDT-02
**Success Criteria** (what must be TRUE):
  1. 用户打开 nanobot 配置编辑对话框时，JSON 内容显示语法高亮（字符串、数字、布尔值、键名颜色区分）
  2. 编辑器显示行号和括号匹配，用户可清晰定位代码位置
  3. 用户输入非法 JSON（如缺少逗号、引号不匹配）时，编辑器在错误位置标注红色波浪线并显示错误信息
  4. 表单字段与 JSON 编辑器之间的双向同步（syncGuard）在切换到 Ace Editor 后仍然正常工作
**Plans**: TBD

Plans:
- [ ] 55-01: Vendor Ace Editor files and integrate into config dialog
- [ ] 55-02: Preserving syncGuard bidirectional sync with Ace Editor

### Phase 56: Config Directory Backend
**Goal**: 后端支持自定义配置目录路径，自动创建目录并读取已有配置
**Depends on**: Nothing
**Requirements**: CFG-02, CFG-03
**Success Criteria** (what must be TRUE):
  1. 用户通过 API 创建实例时可指定 config_dir 字段，指定后 nanobot 配置保存到该目录
  2. config_dir 为空字符串时，系统使用默认路径（向后兼容现有行为）
  3. 指定的配置目录不存在时，系统自动创建目录（os.MkdirAll）
  4. 指定的配置目录已存在且包含 config.json 时，创建实例时读取并返回该文件内容
  5. 包含路径遍历字符（..）的目录路径被拒绝，返回验证错误
**Plans**: TBD

Plans:
- [ ] 56-01: Add config_dir field to InstanceConfig and ParseConfigPathWithDir
- [ ] 56-02: Path validation and auto-create/read directory logic

### Phase 57: Config Directory Frontend
**Goal**: 用户在创建实例对话框中可直接填写配置目录和编辑 nanobot 配置，一站式完成实例创建与配置
**Depends on**: Phase 55, Phase 56
**Requirements**: CFG-01
**Success Criteria** (what must be TRUE):
  1. 创建实例对话框中包含配置目录路径输入框，用户可填写自定义路径
  2. 创建实例对话框中包含 nanobot 配置编辑区域（复用 Ace Editor 组件），用户可在同一界面填写实例信息和配置
  3. 用户填写已存在配置的目录路径时，编辑器自动加载该目录下的 config.json 内容
  4. 用户创建实例后，配置内容自动保存到指定目录
**Plans**: TBD
**UI hint**: yes

Plans:
- [ ] 57-01: Add config_dir field and config editor to create instance dialog

## Progress

**Execution Order:**
Phases execute in numeric order: 54 -> 55 -> 56 -> 57
Note: Phases 55 and 56 have no mutual dependencies and can be built in parallel.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 54. Delete Button Protection | v0.18.0 | 0/1 | Not started | - |
| 55. JSON Editor Integration | v0.18.0 | 0/2 | Not started | - |
| 56. Config Directory Backend | v0.18.0 | 0/2 | Not started | - |
| 57. Config Directory Frontend | v0.18.0 | 0/1 | Not started | - |

---
*Last updated: 2026-04-13 (v0.18.0 roadmap created)*
