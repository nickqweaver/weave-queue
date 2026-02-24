package server

import (
	"context"
	"fmt"
	"os/signal"
	"sync"
	"syscall"

	"github.com/nickqweaver/weave-queue/internal/store"
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
	BatchSize       int
	MaxQueue        int
	MaxConcurrency  int
	MaxRetries      int
	MaxColdTimeout  int
	LeaseDurationMS int
}

func NewServer(s store.Store, c Config) Server {
	pending := make(chan Req, c.MaxQueue)
	finished := make(chan Res, c.BatchSize)

	consumer := NewConsumer(c.MaxConcurrency, pending, finished)
	committer := NewCommitter(s, finished, c.MaxRetries)
	fetcher := NewFetcher(pending, c.BatchSize, c.MaxRetries, c.MaxColdTimeout, c.LeaseDurationMS)
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
		s.committer.run()
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
