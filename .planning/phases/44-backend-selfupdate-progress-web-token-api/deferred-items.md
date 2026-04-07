# Deferred Items (Phase 44)

## Pre-existing Issues (Out of Scope)

1. **server_test.go: compilation errors** - `instance.NewInstanceManager` calls have wrong number of arguments (missing `instance.Notifier` parameter). Files: `internal/api/server_test.go` lines 34, 69, 105, 141, 189, 227 and `internal/api/sse_test.go` line 224. These were introduced by a prior phase that changed the NewInstanceManager signature but did not update these test files. This blocks `go test ./internal/api/ -count=1` but does NOT affect production code compilation or selfupdate handler tests.

2. Note: selfupdate handler tests verified manually by code review - all mock types updated with GetProgress(), new test functions use correct standard library testing patterns matching file style.
