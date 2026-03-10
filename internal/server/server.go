package server

import (
	"context"
	"fmt"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
	"github.com/nickqweaver/weave-queue/internal/utils"
)

const (
	defaultLeaseTTL        = 15 * time.Second
	defaultRetryFetchRatio = 0.20
)

type Status int

const (
	Ack Status = iota
	NAck
)

type Res struct {
	Status  Status
	ID      string
	Job     store.Job
	Message string
	From    int
}

type Req struct {
	Job store.Job
}

type Server struct {
	store     store.Store
	fetcher   *Fetcher
	consumer  *Consumer
	committer *Committer

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

type Config struct {
	BatchSize      int
	MaxQueue       int
	MaxConcurrency int
	MaxColdTimeout int
	ClaimOptions   *store.ClaimOptions
}

type FetcherConfig struct {
	BatchSize          int
	MaxColdTimeout     int
	MaxRetries         int
	LeaseTTL           time.Duration
	RetryFetchRatio    float64
	RetryBackoffBaseMS int
	RetryBackoffMaxMS  int
}

type ConsumerConfig struct {
	MaxConcurrency int
	HeartbeatEvery time.Duration
}

type CommitterConfig struct {
	MaxRetries         int
	RetryBackoffBaseMS int
	RetryBackoffMaxMS  int
	LeaseTTL           time.Duration
}

type runtimeConfig struct {
	batchSize int
	maxQueue  int
	fetcher   FetcherConfig
	consumer  ConsumerConfig
	committer CommitterConfig
}

func NewServer(s store.Store, config Config) Server {
	rc := normalizeConfig(config)

	pending := make(chan Req, rc.maxQueue)
	finished := make(chan Res, rc.batchSize)
	heartbeat := make(chan HeartBeat)

	consumer := NewConsumer(rc.consumer, pending, finished, heartbeat)
	committer := NewCommitter(
		rc.committer,
		s,
		finished,
		heartbeat,
	)
	fetcher := NewFetcher(
		rc.fetcher,
		pending,
	)
	server := Server{store: s, consumer: consumer, fetcher: fetcher, committer: committer}

	return server
}

func normalizeConfig(config Config) runtimeConfig {
	claimOpts := store.ClaimOptions{}
	if config.ClaimOptions != nil {
		claimOpts = *config.ClaimOptions
	}

	leaseTTL := claimOpts.LeaseTTL
	if leaseTTL <= 0 {
		leaseTTL = defaultLeaseTTL
	}

	retryFetchRatio := claimOpts.RetryFetchRatio
	if retryFetchRatio <= 0 {
		retryFetchRatio = defaultRetryFetchRatio
	}
	if retryFetchRatio > 1 {
		retryFetchRatio = 1
	}

	retryBackoffBaseMS := claimOpts.RetryBackoffBaseMS
	if retryBackoffBaseMS <= 0 {
		retryBackoffBaseMS = utils.DefaultRetryBackoffBaseMS
	}

	retryBackoffMaxMS := claimOpts.RetryBackoffMaxMS
	if retryBackoffMaxMS <= 0 {
		retryBackoffMaxMS = utils.DefaultRetryBackoffMaxMS
	}

	heartbeatEvery := leaseTTL / 3
	if heartbeatEvery <= 0 {
		heartbeatEvery = leaseTTL
	}

	return runtimeConfig{
		batchSize: config.BatchSize,
		maxQueue:  config.MaxQueue,
		fetcher: FetcherConfig{
			BatchSize:          config.BatchSize,
			MaxColdTimeout:     config.MaxColdTimeout,
			MaxRetries:         claimOpts.MaxRetries,
			LeaseTTL:           leaseTTL,
			RetryFetchRatio:    retryFetchRatio,
			RetryBackoffBaseMS: retryBackoffBaseMS,
			RetryBackoffMaxMS:  retryBackoffMaxMS,
		},
		consumer: ConsumerConfig{
			MaxConcurrency: config.MaxConcurrency,
			HeartbeatEvery: heartbeatEvery,
		},
		committer: CommitterConfig{
			MaxRetries:         max(0, claimOpts.MaxRetries),
			RetryBackoffBaseMS: retryBackoffBaseMS,
			RetryBackoffMaxMS:  retryBackoffMaxMS,
			LeaseTTL:           leaseTTL,
		},
	}
}

func (s *Server) Run() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	s.done = make(chan struct{})
	s.cancel = stop
	done := s.done
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.mu.Unlock()
		close(done)
	}()

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		s.committer.run(ctx)
	}()

	go func() {
		defer wg.Done()
		s.consumer.Run(ctx, "my_queue")
	}()

	go func() {
		defer wg.Done()
		s.fetcher.fetch(ctx, s.store)
	}()

	<-ctx.Done()
	wg.Wait()
	s.Cleanup()
}

func (s *Server) Close() {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.mu.Unlock()

	if done == nil {
		return
	}

	if cancel != nil {
		cancel()
	}
	<-done
}

func (s *Server) Cleanup() {
	// cleanup will call all asset
	fmt.Println()
	fmt.Println("\nShutting down server..")
}
