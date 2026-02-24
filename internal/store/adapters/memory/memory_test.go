package memory

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
)

const testLeaseDurationMS = 5000

func TestFetchAndClaim_ClaimsTwentyPercentDueRetries(t *testing.T) {
	m := NewMemoryStore()
	now := time.Now().UTC()
	past := now.Add(-time.Minute)

	for i := range 5 {
		addJobs(t, m, store.Job{
			ID:      fmt.Sprintf("retry-%d", i+1),
			Status:  store.Failed,
			RetryAt: &past,
		})
	}

	for i := range 20 {
		addJobs(t, m, store.Job{
			ID:     fmt.Sprintf("ready-%d", i+1),
			Status: store.Ready,
		})
	}

	claimed := m.FetchAndClaim(store.Ready, store.InFlight, 10, testLeaseDurationMS)
	if got := len(claimed); got != 10 {
		t.Fatalf("expected 10 claimed jobs, got %d", got)
	}

	retryCount := 0
	readyCount := 0
	claimedRetryIDs := make(map[string]struct{})

	for _, job := range claimed {
		if job.Status != store.InFlight {
			t.Fatalf("expected claimed status %s, got %s", store.InFlight, job.Status)
		}
		if job.LeaseExpiresAt == nil {
			t.Fatalf("expected claimed job %s to have a lease expiration", job.ID)
		}

		if strings.HasPrefix(job.ID, "retry-") {
			retryCount++
			claimedRetryIDs[job.ID] = struct{}{}
		}
		if strings.HasPrefix(job.ID, "ready-") {
			readyCount++
		}
	}

	if retryCount != 2 {
		t.Fatalf("expected 2 retry jobs (20%% of batch), got %d", retryCount)
	}
	if readyCount != 8 {
		t.Fatalf("expected 8 fresh jobs, got %d", readyCount)
	}

	all := byID(m.GetAllJobs())
	for id := range claimedRetryIDs {
		job := all[id]
		if job.Status != store.InFlight {
			t.Fatalf("expected claimed retry job %s status %s, got %s", id, store.InFlight, job.Status)
		}
		if job.RetryAt != nil {
			t.Fatalf("expected claimed retry job %s retryAt to be cleared", id)
		}
	}
}

func TestFetchAndClaim_SmallBatchClaimsAtLeastOneRetry(t *testing.T) {
	m := NewMemoryStore()
	now := time.Now().UTC()
	past := now.Add(-time.Minute)

	addJobs(t, m,
		store.Job{ID: "retry-1", Status: store.Failed, RetryAt: &past},
		store.Job{ID: "ready-1", Status: store.Ready},
		store.Job{ID: "ready-2", Status: store.Ready},
		store.Job{ID: "ready-3", Status: store.Ready},
		store.Job{ID: "ready-4", Status: store.Ready},
	)

	claimed := m.FetchAndClaim(store.Ready, store.InFlight, 4, testLeaseDurationMS)
	if got := len(claimed); got != 4 {
		t.Fatalf("expected 4 claimed jobs, got %d", got)
	}

	retryCount := 0
	for _, job := range claimed {
		if strings.HasPrefix(job.ID, "retry-") {
			retryCount++
		}
	}

	if retryCount != 1 {
		t.Fatalf("expected exactly 1 retry job for small batch, got %d", retryCount)
	}
}

func TestFetchAndClaim_FillsBatchFromDueRetriesWhenNoFreshJobs(t *testing.T) {
	m := NewMemoryStore()
	now := time.Now().UTC()
	past := now.Add(-time.Minute)

	for i := range 20 {
		addJobs(t, m, store.Job{
			ID:      fmt.Sprintf("retry-%d", i+1),
			Status:  store.Failed,
			RetryAt: &past,
		})
	}

	claimed := m.FetchAndClaim(store.Ready, store.InFlight, 10, testLeaseDurationMS)
	if got := len(claimed); got != 10 {
		t.Fatalf("expected 10 claimed jobs, got %d", got)
	}

	for _, job := range claimed {
		if !strings.HasPrefix(job.ID, "retry-") {
			t.Fatalf("expected only retry jobs, got %s", job.ID)
		}
		if job.Status != store.InFlight {
			t.Fatalf("expected claimed status %s, got %s", store.InFlight, job.Status)
		}
		if job.LeaseExpiresAt == nil {
			t.Fatalf("expected claimed job %s to have a lease expiration", job.ID)
		}
		if job.RetryAt != nil {
			t.Fatalf("expected claimed retry job %s retryAt to be cleared", job.ID)
		}
	}
}

func TestFetchAndClaim_BackfillsWithDueRetriesWhenFreshInsufficient(t *testing.T) {
	m := NewMemoryStore()
	now := time.Now().UTC()
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	addJobs(t, m,
		store.Job{ID: "retry-due-1", Status: store.Failed, RetryAt: &past},
		store.Job{ID: "retry-due-2", Status: store.Failed, RetryAt: &past},
		store.Job{ID: "retry-due-3", Status: store.Failed, RetryAt: &past},
		store.Job{ID: "retry-due-4", Status: store.Failed, RetryAt: &past},
		store.Job{ID: "retry-due-5", Status: store.Failed, RetryAt: &past},
		store.Job{ID: "retry-future", Status: store.Failed, RetryAt: &future},
		store.Job{ID: "retry-nil", Status: store.Failed},
		store.Job{ID: "ready-1", Status: store.Ready},
		store.Job{ID: "ready-2", Status: store.Ready},
	)

	claimed := m.FetchAndClaim(store.Ready, store.InFlight, 6, testLeaseDurationMS)
	if got := len(claimed); got != 6 {
		t.Fatalf("expected 6 claimed jobs, got %d", got)
	}

	retryCount := 0
	readyCount := 0
	claimedIDs := byID(claimed)
	for _, job := range claimed {
		if strings.HasPrefix(job.ID, "retry-") {
			retryCount++
		}
		if strings.HasPrefix(job.ID, "ready-") {
			readyCount++
		}
	}

	if retryCount != 4 {
		t.Fatalf("expected 4 retry jobs after backfill, got %d", retryCount)
	}
	if readyCount != 2 {
		t.Fatalf("expected 2 fresh jobs, got %d", readyCount)
	}
	if _, ok := claimedIDs["retry-future"]; ok {
		t.Fatalf("did not expect future retry job to be claimed")
	}
	if _, ok := claimedIDs["retry-nil"]; ok {
		t.Fatalf("did not expect nil retryAt job to be claimed")
	}
}

func TestFetchAndClaim_OnlyClaimsDueRetries(t *testing.T) {
	m := NewMemoryStore()
	now := time.Now().UTC()
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	addJobs(t, m,
		store.Job{ID: "retry-due", Status: store.Failed, RetryAt: &past},
		store.Job{ID: "retry-future", Status: store.Failed, RetryAt: &future},
		store.Job{ID: "retry-nil", Status: store.Failed},
	)

	for i := range 4 {
		addJobs(t, m, store.Job{ID: fmt.Sprintf("ready-%d", i+1), Status: store.Ready})
	}

	claimed := m.FetchAndClaim(store.Ready, store.InFlight, 5, testLeaseDurationMS)
	claimedIDs := byID(claimed)

	if _, ok := claimedIDs["retry-due"]; !ok {
		t.Fatalf("expected due retry job to be claimed")
	}
	if _, ok := claimedIDs["retry-future"]; ok {
		t.Fatalf("did not expect future retry job to be claimed")
	}
	if _, ok := claimedIDs["retry-nil"]; ok {
		t.Fatalf("did not expect retry job with nil retryAt to be claimed")
	}

	all := byID(m.GetAllJobs())
	if all["retry-future"].Status != store.Failed {
		t.Fatalf("expected retry-future status %s, got %s", store.Failed, all["retry-future"].Status)
	}
	if all["retry-future"].RetryAt == nil || !all["retry-future"].RetryAt.Equal(future) {
		t.Fatalf("expected retry-future retryAt to remain unchanged")
	}
	if all["retry-nil"].Status != store.Failed {
		t.Fatalf("expected retry-nil status %s, got %s", store.Failed, all["retry-nil"].Status)
	}
}

func TestFetchAndClaim_ReturnsPartialBatchWhenInsufficientJobs(t *testing.T) {
	m := NewMemoryStore()
	now := time.Now().UTC()
	past := now.Add(-time.Minute)

	addJobs(t, m,
		store.Job{ID: "retry-1", Status: store.Failed, RetryAt: &past},
		store.Job{ID: "ready-1", Status: store.Ready},
	)

	claimed := m.FetchAndClaim(store.Ready, store.InFlight, 10, testLeaseDurationMS)
	if got := len(claimed); got != 2 {
		t.Fatalf("expected 2 claimed jobs, got %d", got)
	}

	claimedIDs := byID(claimed)
	if _, ok := claimedIDs["retry-1"]; !ok {
		t.Fatalf("expected retry-1 to be claimed")
	}
	if _, ok := claimedIDs["ready-1"]; !ok {
		t.Fatalf("expected ready-1 to be claimed")
	}
}

func TestFetchAndClaim_UsesCurrStatusForFreshJobs(t *testing.T) {
	m := NewMemoryStore()

	addJobs(t, m,
		store.Job{ID: "fresh-succeeded", Status: store.Succeeded},
		store.Job{ID: "ready-1", Status: store.Ready},
	)

	claimed := m.FetchAndClaim(store.Succeeded, store.InFlight, 5, testLeaseDurationMS)
	if got := len(claimed); got != 1 {
		t.Fatalf("expected 1 claimed job, got %d", got)
	}
	if claimed[0].ID != "fresh-succeeded" {
		t.Fatalf("expected claimed id fresh-succeeded, got %s", claimed[0].ID)
	}

	all := byID(m.GetAllJobs())
	if all["fresh-succeeded"].Status != store.InFlight {
		t.Fatalf("expected fresh-succeeded status %s, got %s", store.InFlight, all["fresh-succeeded"].Status)
	}
	if all["ready-1"].Status != store.Ready {
		t.Fatalf("expected ready-1 status %s, got %s", store.Ready, all["ready-1"].Status)
	}
}

func addJobs(t *testing.T, m *MemoryStore, jobs ...store.Job) {
	t.Helper()
	for _, job := range jobs {
		if err := m.AddJob(job); err != nil {
			t.Fatalf("failed adding job %s: %v", job.ID, err)
		}
	}
}

func byID(jobs []store.Job) map[string]store.Job {
	indexed := make(map[string]store.Job, len(jobs))
	for _, job := range jobs {
		indexed[job.ID] = job
	}
	return indexed
}
