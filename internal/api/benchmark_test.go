package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/updatelog"
)

// benchCreateUpdateLog creates a test UpdateLog with the given index used to vary the ID.
func benchCreateUpdateLog(i int) updatelog.UpdateLog {
	return updatelog.UpdateLog{
		ID:          "bench-id-" + strconv.Itoa(i),
		StartTime:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Second),
		EndTime:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i)*time.Second + 5*time.Second),
		Duration:    5000,
		Status:      updatelog.StatusSuccess,
		TriggeredBy: "benchmark",
		Instances: []updatelog.InstanceUpdateDetail{
			{Name: "instance-1", Port: 18790, Status: "success"},
		},
	}
}

// benchPopulateLogger pre-populates the UpdateLogger with n records for benchmarking.
func benchPopulateLogger(ul *updatelog.UpdateLogger, n int) {
	for i := 0; i < n; i++ {
		ul.Record(benchCreateUpdateLog(i))
	}
}

// BenchmarkQueryHandler_1000Records benchmarks full HTTP handler cycle with 1000 records.
func BenchmarkQueryHandler_1000Records(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")
	benchPopulateLogger(ul, 1000)

	handler := NewQueryHandler(ul, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/update-logs?limit=20&offset=0", nil)
		w := httptest.NewRecorder()
		handler.Handle(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp map[string]json.RawMessage
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			b.Fatalf("failed to decode response: %v", err)
		}
	}
}
