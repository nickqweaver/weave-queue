package server

import (
	"strings"
	"testing"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
	memory "github.com/nickqweaver/weave-queue/internal/store/adapters/memory"
)

func TestNewServer_WiresComponentsAndChannelCapacities(t *testing.T) {
	mem := memory.NewMemoryStore()
	cfg := Config{
		BatchSize:      7,
		MaxQueue:       11,
		MaxConcurrency: 3,
		MaxColdTimeout: 9000,
		ClaimOptions: &store.ClaimOptions{
			MaxRetries:         5,
			LeaseTTL:           5 * time.Second,
			RetryFetchRatio:    0.35,
			RetryBackoffBaseMS: 700,
			RetryBackoffMaxMS:  40_000,
		},
	}

	s, err := NewServer(mem, cfg)
	if err != nil {
		t.Fatalf("expected server to build, got error: %v", err)
	}

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
	if s.fetcher.MaxRetries != cfg.ClaimOptions.MaxRetries {
		t.Fatalf("expected fetcher max retries %d, got %d", cfg.ClaimOptions.MaxRetries, s.fetcher.MaxRetries)
	}
	if s.fetcher.MaxColdTimeout != cfg.MaxColdTimeout {
		t.Fatalf("expected fetcher cold timeout %d, got %d", cfg.MaxColdTimeout, s.fetcher.MaxColdTimeout)
	}
	if s.fetcher.LeaseTTL != cfg.ClaimOptions.LeaseTTL {
		t.Fatalf("expected fetcher lease ttl %v, got %v", cfg.ClaimOptions.LeaseTTL, s.fetcher.LeaseTTL)
	}
	if s.fetcher.RetryFetchRatio != cfg.ClaimOptions.RetryFetchRatio {
		t.Fatalf("expected fetcher retry fetch ratio %.2f, got %.2f", cfg.ClaimOptions.RetryFetchRatio, s.fetcher.RetryFetchRatio)
	}
	if s.fetcher.RetryBackoffBaseMS != cfg.ClaimOptions.RetryBackoffBaseMS {
		t.Fatalf("expected fetcher retry backoff base %d, got %d", cfg.ClaimOptions.RetryBackoffBaseMS, s.fetcher.RetryBackoffBaseMS)
	}
	if s.fetcher.RetryBackoffMaxMS != cfg.ClaimOptions.RetryBackoffMaxMS {
		t.Fatalf("expected fetcher retry backoff max %d, got %d", cfg.ClaimOptions.RetryBackoffMaxMS, s.fetcher.RetryBackoffMaxMS)
	}
	if s.consumer.concurrency != cfg.MaxConcurrency {
		t.Fatalf("expected consumer concurrency %d, got %d", cfg.MaxConcurrency, s.consumer.concurrency)
	}
	if s.consumer.worker.HeartbeatEvery != cfg.ClaimOptions.LeaseTTL/3 {
		t.Fatalf("expected consumer heartbeat interval %v, got %v", cfg.ClaimOptions.LeaseTTL/3, s.consumer.worker.HeartbeatEvery)
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
		BatchSize:      4,
		MaxQueue:       8,
		MaxConcurrency: 2,
		MaxColdTimeout: 500,
		ClaimOptions: &store.ClaimOptions{
			MaxRetries:         3,
			LeaseTTL:           5 * time.Second,
			RetryFetchRatio:    0.20,
			RetryBackoffBaseMS: 500,
			RetryBackoffMaxMS:  30_000,
		},
	}

	s, err := NewServer(mem, cfg)
	if err != nil {
		t.Fatalf("expected server to build, got error: %v", err)
	}

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

func TestNewServer_NormalizesClaimOptionsOnce(t *testing.T) {
	mem := memory.NewMemoryStore()

	s, err := NewServer(mem, Config{
		BatchSize:      4,
		MaxQueue:       8,
		MaxConcurrency: 2,
		MaxColdTimeout: 500,
		ClaimOptions:   &store.ClaimOptions{},
	})
	if err != nil {
		t.Fatalf("expected server to build, got error: %v", err)
	}

	if s.fetcher.RetryFetchRatio != defaultRetryFetchRatio {
		t.Fatalf("expected retry fetch ratio %.2f, got %.2f", defaultRetryFetchRatio, s.fetcher.RetryFetchRatio)
	}

	if s.fetcher.RetryBackoffBaseMS != 500 {
		t.Fatalf("expected retry backoff base %d, got %d", 500, s.fetcher.RetryBackoffBaseMS)
	}
	if s.fetcher.RetryBackoffMaxMS != 30_000 {
		t.Fatalf("expected retry backoff max %d, got %d", 30_000, s.fetcher.RetryBackoffMaxMS)
	}
	if s.committer.retryBackoffBaseMS != 500 {
		t.Fatalf("expected committer retry backoff base %d, got %d", 500, s.committer.retryBackoffBaseMS)
	}
	if s.committer.retryBackoffMaxMS != 30_000 {
		t.Fatalf("expected committer retry backoff max %d, got %d", 30_000, s.committer.retryBackoffMaxMS)
	}
	if s.fetcher.LeaseTTL != defaultLeaseTTL {
		t.Fatalf("expected lease ttl %v, got %v", defaultLeaseTTL, s.fetcher.LeaseTTL)
	}
	if s.consumer.worker.HeartbeatEvery != defaultLeaseTTL/3 {
		t.Fatalf("expected heartbeat interval %v, got %v", defaultLeaseTTL/3, s.consumer.worker.HeartbeatEvery)
	}

	s, err = NewServer(mem, Config{
		BatchSize:      4,
		MaxQueue:       8,
		MaxConcurrency: 2,
		MaxColdTimeout: 500,
		ClaimOptions: &store.ClaimOptions{
			RetryFetchRatio:    0.60,
			RetryBackoffBaseMS: 5000,
			RetryBackoffMaxMS:  10000,
		},
	})
	if err != nil {
		t.Fatalf("expected server to build, got error: %v", err)
	}

	if s.fetcher.RetryFetchRatio != 0.60 {
		t.Fatalf("expected retry fetch ratio %.2f, got %.2f", 0.60, s.fetcher.RetryFetchRatio)
	}

	if s.fetcher.RetryBackoffBaseMS != 5000 {
		t.Fatalf("expected retry backoff base %d, got %d", 5000, s.fetcher.RetryBackoffBaseMS)
	}
	if s.fetcher.RetryBackoffMaxMS != 10000 {
		t.Fatalf("expected retry backoff max %d, got %d", 10000, s.fetcher.RetryBackoffMaxMS)
	}
	if s.committer.retryBackoffBaseMS != 5000 {
		t.Fatalf("expected committer retry backoff base %d, got %d", 5000, s.committer.retryBackoffBaseMS)
	}
	if s.committer.retryBackoffMaxMS != 10000 {
		t.Fatalf("expected committer retry backoff max %d, got %d", 10000, s.committer.retryBackoffMaxMS)
	}
}

func TestNewServer_HandlesNilClaimOptions(t *testing.T) {
	mem := memory.NewMemoryStore()

	s, err := NewServer(mem, Config{
		BatchSize:      4,
		MaxQueue:       8,
		MaxConcurrency: 2,
		MaxColdTimeout: 500,
	})
	if err != nil {
		t.Fatalf("expected server to build, got error: %v", err)
	}

	if s.fetcher.LeaseTTL != defaultLeaseTTL {
		t.Fatalf("expected default lease ttl %v, got %v", defaultLeaseTTL, s.fetcher.LeaseTTL)
	}
	if s.consumer.worker.HeartbeatEvery != defaultLeaseTTL/3 {
		t.Fatalf("expected default heartbeat interval %v, got %v", defaultLeaseTTL/3, s.consumer.worker.HeartbeatEvery)
	}
}

func TestNewServer_RejectsInvalidConfig(t *testing.T) {
	mem := memory.NewMemoryStore()

	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "missing batch size",
			cfg:  Config{MaxQueue: 8, MaxConcurrency: 2, MaxColdTimeout: 500},
			want: "BatchSize",
		},
		{
			name: "missing queue size",
			cfg:  Config{BatchSize: 4, MaxConcurrency: 2, MaxColdTimeout: 500},
			want: "MaxQueue",
		},
		{
			name: "missing concurrency",
			cfg:  Config{BatchSize: 4, MaxQueue: 8, MaxColdTimeout: 500},
			want: "MaxConcurrency",
		},
		{
			name: "missing cold timeout",
			cfg:  Config{BatchSize: 4, MaxQueue: 8, MaxConcurrency: 2},
			want: "MaxColdTimeout",
		},
		{
			name: "negative max retries",
			cfg:  Config{BatchSize: 4, MaxQueue: 8, MaxConcurrency: 2, MaxColdTimeout: 500, ClaimOptions: &store.ClaimOptions{MaxRetries: -1}},
			want: "MaxRetries",
		},
		{
			name: "retry ratio over one",
			cfg:  Config{BatchSize: 4, MaxQueue: 8, MaxConcurrency: 2, MaxColdTimeout: 500, ClaimOptions: &store.ClaimOptions{RetryFetchRatio: 1.2}},
			want: "RetryFetchRatio",
		},
		{
			name: "negative lease ttl",
			cfg:  Config{BatchSize: 4, MaxQueue: 8, MaxConcurrency: 2, MaxColdTimeout: 500, ClaimOptions: &store.ClaimOptions{LeaseTTL: -time.Second}},
			want: "LeaseTTL",
		},
		{
			name: "invalid backoff range",
			cfg:  Config{BatchSize: 4, MaxQueue: 8, MaxConcurrency: 2, MaxColdTimeout: 500, ClaimOptions: &store.ClaimOptions{RetryBackoffBaseMS: 5000, RetryBackoffMaxMS: 1000}},
			want: "RetryBackoffBaseMS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewServer(mem, tt.cfg)
			if err == nil {
				t.Fatal("expected config validation error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error to mention %q, got %v", tt.want, err)
			}
		})
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
