package server

import (
	"testing"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
	memory "github.com/nickqweaver/weave-queue/internal/store/adapters/memory"
	"github.com/nickqweaver/weave-queue/internal/utils"
)

func TestNewServer_WiresComponentsAndChannelCapacities(t *testing.T) {
	mem := memory.NewMemoryStore()
	cfg := Config{
		BatchSize:          7,
		MaxQueue:           11,
		MaxConcurrency:     3,
		MaxRetries:         5,
		MaxColdTimeout:     9000,
		LeaseDurationMS:    5000,
		RetryFetchRatio:    0.35,
		RetryBackoffBaseMS: 700,
		RetryBackoffMaxMS:  40_000,
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
	if s.fetcher.LeaseDurationMS != cfg.LeaseDurationMS {
		t.Fatalf("expected fetcher lease duration %d, got %d", cfg.LeaseDurationMS, s.fetcher.LeaseDurationMS)
	}
	if s.fetcher.RetryFetchRatio != cfg.RetryFetchRatio {
		t.Fatalf("expected fetcher retry fetch ratio %.2f, got %.2f", cfg.RetryFetchRatio, s.fetcher.RetryFetchRatio)
	}
	if s.fetcher.RetryBackoffBaseMS != cfg.RetryBackoffBaseMS {
		t.Fatalf("expected fetcher retry backoff base %d, got %d", cfg.RetryBackoffBaseMS, s.fetcher.RetryBackoffBaseMS)
	}
	if s.fetcher.RetryBackoffMaxMS != cfg.RetryBackoffMaxMS {
		t.Fatalf("expected fetcher retry backoff max %d, got %d", cfg.RetryBackoffMaxMS, s.fetcher.RetryBackoffMaxMS)
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

func TestServerClose_ShutsDownRunAndIsIdempotent(t *testing.T) {
	mem := memory.NewMemoryStore()
	cfg := Config{
		BatchSize:          4,
		MaxQueue:           8,
		MaxConcurrency:     2,
		MaxRetries:         3,
		MaxColdTimeout:     500,
		LeaseDurationMS:    5000,
		RetryFetchRatio:    0.20,
		RetryBackoffBaseMS: 500,
		RetryBackoffMaxMS:  30_000,
	}

	s := NewServer(mem, cfg)

	runDone := make(chan struct{})
	go func() {
		s.Run()
		close(runDone)
	}()

	waitForServerStart(t, &s, time.Second)
	s.Close()

	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop after Close")
	}

	closeDone := make(chan struct{})
	go func() {
		s.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected second Close call to return immediately")
	}
}

func TestNewServer_DefaultsBackoffValuesWithoutClampingToBase(t *testing.T) {
	mem := memory.NewMemoryStore()

	s := NewServer(mem, Config{
		BatchSize:      4,
		MaxQueue:       8,
		MaxConcurrency: 2,
	})

	if s.fetcher.RetryFetchRatio != defaultRetryFetchRatio {
		t.Fatalf("expected default retry fetch ratio %.2f, got %.2f", defaultRetryFetchRatio, s.fetcher.RetryFetchRatio)
	}

	if s.fetcher.RetryBackoffBaseMS != utils.DefaultRetryBackoffBaseMS {
		t.Fatalf("expected default retry backoff base %d, got %d", utils.DefaultRetryBackoffBaseMS, s.fetcher.RetryBackoffBaseMS)
	}
	if s.fetcher.RetryBackoffMaxMS != utils.DefaultRetryBackoffMaxMS {
		t.Fatalf("expected default retry backoff max %d, got %d", utils.DefaultRetryBackoffMaxMS, s.fetcher.RetryBackoffMaxMS)
	}
	if s.committer.retryBackoffBaseMS != utils.DefaultRetryBackoffBaseMS {
		t.Fatalf("expected committer default retry backoff base %d, got %d", utils.DefaultRetryBackoffBaseMS, s.committer.retryBackoffBaseMS)
	}
	if s.committer.retryBackoffMaxMS != utils.DefaultRetryBackoffMaxMS {
		t.Fatalf("expected committer default retry backoff max %d, got %d", utils.DefaultRetryBackoffMaxMS, s.committer.retryBackoffMaxMS)
	}

	s = NewServer(mem, Config{
		BatchSize:          4,
		MaxQueue:           8,
		MaxConcurrency:     2,
		RetryFetchRatio:    0.60,
		RetryBackoffBaseMS: 5000,
		RetryBackoffMaxMS:  1000,
	})

	if s.fetcher.RetryFetchRatio != 0.60 {
		t.Fatalf("expected retry fetch ratio %.2f, got %.2f", 0.60, s.fetcher.RetryFetchRatio)
	}

	if s.fetcher.RetryBackoffBaseMS != 5000 {
		t.Fatalf("expected retry backoff base %d, got %d", 5000, s.fetcher.RetryBackoffBaseMS)
	}
	if s.fetcher.RetryBackoffMaxMS != 1000 {
		t.Fatalf("expected retry backoff max %d, got %d", 1000, s.fetcher.RetryBackoffMaxMS)
	}
	if s.committer.retryBackoffBaseMS != 5000 {
		t.Fatalf("expected committer retry backoff base %d, got %d", 5000, s.committer.retryBackoffBaseMS)
	}
	if s.committer.retryBackoffMaxMS != 1000 {
		t.Fatalf("expected committer retry backoff max %d, got %d", 1000, s.committer.retryBackoffMaxMS)
	}
}

func waitForServerStart(t *testing.T, s *Server, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		running := s.cancel != nil
		s.mu.Unlock()

		if running {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("server never started")
}
