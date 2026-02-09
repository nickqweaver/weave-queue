package client

import "github.com/nickqweaver/weave-queue/internal/store"

type Producer struct {
	store store.Store
}

func (p Producer) Enqueue(queue string, id string) {
	// Create Job in Database with data
	job := store.Job{ID: id, Status: store.Ready, Queue: queue}
	p.store.AddJob(job)
}

func NewProducer(store store.Store) *Producer {
	return &Producer{
		store: store,
	}
}
