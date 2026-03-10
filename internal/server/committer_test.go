package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
	memory "github.com/nickqweaver/weave-queue/internal/store/adapters/memory"
)

func TestCommitterBatchWrite_MapsAckAndNackStatuses(t *testing.T) {
	s := newCommitterStoreStub("job-1", "job-2")
	c := NewCommitter(newCommitterConfig(3), s, make(chan Res, 1), make(chan HeartBeat, 1))
	before := time.Now().UTC()

	c.batchWrite([]Res{
		{ID: "job-1", Status: Ack, Job: store.Job{ID: "job-1", Retries: 0}},
		{ID: "job-2", Status: NAck, Job: store.Job{ID: "job-2", Retries: 0}},
	})

	job, ok := s.job("job-1")
	if !ok {
		t.Fatalf("job-1 not found in store")
	}
	if job.Status != store.Succeeded {
		t.Fatalf("expected job-1 status %s, got %s", store.Succeeded, job.Status)
	}
	if job.RetryAt != nil {
		t.Fatalf("expected job-1 retryAt to be nil after ack")
	}

	job, ok = s.job("job-2")
	if !ok {
		t.Fatalf("job-2 not found in store")
	}
	if job.Status != store.Failed {
		t.Fatalf("expected job-2 status %s, got %s", store.Failed, job.Status)
	}
	if job.Retries != 1 {
		t.Fatalf("expected job-2 retries to increment to 1, got %d", job.Retries)
	}
	if job.RetryAt == nil {
		t.Fatalf("expected job-2 retryAt to be scheduled")
	}
	if !job.RetryAt.After(before) {
		t.Fatalf("expected job-2 retryAt %v to be after %v", job.RetryAt, before)
	}
}

func TestCommitterRun_FlushesFinalPartialBatchOnClose(t *testing.T) {
	s := newCommitterStoreStub("1", "2", "3")
	res := make(chan Res, 4)
	heartbeat := make(chan HeartBeat, 1)
	c := NewCommitter(newCommitterConfig(3), s, res, heartbeat)

	res <- Res{ID: "1", Status: Ack, Job: store.Job{ID: "1", Retries: 0}}
	res <- Res{ID: "2", Status: Ack, Job: store.Job{ID: "2", Retries: 0}}
	res <- Res{ID: "3", Status: NAck, Job: store.Job{ID: "3", Retries: 1}}
	close(res)
	close(heartbeat)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.run(ctx)

	if s.updateCount() != 3 {
		t.Fatalf("expected 3 update calls, got %d", s.updateCount())
	}

	job, _ := s.job("1")
	if job.Status != store.Succeeded {
		t.Fatalf("expected job 1 status %s, got %s", store.Succeeded, job.Status)
	}
	job, _ = s.job("2")
	if job.Status != store.Succeeded {
		t.Fatalf("expected job 2 status %s, got %s", store.Succeeded, job.Status)
	}
	job, _ = s.job("3")
	if job.Status != store.Failed {
		t.Fatalf("expected job 3 status %s, got %s", store.Failed, job.Status)
	}
	if job.Retries != 2 {
		t.Fatalf("expected job 3 retries to increment to 2, got %d", job.Retries)
	}
	if job.RetryAt == nil {
		t.Fatalf("expected job 3 retryAt to be scheduled")
	}
}

func TestCommitterBatchWrite_ContinuesAfterUpdateError(t *testing.T) {
	s := newCommitterStoreStub("1", "3")
	s.setFailure("2", errors.New("forced update error"))
	c := NewCommitter(newCommitterConfig(3), s, make(chan Res, 1), make(chan HeartBeat, 1))

	c.batchWrite([]Res{
		{ID: "1", Status: Ack, Job: store.Job{ID: "1", Retries: 0}},
		{ID: "2", Status: Ack, Job: store.Job{ID: "2", Retries: 0}},
		{ID: "3", Status: NAck, Job: store.Job{ID: "3", Retries: 0}},
	})

	if s.updateCount() != 3 {
		t.Fatalf("expected 3 update calls, got %d", s.updateCount())
	}

	job, _ := s.job("1")
	if job.Status != store.Succeeded {
		t.Fatalf("expected job 1 status %s, got %s", store.Succeeded, job.Status)
	}
	job, _ = s.job("3")
	if job.Status != store.Failed {
		t.Fatalf("expected job 3 status %s, got %s", store.Failed, job.Status)
	}
	if job.Retries != 1 {
		t.Fatalf("expected job 3 retries to increment to 1, got %d", job.Retries)
	}
}

func TestCommitterBatchWrite_StopsRetryingAtMaxRetries(t *testing.T) {
	const maxRetries = 3

	s := newCommitterStoreStub("job-1")
	seed := store.Job{ID: "job-1", Status: store.InFlight, Retries: maxRetries}
	if err := s.AddJob(seed); err != nil {
		t.Fatalf("failed seeding job: %v", err)
	}

	c := NewCommitter(newCommitterConfig(maxRetries), s, make(chan Res, 1), make(chan HeartBeat, 1))
	c.batchWrite([]Res{
		{ID: "job-1", Status: NAck, Job: seed},
	})

	job, ok := s.job("job-1")
	if !ok {
		t.Fatalf("job-1 not found in store")
	}
	if job.Status != store.Failed {
		t.Fatalf("expected terminal status %s, got %s", store.Failed, job.Status)
	}
	if job.Retries != maxRetries+1 {
		t.Fatalf("expected retries to increment to %d, got %d", maxRetries+1, job.Retries)
	}
	if job.RetryAt != nil {
		t.Fatalf("expected retryAt to be nil once max retries is exceeded")
	}
}

func TestCommitterRun_HeartbeatRenewsLease(t *testing.T) {
	mem := memory.NewMemoryStore()
	res := make(chan Res)
	heartbeat := make(chan HeartBeat, 1)
	leaseTTL := 50 * time.Millisecond
	committer := NewCommitter(CommitterConfig{
		MaxRetries:         3,
		LeaseTTL:           leaseTTL,
		RetryBackoffBaseMS: 500,
		RetryBackoffMaxMS:  30_000,
	}, mem, res, heartbeat)

	now := time.Now().UTC()
	initialLease := now.Add(10 * time.Millisecond)
	job := store.Job{ID: "job-1", Status: store.InFlight, LeaseExpiresAt: &initialLease}
	if err := mem.AddJob(job); err != nil {
		t.Fatalf("failed to seed job: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	close(res)
	go func() {
		committer.run(ctx)
		close(runDone)
	}()

	beforeRenewal := time.Now().UTC()
	heartbeat <- HeartBeat{Worker: 1, Job: job.ID}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		stored := jobsByID(mem.GetAllJobs())[job.ID]
		if stored.LeaseExpiresAt != nil && stored.LeaseExpiresAt.After(beforeRenewal.Add(leaseTTL-10*time.Millisecond)) {
			cancel()
			close(heartbeat)
			<-runDone
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	close(heartbeat)
	<-runDone
	t.Fatal("expected heartbeat to renew lease")
}

func TestHeartbeatPreventsFalseReclaimUntilHeartbeatsStop(t *testing.T) {
	mem := memory.NewMemoryStore()
	res := make(chan Res)
	heartbeat := make(chan HeartBeat, 4)
	leaseTTL := 30 * time.Millisecond
	committer := NewCommitter(CommitterConfig{
		MaxRetries:         3,
		LeaseTTL:           leaseTTL,
		RetryBackoffBaseMS: 500,
		RetryBackoffMaxMS:  30_000,
	}, mem, res, heartbeat)

	now := time.Now().UTC()
	initialLease := now.Add(15 * time.Millisecond)
	job := store.Job{ID: "job-1", Status: store.InFlight, LeaseExpiresAt: &initialLease}
	if err := mem.AddJob(job); err != nil {
		t.Fatalf("failed to seed job: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	close(res)
	go func() {
		committer.run(ctx)
		close(runDone)
	}()

	claimOpts := store.ClaimOptions{
		Limit:              1,
		LeaseTTL:           leaseTTL,
		MaxRetries:         3,
		RetryFetchRatio:    0.20,
		RetryBackoffBaseMS: 500,
		RetryBackoffMaxMS:  30_000,
	}

	for i := 0; i < 3; i++ {
		time.Sleep(10 * time.Millisecond)
		heartbeat <- HeartBeat{Worker: 1, Job: job.ID}
		time.Sleep(10 * time.Millisecond)

		claimed := mem.ClaimAvailable(claimOpts)
		if len(claimed) != 0 {
			cancel()
			close(heartbeat)
			<-runDone
			t.Fatalf("expected no jobs claimed while heartbeats are active, got %d", len(claimed))
		}

		stored := jobsByID(mem.GetAllJobs())[job.ID]
		if stored.Status != store.InFlight {
			cancel()
			close(heartbeat)
			<-runDone
			t.Fatalf("expected job to remain %s during active heartbeats, got %s", store.InFlight, stored.Status)
		}
	}

	time.Sleep(leaseTTL + 10*time.Millisecond)
	claimed := mem.ClaimAvailable(claimOpts)
	if len(claimed) != 0 {
		cancel()
		close(heartbeat)
		<-runDone
		t.Fatalf("expected no jobs claimed after expiry recovery, got %d", len(claimed))
	}

	stored := jobsByID(mem.GetAllJobs())[job.ID]
	if stored.Status != store.Failed {
		cancel()
		close(heartbeat)
		<-runDone
		t.Fatalf("expected job to be recovered as %s after heartbeats stop, got %s", store.Failed, stored.Status)
	}
	if stored.Retries != 1 {
		cancel()
		close(heartbeat)
		<-runDone
		t.Fatalf("expected recovered job retries to be 1, got %d", stored.Retries)
	}
	if stored.RetryAt == nil {
		cancel()
		close(heartbeat)
		<-runDone
		t.Fatal("expected recovered job retryAt to be scheduled")
	}

	cancel()
	close(heartbeat)
	<-runDone
}

type committerStoreStub struct {
	mu      sync.Mutex
	jobs    map[string]store.Job
	failIDs map[string]error
	updates int
}

func newCommitterStoreStub(ids ...string) *committerStoreStub {
	jobs := make(map[string]store.Job, len(ids))
	for _, id := range ids {
		jobs[id] = store.Job{ID: id, Status: store.Ready}
	}

	return &committerStoreStub{
		jobs:    jobs,
		failIDs: make(map[string]error),
	}
}

func newCommitterConfig(maxRetries int) CommitterConfig {
	return CommitterConfig{
		MaxRetries:         maxRetries,
		LeaseTTL:           15 * time.Second,
		RetryBackoffBaseMS: 500,
		RetryBackoffMaxMS:  30_000,
	}
}

func jobsByID(jobs []store.Job) map[string]store.Job {
	indexed := make(map[string]store.Job, len(jobs))
	for _, job := range jobs {
		indexed[job.ID] = job
	}
	return indexed
}

func (s *committerStoreStub) setFailure(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failIDs[id] = err
}

func (s *committerStoreStub) job(id string) (store.Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	return job, ok
}

func (s *committerStoreStub) updateCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updates
}

func (s *committerStoreStub) FetchJobs(status store.Status, limit int) []store.Job {
	return nil
}

func (s *committerStoreStub) ClaimAvailable(opts store.ClaimOptions) []store.Job {
	return nil
}

func (s *committerStoreStub) FailJob(id string) error {
	return nil
}

func (s *committerStoreStub) AddJob(job store.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return nil
}

func (s *committerStoreStub) UpdateJob(id string, update store.JobUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates++

	if err, ok := s.failIDs[id]; ok {
		return err
	}

	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	job.Status = update.Status
	if update.Retries != nil {
		job.Retries = *update.Retries
	}
	job.RetryAt = update.RetryAt
	s.jobs[id] = job
	return nil
}

func (s *committerStoreStub) GetAllJobs() []store.Job {
	return nil
}
