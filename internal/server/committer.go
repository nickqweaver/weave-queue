package server

import (
	"fmt"

	"github.com/nickqweaver/weave-queue/internal/store"
)

type Commiter struct {
	store store.Store
	res   <-chan Res
}

func NewCommiter(s store.Store, res chan Res) *Commiter {
	return &Commiter{
		store: s,
		res:   res,
	}
}

func (c *Commiter) batchWrite(batch []Res) {
	for _, j := range batch {
		if j.Status == Ack {
			c.store.UpdateJob(j.ID, store.JobUpdate{Status: store.Succeeded})
		} else {
			c.store.UpdateJob(j.ID, store.JobUpdate{Status: store.Failed})
		}
	}
}

func (c *Commiter) run() {
	batchSize := max(1, cap(c.res))

	batch := make([]Res, 0, batchSize)
	for r := range c.res {
		batch = append(batch, r)

		if len(batch) == batchSize {
			c.batchWrite(batch)
			fmt.Println("Writing a batch!", len(batch))
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		c.batchWrite(batch)
		fmt.Println("Flushing final batch!", len(batch))
	}

	fmt.Println("Committer stopped")
}
