package memory

import "github.com/nickqweaver/weave-queue/internal/store"

type MemoryStore struct {
	jobs []store.Job
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs: []store.Job{},
	}
}

func (n *MemoryStore) FetchJobs(status store.Status, offset int, limit int) []store.Job {
	return n.jobs
}

func (n *MemoryStore) FailJob(id string) {}

func (n *MemoryStore) AddJob(job store.Job) {
	n.jobs = append(n.jobs, job)
}

func (n *MemoryStore) UpdateJob(id string, update store.UpdateJob) {
	jobs := n.jobs
	for i, v := range jobs {
		if jobs[i].ID == id {
			jobs[i].Status = update.Status
			return
		}
	}
}
