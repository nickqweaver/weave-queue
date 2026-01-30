package main

import (
	"strconv"

	queue "github.com/nickqweaver/weave-queue/internal"
	memory "github.com/nickqweaver/weave-queue/internal/store/adapters/memory"
)

func main() {
	mem := memory.NewMemoryStore()
	producer := queue.NewProducer(mem)

	for id := 1; id <= 100_000; id++ {
		producer.Enqueue("test", strconv.Itoa(id))
	}

	// jobs := make(chan store.Job)
	consumer := queue.NewConsumer(mem, 10)
	consumer.Run("test", 10)
}
