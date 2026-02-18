package memory

import (
	"fmt"
	"sync"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
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

func (n *MemoryStore) FetchAndClaim(curr store.Status, to store.Status, limit int) []store.Job {
	n.mu.Lock()
	defer n.mu.Unlock()
	var filtered []store.Job

	for i, j := range n.jobs {
		if j.Status == curr {
			now := time.Now().UTC()
			n.jobs[i].Status = to
			n.jobs[i].LeasedAt = &now
			filtered = append(filtered, n.jobs[i])
			if len(filtered) == limit {
				return filtered
			}
		}
	}

	// Fallback if we don't have n limit items to return
	return filtered
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
