package server

import (
	"context"
	"fmt"
	"sync"
)

type Pending struct {
	req <-chan Req
	res chan<- Res
}

type Consumer struct {
	pending     Pending
	concurrency int
	heartbeat   chan HeartBeat
	worker      WorkerConfig
}

type HeartBeat struct {
	Worker     int
	Job        string
	LeaseToken int
}

func NewConsumer(
	opts ConsumerConfig,
	req chan Req,
	res chan Res,
	heartbeat chan HeartBeat,
) *Consumer {
	return &Consumer{
		pending:     Pending{req: req, res: res},
		concurrency: opts.MaxConcurrency,
		heartbeat:   heartbeat,
		worker:      opts.Worker,
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
				beatEvery: c.worker.HeartbeatEvery,
			}
			w.Run(ctx)
		}(w)
	}
	// Wait till all workers have finished
	wg.Wait()
}
