package updatelog

import (
	"log/slog"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

// benchLogger creates a logger for benchmark tests.
func benchLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

// createBenchUpdateLog creates a test UpdateLog with the given index used to vary the ID.
func createBenchUpdateLog(i int) UpdateLog {
	return UpdateLog{
		ID:          "bench-id-" + strconv.Itoa(i),
		StartTime:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Second),
		EndTime:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i)*time.Second + 5*time.Second),
		Duration:    5000,
		Status:      StatusSuccess,
		TriggeredBy: "benchmark",
		Instances: []InstanceUpdateDetail{
			{Name: "instance-1", Port: 18790, Status: "success"},
		},
	}
}

// populateLogger pre-populates the UpdateLogger with n records.
func populateLogger(ul *UpdateLogger, n int) {
	for i := 0; i < n; i++ {
		ul.Record(createBenchUpdateLog(i))
	}
}

// BenchmarkGetPage_1000Records benchmarks GetPage with 1000 in-memory records.
func BenchmarkGetPage_1000Records(b *testing.B) {
	ul := NewUpdateLogger(benchLogger(), "")
	populateLogger(ul, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logs, total := ul.GetPage(20, 0)
		if total != 1000 {
			b.Fatalf("expected total 1000, got %d", total)
		}
		if len(logs) != 20 {
			b.Fatalf("expected 20 logs, got %d", len(logs))
		}
	}
}

// BenchmarkGetPage_5000Records benchmarks GetPage with 5000 records.
func BenchmarkGetPage_5000Records(b *testing.B) {
	ul := NewUpdateLogger(benchLogger(), "")
	populateLogger(ul, 5000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logs, total := ul.GetPage(50, 100)
		if total != 5000 {
			b.Fatalf("expected total 5000, got %d", total)
		}
		if len(logs) != 50 {
			b.Fatalf("expected 50 logs, got %d", len(logs))
		}
	}
}

// BenchmarkRecord_Concurrent benchmarks Record() called from multiple goroutines simultaneously.
// Verifies no deadlocks or race conditions under concurrent load.
func BenchmarkRecord_Concurrent(b *testing.B) {
	ul := NewUpdateLogger(benchLogger(), "")

	b.ResetTimer()
	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		// Launch 10 concurrent goroutines for each benchmark iteration
		for g := 0; g < 10; g++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				ul.Record(createBenchUpdateLog(idx))
			}(i*10 + g)
		}
	}
	wg.Wait()

	// Verify all records were recorded (after timer stops)
	all := ul.GetAll()
	expectedCount := b.N * 10
	if len(all) != expectedCount {
		b.Fatalf("expected %d records, got %d", expectedCount, len(all))
	}
}
