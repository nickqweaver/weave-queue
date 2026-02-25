package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
)

// mockStore is a test double for the Store interface.
type mockStore struct {
	jobs    []Job
	claims  []Job
	updates map[string]JobUpdate
}

func newMockStore() *mockStore {
	return &mockStore{
		jobs:    make([]Job, 0),
		claims:  make([]Job, 0),
		updates: make(map[string]JobUpdate),
	}
}

func (m *mockStore) AddJob(ctx context.Context, job Job) error {
	m.jobs = append(m.jobs, job)
	return nil
}

func (m *mockStore) ClaimAvailable(ctx context.Context, opts ClaimOptions) ([]Job, error) {
	return m.claims, nil
}

func (m *mockStore) UpdateJob(ctx context.Context, id string, update JobUpdate) error {
	m.updates[id] = update
	return nil
}

func (m *mockStore) GetJob(ctx context.Context, id string) (Job, error) {
	return Job{}, errors.New("not implemented")
}

func TestNewRuntime_ValidatesConfig(t *testing.T) {
	store := newMockStore()
	invalidConfig := Config{
		BatchSize: 100,
		MaxQueue:  10, // BatchSize > MaxQueue should fail
	}

	_, err := NewRuntime(store, invalidConfig)
	if err == nil {
		t.Error("expected error for invalid config, got nil")
	}
}

func TestNewRuntime_AppliesDefaults(t *testing.T) {
	store := newMockStore()
	config := Config{} // Empty config

	runtime, err := NewRuntime(store, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runtime == nil {
		t.Fatal("expected non-nil runtime")
	}

	// Check that defaults were applied
	if runtime.config.MaxConcurrency != 10 {
		t.Errorf("MaxConcurrency = %d, want 10", runtime.config.MaxConcurrency)
	}

	if runtime.config.MaxQueue != 100 {
		t.Errorf("MaxQueue = %d, want 100", runtime.config.MaxQueue)
	}
}

func TestRuntime_Register(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()

	runtime, err := NewRuntime(store, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := HandlerFunc(func(ctx context.Context, job Job) error {
		return nil
	})

	// First registration should succeed
	if err := runtime.Register("test.task", handler); err != nil {
		t.Errorf("first registration failed: %v", err)
	}

	// Second registration for same task should fail
	if err := runtime.Register("test.task", handler); err == nil {
		t.Error("expected error for duplicate registration, got nil")
	} else if _, ok := err.(*DuplicateRegistrationError); !ok {
		t.Errorf("expected DuplicateRegistrationError, got %T", err)
	}
}

func TestRuntime_RegisterFunc(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()

	runtime, err := NewRuntime(store, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	called := false
	handler := func(ctx context.Context, job Job) error {
		called = true
		return nil
	}

	if err := runtime.RegisterFunc("test.func", handler); err != nil {
		t.Errorf("registration failed: %v", err)
	}

	// Verify handler is stored (we can't easily invoke it without more setup)
	if len(runtime.handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(runtime.handlers))
	}

	// Verify called is not accessed (handler not invoked)
	_ = called
}

func TestDuplicateRegistrationError(t *testing.T) {
	err := &DuplicateRegistrationError{TaskName: "my.task"}
	expected := "handler already registered for task: my.task"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestRuntime_RunWithContext(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()
	config.MaxConcurrency = 1
	config.MaxQueue = 10
	config.BatchSize = 5

	runtime, err := NewRuntime(store, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create a cancellable context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run should block until context is cancelled
	done := make(chan struct{})
	go func() {
		runtime.Run(ctx)
		close(done)
	}()

	// Wait for run to complete (context timeout)
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("Run did not stop after context cancellation")
	}
}

func TestRuntime_Stop(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()

	runtime, err := NewRuntime(store, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stop should not panic even if not running
	runtime.Stop()
}

func TestConvertStatus(t *testing.T) {
	tests := []struct {
		internal store.Status
		public   JobStatus
	}{
		{store.Ready, JobStatusReady},
		{store.InFlight, JobStatusInFlight},
		{store.Failed, JobStatusFailed},
		{store.Succeeded, JobStatusSucceeded},
	}

	for _, tt := range tests {
		// Test internal -> public
		got := convertStatusFromInternal(tt.internal)
		if got != tt.public {
			t.Errorf("convertStatusFromInternal(%v) = %v, want %v", tt.internal, got, tt.public)
		}

		// Test public -> internal
		got2 := convertStatusToInternal(tt.public)
		if got2 != tt.internal {
			t.Errorf("convertStatusToInternal(%v) = %v, want %v", tt.public, got2, tt.internal)
		}
	}

	// Test unknown status defaults to Ready
	unknownInternal := convertStatusToInternal(JobStatus(999))
	if unknownInternal != store.Ready {
		t.Errorf("convertStatusToInternal(unknown) = %v, want Ready", unknownInternal)
	}

	unknownPublic := convertStatusFromInternal(store.Status(999))
	if unknownPublic != JobStatusReady {
		t.Errorf("convertStatusFromInternal(unknown) = %v, want Ready", unknownPublic)
	}
}
