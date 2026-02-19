package server

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/nickqweaver/weave-queue/internal/store"
)

func TestCommitterBatchWrite_MapsAckAndNackStatuses(t *testing.T) {
	s := newCommitterStoreStub("job-1", "job-2")
	c := NewCommitter(s, make(chan Res, 1))

	c.batchWrite([]Res{
		{ID: "job-1", Status: Ack},
		{ID: "job-2", Status: NAck},
	})

	status, ok := s.status("job-1")
	if !ok {
		t.Fatalf("job-1 not found in store")
	}
	if status != store.Succeeded {
		t.Fatalf("expected job-1 status %s, got %s", store.Succeeded, status)
	}

	status, ok = s.status("job-2")
	if !ok {
		t.Fatalf("job-2 not found in store")
	}
	if status != store.Failed {
		t.Fatalf("expected job-2 status %s, got %s", store.Failed, status)
	}
}

func TestCommitterRun_FlushesFinalPartialBatchOnClose(t *testing.T) {
	s := newCommitterStoreStub("1", "2", "3")
	res := make(chan Res, 4)
	c := NewCommitter(s, res)

	res <- Res{ID: "1", Status: Ack}
	res <- Res{ID: "2", Status: Ack}
	res <- Res{ID: "3", Status: NAck}
	close(res)

	c.run()

	if s.updateCount() != 3 {
		t.Fatalf("expected 3 update calls, got %d", s.updateCount())
	}

	status, _ := s.status("1")
	if status != store.Succeeded {
		t.Fatalf("expected job 1 status %s, got %s", store.Succeeded, status)
	}
	status, _ = s.status("2")
	if status != store.Succeeded {
		t.Fatalf("expected job 2 status %s, got %s", store.Succeeded, status)
	}
	status, _ = s.status("3")
	if status != store.Failed {
		t.Fatalf("expected job 3 status %s, got %s", store.Failed, status)
	}
}

func TestCommitterBatchWrite_ContinuesAfterUpdateError(t *testing.T) {
	s := newCommitterStoreStub("1", "3")
	s.setFailure("2", errors.New("forced update error"))
	c := NewCommitter(s, make(chan Res, 1))

	c.batchWrite([]Res{
		{ID: "1", Status: Ack},
		{ID: "2", Status: Ack},
		{ID: "3", Status: NAck},
	})

	if s.updateCount() != 3 {
		t.Fatalf("expected 3 update calls, got %d", s.updateCount())
	}

	status, _ := s.status("1")
	if status != store.Succeeded {
		t.Fatalf("expected job 1 status %s, got %s", store.Succeeded, status)
	}
	status, _ = s.status("3")
	if status != store.Failed {
		t.Fatalf("expected job 3 status %s, got %s", store.Failed, status)
	}
}

type committerStoreStub struct {
	mu       sync.Mutex
	statuses map[string]store.Status
	failIDs  map[string]error
	updates  int
}

func newCommitterStoreStub(ids ...string) *committerStoreStub {
	statuses := make(map[string]store.Status, len(ids))
	for _, id := range ids {
		statuses[id] = store.Ready
	}

	return &committerStoreStub{
		statuses: statuses,
		failIDs:  make(map[string]error),
	}
}

func (s *committerStoreStub) setFailure(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failIDs[id] = err
}

func (s *committerStoreStub) status(id string) (store.Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, ok := s.statuses[id]
	return status, ok
}

func (s *committerStoreStub) updateCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updates
}

func (s *committerStoreStub) FetchJobs(status store.Status, limit int) []store.Job {
	return nil
}

func (s *committerStoreStub) FetchAndClaim(curr store.Status, to store.Status, limit int) []store.Job {
	return nil
}

func (s *committerStoreStub) FailJob(id string) error {
	return nil
}

func (s *committerStoreStub) AddJob(job store.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses[job.ID] = job.Status
	return nil
}

func (s *committerStoreStub) UpdateJob(id string, update store.JobUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates++

	if err, ok := s.failIDs[id]; ok {
		return err
	}

	if _, ok := s.statuses[id]; !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	s.statuses[id] = update.Status
	return nil
}

func (s *committerStoreStub) GetAllJobs() []store.Job {
	return nil
}
