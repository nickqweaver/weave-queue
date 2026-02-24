package server

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
	memory "github.com/nickqweaver/weave-queue/internal/store/adapters/memory"
)

func TestWorkerRun_TenJobs_OneTimeout(t *testing.T) {
	const totalJobs = 10

	req := make(chan Req, totalJobs)
	res := make(chan Res, totalJobs)
	w := NewWorker(1, req, res)

	jobs, timedOutID := jobsWithOneExpiredLease(totalJobs, 4)
	for _, job := range jobs {
		req <- Req{Job: job}
	}
	close(req)

	w.Run(context.Background())

	ackCount := 0
	nackCount := 0
	var timedOutRes Res
	foundTimedOut := false

	for i := 0; i < totalJobs; i++ {
		select {
		case r := <-res:
			if r.Status == Ack {
				ackCount++
			} else {
				nackCount++
			}

			if r.ID == timedOutID {
				timedOutRes = r
				foundTimedOut = true
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for worker response %d/%d", i+1, totalJobs)
		}
	}

	if ackCount != totalJobs-1 {
		t.Fatalf("expected %d acks, got %d", totalJobs-1, ackCount)
	}
	if nackCount != 1 {
		t.Fatalf("expected 1 nack, got %d", nackCount)
	}
	if !foundTimedOut {
		t.Fatalf("did not receive response for timed out job %s", timedOutID)
	}
	if timedOutRes.Status != NAck {
		t.Fatalf("expected timed out job %s to be nacked, got status %v", timedOutID, timedOutRes.Status)
	}
	if !strings.Contains(timedOutRes.Message, context.DeadlineExceeded.Error()) {
		t.Fatalf("expected timeout message to contain %q, got %q", context.DeadlineExceeded.Error(), timedOutRes.Message)
	}
}

func TestWorkerRun_JobWithoutLeaseIsNacked(t *testing.T) {
	const totalJobs = 3

	req := make(chan Req, totalJobs)
	res := make(chan Res, totalJobs)
	w := NewWorker(1, req, res)

	now := time.Now().UTC()
	leaseExpiresAt := now.Add(5 * time.Second)
	missingLeaseID := "7"

	jobs := []store.Job{
		{ID: "1", Queue: "my_queue", Status: store.InFlight, Timeout: 5000, LeaseExpiresAt: &leaseExpiresAt},
		{ID: missingLeaseID, Queue: "my_queue", Status: store.InFlight, Timeout: 5000, LeaseExpiresAt: nil},
		{ID: "3", Queue: "my_queue", Status: store.InFlight, Timeout: 5000, LeaseExpiresAt: &leaseExpiresAt},
	}

	for _, job := range jobs {
		req <- Req{Job: job}
	}
	close(req)

	w.Run(context.Background())

	ackCount := 0
	nackCount := 0
	missingLeaseRes := Res{}
	foundMissingLease := false

	for i := 0; i < totalJobs; i++ {
		select {
		case r := <-res:
			if r.Status == Ack {
				ackCount++
			} else {
				nackCount++
			}

			if r.ID == missingLeaseID {
				missingLeaseRes = r
				foundMissingLease = true
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for worker response %d/%d", i+1, totalJobs)
		}
	}

	if ackCount != 2 {
		t.Fatalf("expected 2 acks, got %d", ackCount)
	}
	if nackCount != 1 {
		t.Fatalf("expected 1 nack, got %d", nackCount)
	}
	if !foundMissingLease {
		t.Fatalf("did not receive response for unleased job %s", missingLeaseID)
	}
	if missingLeaseRes.Status != NAck {
		t.Fatalf("expected unleased job %s to be nacked, got status %v", missingLeaseID, missingLeaseRes.Status)
	}
	if missingLeaseRes.Message != "Job has not been leased" {
		t.Fatalf("expected unleased job message %q, got %q", "Job has not been leased", missingLeaseRes.Message)
	}
}

func TestWorkerAndCommitter_TenJobs_OneTimeoutMarksFailed(t *testing.T) {
	const totalJobs = 10

	mem := memory.NewMemoryStore()
	req := make(chan Req, totalJobs)
	res := make(chan Res, totalJobs)
	w := NewWorker(1, req, res)
	committer := NewCommitter(mem, res, 3, 500, 30_000)

	jobs, timedOutID := jobsWithOneExpiredLease(totalJobs, 6)
	for _, job := range jobs {
		if err := mem.AddJob(job); err != nil {
			t.Fatalf("failed to seed job %s: %v", job.ID, err)
		}
		req <- Req{Job: job}
	}
	close(req)

	beforeCommit := time.Now().UTC()
	w.Run(context.Background())
	close(res)
	committer.run()

	allJobs := mem.GetAllJobs()
	if len(allJobs) != totalJobs {
		t.Fatalf("expected %d jobs in store, got %d", totalJobs, len(allJobs))
	}

	succeeded := 0
	failed := 0
	timedOutJob := store.Job{}
	foundTimedOut := false

	for _, job := range allJobs {
		switch job.Status {
		case store.Succeeded:
			succeeded++
		case store.Failed:
			failed++
		}

		if job.ID == timedOutID {
			timedOutJob = job
			foundTimedOut = true
		}
	}

	if succeeded != totalJobs-1 {
		t.Fatalf("expected %d succeeded jobs, got %d", totalJobs-1, succeeded)
	}
	if failed != 1 {
		t.Fatalf("expected 1 failed job, got %d", failed)
	}
	if !foundTimedOut {
		t.Fatalf("timed out job %s not found in store", timedOutID)
	}
	if timedOutJob.Status != store.Failed {
		t.Fatalf("expected timed out job %s to be failed, got %s", timedOutID, timedOutJob.Status)
	}
	if timedOutJob.Retries != 1 {
		t.Fatalf("expected timed out job %s retries to be 1, got %d", timedOutID, timedOutJob.Retries)
	}
	if timedOutJob.RetryAt == nil {
		t.Fatalf("expected timed out job %s retryAt to be scheduled", timedOutID)
	}
	if !timedOutJob.RetryAt.After(beforeCommit) {
		t.Fatalf("expected timed out job %s retryAt %v to be after %v", timedOutID, timedOutJob.RetryAt, beforeCommit)
	}
}

func jobsWithOneExpiredLease(total int, expiredIndex int) ([]store.Job, string) {
	now := time.Now().UTC()
	jobs := make([]store.Job, 0, total)
	timedOutID := ""

	for i := 0; i < total; i++ {
		id := strconv.Itoa((i * 2) + 1)
		leaseExpiresAt := now.Add(5 * time.Second)
		timeout := 5000

		if i == expiredIndex {
			leaseExpiresAt = now.Add(-2 * time.Second)
			timeout = 1
			timedOutID = id
		}

		lease := leaseExpiresAt
		jobs = append(jobs, store.Job{
			ID:             id,
			Queue:          "my_queue",
			Status:         store.InFlight,
			Timeout:        timeout,
			LeaseExpiresAt: &lease,
		})
	}

	return jobs, timedOutID
}
