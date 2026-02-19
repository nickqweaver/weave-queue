package server

import (
	"testing"

	"github.com/nickqweaver/weave-queue/internal/store"
	memory "github.com/nickqweaver/weave-queue/internal/store/adapters/memory"
)

func TestNewServer_WiresComponentsAndChannelCapacities(t *testing.T) {
	mem := memory.NewMemoryStore()
	cfg := Config{
		BatchSize:      7,
		MaxQueue:       11,
		MaxConcurrency: 3,
		MaxRetries:     5,
		MaxColdTimeout: 9000,
	}

	s := NewServer(mem, cfg)

	if s.fetcher == nil {
		t.Fatal("expected fetcher to be initialized")
	}
	if s.consumer == nil {
		t.Fatal("expected consumer to be initialized")
	}
	if s.committer == nil {
		t.Fatal("expected committer to be initialized")
	}

	if s.fetcher.BatchSize != cfg.BatchSize {
		t.Fatalf("expected fetcher batch size %d, got %d", cfg.BatchSize, s.fetcher.BatchSize)
	}
	if s.fetcher.MaxRetries != cfg.MaxRetries {
		t.Fatalf("expected fetcher max retries %d, got %d", cfg.MaxRetries, s.fetcher.MaxRetries)
	}
	if s.fetcher.MaxColdTimeout != cfg.MaxColdTimeout {
		t.Fatalf("expected fetcher cold timeout %d, got %d", cfg.MaxColdTimeout, s.fetcher.MaxColdTimeout)
	}
	if s.consumer.concurrency != cfg.MaxConcurrency {
		t.Fatalf("expected consumer concurrency %d, got %d", cfg.MaxConcurrency, s.consumer.concurrency)
	}

	if cap(s.fetcher.pending) != cfg.MaxQueue {
		t.Fatalf("expected pending channel cap %d, got %d", cfg.MaxQueue, cap(s.fetcher.pending))
	}
	if cap(s.consumer.pending.req) != cfg.MaxQueue {
		t.Fatalf("expected consumer req channel cap %d, got %d", cfg.MaxQueue, cap(s.consumer.pending.req))
	}
	if cap(s.consumer.pending.res) != cfg.BatchSize {
		t.Fatalf("expected consumer res channel cap %d, got %d", cfg.BatchSize, cap(s.consumer.pending.res))
	}
	if cap(s.committer.res) != cfg.BatchSize {
		t.Fatalf("expected committer res channel cap %d, got %d", cfg.BatchSize, cap(s.committer.res))
	}

	s.fetcher.pending <- Req{Job: store.Job{ID: "req-wire"}}
	req := <-s.consumer.pending.req
	if req.Job.ID != "req-wire" {
		t.Fatalf("expected req id %q, got %q", "req-wire", req.Job.ID)
	}

	s.consumer.pending.res <- Res{ID: "res-wire", Status: Ack}
	res := <-s.committer.res
	if res.ID != "res-wire" {
		t.Fatalf("expected res id %q, got %q", "res-wire", res.ID)
	}
	if res.Status != Ack {
		t.Fatalf("expected res status %v, got %v", Ack, res.Status)
	}
}
