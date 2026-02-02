package main

import (
	"strconv"

	queue "github.com/nickqweaver/weave-queue/internal"
	memory "github.com/nickqweaver/weave-queue/internal/store/adapters/memory"
)

func addJobs(n int, p queue.Producer) {
	for id := 1; id <= 100_000; id++ {
		p.Enqueue("test", strconv.Itoa(id))
	}
}

func main() {
	mem := memory.NewMemoryStore()
	producer := queue.NewProducer(mem)

	addJobs(100_000, producer)

	// go func() {
	// 	for tick := range time.Tick(30 * time.Second) {
	// 		fmt.Println("More Jobs Incomming", tick)
	// 		addJobs(100_000, producer)
	// 	}
	// }()
	// jobs := make(chan store.Job)
	consumer := queue.NewConsumer(mem, 10)
	consumer.Run("test", 10)
}
