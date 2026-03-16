# Domain Pitfalls

**Domain:** Adding real-time log viewing to Go application (SSE streaming + process output capture)
**Researched:** 2026-03-16
**Confidence:** HIGH

## Critical Pitfalls

### Pitfall 1: SSE Connection Goroutine Leak

**What goes wrong:**
SSE handlers spawn goroutines to stream data to clients, but when clients disconnect (close browser, network issue), the server-side goroutine continues running indefinitely, waiting to write to a closed connection. This leads to memory leaks and exhausted file descriptors over time.

**Why it happens:**
- Developers forget to monitor `r.Context().Done()` channel
- SSE connections are long-lived, making leaks hard to detect during testing
- Network interruptions don't always trigger immediate errors

**How to avoid:**
```go
// CORRECT: Use Context to detect client disconnect
func handleSSE(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }

    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    // Channel to receive log lines
    logChan := make(chan string)
    defer close(logChan)

    for {
        select {
        case logLine := <-logChan:
            fmt.Fprintf(w, "data: %s\n\n", logLine)
            flusher.Flush()

        case <-r.Context().Done(): // CRITICAL: Detect client disconnect
            log.Println("Client disconnected")
            return // Exit handler, cleanup via defer
        }
    }
}
```

**Warning signs:**
- Memory usage steadily increases over days/weeks
- `runtime.NumGoroutine()` count keeps growing
- Eventually: "too many open files" error
- `netstat` shows many connections in CLOSE_WAIT state

**Phase to address:**
Phase 2 (SSE Endpoint) — Must implement during SSE handler creation, not as an afterthought

**Sources:**
- [Server Sent Events (SSE) Server implementation with Go - Dev.to](https://dev.to/mirzaakhena/server-sent-events-sse-server-implementation-with-go-4ck2)
- [How to determine that an SSE connection was closed? - Stack Overflow](https://stackoverflow.com/questions/27824948/how-to-determine-that-an-sse-connection-was-closed)

---

### Pitfall 2: Process stdout/stderr Pipe Deadlock

**What goes wrong:**
When capturing nanobot process output, `cmd.Wait()` hangs indefinitely even after the process exits. The stdout/stderr pipes remain open, preventing the parent process from completing cleanup.

**Why it happens:**
- Go's `os/exec` spawns goroutines to copy stdout/stderr from child process
- If pipe buffer fills (typically 64KB on Windows), child process blocks on write
- `cmd.Wait()` waits for these copy goroutines to finish, which are blocked reading from pipes
- Classic deadlock: parent waits for child → child waits for parent to read → parent can't read because it's waiting

**How to avoid:**
```go
// CORRECT: Use io.TeeReader or consume output concurrently
func captureProcessOutput(cmd *exec.Cmd) ([]byte, []byte, error) {
    var stdoutBuf, stderrBuf bytes.Buffer

    // Create pipes before Start()
    stdoutPipe, err := cmd.StdoutPipe()
    if err != nil {
        return nil, nil, err
    }
    stderrPipe, err := cmd.StderrPipe()
    if err != nil {
        return nil, nil, err
    }

    if err := cmd.Start(); err != nil {
        return nil, nil, err
    }

    // CRITICAL: Read from pipes concurrently with Wait()
    var wg sync.WaitGroup
    wg.Add(2)

    go func() {
        defer wg.Done()
        io.Copy(&stdoutBuf, stdoutPipe)
    }()

    go func() {
        defer wg.Done()
        io.Copy(&stderrBuf, stderrPipe)
    }()

    // Wait for process to exit
    err = cmd.Wait()

    // Wait for pipe readers to finish
    wg.Wait()

    return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
}
```

**Alternative approach (simpler):**
```go
// Use CombinedOutput or separate buffers without pipes
cmd.Stdout = &stdoutBuf
cmd.Stderr = &stderrBuf
err := cmd.Run() // Safe: doesn't use pipes
```

**Warning signs:**
- `cmd.Wait()` never returns even though process exited
- Stack trace shows goroutines blocked in `io.copyBuffer` or `os.(*File).Read`
- Process appears in task manager but parent hangs

**Phase to address:**
Phase 1 (Log Capture) — Must design correctly from the start; retrofitting is expensive

**Sources:**
- [os/exec: possible race handling stdout&stderr pipes #69060 - GitHub](https://github.com/golang/go/issues/69060)
- [The complete guide to Go net/http timeouts - Cloudflare](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/)
- [Deadlock when redirecting Windows cmd.exe I/O using pipes - Stack Overflow](https://stackoverflow.com/questions/79130976/deadlock-when-redirecting-windows-cmdexe-i-o-using-pipes)

---

### Pitfall 3: Ring Buffer Data Race / Corruption

**What goes wrong:**
When using a ring buffer to store recent log lines (5000 lines), concurrent writes from process capture goroutine and reads from SSE streaming goroutines cause data races. Symptoms include corrupted log lines, crashes, or readers seeing partially written data.

**Why it happens:**
- Ring buffer involves: write index, read index, count, and data array
- Without proper synchronization, writer updates indices while reader reads them
- Go's race detector may not catch all issues in ring buffer implementations
- Overwrite policy (when buffer full) is particularly racy

**How to avoid:**
```go
// CORRECT: Use proper synchronization
type RingBuffer struct {
    mu     sync.RWMutex // Use RWMutex for read-heavy workload
    data   []string
    head   int // Next write position
    tail   int // Oldest data position
    count  int // Number of items
    size   int
}

func (rb *RingBuffer) Write(line string) {
    rb.mu.Lock()
    defer rb.mu.Unlock()

    rb.data[rb.head] = line
    rb.head = (rb.head + 1) % rb.size

    if rb.count < rb.size {
        rb.count++
    } else {
        // Buffer full, advance tail
        rb.tail = (rb.tail + 1) % rb.size
    }
}

func (rb *RingBuffer) ReadAll() []string {
    rb.mu.RLock()
    defer rb.mu.RUnlock()

    result := make([]string, rb.count)
    for i := 0; i < rb.count; i++ {
        idx := (rb.tail + i) % rb.size
        result[i] = rb.data[idx]
    }
    return result
}
```

**Alternative: Use proven library**
```go
import "github.com/smallnest/ringbuffer"

rb := ringbuffer.New(5000)
rb.Write([]byte(logLine))
// Thread-safe implementation already handles synchronization
```

**Warning signs:**
- `go test -race` reports data races
- Log lines appear truncated or intermixed
- Sporadic crashes with "index out of range"
- SSE clients receive corrupted data

**Phase to address:**
Phase 1 (Log Capture) — Must use thread-safe implementation from the beginning

**Sources:**
- [smallnest/ringbuffer - GitHub](https://github.com/smallnest/ringbuffer)
- [How to ensure thread-safety in Go - Medium](https://bylucasqueiroz.medium.com/how-to-ensure-thread-safety-in-go-f928e21b6470)
- [How to Create Thread-Safe Cache in Go - OneUptime](https://oneuptime.com/blog/post/2026-01-30-go-thread-safe-cache/view)

---

### Pitfall 4: HTTP WriteTimeout Breaks SSE Streaming

**What goes wrong:**
SSE connections drop after 5 minutes (or configured timeout) even though client is actively receiving data. This happens because `http.Server.WriteTimeout` applies to the entire connection lifetime, not idle time.

**Why it happens:**
- `WriteTimeout` in `http.Server` is an absolute deadline from request start
- SSE connections are long-lived (minutes/hours)
- Server's `SetWriteDeadline` is called once at request start and never updated
- Even though data is being sent, the absolute timeout expires

**How to avoid:**
```go
// CORRECT: Disable WriteTimeout for SSE endpoints
srv := &http.Server{
    Addr:         ":8080",
    Handler:      mux,
    ReadTimeout:  10 * time.Second,
    // WriteTimeout: 10 * time.Second, // BAD: Breaks SSE
}

// BETTER: Use IdleTimeout (Go 1.8+) for keep-alive
srv := &http.Server{
    Addr:         ":8080",
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 0, // Disable for SSE
    IdleTimeout:  120 * time.Second, // Applies to idle connections only
}

// BEST: Context-based timeout for regular endpoints, SSE exempt
func sseHandler(w http.ResponseWriter, r *http.Request) {
    // SSE streams indefinitely until client disconnects
    // No timeout enforcement here
    <-r.Context().Done() // Detect client disconnect
}
```

**Warning signs:**
- SSE clients disconnect every 5 minutes (or exactly at WriteTimeout value)
- Browser console shows "EventSource's error event"
- Server logs show connection closed after fixed duration
- Clients auto-reconnect but lose logs during gap

**Phase to address:**
Phase 2 (SSE Endpoint) — Must configure HTTP server correctly before deploying

**Sources:**
- [The complete guide to Go net/http timeouts - Cloudflare](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/)
- [SSE connection keeps failing every 5 minutes - Stack Overflow](https://stackoverflow.com/questions/71207873/sse-connection-keeps-failing-every-5-minutes)
- [SSE server disconnects clients after inactivity #86 - GitHub](https://github.com/mark3labs/mcp-go/issues/86)

---

### Pitfall 5: Memory Leak from Unbounded Log Buffer

**What goes wrong:**
Application memory grows unbounded over time (days/weeks) because log lines accumulate faster than they're discarded. Even with a 5000-line ring buffer, if lines average 1KB and aren't properly overwritten, memory usage spirals.

**Why it happens:**
- Ring buffer stores strings; strings in Go are immutable
- Old strings may not be garbage collected if references remain
- High log volume (nanobot verbose mode) fills buffer quickly
- SSE clients reading from buffer may hold references longer than expected

**How to avoid:**
```go
// WRONG: Slices of slices can pin memory
type RingBuffer struct {
    data [][]byte // Each element may reference larger backing array
}

// CORRECT: Copy data to avoid pinning
type RingBuffer struct {
    data [][]byte
    mu   sync.RWMutex
}

func (rb *RingBuffer) Write(line []byte) {
    rb.mu.Lock()
    defer rb.mu.Unlock()

    // CRITICAL: Copy the byte slice to avoid pinning source buffer
    copied := make([]byte, len(line))
    copy(copied, line)

    rb.data[rb.head] = copied
    // ... rest of ring buffer logic
}

// ALSO: Enforce max line length
const maxLineLength = 10 * 1024 // 10KB max per line
if len(line) > maxLineLength {
    line = line[:maxLineLength]
}
```

**Monitor memory with:**
```go
// Add to health check endpoint
var m runtime.MemStats
runtime.ReadMemStats(&m)
log.Printf("HeapAlloc = %v MB", m.HeapAlloc/1024/1024)
log.Printf("NumGoroutine = %d", runtime.NumGoroutine())
```

**Warning signs:**
- Memory usage grows linearly over time
- `runtime.MemStats.HeapAlloc` never stabilizes
- GC runs frequently but frees little memory
- Out of memory crash after days/weeks of uptime

**Phase to address:**
Phase 1 (Log Capture) — Must design memory management correctly from start

**Sources:**
- [Golang Memory Leaks: Detection, Fixes, and Best Practices - Medium](https://medium.com/@mojimich2015/golang-memory-leaks-detection-fixes-and-best-practices-81749e9d698b)
- [How to Monitor Go Runtime Metrics - OneUptime](https://oneuptime.com/blog/post/2026-02-06-monitor-go-runtime-metrics-gc-goroutines-memory-opentelemetry/view)
- [How to Debug Memory Leaks in Go Applications - OneUptime](https://oneuptime.com/blog/post/2026-01-07-go-debug-memory-leaks/view)

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Use `cmd.StdoutPipe()` without concurrent reading | Simpler code | Deadlock when buffer fills | Never — always use concurrent reading or buffer |
| Skip `r.Context().Done()` check | Faster initial implementation | Goroutine leak, memory leak, file descriptor exhaustion | Never — always monitor client disconnect |
| Use `[]string` ring buffer without copying | Avoids allocation | Memory pinning, unbounded growth | Never — always copy strings/bytes |
| Set `WriteTimeout` globally | Prevents slowloris attacks | Breaks SSE after timeout expires | Use separate server for SSE or per-endpoint timeout |
| Skip ring buffer, stream directly to SSE | Less code, no buffer management | Clients miss logs if they connect late, no history | Only for truly real-time data where history doesn't matter |
| Use `cmd.CombinedOutput()` instead of separate pipes | Simpler code | Can't distinguish stdout vs stderr | When you don't need to differentiate stdout/stderr |

## Integration Gotchas

Common mistakes when connecting to external services or systems.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| HTTP Server with SSE | Set `WriteTimeout` globally | Disable `WriteTimeout` for SSE endpoints, use `IdleTimeout` instead |
| Process stdout/stderr capture | Call `cmd.Wait()` before reading pipes | Read pipes concurrently with `Wait()` using goroutines |
| Ring buffer for multiple readers | Single read index shared by all SSE clients | Each client tracks its own position in buffer |
| Log line buffering | Use unbuffered channels for log lines | Use buffered channels (size 100-1000) to absorb bursts |
| Windows process management | Assume processes exit cleanly on `cmd.Process.Kill()` | Use `cmd.Process.Signal(os.Interrupt)` first, then `Kill()` after timeout |
| SSE reconnection | Assume browser reconnects immediately | Include `retry: 3000\n` in SSE messages to control reconnect delay |

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Ring buffer with mutex contention | High CPU usage, slow log capture under load | Use `sync.RWMutex` (read-heavy), or lock-free ring buffer | 100+ concurrent SSE clients or 10K+ log lines/sec |
| SSE Flush on every line | High CPU, network overhead | Batch multiple lines, flush every 100ms or 50 lines | 1000+ log lines/second |
| String concatenation for log lines | High GC pressure, allocations | Use `strings.Builder` or `[]byte` with pre-allocated capacity | 10K+ log lines/second |
| Log line without length limit | Out of memory crash | Enforce max line length (e.g., 10KB) before buffering | Malicious/buggy nanobot output with massive lines |
| SSE without connection limit | Server exhaustion, OOM | Limit concurrent SSE connections per instance | 1000+ concurrent viewers |

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| SSE endpoint without rate limiting | DoS via many SSE connections | Limit concurrent SSE connections per IP/instance |
| Log line injection (XSS via logs) | XSS in Web UI, credential theft | Escape HTML in logs before sending to SSE, sanitize in Web UI |
| Unbounded log line size | Memory exhaustion DoS | Enforce max line length (10KB), truncate if exceeded |
| SSE accessible from any origin | CSRF, data exfiltration | Set `Access-Control-Allow-Origin` to specific domains, not `*` |
| Log content leaks secrets | Credentials in logs visible to SSE clients | Filter logs for secrets (tokens, passwords) before buffering |

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| No log history on connect | Miss logs that happened before connection | Ring buffer provides last 5000 lines immediately on connect |
| Infinite scroll performance | Browser freezes with 10K+ lines in DOM | Virtual scrolling (render only visible lines), cap at 1000 lines in UI |
| No visual feedback during reconnect | Users think logs are broken when network glitches | Show "Reconnecting..." status, auto-reconnect with backoff |
| Logs flood screen | Can't read anything when nanobot is verbose | Pause auto-scroll when user scrolls up, "Resume" button |
| All log levels same color | Hard to spot errors/warnings | Color-code by level: ERROR=red, WARN=yellow, INFO=gray, DEBUG=blue |
| No timestamp in logs | Can't correlate events | Include timestamp in each log line (UTC, ISO8601 format) |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **SSE Handler:** Often missing `r.Context().Done()` check — verify client disconnect detection works (test: close browser tab, check server logs for cleanup)
- [ ] **Process Capture:** Often missing concurrent pipe reading — verify `cmd.Wait()` returns even with verbose output (test: run process with massive stdout)
- [ ] **Ring Buffer:** Often missing thread-safety — verify with `go test -race` (test: concurrent Write and ReadAll in loop)
- [ ] **SSE Streaming:** Often missing Flusher check — verify `w.(http.Flusher)` assertion (test: HTTP/1.1 client should work)
- [ ] **Memory Management:** Often missing byte slice copying — verify memory doesn't grow unbounded (test: run for 24 hours with monitoring)
- [ ] **Connection Cleanup:** Often missing channel close on disconnect — verify no goroutine leaks (test: `runtime.NumGoroutine()` before/after 100 connections)
- [ ] **Error Handling:** Often missing pipe read error handling — verify `io.Copy` errors are logged (test: kill child process mid-output)

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Goroutine leak | MEDIUM | 1. Identify leak via `runtime.NumGoroutine()` or pprof<br>2. Add `r.Context().Done()` check<br>3. Restart server to clear leaked goroutines<br>4. Monitor for recurrence |
| Process pipe deadlock | LOW | 1. Kill stuck process manually<br>2. Fix code to read pipes concurrently<br>3. Redeploy<br>4. No data loss (logs buffered by OS) |
| Memory leak | HIGH | 1. Take heap dump (`pprof.WriteHeapProfile`)<br>2. Identify retained objects<br>3. Fix ring buffer to copy byte slices<br>4. Restart server to free memory<br>5. May have lost recent logs during OOM |
| SSE timeout disconnect | LOW | 1. Adjust `http.Server.IdleTimeout` or disable `WriteTimeout`<br>2. Redeploy<br>3. Clients auto-reconnect, minimal disruption |
| Ring buffer data race | MEDIUM | 1. Add `sync.RWMutex` to ring buffer<br>2. Test with `-race` flag<br>3. Redeploy<br>4. May have corrupted logs in buffer (transient) |
| OOM from unbounded logs | HIGH | 1. Kill process (may happen automatically)<br>2. Add max line length enforcement<br>3. Add ring buffer size limit<br>4. Redeploy<br>5. Lost all logs in buffer |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| SSE Connection Goroutine Leak | Phase 2 (SSE Endpoint) | Test: Connect 10 SSE clients, close all, verify `runtime.NumGoroutine()` returns to baseline |
| Process stdout/stderr Pipe Deadlock | Phase 1 (Log Capture) | Test: Capture process with 10MB stdout, verify `cmd.Wait()` returns within 5 seconds |
| Ring Buffer Data Race | Phase 1 (Log Capture) | Test: Run `go test -race` with concurrent Write/ReadAll in 100 goroutines for 10 seconds |
| HTTP WriteTimeout Breaks SSE | Phase 2 (SSE Endpoint) | Test: Connect SSE client, verify connection stays alive for >10 minutes with periodic data |
| Memory Leak from Unbounded Log Buffer | Phase 1 (Log Capture) | Test: Run for 1 hour with 1000 lines/sec, verify `runtime.MemStats.HeapAlloc` stays stable |

## Sources

**SSE and Connection Management:**
- [The complete guide to Go net/http timeouts - Cloudflare](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) — Definitive guide on HTTP timeouts in Go
- [Server Sent Events (SSE) Server implementation with Go - Dev.to](https://dev.to/mirzaakhena/server-sent-events-sse-server-implementation-with-go-4ck2) — Practical SSE implementation patterns
- [How to Stream Events with Server-Sent Events in Go - OneUptime](https://oneuptime.com/blog/post/2026-01-25-server-sent-events-streaming-go/view)
- [How to Build Real-time Applications with Go and SSE - OneUptime](https://oneuptime.com/blog/post/2026-02-01-go-realtime-applications-sse/view)
- [SSE connection keeps failing every 5 minutes - Stack Overflow](https://stackoverflow.com/questions/71207873/sse-connection-keeps-failing-every-5-minutes) — Common timeout issue
- [SSE server disconnects clients after inactivity #86 - GitHub](https://github.com/mark3labs/mcp-go/issues/86)

**Process Output Capture:**
- [os/exec: possible race handling stdout&stderr pipes #69060 - GitHub](https://github.com/golang/go/issues/69060) — Pipe deadlock investigation
- [Deadlock when redirecting Windows cmd.exe I/O using pipes - Stack Overflow](https://stackoverflow.com/questions/79130976/deadlock-when-redirecting-windows-cmdexe-i-o-using-pipes) — Windows-specific pipe issues

**Concurrency and Thread Safety:**
- [smallnest/ringbuffer - GitHub](https://github.com/smallnest/ringbuffer) — Thread-safe ring buffer implementation
- [How to ensure thread-safety in Go - Medium](https://bylucasqueiroz.medium.com/how-to-ensure-thread-safety-in-go-f928e21b6470)
- [How to Create Thread-Safe Cache in Go - OneUptime](https://oneuptime.com/blog/post/2026-01-30-go-thread-safe-cache/view)

**Memory Management:**
- [Golang Memory Leaks: Detection, Fixes, and Best Practices - Medium](https://medium.com/@mojimich2015/golang-memory-leaks-detection-fixes-and-best-practices-81749e9d698b)
- [How to Debug Memory Leaks in Go Applications - OneUptime](https://oneuptime.com/blog/post/2026-01-07-go-debug-memory-leaks/view)
- [How to Monitor Go Runtime Metrics - OneUptime](https://oneuptime.com/blog/post/2026-02-06-monitor-go-runtime-metrics-gc-goroutines-memory-opentelemetry/view)

**Ring Buffers:**
- [Ring Buffer循环队列防止语音数据丢失 - CSDN](https://blog.csdn.net/weixin_42476987/article/details/154926295)
- [Ring Buffers: High Performance IPC - Global Engineering](https://engineering.global.com/ring-buffers-high-performance-ipc-ae39b8bb74d4)
- [39M op/s, zero-allocation ring buffer - Reddit r/golang](https://www.reddit.com/r/golang/comments/1o7i34w/why_i_built_a_39m_ops_zeroallocation_ring_buffer/)

---
*Pitfalls research for: Real-time log viewing with SSE and process output capture*
*Researched: 2026-03-16*
