# Phase 38: Self-Update Core - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-29
**Phase:** 38-self-update-core
**Areas discussed:** 下载与解压流程, SHA256 校验方式, 包公共 API 设计, 配置节设计

---

## 下载与解压流程

| Option | Description | Selected |
|--------|-------------|----------|
| 内存解压 | 下载 ZIP 到内存，archive/zip 解压提取 exe 到 bytes.Buffer，直接传 io.Reader 给 Apply() | ✓ |
| 临时文件解压 | 下载 ZIP 到临时文件，解压 exe 到临时文件，传文件 Reader 给 Apply()，最后清理 | |

**User's choice:** 内存解压（推荐）
**Notes:** ZIP 约 10MB（Go exe），内存占用可接受。无临时文件清理问题。

---

## SHA256 校验方式

| Option | Description | Selected |
|--------|-------------|----------|
| checksums.txt 校验 ZIP | 下载 checksums.txt，解析 ZIP 文件 SHA256，计算实际 ZIP hash 比对。双重验证 | ✓ |
| 仅下载后自算 hash | 不下载 checksums.txt，直接对解压后的 exe 计算 SHA256 作为元数据返回 | |
| checksums.txt 校验 exe | 下载 checksums.txt 解析 exe SHA256，解压后对 exe 计算实际 SHA256 比对 | |

**User's choice:** checksums.txt 校验 ZIP（推荐）
**Notes:** 先校验 ZIP 传输完整性，通过后再解压。Phase 37 GoReleaser 已配置 checksums.txt 生成。

---

## 包公共 API 设计

| Option | Description | Selected |
|--------|-------------|----------|
| Updater struct + 方法 | NewUpdater(config) 创建实例，暴露 CheckLatest()、NeedUpdate()、Update() 方法 | ✓ |
| 函数式 API | 暴露 CheckLatest() 和 Apply() 函数，缓存用包级别变量 | |
| 接口 + 实现 | 定义 Updater interface + DefaultUpdater 实现 | |

**User's choice:** Updater struct + 方法（推荐）
**Notes:** 缓存和 http.Client 封装在 struct 内部。Phase 39 handler 通过 struct 方法控制更新流程。

---

## 配置节设计

| Option | Description | Selected |
|--------|-------------|----------|
| 最小配置 | self_update 节仅 github_owner 和 github_repo 两个字段 | ✓ |
| 扩展配置 | github_owner、github_repo、cache_ttl、timeout 等可配置 | |
| 无配置硬编码 | owner/repo 硬编码在代码中 | |

**User's choice:** 最小配置（推荐）
**Notes:** 缓存 TTL=1h、HTTP timeout=30s 等硬编码为包常量。

---

## Claude's Discretion

- semver 解析实现方式
- 缓存具体实现细节
- GitHub API 错误处理和重试策略
- 文件拆分策略
- 测试策略和 mock 方式

## Deferred Ideas

None — discussion stayed within phase scope
