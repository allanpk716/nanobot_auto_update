# Pitfalls Research: Update Log Recording and Query System

**Domain:** Adding log recording and query features to existing Go HTTP API
**Researched:** 2026-03-26
**Confidence:** HIGH

## Critical Pitfalls

### Pitfall 1: Concurrent Write Conflicts (Multiple Goroutines Writing to Same File)

**What goes wrong:**
Multiple HTTP API requests trigger updates concurrently, causing multiple goroutines to write to the same JSON Lines log file simultaneously. This leads to:
- Interleaved/corrupted JSON lines (partial writes mixing together)
- Lost log entries (race conditions overwriting data)
- Invalid JSON Lines format (readers can't parse the file)

**Why it happens:**
Developers assume file operations are atomic or forget that HTTP handlers run concurrently. Opening a file with `os.OpenFile(os.O_APPEND)` does NOT guarantee atomic appends across multiple goroutines - the append operation is still subject to race conditions.

**How to avoid:**
1. **Use sync.Mutex for file write serialization** (preferred for this use case):
   ```go
   type LogWriter struct {
       mu   sync.Mutex
       file *os.File
   }

   func (lw *LogWriter) WriteLog(entry LogEntry) error {
       lw.mu.Lock()
       defer lw.mu.Unlock()

       data, err := json.Marshal(entry)
       if err != nil {
           return err
       }
       data = append(data, '\n')
       _, err = lw.file.Write(data)
       return err
   }
   ```
2. **Alternative: Single goroutine + channel pattern** (if you need more complex coordination):
   ```go
   type LogWriter struct {
       logCh chan LogEntry
   }
   // Single goroutine processes all writes sequentially
   ```

**Warning signs:**
- JSON Lines file has malformed entries when viewed in text editor
- Query API returns JSON parse errors
- Log entries disappear after concurrent trigger-update calls
- Tests pass in isolation but fail under load testing

**Phase to address:** Recording Phase (Phase 1 - Log Structure)

---

### Pitfall 2: File Descriptor Leaks (Not Closing Files Properly)

**What goes wrong:**
File handles are opened but never closed, eventually hitting the OS file descriptor limit (typically 1024 on Windows). The application crashes with "too many open files" error after running for days/weeks.

**Why it happens:**
1. Forgetting to call `file.Close()` in error paths
2. Not using `defer` for cleanup
3. Only closing files in success cases, not in error branches

**How to avoid:**
**Always use defer for file.Close() immediately after opening:**
```go
func writeLog(filepath string, entry LogEntry) error {
    file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    defer file.Close()  // <-- Always executed, even on errors

    data, err := json.Marshal(entry)
    if err != nil {
        return err  // file.Close() called automatically
    }

    _, err = file.Write(append(data, '\n'))
    return err  // file.Close() called automatically
}
```

**Warning signs:**
- Application runs fine for hours/days then suddenly fails
- `lsof` or Process Explorer shows many open file handles
- "too many open files" error in logs
- Memory usage slowly increases over time

**Phase to address:** Recording Phase (Phase 1 - Log Structure)

---

### Pitfall 3: Disk Space Exhaustion (Logs Growing Unbounded)

**What goes wrong:**
JSON Lines log files accumulate indefinitely, eventually filling the entire disk. This causes:
- Application crashes when trying to write new logs
- System becomes unstable (Windows can't function with 0 bytes free)
- Query performance degrades (scanning huge files)

**Why it happens:**
1. No retention policy implemented (log files never deleted)
2. Underestimated log volume (each update generates large JSON with full stdout/stderr)
3. Cleanup logic never triggered or buggy

**How to avoid:**
1. **Implement time-based retention (7 days as per requirements):**
   ```go
   func cleanupOldLogs(logDir string, retentionDays int) error {
       cutoff := time.Now().AddDate(0, 0, -retentionDays)

       entries, err := os.ReadDir(logDir)
       if err != nil {
           return err
       }

       for _, entry := range entries {
           info, err := entry.Info()
           if err != nil {
               continue
           }

           if info.ModTime().Before(cutoff) {
               os.Remove(filepath.Join(logDir, entry.Name()))
           }
       }
       return nil
   }
   ```
2. **Schedule cleanup on startup + periodic intervals:**
   ```go
   // Run on startup
   cleanupOldLogs(logDir, 7)

   // Run daily
   go func() {
       ticker := time.NewTicker(24 * time.Hour)
       for range ticker.C {
           cleanupOldLogs(logDir, 7)
       }
   }()
   ```

**Warning signs:**
- Disk usage alerts from monitoring
- Query API response times increasing
- Log file size grows to gigabytes
- Application fails to write with "no space left on device"

**Phase to address:** Storage Phase (Phase 2 - File Persistence)

---

### Pitfall 4: Query Performance Issues (Scanning Large Files)

**What goes wrong:**
Reading large JSON Lines files (thousands of entries) with naive approaches causes:
- High memory usage (loading entire file into memory)
- Slow API response times (seconds to return paginated results)
- Timeouts on HTTP requests

**Why it happens:**
1. Using `os.ReadFile()` to load entire file before pagination
2. Not using streaming/buffered I/O
3. Parsing all JSON lines even when only returning N entries
4. Reverse pagination requires reading entire file

**How to avoid:**
1. **Use bufio.Scanner for memory-efficient line reading:**
   ```go
   func queryLogs(filepath string, limit, offset int) ([]LogEntry, error) {
       file, err := os.Open(filepath)
       if err != nil {
           return nil, err
       }
       defer file.Close()

       scanner := bufio.NewScanner(file)
       // Increase buffer for long JSON lines
       buf := make([]byte, 0, 64*1024)
       scanner.Buffer(buf, 1024*1024) // 1MB max line length

       var results []LogEntry
       lineNum := 0

       for scanner.Scan() {
           lineNum++

           // Skip offset lines
           if lineNum <= offset {
               continue
           }

           // Parse and collect up to limit
           if lineNum > offset && lineNum <= offset+limit {
               var entry LogEntry
               if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
                   log.Printf("Skipping malformed line %d: %v", lineNum, err)
                   continue
               }
               results = append(results, entry)
           }

           // Stop early if we have enough
           if len(results) >= limit {
               break
           }
       }

       return results, scanner.Err()
   }
   ```
2. **For reverse chronological order (newest first):**
   - Read entire file and reverse in memory (acceptable for 7 days of logs)
   - OR: Maintain separate index file with line offsets
   - OR: Write new entries at beginning of file (more complex)

**Warning signs:**
- Query API takes >1 second to respond
- Memory spikes during query operations
- HTTP client timeouts on log queries
- Application becomes unresponsive during queries

**Phase to address:** Query Phase (Phase 3 - Query API)

---

### Pitfall 5: JSON Lines Corruption (Partial Writes, Concurrent Access)

**What goes wrong:**
JSON Lines file contains malformed entries:
- Incomplete JSON objects (process crash during write)
- Mixed partial writes from concurrent access
- Missing newline between entries

**Why it happens:**
1. Application crashes mid-write (power loss, panic, os.Exit)
2. Concurrent writes without proper synchronization (see Pitfall 1)
3. Not flushing/syncing data to disk properly

**How to avoid:**
1. **Use sync.Mutex for concurrent access** (covered in Pitfall 1)
2. **Handle partial writes gracefully on read:**
   ```go
   for scanner.Scan() {
       var entry LogEntry
       if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
           // Log and skip malformed lines instead of failing
           log.Printf("Skipping corrupted line %d: %v", lineNum, err)
           continue
       }
       results = append(results, entry)
   }
   ```
3. **For critical durability, use atomic rename pattern:**
   ```go
   // NOT recommended for append-heavy JSONL logs (performance overhead)
   // But useful for critical single-file writes:
   // 1. Write to temp file
   // 2. file.Sync() to flush to disk
   // 3. os.Rename(temp, final)
   ```

**Warning signs:**
- `json.Unmarshal` errors when reading log files
- Query API returns 500 errors on specific log entries
- Log file viewed in editor shows incomplete JSON objects

**Phase to address:** Recording Phase (Phase 1 - Log Structure) + Query Phase (Phase 3 - Query API)

---

### Pitfall 6: Time Zone Handling (Timestamps in Logs)

**What goes wrong:**
Timestamps in log entries are ambiguous or inconsistent:
- Logs show different time zones depending on server location
- Daylight Saving Time causes duplicate timestamps (1:00-1:59 AM happens twice)
- Queries by timestamp return wrong results
- Displaying logs to users shows confusing times

**Why it happens:**
1. Using `time.Now()` without specifying timezone
2. Storing local time instead of UTC
3. DST transitions create ambiguity in local time
4. Different servers in different timezones

**How to avoid:**
1. **Always store timestamps in UTC:**
   ```go
   type LogEntry struct {
       ID        string    `json:"id"`
       StartTime time.Time `json:"start_time"`  // Always UTC
       EndTime   time.Time `json:"end_time"`    // Always UTC
       // ...
   }

   // Always use time.Now().UTC() or just time.Now() (it's UTC internally)
   entry := LogEntry{
       StartTime: time.Now(), // Go stores time.Time in UTC by default
   }
   ```
2. **Format timestamps with timezone info (RFC3339):**
   ```go
   // JSON marshaling will include timezone
   timestamp := time.Now().Format(time.RFC3339) // "2026-03-26T15:04:05Z07:00"
   ```
3. **Convert to local time only for display:**
   ```go
   func displayTime(utcTime time.Time, userTimezone string) string {
       loc, _ := time.LoadLocation(userTimezone)
       return utcTime.In(loc).Format("2006-01-02 15:04:05 MST")
   }
   ```

**Warning signs:**
- Log timestamps jump by 1 hour after DST change
- Same timestamp appears twice in logs (DST fall-back)
- Queries by time range miss entries
- Users report "logs show wrong time"

**Phase to address:** Recording Phase (Phase 1 - Log Structure)

---

### Pitfall 7: Pagination Edge Cases (Offset Beyond File Length)

**What goes wrong:**
Query API with invalid pagination parameters causes:
- Panic when offset > number of lines (index out of bounds)
- Confusing error messages
- HTTP 500 instead of proper error handling

**Why it happens:**
1. Not validating offset/limit parameters
2. Assuming file has enough lines without checking
3. Client sends invalid pagination (offset=-1, limit=0)

**How to avoid:**
1. **Validate and clamp pagination parameters:**
   ```go
   func queryLogs(filepath string, limit, offset int) ([]LogEntry, int, error) {
       // Validate inputs
       if limit < 0 {
           limit = 10 // Default
       }
       if limit > 100 {
           limit = 100 // Max limit
       }
       if offset < 0 {
           offset = 0
       }

       // ... read file ...

       // Handle case where offset > total lines
       if offset >= totalLines {
           return []LogEntry{}, totalLines, nil // Return empty, not error
       }

       return results, totalLines, nil
   }
   ```
2. **Return total count for client pagination:**
   ```json
   {
     "logs": [...],
     "pagination": {
       "total": 45,
       "offset": 30,
       "limit": 10,
       "has_more": true
     }
   }
   ```

**Warning signs:**
- API returns 500 errors on pagination
- Client pagination shows negative page numbers
- Empty results returned as errors instead of valid empty arrays

**Phase to address:** Query Phase (Phase 3 - Query API)

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| `os.ReadFile()` entire file before pagination | Faster to implement | OOM on large files, slow queries | Never for query API |
| Skip error handling for JSON parse errors | Simpler code | Silent data loss, hard to debug corrupted logs | Never - always log parse errors |
| No file synchronization (`file.Sync()`) | Faster writes | Data loss on crash/power failure | Acceptable for non-critical logs |
| Single monolithic log file | Simpler implementation | No parallel reads, single point of failure | MVP only - split by date in production |
| No pagination validation | Less code | API crashes on invalid inputs, poor DX | Never - validate always |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| **trigger-update API** | Write log entry before update completes | Write log entry AFTER update completes with final status |
| **trigger-update concurrent calls** | Assume only one request at a time | Use `sync.Atomic.Bool` to prevent concurrent updates (already implemented in v0.5) |
| **Existing ring buffer** | Try to reuse ring buffer for persistent logs | Keep ring buffer for real-time SSE, separate JSONL for persistent history |
| **Pushover notifications** | Log notification failures as log write failures | Separate concerns: notification failures != log write failures |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| **Loading entire file for pagination** | Memory spikes, slow queries, timeouts | Use `bufio.Scanner` with early termination | >1MB log files |
| **No early termination in pagination** | Scanning entire file even for first page | Break loop when `len(results) >= limit` | >100 log entries |
| **JSON marshaling in hot path** | High CPU usage, slow writes | Pre-allocate buffers, pool `json.Marshal` | >100 writes/sec |
| **Scanning file twice (count + query)** | 2x I/O, slower queries | Count during single pass | Any file size |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| **No authentication on query API** | Anyone can view update history | Use same Bearer Token auth as trigger-update (already in requirements) |
| **Logging sensitive data** | Credentials/tokens leaked in log file | Sanitize stdout/stderr before logging, filter sensitive patterns |
| **Unbounded log file growth** | Disk DoS, system crash | Implement 7-day retention + file size limits |
| **Path traversal in log file paths** | Read/write arbitrary files | Validate log directory path, use `filepath.Base()` for filenames |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| **No total count in pagination** | Users can't know total pages | Return `total` count in pagination metadata |
| **Reverse chronological order missing** | Users see oldest entries first | Return newest entries first (most relevant) |
| **No timestamp in user's timezone** | Users confused by UTC times | Display in local timezone on frontend |
| **Malformed JSON errors not handled** | API returns 500, users confused | Skip malformed entries, return partial results + warning |

## "Looks Done But Isn't" Checklist

- [ ] **Log Recording:** Often missing concurrent write protection — verify with load testing (10 concurrent trigger-update calls)
- [ ] **File Persistence:** Often missing `defer file.Close()` — verify with lsof/Process Explorer for file descriptor leaks
- [ ] **Disk Cleanup:** Often not scheduled regularly — verify cleanup runs on startup + daily
- [ ] **Query Performance:** Often loads entire file — verify memory usage with 1000+ log entries
- [ ] **Timezone Handling:** Often uses local time — verify timestamps are UTC in JSON
- [ ] **Pagination:** Often crashes on offset > total — verify with `offset=999999` returns empty, not error
- [ ] **Error Handling:** Often fails on malformed JSON lines — verify query API skips bad lines and returns partial results

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| **Concurrent write corruption** | MEDIUM | Stop application → backup file → manually repair JSON lines → add mutex protection → redeploy |
| **File descriptor leak** | LOW | Restart application → add `defer file.Close()` |
| **Disk space exhaustion** | HIGH | Stop application → delete old log files manually → implement retention policy → monitor disk usage |
| **Corrupted JSON Lines** | MEDIUM | Parse file line-by-line → skip malformed entries → write clean file → add parse error logging |
| **Wrong timezone in logs** | MEDIUM | Write migration script to parse + convert timestamps → update all entries to UTC |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| **Concurrent Write Conflicts** | Phase 1 (Recording) - Log Structure | Load test: 10 concurrent trigger-update calls, verify no corrupted lines |
| **File Descriptor Leaks** | Phase 1 (Recording) - Log Structure | Integration test: query logs 1000 times, verify file descriptors released |
| **Disk Space Exhaustion** | Phase 2 (Storage) - File Persistence | Unit test: mock old files, verify cleanup deleted them |
| **Query Performance** | Phase 3 (Query) - Query API | Performance test: query with 1000+ entries, verify <500ms response |
| **JSON Lines Corruption** | Phase 1 (Recording) + Phase 3 (Query) | Integration test: write malformed line manually, verify query skips it |
| **Time Zone Handling** | Phase 1 (Recording) - Log Structure | Unit test: verify stored timestamp is UTC, display conversion works |
| **Pagination Edge Cases** | Phase 3 (Query) - Query API | Unit test: offset=999999 returns empty array, not error |

## Sources

- **Concurrent File Writing:**
  - [Concurrent writing to a file - Stack Overflow](https://stackoverflow.com/questions/29981050/concurrent-writing-to-a-file)
  - [Calling file.Write() concurrently - Google Groups](https://groups.google.com/g/golang-nuts/c/-VbfWjGIRLA)

- **File Descriptor Leaks:**
  - [File descriptors are leaking on web server - Go Nuts Google Groups](https://groups.google.com/g/golang-nuts/c/JFhGwh1q9xU)
  - [net/http file descriptor leak - GitHub golang/go #46267](https://github.com/golang/go/issues/46267)
  - [HTTP Resource Leak Mysteries in Go - Coder Blog](https://coder.com/blog/go-leak-mysteries)

- **Log Rotation & Disk Space:**
  - [Logs rotation using Golang slog package - Medium](https://medium.com/@piusalfred/logs-rotation-using-golang-slog-package-9579621c7ed9)
  - [Implementing Log File Rotation in Go - Dev.to](https://dev.to/leapcell/implementing-log-file-rotation-in-go-insights-from-logrus-zap-and-slog-5b9o)
  - [Docker container logs taking all my disk space - Stack Overflow](https://stackoverflow.com/questions/31829587/docker-container-logs-taking-all-my-disk-space)

- **Time Zone Handling:**
  - [Deep Dive into Time in Go: DST & Production Pitfalls - Medium](https://medium.com/@engineeringvault/deep-dive-into-time-in-go-functions-internals-dst-production-pitfalls-a5dbde118651)
  - [Important Considerations When Using Go's Time Package - dev.to](https://dev.to/rezmoss/important-considerations-when-using-gos-time-package-910-3aim)

- **Pagination Performance:**
  - [How To Do Pagination in Postgres with Golang - Medium](https://medium.easyread.co/how-to-do-pagination-in-postgres-with-golang-in-4-common-ways-12365b9fb528)
  - [Offset Pagination vs Cursor Pagination - Stack Overflow](https://stackoverflow.com/questions/55744926/offset-pagination-vs-cursor-pagination)

- **Atomic File Writes:**
  - [Google renameio Package - GitHub](https://github.com/google/renameio)
  - [Go Proposal: Atomic File Creation - GitHub #22397](https://github.com/golang/go/issues/22397)

- **JSON Lines Reading:**
  - [How to read a text file line-by-line in Go - Stack Overflow](https://stackoverflow.com/questions/21124327/how-to-read-a-text-file-line-by-line-in-go-when-some-lines-are-long-enough-to-ca)

- **Request Deduplication:**
  - [How to Prevent Duplicate API Requests with Deduplication in Go - OneUptime](https://oneuptime.com/blog/post/2026-01-25-prevent-duplicate-api-requests-deduplication-go/view)
  - [SingleFlight: Smart Request Deduplication - DEV Community](https://dev.to/serifcolakel/singleflight-smart-request-deduplication-33og)

- **Go Concurrency Best Practices:**
  - [Go Wiki: Use a sync.Mutex or a channel? - go.dev](https://go.dev/wiki/MutexOrChannel)
  - [sync or channel? The Right Choice - Dev.to](https://dev.to/leapcell/sync-or-channel-the-right-choice-for-go-synchronization-2m7i)

---
*Pitfalls research for: Update Log Recording and Query System*
*Researched: 2026-03-26*
