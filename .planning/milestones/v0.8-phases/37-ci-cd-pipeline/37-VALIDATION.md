---
phase: 37
slug: ci-cd-pipeline
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-29
---

# Phase 37 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | 无代码变更，使用 GoReleaser CLI 验证 |
| **Config file** | 不适用 |
| **Quick run command** | `goreleaser check .goreleaser.yaml` |
| **Full suite command** | `goreleaser release --snapshot --clean` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `goreleaser check .goreleaser.yaml`
- **After every plan wave:** Run `goreleaser release --snapshot --clean`
- **Before `/gsd:verify-work`:** Full snapshot build must succeed
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 37-01-01 | 01 | 1 | CICD-02 | config | `goreleaser check .goreleaser.yaml` | ❌ W0 | ⬜ pending |
| 37-01-02 | 01 | 1 | CICD-01 | config | `cat .github/workflows/release.yml` | ❌ W0 | ⬜ pending |
| 37-01-03 | 01 | 1 | CICD-03 | smoke | `goreleaser release --snapshot --clean` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] 本地安装 GoReleaser CLI (`go install github.com/goreleaser/goreleaser/v2@latest`) — 用于本地验证
- [ ] 需要推送到 GitHub 并创建 tag 才能完整验证 workflow — 本地只能验证 GoReleaser 配置

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| v* tag push 触发 GitHub Actions workflow | CICD-01 | 需要 GitHub 远程仓库和 tag push | 推送 v* tag 后观察 GitHub Actions 是否自动触发 |
| Windows amd64 二进制发布到 GitHub Releases | CICD-02 | 需要 GitHub Actions 完整运行 | 推送 tag 后检查 Release 页面产物是否正确 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
