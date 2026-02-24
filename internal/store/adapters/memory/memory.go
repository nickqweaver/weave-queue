package memory

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
	"github.com/nickqweaver/weave-queue/internal/utils"
)

type MemoryStore struct {
	mu   sync.Mutex
	jobs []store.Job
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs: []store.Job{},
	}
}

func (n *MemoryStore) FetchJobs(status store.Status, limit int) []store.Job {
	n.mu.Lock()
	defer n.mu.Unlock()
	var filtered []store.Job

	for i, j := range n.jobs {
		if j.Status == status {
			filtered = append(filtered, n.jobs[i])
			if len(filtered) == limit {
				return filtered
			}
		}
	}

	// Fallback if we don't have n limit items to return
	return filtered
}

func (n *MemoryStore) ClaimAvailable(opts store.ClaimOptions) []store.Job {
	n.mu.Lock()
	defer n.mu.Unlock()

	if opts.Limit <= 0 {
		return nil
	}

	maxRetries := max(0, opts.MaxRetries)
	retryFetchRatio := opts.RetryFetchRatio
	if retryFetchRatio < 0 {
		retryFetchRatio = 0
	}
	if retryFetchRatio > 1 {
		retryFetchRatio = 1
	}
	now := time.Now().UTC()
	n.recoverExpiredLeasesLocked(now, maxRetries, opts.RetryBackoffBaseMS, opts.RetryBackoffMaxMS)

	leaseExpiresAt := now.Add(time.Millisecond * time.Duration(max(0, opts.LeaseDurationMS)))
	claimed := make([]store.Job, 0, opts.Limit)
	retryTarget := int(math.Floor(float64(opts.Limit) * retryFetchRatio))
	if retryFetchRatio > 0 && retryTarget == 0 {
		retryTarget = 1
	}
	retryTarget = min(opts.Limit, retryTarget)

	for i := range n.jobs {
		if len(claimed) >= retryTarget {
			break
		}

		job := n.jobs[i]
		if !isDueRetry(job, now) {
			continue
		}

		n.jobs[i].Status = store.InFlight
		n.jobs[i].LeaseExpiresAt = &leaseExpiresAt
		n.jobs[i].RetryAt = nil
		claimed = append(claimed, n.jobs[i])
	}

	for i := range n.jobs {
		if len(claimed) == opts.Limit {
			break
		}

		if n.jobs[i].Status != store.Ready {
			continue
		}

		n.jobs[i].Status = store.InFlight
		n.jobs[i].LeaseExpiresAt = &leaseExpiresAt
		claimed = append(claimed, n.jobs[i])
	}

	for i := range n.jobs {
		if len(claimed) == opts.Limit {
			break
		}

		job := n.jobs[i]
		if !isDueRetry(job, now) {
			continue
		}

		n.jobs[i].Status = store.InFlight
		n.jobs[i].LeaseExpiresAt = &leaseExpiresAt
		n.jobs[i].RetryAt = nil
		claimed = append(claimed, n.jobs[i])
	}

	return claimed
}

func (n *MemoryStore) FailJob(id string) error {
	return nil
}

func (n *MemoryStore) AddJob(job store.Job) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.jobs = append(n.jobs, job)
	return nil
}

func (n *MemoryStore) UpdateJob(id string, update store.JobUpdate) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i := range n.jobs {
		if n.jobs[i].ID == id {
			n.jobs[i].Status = update.Status
			if update.Retries != nil {
				n.jobs[i].Retries = *update.Retries
			}
			n.jobs[i].RetryAt = update.RetryAt
			return nil
		}
	}
	return fmt.Errorf("job not found: %s", id)
}

func (n *MemoryStore) GetAllJobs() []store.Job {
	n.mu.Lock()
	defer n.mu.Unlock()

	result := make([]store.Job, len(n.jobs))
	copy(result, n.jobs)
	return result
}

func (n *MemoryStore) recoverExpiredLeasesLocked(now time.Time, maxRetries int, retryBackoffBaseMS int, retryBackoffMaxMS int) {
	for i := range n.jobs {
		if n.jobs[i].Status != store.InFlight {
			continue
		}

		if n.jobs[i].LeaseExpiresAt == nil || n.jobs[i].LeaseExpiresAt.After(now) {
			continue
		}

		n.jobs[i].LeaseExpiresAt = nil

		nextRetries := n.jobs[i].Retries + 1
		n.jobs[i].Retries = nextRetries
		n.jobs[i].Status = store.Failed

		if nextRetries <= maxRetries {
			retryAt := now.Add(utils.RetryDelay(nextRetries, retryBackoffBaseMS, retryBackoffMaxMS))
			n.jobs[i].RetryAt = &retryAt
			continue
		}

		n.jobs[i].RetryAt = nil
	}
}

func isDueRetry(job store.Job, now time.Time) bool {
	if job.Status != store.Failed {
		return false
	}

	if job.RetryAt == nil {
		return false
	}

	return !job.RetryAt.After(now)
}
