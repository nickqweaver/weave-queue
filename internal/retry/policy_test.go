package retry

import (
	"testing"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
)

func TestFailureUpdate_SchedulesRetryWithinLimit(t *testing.T) {
	now := time.Now().UTC()
	update := FailureUpdate(store.Job{Retries: 1}, now, Config{
		MaxRetries:    3,
		BackoffBaseMS: 500,
		BackoffMaxMS:  30_000,
	})

	if update.Status != store.Failed {
		t.Fatalf("expected status %s, got %s", store.Failed, update.Status)
	}
	if update.Retries == nil || *update.Retries != 2 {
		t.Fatalf("expected retries to be 2, got %+v", update.Retries)
	}
	if update.RetryAt == nil {
		t.Fatal("expected retryAt to be scheduled")
	}
	if !update.RetryAt.After(now) {
		t.Fatalf("expected retryAt %v to be after %v", update.RetryAt, now)
	}
}

func TestFailureUpdate_MarksTerminalFailurePastMaxRetries(t *testing.T) {
	now := time.Now().UTC()
	update := FailureUpdate(store.Job{Retries: 3}, now, Config{
		MaxRetries:    3,
		BackoffBaseMS: 500,
		BackoffMaxMS:  30_000,
	})

	if update.Status != store.Failed {
		t.Fatalf("expected status %s, got %s", store.Failed, update.Status)
	}
	if update.Retries == nil || *update.Retries != 4 {
		t.Fatalf("expected retries to be 4, got %+v", update.Retries)
	}
	if update.RetryAt != nil {
		t.Fatalf("expected retryAt to be nil, got %v", update.RetryAt)
	}
}
