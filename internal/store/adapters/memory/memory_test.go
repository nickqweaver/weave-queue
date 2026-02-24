package memory

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
)

const testLeaseTTL = 5 * time.Second

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

	claimed := m.ClaimAvailable(claimOpts(10))
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

	claimed := m.ClaimAvailable(claimOpts(4))
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

	claimed := m.ClaimAvailable(claimOpts(10))
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

	claimed := m.ClaimAvailable(claimOpts(6))
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

	claimed := m.ClaimAvailable(claimOpts(5))
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

	claimed := m.ClaimAvailable(claimOpts(10))
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

func TestClaimAvailable_ClaimsOnlyReadyForFreshJobs(t *testing.T) {
	m := NewMemoryStore()

	addJobs(t, m,
		store.Job{ID: "fresh-succeeded", Status: store.Succeeded},
		store.Job{ID: "ready-1", Status: store.Ready},
	)

	claimed := m.ClaimAvailable(claimOpts(5))
	if got := len(claimed); got != 1 {
		t.Fatalf("expected 1 claimed job, got %d", got)
	}
	if claimed[0].ID != "ready-1" {
		t.Fatalf("expected claimed id ready-1, got %s", claimed[0].ID)
	}

	all := byID(m.GetAllJobs())
	if all["fresh-succeeded"].Status != store.Succeeded {
		t.Fatalf("expected fresh-succeeded status %s, got %s", store.Succeeded, all["fresh-succeeded"].Status)
	}
	if all["ready-1"].Status != store.InFlight {
		t.Fatalf("expected ready-1 status %s, got %s", store.InFlight, all["ready-1"].Status)
	}
}

func TestClaimAvailable_RecoversExpiredLeaseWithBackoff(t *testing.T) {
	m := NewMemoryStore()
	now := time.Now().UTC()
	expired := now.Add(-time.Second)
	active := now.Add(time.Minute)

	addJobs(t, m,
		store.Job{ID: "inflight-expired", Status: store.InFlight, LeaseExpiresAt: &expired},
		store.Job{ID: "inflight-active", Status: store.InFlight, LeaseExpiresAt: &active},
		store.Job{ID: "ready-1", Status: store.Ready},
	)

	claimed := m.ClaimAvailable(claimOpts(2))
	if got := len(claimed); got != 1 {
		t.Fatalf("expected 1 claimed job, got %d", got)
	}
	if claimed[0].ID != "ready-1" {
		t.Fatalf("expected ready-1 to be claimed, got %s", claimed[0].ID)
	}

	all := byID(m.GetAllJobs())
	expiredJob := all["inflight-expired"]
	if expiredJob.Status != store.Failed {
		t.Fatalf("expected expired job status %s, got %s", store.Failed, expiredJob.Status)
	}
	if expiredJob.Retries != 1 {
		t.Fatalf("expected expired job retries to increment to 1, got %d", expiredJob.Retries)
	}
	if expiredJob.LeaseExpiresAt != nil {
		t.Fatalf("expected expired job lease to be cleared")
	}
	if expiredJob.RetryAt == nil {
		t.Fatalf("expected expired job retryAt to be scheduled")
	}
	if !expiredJob.RetryAt.After(now) {
		t.Fatalf("expected expired job retryAt %v to be after %v", expiredJob.RetryAt, now)
	}

	activeJob := all["inflight-active"]
	if activeJob.Status != store.InFlight {
		t.Fatalf("expected active in-flight status %s, got %s", store.InFlight, activeJob.Status)
	}
	if activeJob.LeaseExpiresAt == nil || !activeJob.LeaseExpiresAt.Equal(active) {
		t.Fatalf("expected active in-flight lease to remain unchanged")
	}
}

func TestClaimAvailable_ExpiredLeasePastMaxRetriesIsTerminal(t *testing.T) {
	m := NewMemoryStore()
	now := time.Now().UTC()
	expired := now.Add(-time.Second)

	addJobs(t, m,
		store.Job{ID: "terminal-expired", Status: store.InFlight, LeaseExpiresAt: &expired, Retries: 3},
	)

	opts := claimOpts(1)
	opts.MaxRetries = 3

	claimed := m.ClaimAvailable(opts)
	if got := len(claimed); got != 0 {
		t.Fatalf("expected no claimed jobs, got %d", got)
	}

	all := byID(m.GetAllJobs())
	job := all["terminal-expired"]
	if job.Status != store.Failed {
		t.Fatalf("expected terminal status %s, got %s", store.Failed, job.Status)
	}
	if job.Retries != 4 {
		t.Fatalf("expected retries to increment to 4, got %d", job.Retries)
	}
	if job.RetryAt != nil {
		t.Fatalf("expected terminal expired job retryAt to be nil")
	}
	if job.LeaseExpiresAt != nil {
		t.Fatalf("expected terminal expired job lease to be cleared")
	}
}

func TestClaimAvailable_UsesConfiguredRetryFetchRatio(t *testing.T) {
	m := NewMemoryStore()
	now := time.Now().UTC()
	past := now.Add(-time.Minute)

	for i := range 10 {
		addJobs(t, m, store.Job{
			ID:      fmt.Sprintf("retry-%d", i+1),
			Status:  store.Failed,
			RetryAt: &past,
		})
	}

	for i := range 10 {
		addJobs(t, m, store.Job{
			ID:     fmt.Sprintf("ready-%d", i+1),
			Status: store.Ready,
		})
	}

	opts := claimOpts(10)
	opts.RetryFetchRatio = 0.50

	claimed := m.ClaimAvailable(opts)
	if got := len(claimed); got != 10 {
		t.Fatalf("expected 10 claimed jobs, got %d", got)
	}

	retryCount := 0
	readyCount := 0

	for _, job := range claimed {
		if strings.HasPrefix(job.ID, "retry-") {
			retryCount++
		}
		if strings.HasPrefix(job.ID, "ready-") {
			readyCount++
		}
	}

	if retryCount != 5 {
		t.Fatalf("expected 5 retry jobs from configured ratio, got %d", retryCount)
	}
	if readyCount != 5 {
		t.Fatalf("expected 5 fresh jobs from configured ratio, got %d", readyCount)
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

func claimOpts(limit int) store.ClaimOptions {
	return store.ClaimOptions{
		Limit:              limit,
		LeaseTTL:           testLeaseTTL,
		RetryFetchRatio:    0.20,
		MaxRetries:         3,
		RetryBackoffBaseMS: 500,
		RetryBackoffMaxMS:  30_000,
	}
}
