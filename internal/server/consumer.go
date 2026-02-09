package server

import (
	"context"
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

func (c *Consumer) Run(ctx context.Context, queue string) {
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
}
