package main

import (
	queue "github.com/nickqweaver/weave-queue/internal"
	memory "github.com/nickqweaver/weave-queue/internal/store/adapters/memory"
)

func main() {
	mem := memory.NewMemoryStore()
	producer := queue.NewProducer(mem)

	for id := 1; id <= 1000000; id++ {
		producer.Enqueue("test", id)
	}

	jobs := make(chan *queue.Job)
	consumer := queue.Consumer{InFlight: jobs}
	consumer.Run("test", 10)
}
