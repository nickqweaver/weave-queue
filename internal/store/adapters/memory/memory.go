package memory

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
)

const retryFetchRatio = 0.20

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

func (n *MemoryStore) FetchAndClaim(curr store.Status, to store.Status, limit int) []store.Job {
	n.mu.Lock()
	defer n.mu.Unlock()

	if limit <= 0 {
		return nil
	}

	now := time.Now().UTC()
	claimed := make([]store.Job, 0, limit)
	retryLimit := max(1, int(math.Floor(float64(limit)*retryFetchRatio)))

	for i := range n.jobs {
		if len(claimed) >= retryLimit {
			break
		}

		job := n.jobs[i]
		if job.Status != store.Failed {
			continue
		}

		if job.RetryAt == nil || job.RetryAt.After(now) {
			continue
		}

		n.jobs[i].Status = to
		n.jobs[i].LeasedAt = &now
		n.jobs[i].RetryAt = nil
		claimed = append(claimed, n.jobs[i])
	}

	for i := range n.jobs {
		if len(claimed) == limit {
			break
		}

		if n.jobs[i].Status != curr {
			continue
		}

		n.jobs[i].Status = to
		n.jobs[i].LeasedAt = &now
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
