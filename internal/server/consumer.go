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
}

func NewConsumer(concurrency int, req chan Req, res chan Res) *Consumer {
	return &Consumer{
		pending:     Pending{req: req, res: res},
		concurrency: concurrency,
	}
}

func (c *Consumer) Cleanup() {
	fmt.Println("Shutting down consumer, closing response channel")
	close(c.pending.res)
}

func (c *Consumer) Run(ctx context.Context, queue string) {
	defer c.Cleanup()
	var wg sync.WaitGroup
	wg.Add(c.concurrency)

	// Spawn the workers...
	for w := 1; w <= c.concurrency; w++ {
		go func(id int) {
			defer wg.Done()
			w := NewWorker(id, c.pending.req, c.pending.res)
			w.Run(ctx)
		}(w)
	}
	// Wait till all workers have finished
	wg.Wait()
}
