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

func NewServer(s store.Store, config Config) Server {
	c := config.ClaimOptions
	leaseTTL := c.LeaseTTL
	if leaseTTL <= 0 {
		leaseTTL = defaultLeaseTTL
	}

	retryFetchRatio := c.RetryFetchRatio
	if retryFetchRatio <= 0 {
		retryFetchRatio = defaultRetryFetchRatio
	}
	if retryFetchRatio > 1 {
		retryFetchRatio = 1
	}

	retryBackoffBaseMS := c.RetryBackoffBaseMS
	if retryBackoffBaseMS <= 0 {
		retryBackoffBaseMS = utils.DefaultRetryBackoffBaseMS
	}

	retryBackoffMaxMS := c.RetryBackoffMaxMS
	if retryBackoffMaxMS <= 0 {
		retryBackoffMaxMS = utils.DefaultRetryBackoffMaxMS
	}

	pending := make(chan Req, config.MaxQueue)
	finished := make(chan Res, config.BatchSize)
	heartbeat := make(chan HeartBeat)

	consumer := NewConsumer(&config, pending, finished, heartbeat)
	committer := NewCommitter(
		&config,
		s,
		finished,
		heartbeat,
	)
	fetcher := NewFetcher(
		&config,
		pending,
	)
	server := Server{store: s, consumer: consumer, fetcher: fetcher, committer: committer}

	return server
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
