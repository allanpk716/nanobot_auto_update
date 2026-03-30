package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/selfupdate"
)

// mockSelfUpdateChecker is a mock implementation of SelfUpdateChecker for testing.
type mockSelfUpdateChecker struct {
	needsUpdate bool
	releaseInfo *selfupdate.ReleaseInfo
	err         error
	updateErr   error
	updateCalls int
}

func (m *mockSelfUpdateChecker) NeedUpdate(currentVersion string) (bool, *selfupdate.ReleaseInfo, error) {
	return m.needsUpdate, m.releaseInfo, m.err
}

func (m *mockSelfUpdateChecker) Update(currentVersion string) error {
	m.updateCalls++
	return m.updateErr
}

// mockUpdateMutex is a mock implementation of UpdateMutex for testing.
type mockUpdateMutex struct {
	isUpdating   atomic.Bool
	tryLockCalls int
}

func (m *mockUpdateMutex) TryLockUpdate() bool {
	m.tryLockCalls++
	return m.isUpdating.CompareAndSwap(false, true)
}

func (m *mockUpdateMutex) UnlockUpdate() {
	m.isUpdating.Store(false)
}

func (m *mockUpdateMutex) IsUpdating() bool {
	return m.isUpdating.Load()
}

// newTestSelfUpdateHandler creates a SelfUpdateHandler with mocks for testing.
func newTestSelfUpdateHandler(checker SelfUpdateChecker, mutex *mockUpdateMutex, notif Notifier) *SelfUpdateHandler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	h := NewSelfUpdateHandler(checker, "dev", mutex, notif, logger)
	// Override restartFn to prevent os.Exit(0) during tests
	h.restartFn = func(exePath string) {}
	return h
}

// mockNotifier records notification calls for testing
type mockNotifier struct {
	mu    sync.Mutex
	calls []mockNotifyCall
	err   error // if set, Notify returns this error
}

type mockNotifyCall struct {
	title   string
	message string
}

func (m *mockNotifier) Notify(title, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockNotifyCall{title: title, message: message})
	return m.err
}

func (m *mockNotifier) getCalls() []mockNotifyCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]mockNotifyCall, len(m.calls))
	copy(result, m.calls)
	return result
}

// TestSelfUpdateCheck_Success tests GET check returns 200 with version info
func TestSelfUpdateCheck_Success(t *testing.T) {
	checker := &mockSelfUpdateChecker{
		needsUpdate: true,
		releaseInfo: &selfupdate.ReleaseInfo{
			Version:      "v1.0.0",
			ReleaseNotes: "Test release",
			PublishedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			DownloadURL:  "https://example.com/download",
		},
	}
	mutex := &mockUpdateMutex{}
	handler := newTestSelfUpdateHandler(checker, mutex, nil)

	req := httptest.NewRequest("GET", "/api/v1/self-update/check", nil)
	rec := httptest.NewRecorder()

	handler.HandleCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var response SelfUpdateCheckResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if response.CurrentVersion != "dev" {
		t.Errorf("current_version = %q, want %q", response.CurrentVersion, "dev")
	}
	if response.LatestVersion != "v1.0.0" {
		t.Errorf("latest_version = %q, want %q", response.LatestVersion, "v1.0.0")
	}
	if response.NeedsUpdate != true {
		t.Errorf("needs_update = %v, want true", response.NeedsUpdate)
	}
	if response.SelfUpdateStatus != "idle" {
		t.Errorf("self_update_status = %q, want %q", response.SelfUpdateStatus, "idle")
	}
}

// TestSelfUpdateCheck_Error tests GET check returns 500 when NeedUpdate fails
func TestSelfUpdateCheck_Error(t *testing.T) {
	checker := &mockSelfUpdateChecker{
		err: errors.New("api error"),
	}
	mutex := &mockUpdateMutex{}
	handler := newTestSelfUpdateHandler(checker, mutex, nil)

	req := httptest.NewRequest("GET", "/api/v1/self-update/check", nil)
	rec := httptest.NewRecorder()

	handler.HandleCheck(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestSelfUpdateUpdate_Accepted tests POST returns 202 Accepted (D-01, D-04)
func TestSelfUpdateUpdate_Accepted(t *testing.T) {
	checker := &mockSelfUpdateChecker{
		updateErr: nil,
	}
	mutex := &mockUpdateMutex{}
	handler := newTestSelfUpdateHandler(checker, mutex, nil)

	req := httptest.NewRequest("POST", "/api/v1/self-update", nil)
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if response["status"] != "accepted" {
		t.Errorf("status = %q, want %q", response["status"], "accepted")
	}
	if response["message"] != "Self-update started" {
		t.Errorf("message = %q, want %q", response["message"], "Self-update started")
	}

	// Wait for goroutine to finish and verify status changed to "updated"
	time.Sleep(100 * time.Millisecond)

	currentStatus := handler.status.Load().(*SelfUpdateStatus)
	if currentStatus.Status != "updated" {
		t.Errorf("status = %q, want %q after goroutine completes", currentStatus.Status, "updated")
	}
}

// TestSelfUpdateUpdate_Conflict tests POST returns 409 when lock already held (D-02, API-02)
func TestSelfUpdateUpdate_Conflict(t *testing.T) {
	checker := &mockSelfUpdateChecker{}
	mutex := &mockUpdateMutex{}

	// Pre-lock the mutex
	mutex.TryLockUpdate()

	handler := newTestSelfUpdateHandler(checker, mutex, nil)

	req := httptest.NewRequest("POST", "/api/v1/self-update", nil)
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusConflict)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if response["error"] != "conflict" {
		t.Errorf("error = %q, want %q", response["error"], "conflict")
	}
}

// TestSelfUpdateUpdate_Failed tests POST goroutine error sets status to "failed"
func TestSelfUpdateUpdate_Failed(t *testing.T) {
	checker := &mockSelfUpdateChecker{
		updateErr: errors.New("download failed"),
	}
	mutex := &mockUpdateMutex{}
	handler := newTestSelfUpdateHandler(checker, mutex, nil)

	req := httptest.NewRequest("POST", "/api/v1/self-update", nil)
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Wait for goroutine to finish
	time.Sleep(100 * time.Millisecond)

	currentStatus := handler.status.Load().(*SelfUpdateStatus)
	if currentStatus.Status != "failed" {
		t.Errorf("status = %q, want %q", currentStatus.Status, "failed")
	}
	if currentStatus.Error != "download failed" {
		t.Errorf("error = %q, want %q", currentStatus.Error, "download failed")
	}
}

// TestSelfUpdateUpdate_PanicRecovery tests goroutine panic does not leave lock held (Pitfall 1)
func TestSelfUpdateUpdate_PanicRecovery(t *testing.T) {
	// Create a checker that panics on Update
	panicChecker := &panicSelfUpdateChecker{}
	mutex := &mockUpdateMutex{}
	handler := newTestSelfUpdateHandler(panicChecker, mutex, nil)

	req := httptest.NewRequest("POST", "/api/v1/self-update", nil)
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Wait for goroutine to finish
	time.Sleep(100 * time.Millisecond)

	currentStatus := handler.status.Load().(*SelfUpdateStatus)
	if currentStatus.Status != "failed" {
		t.Errorf("status = %q, want %q", currentStatus.Status, "failed")
	}

	// Verify mutex is unlocked: TryLockUpdate() returns true (lock was released by defer)
	if !mutex.TryLockUpdate() {
		t.Error("Expected mutex to be unlocked after panic recovery, but TryLockUpdate returned false")
	}
}

// panicSelfUpdateChecker is a mock that panics on Update
type panicSelfUpdateChecker struct {
	needsUpdate bool
	releaseInfo *selfupdate.ReleaseInfo
	err         error
}

func (m *panicSelfUpdateChecker) NeedUpdate(currentVersion string) (bool, *selfupdate.ReleaseInfo, error) {
	return m.needsUpdate, m.releaseInfo, m.err
}

func (m *panicSelfUpdateChecker) Update(currentVersion string) error {
	panic("unexpected panic during update")
}

// TestSelfUpdateCheck_StatusDuringUpdate tests check shows "updating" status
func TestSelfUpdateCheck_StatusDuringUpdate(t *testing.T) {
	// Create a checker that blocks on Update for a while
	slowChecker := &slowSelfUpdateChecker{
		done: make(chan struct{}),
	}
	mutex := &mockUpdateMutex{}
	handler := newTestSelfUpdateHandler(slowChecker, mutex, nil)

	// Start the update
	req := httptest.NewRequest("POST", "/api/v1/self-update", nil)
	rec := httptest.NewRecorder()
	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Immediately check status - should be "updating"
	time.Sleep(20 * time.Millisecond)
	currentStatus := handler.status.Load().(*SelfUpdateStatus)
	if currentStatus.Status != "updating" {
		t.Errorf("status during update = %q, want %q", currentStatus.Status, "updating")
	}

	// Signal the slow checker to complete
	close(slowChecker.done)

	// Wait for goroutine to finish
	time.Sleep(100 * time.Millisecond)

	currentStatus = handler.status.Load().(*SelfUpdateStatus)
	if currentStatus.Status != "updated" {
		t.Errorf("status after update = %q, want %q", currentStatus.Status, "updated")
	}
}

// slowSelfUpdateChecker is a mock that waits on a channel before completing Update
type slowSelfUpdateChecker struct {
	done       chan struct{}
	needsUpdate bool
	releaseInfo *selfupdate.ReleaseInfo
	err         error
}

func (m *slowSelfUpdateChecker) NeedUpdate(currentVersion string) (bool, *selfupdate.ReleaseInfo, error) {
	return m.needsUpdate, m.releaseInfo, m.err
}

func (m *slowSelfUpdateChecker) Update(currentVersion string) error {
	<-m.done
	return nil
}

// TestSelfUpdateAuth tests auth middleware integration (API-01)
func TestSelfUpdateAuth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.APIConfig{
		Port:        8080,
		BearerToken: "valid-token-12345678901234567890",
		Timeout:     30 * time.Second,
	}

	checker := &mockSelfUpdateChecker{
		needsUpdate: true,
		releaseInfo: &selfupdate.ReleaseInfo{
			Version:     "v1.0.0",
			PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		updateErr: nil,
	}
	mutex := &mockUpdateMutex{}

	selfUpdateHandler := NewSelfUpdateHandler(checker, "dev", mutex, nil, logger)
	selfUpdateHandler.restartFn = func(exePath string) {} // prevent os.Exit in tests
	authMiddleware := AuthMiddleware(cfg.BearerToken, logger)

	checkHandler := authMiddleware(http.HandlerFunc(selfUpdateHandler.HandleCheck))
	updateHandler := authMiddleware(http.HandlerFunc(selfUpdateHandler.HandleUpdate))

	tests := []struct {
		name           string
		handler        http.Handler
		method         string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "check no auth header",
			handler:        checkHandler,
			method:         "GET",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "check invalid token",
			handler:        checkHandler,
			method:         "GET",
			authHeader:     "Bearer invalid-token-00000000000000000000",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "check valid token",
			handler:        checkHandler,
			method:         "GET",
			authHeader:     "Bearer " + cfg.BearerToken,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "update no auth header",
			handler:        updateHandler,
			method:         "POST",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "update valid token",
			handler:        updateHandler,
			method:         "POST",
			authHeader:     "Bearer " + cfg.BearerToken,
			expectedStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/self-update", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			tt.handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status code = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

// TestSelfUpdateUpdate_StartNotification verifies start notification is sent (D-03)
func TestSelfUpdateUpdate_StartNotification(t *testing.T) {
	checker := &mockSelfUpdateChecker{
		updateErr: nil,
	}
	mutex := &mockUpdateMutex{}
	notif := &mockNotifier{}
	handler := newTestSelfUpdateHandler(checker, mutex, notif)

	req := httptest.NewRequest("POST", "/api/v1/self-update", nil)
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Wait for start notification goroutine and update goroutine
	time.Sleep(200 * time.Millisecond)

	calls := notif.getCalls()
	foundStart := false
	for _, call := range calls {
		if call.title == "Nanobot 自更新开始" {
			foundStart = true
			if call.message != "当前版本: dev" {
				t.Errorf("start notification message = %q, want %q", call.message, "当前版本: dev")
			}
		}
	}
	if !foundStart {
		t.Error("Expected start notification with title 'Nanobot 自更新开始', but none found")
	}
}

// TestSelfUpdateUpdate_FailureNotification verifies failure notification is sent on error (D-03)
func TestSelfUpdateUpdate_FailureNotification(t *testing.T) {
	checker := &mockSelfUpdateChecker{
		updateErr: errors.New("download failed"),
	}
	mutex := &mockUpdateMutex{}
	notif := &mockNotifier{}
	handler := newTestSelfUpdateHandler(checker, mutex, notif)

	req := httptest.NewRequest("POST", "/api/v1/self-update", nil)
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Wait for goroutines
	time.Sleep(200 * time.Millisecond)

	calls := notif.getCalls()
	foundFailure := false
	for _, call := range calls {
		if call.title == "Nanobot 自更新失败" {
			foundFailure = true
			if call.message == "" {
				t.Error("failure notification message is empty")
			}
		}
	}
	if !foundFailure {
		t.Errorf("Expected failure notification with title 'Nanobot 自更新失败', got calls: %+v", calls)
	}
}

// TestSelfUpdateUpdate_NilNotifier verifies no panic when notifier is nil
func TestSelfUpdateUpdate_NilNotifier(t *testing.T) {
	checker := &mockSelfUpdateChecker{
		updateErr: nil,
	}
	mutex := &mockUpdateMutex{}
	handler := newTestSelfUpdateHandler(checker, mutex, nil)

	req := httptest.NewRequest("POST", "/api/v1/self-update", nil)
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Wait for goroutine to finish
	time.Sleep(100 * time.Millisecond)

	currentStatus := handler.status.Load().(*SelfUpdateStatus)
	if currentStatus.Status != "updated" {
		t.Errorf("status = %q, want %q (nil notifier should not affect update)", currentStatus.Status, "updated")
	}
}

// NOTE: The self-spawn + os.Exit(0) code path after successful update
// cannot be unit tested (it terminates the test process).
// This pattern was validated by the Phase 36 PoC (tmp/poc_selfupdate.go).
