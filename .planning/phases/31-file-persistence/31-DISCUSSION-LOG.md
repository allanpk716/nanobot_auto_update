# Phase 31: File Persistence - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-28
**Phase:** 31-file-persistence
**Areas discussed:** 写入策略, 文件 I/O 错误处理, 生命周期集成, 清理时机

---

## 写入策略

| Option | Description | Selected |
|--------|-------------|----------|
| 同步立即写入 | 每次 Record() 调用时立即序列化 JSON 并追加写入文件 | ✓ |
| 异步批量写入 | Record() 写入内存 buffer，后台 goroutine 定期 flush | |

**User's choice:** 同步立即写入
**Notes:** 数据安全性优先，更新操作频率低（分钟级），延迟影响可忽略

### fsync 策略

| Option | Description | Selected |
|--------|-------------|----------|
| 每次 fsync | 文件只打开一次，每次写入后调用 Sync() | ✓ |
| 依赖 OS 缓存 | 文件保持打开，依赖操作系统缓存刷盘 | |
| 每次写入后关闭 | 文件每次写入后关闭并重新打开 | |

**User's choice:** 每次 fsync
**Notes:** 保证崩溃时的数据安全

### 文件写入方式

| Option | Description | Selected |
|--------|-------------|----------|
| os.OpenFile 直接追加 | 直接打开文件追加写入 | ✓ |
| bufio.Writer 缓冲 | 用 bufio.Writer 包装文件写入，批量刷盘 | |

**User's choice:** os.OpenFile 直接追加
**Notes:** 考虑到每次写入后都要 fsync，bufio 缓冲意义不大

### 内存 vs 文件存储策略

| Option | Description | Selected |
|--------|-------------|----------|
| 内存 + 文件双写 | 继续保留内存 slice + 同步写文件 | ✓ |
| 纯文件写入 | Record() 只写文件，GetAll() 从文件读取 | |

**User's choice:** 内存 + 文件双写
**Notes:** 查询走内存（快速），文件做持久化备份。Phase 32 查询 API 从内存读取。

---

## 文件 I/O 错误处理

### 写入失败策略

| Option | Description | Selected |
|--------|-------------|----------|
| 记录错误 + 内存降级 | 文件写入失败时记录 ERROR 日志，继续内存存储，下次重试 | ✓ |
| 记录错误 + 停止文件写入 | 设置标志位，后续不再尝试写文件 | |
| 重试 + 降级 | 带退避重试（最多 3 次），全部失败后降级 | |

**User's choice:** 记录错误 + 内存降级
**Notes:** 每次都重试 — 自然恢复机制，临时错误恢复后下次写入自动成功

### 文件创建时机

| Option | Description | Selected |
|--------|-------------|----------|
| Record() 内处理 | 文件打开失败时检查目录是否存在，不存在则创建 | ✓ |
| 启动时预检查 | 应用启动时检查并创建 JSONL 文件和目录 | |

**User's choice:** Record() 内处理
**Notes:** 不在启动时预创建文件，首次 Record() 时懒创建

---

## 生命周期集成

### UpdateLogger 创建位置

| Option | Description | Selected |
|--------|-------------|----------|
| main.go 创建 + 传入 | UpdateLogger 在 main.go 中创建，传给 NewServer() | ✓ |
| NewServer 内创建 + Server.Close() flush | 保持 NewServer() 内创建，Server.Close() 负责 flush | |

**User's choice:** main.go 创建 + 传入
**Notes:** UpdateLogger 的生命周期与整个应用一致，不受 HTTP 服务器启停影响

### 清理方法

| Option | Description | Selected |
|--------|-------------|----------|
| Close() 关闭文件 | 添加 Close() 方法，关闭 file handle 和停止 cron | ✓ |
| Close() + Flush() | 还添加 Flush() 方法强制刷盘 | |
| 无清理方法 | 程序退出时 OS 自动关闭文件 | |

**User's choice:** Close() 关闭文件
**Notes:** 同步写入每次都 fsync，Flush() 没有实际作用

---

## 清理时机

### 清理策略

| Option | Description | Selected |
|--------|-------------|----------|
| 仅启动时清理 | 仅在应用启动时执行一次清理 | |
| 启动 + 后台定期清理 | 启动时清理 + 每隔 24 小时后台定时清理 | ✓ |

**User's choice:** 启动 + 后台定期清理
**Notes:** 程序长期运行时也能及时清理旧日志

### 后台清理实现方式

| Option | Description | Selected |
|--------|-------------|----------|
| robfig/cron 定时任务 | 使用现有的 robfig/cron 库添加每日清理任务 | ✓ |
| time.Ticker 简单定时 | 使用 time.Ticker 实现简单的 24 小时定时器 | |
| Record() 内触发检查 | 在每次 Record() 后检查是否需要清理 | |

**User's choice:** robfig/cron 定时任务
**Notes:** 与项目已有的 cron 模式一致，复用现有依赖

### 清理间隔

| Option | Description | Selected |
|--------|-------------|----------|
| 每 24 小时 | 凌晨 3 点执行，与现有 cron 更新任务错开 | ✓ |
| 每 6 小时 | 更频繁，磁盘空间回收更及时 | |

**User's choice:** 每 24 小时
**Notes:** 与日志保留天数一致，每天检查一次足够

---

## Claude's Discretion

- JSONL 文件具体的打开/关闭时机
- 清理 cron 任务的注册方式
- 内存 slice 在启动时是否从文件恢复
- 文件 handle 的错误恢复策略
- 清理任务的具体 cron 表达式

## Deferred Ideas

None — 讨论保持在阶段范围内
