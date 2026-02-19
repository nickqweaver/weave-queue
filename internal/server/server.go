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
	// Close fn to shut everything down gracefully
}

type Config struct {
	BatchSize      int
	MaxQueue       int
	MaxConcurrency int
	MaxRetries     int
	MaxColdTimeout int
}

func NewServer(s store.Store, c Config) Server {
	pending := make(chan Req, c.MaxQueue)
	finished := make(chan Res, c.BatchSize)

	consumer := NewConsumer(c.MaxConcurrency, pending, finished)
	committer := NewCommitter(s, finished)
	fetcher := NewFetcher(pending, c.BatchSize, c.MaxRetries, c.MaxColdTimeout)
	server := Server{store: s, consumer: consumer, fetcher: fetcher, committer: committer}

	return server
}

func (s *Server) Run() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

func (s *Server) Cleanup() {
	// cleanup will call all asset
	fmt.Println()
	fmt.Println("\nShutting down server..")
}
