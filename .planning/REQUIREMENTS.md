# Requirements

## Milestone v0.6: Update Log Recording and Query System

**Goal:** 记录每次 HTTP API 触发的更新操作,并提供查询接口获取更新历史日志

**Status:** Active — Requirements mapped to phases

---

## v0.6 Requirements

### Core Logging

- [ ] **LOG-01**: 系统能够记录每次更新操作的元数据
  - 记录更新 ID (UUID v4)
  - 记录开始时间戳和结束时间戳
  - 记录触发来源 (HTTP request metadata)
  - 记录整体状态 (success/partial_success/failed)

- [ ] **LOG-02**: 系统能够为每次更新生成唯一标识符
  - 使用 UUID v4 作为唯一 ID
  - ID 在 trigger-update 响应中返回
  - ID 可用于后续查询特定更新记录

- [ ] **LOG-03**: 系统能够记录每个实例的更新详情
  - 记录实例名称
  - 记录实例端口
  - 记录更新状态 (success/failed)
  - 记录错误消息 (如果失败)
  - 记录 stdout/stderr 引用 (指向 LogBuffer 历史记录)

- [ ] **LOG-04**: 系统能够计算并存储更新耗时
  - 记录从开始到结束的总耗时 (毫秒级精度)
  - 支持耗时统计和性能分析

### Storage

- [ ] **STORE-01**: 系统能够持久化更新日志到文件
  - 使用 JSON Lines 格式 (每行一个 JSON 对象)
  - 文件路径: ./logs/updates.jsonl
  - 使用原子追加写入避免并发冲突
  - 使用 sync.Mutex 保护并发写入
  - 文件不存在时自动创建

- [ ] **STORE-02**: 系统能够自动清理旧日志记录
  - 保留最近 7 天的日志记录
  - 删除 7 天前的记录
  - 在应用启动时执行清理
  - 使用临时文件 + rename 实现原子性清理
  - 清理过程不阻塞正常的读写操作

### Query API

- [ ] **QUERY-01**: 系统能够提供 HTTP GET /api/v1/update-logs 查询接口
  - 返回 JSON 格式的更新日志列表
  - 返回分页元数据 (总数、当前页、每页数量)
  - 使用 200 OK 状态码返回结果
  - 使用 401 Unauthorized 处理认证失败
  - 使用 500 Internal Server Error 处理服务器错误

- [ ] **QUERY-02**: 查询接口能够使用 Bearer Token 认证保护
  - 复用 Phase 28 的 AuthMiddleware
  - 使用配置文件中的 api_token 进行验证
  - 使用 subtle.ConstantTimeCompare 防止时序攻击
  - 认证失败返回 RFC 7807 JSON 错误格式

- [ ] **QUERY-03**: 查询接口能够支持分页参数
  - 支持 limit 参数 (默认 20,最大 100)
  - 支持 offset 参数 (默认 0,最小 0)
  - 使用 bufio.Scanner 流式读取避免内存问题
  - 实现早期终止 (读取到 limit 后停止)
  - 超出范围时返回空列表而不是错误

---

## Future Requirements (Deferred to v0.6.x / v2+)

### v0.6.x Enhancements

- [ ] **FILTER-01**: 查询接口支持按状态过滤
  - 支持 ?status=success 查询参数
  - 支持 ?status=failed 查询参数
  - 支持多个状态值 (逗号分隔)

- [ ] **FILTER-02**: 查询接口支持按时间范围过滤
  - 支持 ?from=<timestamp> 查询参数
  - 支持 ?to=<timestamp> 查询参数
  - 使用 RFC 3339 时间格式

- [ ] **STORE-03**: 支持可配置的保留天数
  - 在 config.yaml 中添加 update_log.retention_days 配置项
  - 默认值为 7 天
  - 支持用户自定义保留策略

- [ ] **STORE-04**: 日志文件轮转
  - 按日期分片日志文件
  - 或按大小轮转 (例如每 10MB 一个文件)
  - 避免单个文件过大影响查询性能

### v2+ Advanced Features

- [ ] **SEARCH-01**: 全文搜索功能
  - 搜索 stdout/stderr 内容
  - 搜索错误消息
  - 需要索引支持

- [ ] **EXPORT-01**: 日志导出功能
  - 支持 CSV 格式导出
  - 支持 Excel 格式导出
  - 支持 JSON 格式导出 (已实现)

- [ ] **ANALYTICS-01**: 日志分析和统计
  - 更新成功率统计
  - 耗时趋势分析
  - 失败原因分类

- [ ] **COMPRESS-01**: 旧日志压缩
  - 压缩 7 天前的日志而不是删除
  - 使用 gzip 压缩节省磁盘空间
  - 支持解压缩查询历史记录

---

## Out of Scope

明确排除在 v0.6 及后续版本之外的功能:

| Feature | Reason | Alternative |
|---------|--------|-------------|
| **Database storage** | 增加不必要的复杂度,文件存储足够 | JSON Lines 格式提供简单可靠的存储 |
| **Real-time log streaming** | v0.4 已通过 SSE 实现 | 复用现有 /api/v1/logs/:instance 端点 |
| **GUI interface** | 项目定位为 CLI 工具 | 提供 RESTful API,客户端可自行构建 UI |
| **Multi-tenant log isolation** | 系统是单租户 (单一管理员) | 所有日志属于单一操作者 |
| **Log modification API** | 审计日志不可修改 | 只提供读取接口,不提供更新/删除接口 |
| **Cross-platform support** | 仅支持 Windows | 项目目标用户在 Windows 环境 |

---

## Traceability

| REQ-ID | Phase | Plan | Status |
|--------|-------|------|--------|
| LOG-01 | Phase 30 | TBD | Not started |
| LOG-02 | Phase 30 | TBD | Not started |
| LOG-03 | Phase 30 | TBD | Not started |
| LOG-04 | Phase 30 | TBD | Not started |
| STORE-01 | Phase 31 | TBD | Not started |
| STORE-02 | Phase 31 | TBD | Not started |
| QUERY-01 | Phase 32 | TBD | Not started |
| QUERY-02 | Phase 32 | TBD | Not started |
| QUERY-03 | Phase 32 | TBD | Not started |

---

*Last updated: 2026-03-26*
