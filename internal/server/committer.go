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
	go func() {
		var batch []Res
		for r := range c.res {
			batch = append(batch, r)

			if len(batch) == cap(c.res) {
				// batch update
				c.batchWrite(batch)
				fmt.Println("Writing a batch!", len(batch))
				// Reset batch
				batch = []Res{}
			}
		}
	}()
}
