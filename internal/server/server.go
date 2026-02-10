package server

import (
	"context"
	"fmt"
	"os/signal"
	"sync"
	"syscall"
	"time"

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

func NewServer(s store.Store) Server {
	pending := make(chan Req, 1000)
	finished := make(chan Res, 100)

	consumer := NewConsumer(4, pending, finished)
	committer := NewCommitter(s, finished)
	fetcher := NewFetcher(pending, 250, 3, 5000)
	server := Server{store: s, consumer: consumer, fetcher: fetcher, committer: committer}
	fmt.Println("Created Server")

	return server
}

func (s *Server) Run() {
	fmt.Println("Running Server...")
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

	select {
	case <-ctx.Done():
		wg.Wait()
		s.Cleanup()
		fmt.Println("Goodbye")
	}
}

func (s *Server) Cleanup() {
	// cleanup will call all asset
	fmt.Println()
	fmt.Println("\nGracefully shutting down..")
	time.Sleep(time.Second * 5)
}
