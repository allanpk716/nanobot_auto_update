# Feature Research

**Domain:** Update log recording and query system for nanobot auto-updater
**Researched:** 2026-03-26
**Confidence:** HIGH

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Update execution logging** | Every update operation should be recorded for troubleshooting | MEDIUM | Record start/end time, instances updated, success/failure status |
| **Log persistence** | Logs should survive application restart | LOW | Write to file in JSON Lines format (standard for audit logs) |
| **Query by time range** | Users need to find recent updates quickly | LOW | Filter logs by timestamp (last N hours/days) |
| **Log cleanup** | Logs shouldn't grow indefinitely | MEDIUM | Auto-delete logs older than retention period (7 days specified) |
| **Authentication** | Query API must be protected like trigger-update | LOW | Reuse existing Bearer Token authentication |
| **Request ID tracking** | Each update should have unique identifier for debugging | LOW | UUID or timestamp-based ID attached to every update operation |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required, but valuable.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Instance-level detail** | See which specific instances succeeded/failed | MEDIUM | Record per-instance status, error messages, stdout/stderr |
| **Full output capture** | Complete stdout/stderr for debugging failed updates | MEDIUM | Link to existing ring buffer logs or capture separately |
| **Pagination** | Query large datasets efficiently | LOW | Standard limit/offset pattern for API responses |
| **Filter by status** | Quickly find failed updates vs successful ones | LOW | Add `status` query parameter (success/failed/all) |
| **Trigger source tracking** | Know who/what initiated the update | LOW | Record HTTP client info, timestamp, authentication details |
| **Duration tracking** | Measure how long updates take | LOW | Calculate and store elapsed time for performance analysis |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Unlimited log retention** | Keep all history forever | Storage grows unbounded, query performance degrades | Implement 7-day retention with clear documentation |
| **Database storage** | Better querying with SQL | Adds database dependency to simple file-based system | JSON Lines format provides simple querying without DB overhead |
| **Real-time log streaming** | Watch updates as they happen | Already implemented in v0.4 via SSE, don't duplicate | Reuse existing `/api/v1/logs/:instance` SSE endpoint |
| **Log search (text search)** | Find specific error messages | Adds significant complexity (regex, indexing) | Defer to v2; for now, query returns structured data that's easy to filter client-side |
| **Log export (CSV/Excel)** | Download for external analysis | Adds format conversion complexity | JSON API response is already exportable; client can transform if needed |
| **Multi-tenant log isolation** | Separate logs per user | System is single-tenant (one admin user) | Keep simple; all logs are for single operator |

## Feature Dependencies

```
[HTTP POST /api/v1/trigger-update] (existing Phase 28)
    └──triggers──> [Update execution logging]
                       └──writes──> [JSON Lines file]
                                          └──read-by──> [GET /api/v1/update-logs]
                                                              └──requires──> [Bearer Token auth] (existing Phase 28)

[Update execution logging]
    └──requires──> [Request ID generation]
    └──requires──> [Instance status tracking] (existing v0.2)

[JSON Lines file]
    └──managed-by──> [Log cleanup job]
                         └──requires──> [Retention policy config]

[GET /api/v1/update-logs]
    └──supports──> [Pagination (limit/offset)]
    └──supports──> [Status filtering]
    └──supports──> [Time range filtering]
```

### Dependency Notes

- **Update logging requires trigger-update endpoint:** Logging only happens when updates are triggered via existing API
- **Query API requires Bearer Token auth:** Reuse authentication from Phase 28 for consistency
- **Log cleanup requires retention policy:** Need configuration for how long to keep logs (7 days specified in PROJECT.md)
- **Instance status tracking (existing v0.2):** Reuse multi-instance error aggregation and reporting
- **Real-time streaming (existing v0.4):** Don't duplicate SSE functionality; update logs are historical records

## MVP Definition

### Launch With (v0.6)

Minimum viable product — what's needed to validate the concept.

- [x] **Record update metadata** — Update ID, start/end timestamp, trigger source, overall status
- [x] **Record instance results** — For each instance: success/failure, error message, stdout/stderr reference
- [x] **JSON Lines file persistence** — Append-only log file in standard format
- [x] **GET /api/v1/update-logs endpoint** — Query recent update history
- [x] **Pagination support** — limit and offset parameters (standard REST pattern)
- [x] **Bearer Token authentication** — Reuse existing auth mechanism
- [x] **7-day log cleanup** — Time-based deletion of old logs (on startup or scheduled)

### Add After Validation (v0.6.x)

Features to add once core is working.

- [ ] **Filter by status** — Query parameter `?status=success|failed`
- [ ] **Filter by time range** — Query parameters `?from=<timestamp>&to=<timestamp>`
- [ ] **Log file rotation** — Rotate logs daily or by size to prevent single large file
- [ ] **Configurable retention** — Allow customization of 7-day default

### Future Consideration (v2+)

Features to defer until product-market fit is established.

- [ ] **Full-text search** — Search log contents (requires indexing)
- [ ] **Log export formats** — CSV, Excel, PDF export
- [ ] **Multi-file log storage** — Partition logs by day for faster queries
- [ ] **Log compression** — Compress old logs instead of deleting
- [ ] **Log analytics** — Statistics on update success rates, duration trends

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Update execution logging | HIGH | MEDIUM | P1 |
| JSON Lines file persistence | HIGH | LOW | P1 |
| GET /api/v1/update-logs | HIGH | LOW | P1 |
| Bearer Token authentication | HIGH | LOW | P1 (reuse existing) |
| Request ID tracking | HIGH | LOW | P1 |
| Pagination (limit/offset) | MEDIUM | LOW | P1 |
| 7-day log cleanup | HIGH | MEDIUM | P1 |
| Instance-level detail | HIGH | MEDIUM | P1 |
| Full output capture | MEDIUM | MEDIUM | P2 |
| Filter by status | MEDIUM | LOW | P2 |
| Filter by time range | MEDIUM | LOW | P2 |
| Log file rotation | LOW | MEDIUM | P2 |
| Configurable retention | LOW | LOW | P3 |
| Full-text search | LOW | HIGH | P3 |
| Log export formats | LOW | MEDIUM | P3 |
| Multi-file log storage | LOW | MEDIUM | P3 |
| Log compression | LOW | MEDIUM | P3 |
| Log analytics | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for v0.6 launch
- P2: Should have, add when possible (v0.6.x)
- P3: Nice to have, future consideration (v2+)

## What to Record in Update Logs

Based on research and project requirements, each update log entry should contain:

### Core Metadata
```json
{
  "update_id": "uuid-v4",
  "triggered_at": "2026-03-26T10:30:00.123Z",
  "completed_at": "2026-03-26T10:30:45.456Z",
  "duration_ms": 45333,
  "trigger_source": "HTTP API",
  "status": "success|partial|failed",
  "authenticated_as": "bearer-token-identifier"
}
```

### Instance-Level Results
```json
{
  "instances": [
    {
      "name": "gateway",
      "port": 18790,
      "status": "success|failed",
      "error_message": "string or null",
      "started_at": "2026-03-26T10:30:10.000Z",
      "completed_at": "2026-03-26T10:30:25.000Z",
      "duration_ms": 15000,
      "stdout_ref": "ring-buffer-id or null",
      "stderr_ref": "ring-buffer-id or null"
    }
  ]
}
```

### Aggregated Summary
```json
{
  "summary": {
    "total_instances": 2,
    "successful": 1,
    "failed": 1,
    "skipped": 0
  }
}
```

## Query API Design

### Endpoint
```
GET /api/v1/update-logs?limit=20&offset=0&status=failed
```

### Request Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 20 | Max number of records to return |
| `offset` | int | 0 | Number of records to skip |
| `status` | string | "all" | Filter by status: "all", "success", "failed", "partial" |
| `from` | timestamp | null | Filter logs after this timestamp (ISO 8601) |
| `to` | timestamp | null | Filter logs before this timestamp (ISO 8601) |

### Response Format
```json
{
  "logs": [
    {
      "update_id": "uuid-v4",
      "triggered_at": "2026-03-26T10:30:00.123Z",
      "completed_at": "2026-03-26T10:30:45.456Z",
      "duration_ms": 45333,
      "status": "partial",
      "summary": {
        "total_instances": 2,
        "successful": 1,
        "failed": 1,
        "skipped": 0
      },
      "instances": [ /* ... */ ]
    }
  ],
  "pagination": {
    "total": 150,
    "limit": 20,
    "offset": 0,
    "has_more": true
  }
}
```

### Authentication
- Reuse existing Bearer Token authentication from Phase 28
- Same token used for `/api/v1/trigger-update` and `/api/v1/update-logs`
- Return 401 Unauthorized if token invalid or missing

## Log Cleanup Strategy

### Retention Policy
- **Default:** 7 days (as specified in PROJECT.md)
- **Implementation:** Time-based deletion (logs older than `now() - 7 days`)
- **Trigger:** Run on application startup + scheduled daily cleanup

### Cleanup Approaches
Based on research, three main strategies:

1. **Simple file scan (RECOMMENDED for v0.6)**
   - Scan JSON Lines file on startup
   - Parse each line's timestamp
   - Delete entries older than retention period
   - Rewrite file without old entries
   - Simple, no external dependencies

2. **Daily partition files (v2 option)**
   - Create separate log file per day: `update-logs-2026-03-26.jsonl`
   - Delete entire files older than 7 days
   - Faster cleanup (no parsing required)
   - Query must read from multiple files

3. **Database-backed (out of scope)**
   - Store logs in SQLite/PostgreSQL
   - Use SQL `DELETE WHERE created_at < now() - interval '7 days'`
   - Overkill for this use case

### Recommended Approach
For v0.6: **Simple file scan on startup**
- Load entire file into memory
- Parse each JSON line
- Filter out entries older than retention
- Write filtered entries back to file
- Log cleanup statistics (entries removed, file size before/after)

## Competitor Feature Analysis

| Feature | Typical Approach | Our Approach | Rationale |
|---------|------------------|--------------|-----------|
| Log format | JSON Lines | JSON Lines | Industry standard, easy to parse, append-friendly |
| Retention | Configurable (7-90 days) | 7 days fixed | Simple default, configurable in v0.6.x |
| Query API | REST with pagination | REST with pagination | Standard pattern, easy to implement |
| Authentication | API keys or OAuth | Bearer Token | Reuse existing auth, simple and standard |
| Cleanup | Background job or cron | Startup + scheduled | Simple implementation, no external dependencies |
| Storage | Database or files | JSON Lines file | No DB dependency, good for single-tenant |
| Full output | Link to object storage | Reference ring buffer | Reuse v0.4 log buffer, no separate storage |

## Sources

- [Best Practices and Key Components of Log Management in 2026](https://logmanager.com/blog/log-management/log-management-best-practices/) — Industry best practices for log management
- [11 Efficient Log Management Best Practices to Know in 2026](https://www.strongdm.com/blog/log-management-best-practices) — Strategy formulation, retention policies
- [How to Build Request ID Propagation](https://oneuptime.com/blog/post/2026-01-30-request-id-propagation/view) — Request ID tracking for debugging
- [Log Retention: Policies, Best Practices & Tools](https://last9.io/blog/log-retention/) — Retention strategies and compliance
- [Time-based Retention Strategies in Postgres](https://blog.sequinstream.com/time-based-retention-strategies-in-postgres/) — Cleanup implementation patterns (pg_cron, partitions)
- [Audit Logging Best Practices](https://www.sonarsource.com/resources/library/library/audit-logging/) — What to record in audit logs
- [Log Format Standards: JSON, XML, and Key-Value Explained](https://last9.io/blog/log-format/) — JSON Lines format advantages
- [REST API Pagination Best Practices](https://stackoverflow.com/questions/53288275) — No formal RFC standard, but limit/offset is de facto pattern
- [Golang Log Management Libraries](https://github.com/olegiv/go-logger) — lumberjack for log rotation
- [Kafka Retention: 7-day default](https://www.automq.com/blog/comprehensive-guide-kafka-retention-best-practices) — Industry standard retention period

---

*Feature research for: update log recording and query system*
*Researched: 2026-03-26*
