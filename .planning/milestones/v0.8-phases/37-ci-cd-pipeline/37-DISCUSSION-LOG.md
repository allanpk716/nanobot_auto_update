# Phase 37: CI/CD Pipeline - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-29
**Phase:** 37-ci-cd-pipeline
**Areas discussed:** Release 产物格式, Debug 构建版本, 构建验证步骤

---

## Release 产物格式

| Option | Description | Selected |
|--------|-------------|----------|
| ZIP 压缩包 | GoReleaser 默认生成 ZIP，Phase 38 需解压提取 exe 再 Apply。更符合发布习惯，可附带 README | ✓ |
| Raw exe | 直接上传 exe + checksums.txt，Phase 38 直接 Apply 无需解压。更简单但 Release 不规范 | |

**User's choice:** ZIP 压缩包 (Recommended)
**Notes:** Phase 38 自更新代码需要下载 ZIP 并解压提取 exe

---

## Debug 构建版本

| Option | Description | Selected |
|--------|-------------|----------|
| 仅 GUI 版本 | Release 只包含 -H=windowsgui 构建。简洁 | ✓ |
| GUI + Console 两个版本 | 同时发布 GUI 和 console 调试版本。方便排查但增加产物数量 | |

**User's choice:** 仅 GUI 版本 (Recommended)
**Notes:** 与 Makefile build-release 目标一致

---

## 构建验证步骤

| Option | Description | Selected |
|--------|-------------|----------|
| GoReleaser 管一切 | 单一 GoReleaser action，自带 go test。工作流简洁 | ✓ |
| 测试 + 发布分开 | 先独立 job 跑 go test/vet，通过后再触发 GoReleaser。更严格但复杂 | |

**User's choice:** GoReleaser 管一切 (Recommended)
**Notes:** 保持 CI 工作流简单

---

## Claude's Discretion

- GoReleaser 配置细节（archive name template, checksum 算法等）
- GitHub Actions workflow 具体结构（runner 版本、Go 版本等）
- Release name 和 description 模板

## Deferred Ideas

None
