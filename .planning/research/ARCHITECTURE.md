# Architecture Research: Update Log Recording and Query System

**Domain:** HTTP API integration for update log persistence and query
**Researched:** 2026-03-26
**Confidence:** HIGH

## Existing Architecture Overview

### Current System Structure

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP API Layer                           │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │ trigger-     │  │ SSE Handler  │  │ Help Handler │          │
│  │ update       │  │              │  │              │          │
│  │ (w/ Auth)    │  │              │  │ (no auth)    │          │
│  └──────┬───────┘  └──────────────┘  └──────────────┘          │
│         │                                                       │
│  ┌──────▼──────────────────────────────────────────────┐       │
│  │           AuthMiddleware (Bearer Token)             │       │
│  └─────────────────────────────────────────────────────┘       │
├─────────────────────────────────────────────────────────────────┤
│                    Instance Management                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────┐      │
│  │          InstanceManager (concurrent control)        │      │
│  │  - TriggerUpdate() → UpdateAll()                     │      │
│  │  - atomic.Bool isUpdating                            │      │
│  └──────┬───────────────────────────────────────┬──────┘      │
│         │                                       │              │
│  ┌──────▼──────────┐                   ┌───────▼──────────┐   │
│  │ InstanceLifecycle│                   │   Updater (UV)   │   │
│  │  - StopForUpdate │                   │  - Update()      │   │
│  │  - StartAfter... │                   └──────────────────┘   │
│  │  - LogBuffer     │                                          │
│  └──────────────────┘                                          │
├─────────────────────────────────────────────────────────────────┤
│                      Data Persistence                           │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────────┐  ┌──────────────────────────────────┐    │
│  │  LogBuffer (mem) │  │  File Logger (daily rotation)    │    │
│  │  - 5000 lines    │  │  - ./logs/app-YYYY-MM-DD.log     │    │
│  │  - stdout/stderr │  │  - 7 day retention               │    │
│  └──────────────────┘  └──────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

### Current Component Responsibilities

| Component | Responsibility | Integration Point |
|-----------|----------------|-------------------|
| TriggerHandler | HTTP endpoint for update triggering | `internal/api/trigger.go` |
| InstanceManager | Orchestrates stop→update→start flow | `internal/instance/manager.go` |
| InstanceLifecycle | Per-instance lifecycle management | `internal/instance/lifecycle.go` |
| LogBuffer | In-memory circular buffer (5000 lines) | `internal/logbuffer/buffer.go` |
| Updater | UV package manager update logic | `internal/updater/updater.go` |
| File Logger | Application logs with rotation | `internal/logging/logging.go` |
| AuthMiddleware | Bearer token validation | `internal/api/auth.go` |

## Proposed Architecture for Update Log Recording

### Integration Points

**Primary Integration: TriggerHandler**

The update log recording system integrates at the `TriggerHandler.Handle()` level:

```go
// File: internal/api/trigger.go
func (h *TriggerHandler) Handle(w http.ResponseWriter, r *http.Request) {
    // 1. Existing: Validate method
    // 2. Existing: Create context with timeout

    // 3. NEW: Create UpdateLogRecorder
    recorder := NewUpdateLogRecorder(h.logger)

    // 4. Existing: Execute update
    result, err := h.instanceManager.TriggerUpdate(ctx)

    // 5. NEW: Record update result
    logEntry := recorder.Record(ctx, result, err, r.Header.Get("X-Trigger-Source"))

    // 6. NEW: Persist log entry
    if persistErr := h.logStore.Append(logEntry); persistErr != nil {
        h.logger.Error("Failed to persist update log", "error", persistErr)
    }

    // 7. Existing: Return JSON response
    // ...
}
```

**Rationale:**
- TriggerHandler already has access to UpdateResult and errors
- Context timeout management already in place
- Authentication already validated by middleware
- Single integration point minimizes changes to existing code

### New Component Structure

```
internal/
├── updatelog/               # NEW: Update log domain
│   ├── types.go             # UpdateLogEntry, UpdateLogResult structures
│   ├── recorder.go          # UpdateLogRecorder (business logic)
│   ├── store.go             # File-based log persistence (JSON Lines)
│   ├── store_test.go        # Unit tests for store
│   ├── cleanup.go           # 7-day retention cleanup logic
│   └── cleanup_test.go      # Unit tests for cleanup
├── api/
│   ├── trigger.go           # MODIFY: Integrate recorder
│   ├── updatelog_handler.go # NEW: Query API handler
│   └── auth.go              # REUSE: Existing middleware
└── config/
    └── config.go            # MODIFY: Add update log config
```

## Data Flow

### Update Log Recording Flow

```
HTTP POST /api/v1/trigger-update
    ↓
AuthMiddleware validates Bearer token
    ↓
TriggerHandler.Handle()
    ↓
    ├─→ Create UpdateLogRecorder (new)
    ├─→ InstanceManager.TriggerUpdate()
    │       ↓
    │   Stop all instances → UV Update → Start all instances
    │       ↓
    │   Return UpdateResult + error
    ├─→ recorder.Record(result, error) → UpdateLogEntry (new)
    │       ↓
    │   Extract instance logs from LogBuffer (new)
    │       ↓
    │   Build complete log entry
    ├─→ UpdateLogStore.Append(entry) → File write (new)
    │       ↓
    │   Write JSON Lines to ./logs/update-logs.jsonl
    │       ↓
    │   Trigger cleanup if needed
    └─→ Return JSON response (existing)
```

### Update Log Query Flow

```
HTTP GET /api/v1/update-logs?limit=10&offset=0
    ↓
AuthMiddleware validates Bearer token
    ↓
UpdateLogHandler.Handle()
    ↓
    ├─→ Parse limit/offset query params
    ├─→ UpdateLogStore.Query(limit, offset)
    │       ↓
    │   Read ./logs/update-logs.jsonl
    │       ↓
    │   Parse JSON Lines
    │       ↓
    │   Apply pagination
    │       ↓
    │   Return []UpdateLogEntry
    └─→ Return JSON response (200 OK)
```

### Data Transformation Flow

```
UpdateResult (instance package)
    ↓
UpdateLogRecorder.Record()
    ├─ Extract instance names (Stopped, Started)
    ├─ Extract errors (StopFailed, StartFailed)
    ├─ Capture instance logs from LogBuffer.GetHistory()
    ├─ Build InstanceUpdateResult for each instance
    └─ Build UpdateLogEntry
        ↓
UpdateLogStore.Append()
    ├─ Serialize to JSON
    ├─ Append to JSON Lines file
    └─ Trigger cleanup if file size threshold exceeded
```

## Architectural Patterns

### Pattern 1: Domain-Driven Package Structure

**What:** Separate package `internal/updatelog` for all update log concerns

**When to use:** When adding a new bounded context to existing application

**Trade-offs:**
- **Pros:** Clear separation of concerns, testable in isolation, reusable
- **Cons:** Additional package, more files to maintain

**Example:**
```go
// internal/updatelog/recorder.go
package updatelog

type UpdateLogRecorder struct {
    logger *slog.Logger
}

func (r *UpdateLogRecorder) Record(
    ctx context.Context,
    result *instance.UpdateResult,
    updateErr error,
    triggerSource string,
    instanceLogs map[string][]logbuffer.LogEntry,
) *UpdateLogEntry {
    // Transform UpdateResult to UpdateLogEntry
    entry := &UpdateLogEntry{
        ID:              generateID(),
        Timestamp:       time.Now(),
        TriggerSource:   triggerSource,
        Success:         updateErr == nil && !result.HasErrors(),
    }

    // Build instance results with logs
    for _, name := range result.Stopped {
        entry.Instances = append(entry.Instances, InstanceUpdateResult{
            InstanceName: name,
            Status:       "stopped_success",
            Logs:         instanceLogs[name],
        })
    }
    // ... similar for other results

    return entry
}
```

### Pattern 2: File-Based JSON Lines Storage

**What:** Append-only log file with JSON Lines format

**When to use:** When query requirements are simple (recent N entries) and append performance is critical

**Trade-offs:**
- **Pros:** Simple implementation, efficient append, no external dependencies
- **Cons:** O(n) read for pagination, requires full file scan for queries

**Example:**
```go
// internal/updatelog/store.go
type UpdateLogStore struct {
    filePath string
    logger   *slog.Logger
}

func (s *UpdateLogStore) Append(entry *UpdateLogEntry) error {
    // Open file in append mode
    f, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    defer f.Close()

    // Write JSON line
    encoder := json.NewEncoder(f)
    return encoder.Encode(entry)
}

func (s *UpdateLogStore) Query(limit, offset int) ([]*UpdateLogEntry, error) {
    // Read file line by line
    f, err := os.Open(s.filePath)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var entries []*UpdateLogEntry
    scanner := bufio.NewScanner(f)
    lineNum := 0

    for scanner.Scan() {
        lineNum++
        if lineNum <= offset {
            continue
        }
        if lineNum > offset+limit {
            break
        }

        var entry UpdateLogEntry
        if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
            s.logger.Warn("Failed to parse log entry", "line", lineNum, "error", err)
            continue
        }
        entries = append(entries, &entry)
    }

    return entries, scanner.Err()
}
```

### Pattern 3: Lazy Log Capture

**What:** Capture instance logs only when recording an update log (not during normal operation)

**When to use:** When log capture is expensive and only needed for specific operations

**Trade-offs:**
- **Pros:** No performance overhead during normal instance operation
- **Cons:** Requires access to LogBuffer at recording time

**Example:**
```go
// In TriggerHandler.Handle()
func (h *TriggerHandler) Handle(w http.ResponseWriter, r *http.Request) {
    // ... execute update ...

    // Capture instance logs AFTER update completes
    instanceLogs := make(map[string][]logbuffer.LogEntry)
    for _, name := range getAllInstanceNames(result) {
        buffer, err := h.instanceManager.GetLogBuffer(name)
        if err != nil {
            h.logger.Warn("Failed to get log buffer", "instance", name, "error", err)
            continue
        }
        instanceLogs[name] = buffer.GetHistory()
    }

    // Record with logs
    logEntry := recorder.Record(ctx, result, err, triggerSource, instanceLogs)
}
```

## Component Integration Details

### Integration Point 1: TriggerHandler Modification

**File:** `internal/api/trigger.go`

**Changes:**
1. Add `UpdateLogRecorder` and `UpdateLogStore` to TriggerHandler struct
2. Initialize in `NewTriggerHandler()`
3. Call `recorder.Record()` after `TriggerUpdate()` completes
4. Call `store.Append()` to persist log entry

**Why this integration point:**
- Minimal changes to existing code
- Access to complete UpdateResult
- Error context available
- HTTP request context (trigger source) available

### Integration Point 2: Server Registration

**File:** `internal/api/server.go`

**Changes:**
1. Create new `UpdateLogHandler` instance
2. Register new route: `GET /api/v1/update-logs`
3. Wrap with existing `AuthMiddleware` (reuse Bearer token validation)

**Why this integration point:**
- Consistent with existing route registration pattern
- Reuses existing authentication infrastructure
- Centralized route management

### Integration Point 3: Configuration Extension

**File:** `internal/config/config.go`

**Changes:**
1. Add `UpdateLogConfig` struct with retention settings
2. Add to main `Config` struct
3. Add validation for retention days (minimum 1, maximum 365)
4. Set default: 7 days retention

**Why this integration point:**
- Follows existing configuration pattern
- Allows future extensibility (log path, retention policy)
- Consistent with other feature configurations (API, Monitor)

## Scaling Considerations

| Scale | Log Volume | Architecture Adjustments |
|-------|------------|--------------------------|
| 0-100 updates/day | < 100 KB/day | Current architecture sufficient |
| 100-1000 updates/day | ~1 MB/day | Add index file for O(1) seeks |
| 1000+ updates/day | > 10 MB/day | Consider SQLite or database |

### First Bottleneck: File Read Performance

**What breaks first:** Query API response time degrades as log file grows (O(n) read)

**How to fix:**
1. Add in-memory cache of recent N log entries (e.g., 100 entries)
2. Serve queries from cache when possible
3. Fall back to file read only for historical queries

### Second Bottleneck: Disk Space

**What breaks next:** Log files consume too much disk space

**How to fix:**
1. Implement more aggressive cleanup (3-day retention)
2. Add compression for old log files
3. Add log rotation by file size (not just by date)

## Anti-Patterns to Avoid

### Anti-Pattern 1: Storing Logs in Memory Only

**What people do:** Keep update logs only in memory for fast access

**Why it's wrong:** Logs lost on application restart, defeats audit purpose

**Do this instead:** Persist to file immediately after each update, use memory only for optional caching

### Anti-Pattern 2: Capturing Logs During Instance Operation

**What people do:** Continuously capture logs to update log storage while instance is running

**Why it's wrong:** Massive performance overhead, LogBuffer already handles real-time logs

**Do this instead:** Capture logs only when recording update results (lazy capture pattern)

### Anti-Pattern 3: Complex Query Support

**What people do:** Add search, filtering, aggregation to log query API

**Why it's wrong:** Over-engineering for simple requirement (recent N updates), performance issues

**Do this instead:** Keep query simple (limit/offset only), accept O(n) read performance for reasonable file sizes

### Anti-Pattern 4: Separate Authentication for Query API

**What people do:** Create new authentication mechanism for update log query endpoint

**Why it's wrong:** Duplicate logic, inconsistent security model, maintenance burden

**Do this instead:** Reuse existing `AuthMiddleware` with same Bearer token

## Build Order Recommendation

Based on dependency analysis, recommended build order:

### Phase 1: Core Data Structures (no dependencies)
1. **Create `internal/updatelog/types.go`**
   - Define `UpdateLogEntry` struct
   - Define `InstanceUpdateResult` struct
   - Define JSON tags for serialization

2. **Create `internal/updatelog/recorder.go`**
   - Implement `UpdateLogRecorder.Record()` method
   - Transform `instance.UpdateResult` to `UpdateLogEntry`
   - Unit tests for transformation logic

### Phase 2: Persistence Layer (depends on Phase 1)
3. **Create `internal/updatelog/store.go`**
   - Implement `UpdateLogStore.Append()` method
   - Implement `UpdateLogStore.Query()` method
   - Handle JSON Lines file operations
   - Unit tests with temporary files

4. **Create `internal/updatelog/cleanup.go`**
   - Implement 7-day retention cleanup logic
   - Parse timestamp from log entries
   - Delete old entries
   - Unit tests for date calculations

### Phase 3: Integration (depends on Phase 1 & 2)
5. **Modify `internal/api/trigger.go`**
   - Add recorder and store to TriggerHandler
   - Integrate log recording after update completes
   - Capture instance logs from LogBuffer
   - Integration tests

6. **Modify `internal/config/config.go`**
   - Add UpdateLogConfig struct
   - Add validation
   - Set defaults

### Phase 4: Query API (depends on Phase 2)
7. **Create `internal/api/updatelog_handler.go`**
   - Implement query handler
   - Parse limit/offset parameters
   - Call store.Query()
   - Return JSON response
   - Unit tests

8. **Modify `internal/api/server.go`**
   - Register new route
   - Apply AuthMiddleware
   - Integration tests

### Rationale for Build Order

**Why recorder before persistence:**
- Recorder has no dependencies, can be tested in isolation
- Defines the data contract (UpdateLogEntry) that persistence uses
- Allows parallel development of transformation logic

**Why persistence before integration:**
- Integration tests require working persistence layer
- File operations are independent of HTTP layer
- Allows testing persistence with mock recorder

**Why integration before query API:**
- Query API depends on having data to query
- Integration tests validate end-to-end recording flow
- Ensures query API has realistic data to work with

## Testing Strategy

### Unit Tests (isolated, fast)
- **recorder_test.go:** Verify UpdateResult → UpdateLogEntry transformation
- **store_test.go:** Verify file append/read with temp files
- **cleanup_test.go:** Verify retention logic with mock timestamps

### Integration Tests (with dependencies)
- **trigger_test.go:** Verify log recording during trigger-update flow
- **updatelog_handler_test.go:** Verify query API with real store

### End-to-End Tests (full system)
- Start server → Trigger update → Query logs → Verify result

## Configuration Schema

```yaml
# config.yaml
update_log:
  enabled: true              # Enable/disable log recording
  retention_days: 7          # Keep logs for 7 days
  max_file_size_mb: 100      # Rotate file if exceeds 100MB
  log_path: "./logs/update-logs.jsonl"  # Log file path
```

## Error Handling Strategy

| Error Location | Handling | Response |
|----------------|----------|----------|
| Log recording fails | Log error, continue | Return success (update succeeded) |
| Log persistence fails | Log error, continue | Return success (non-critical) |
| Log cleanup fails | Log error, continue | Continue operation |
| Query file read fails | Log error, return error | Return 500 Internal Server Error |
| Invalid query params | Return error | Return 400 Bad Request |

**Rationale:**
- Update log recording is non-critical (audit trail, not core functionality)
- Failed logging should not block successful updates
- Query errors should be reported to user (expected data unavailable)

## Sources

- **Existing Code:** `internal/api/trigger.go` (Phase 28 implementation)
- **Existing Code:** `internal/instance/manager.go` (UpdateResult structure)
- **Existing Code:** `internal/logbuffer/buffer.go` (log capture mechanism)
- **Existing Code:** `internal/api/auth.go` (authentication middleware)
- **Existing Code:** `internal/logging/daily_rotate.go` (file rotation pattern)
- **PROJECT.md:** v0.6 milestone requirements (update log recording and query)

---

*Architecture research for: Update Log Recording and Query System*
*Researched: 2026-03-26*
