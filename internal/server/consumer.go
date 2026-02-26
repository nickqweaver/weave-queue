package server

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Pending struct {
	req <-chan Req
	res chan<- Res
}

type Consumer struct {
	pending     Pending
	concurrency int
	heartbeat   chan HeartBeat
	beatEvery   time.Duration
}

type HeartBeat struct {
	Worker int
	Job    string
}

func NewConsumer(opts *Config, req chan Req, res chan Res, heartbeat chan HeartBeat) *Consumer {
	beatEvery := opts.ClaimOptions.LeaseTTL / 3
	return &Consumer{
		pending:     Pending{req: req, res: res},
		concurrency: opts.MaxConcurrency,
		heartbeat:   heartbeat,
		beatEvery:   beatEvery,
	}
}

func (c *Consumer) Cleanup() {
	fmt.Println("Shutting down consumer, closing response channel")
	close(c.pending.res)
	close(c.heartbeat)
}

func (c *Consumer) Run(ctx context.Context, queue string) {
	defer c.Cleanup()
	var wg sync.WaitGroup
	wg.Add(c.concurrency)

	// Spawn the workers...
	for w := 1; w <= c.concurrency; w++ {
		go func(id int) {
			defer wg.Done()
			w := Worker{
				ID:        id,
				req:       c.pending.req,
				res:       c.pending.res,
				heartbeat: c.heartbeat,
				beatEvery: c.beatEvery,
			}
			w.Run(ctx)
		}(w)
	}
	// Wait till all workers have finished
	wg.Wait()
}
