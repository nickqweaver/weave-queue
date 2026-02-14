package client

import (
	"errors"

	"github.com/nickqweaver/weave-queue/internal/store"
)

type Producer struct {
	store store.Store
}

func (p Producer) Enqueue(queue string, id string) error {
	if queue == "" {
		return errors.New("queue name must not be empty")
	}
	if id == "" {
		return errors.New("job id must not be empty")
	}

	// Create Job in Database with data
	job := store.Job{ID: id, Status: store.Ready, Queue: queue}
	return p.store.AddJob(job)
}

func NewProducer(store store.Store) *Producer {
	return &Producer{
		store: store,
	}
}
