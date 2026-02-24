package server

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
)

func TestConsumerRun_DrainsRequestsAndClosesResponseChannel(t *testing.T) {
	const totalJobs = 25

	req := make(chan Req, totalJobs)
	res := make(chan Res, totalJobs)
	c := NewConsumer(3, req, res)

	now := time.Now().UTC()
	for i := 1; i <= totalJobs; i++ {
		req <- Req{Job: leasedJob(strconv.Itoa(i), now)}
	}
	close(req)

	done := make(chan struct{})
	go func() {
		c.Run(context.Background(), "my_queue")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not finish processing queued jobs")
	}

	responses := 0
	for range res {
		responses++
	}

	if responses != totalJobs {
		t.Fatalf("expected %d responses, got %d", totalJobs, responses)
	}

	if _, ok := <-res; ok {
		t.Fatal("expected response channel to be closed")
	}
}

func TestConsumerRun_NoDroppedOrDuplicateResponses(t *testing.T) {
	const (
		totalJobs   = 120
		concurrency = 4
	)

	req := make(chan Req, totalJobs)
	res := make(chan Res, totalJobs)
	c := NewConsumer(concurrency, req, res)

	now := time.Now().UTC()
	for i := 1; i <= totalJobs; i++ {
		req <- Req{Job: leasedJob(strconv.Itoa(i), now)}
	}
	close(req)

	done := make(chan struct{})
	go func() {
		c.Run(context.Background(), "my_queue")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not finish processing queued jobs")
	}

	counts := make(map[string]int, totalJobs)
	for r := range res {
		counts[r.ID]++
	}

	if len(counts) != totalJobs {
		t.Fatalf("expected %d unique response ids, got %d", totalJobs, len(counts))
	}

	for i := 1; i <= totalJobs; i++ {
		id := strconv.Itoa(i)
		if counts[id] != 1 {
			t.Fatalf("expected exactly one response for job %s, got %d", id, counts[id])
		}
	}
}

func leasedJob(id string, leasedAt time.Time) store.Job {
	lease := leasedAt.Add(time.Minute)
	return store.Job{
		ID:             id,
		Queue:          "my_queue",
		Status:         store.InFlight,
		Timeout:        60000,
		LeaseExpiresAt: &lease,
	}
}
