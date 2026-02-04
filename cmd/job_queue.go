package main

import (
	"fmt"
	"strconv"
	"time"

	queue "github.com/nickqweaver/weave-queue/internal"
	"github.com/nickqweaver/weave-queue/internal/store"
	memory "github.com/nickqweaver/weave-queue/internal/store/adapters/memory"
)

// func addJobs(n int, p queue.Producer) {
// 	for id := 1; id <= 100_000; id++ {
// 		p.Enqueue("test", strconv.Itoa(id))
// 	}
// }

func addJobs(n int, p store.Store) {
	for id := 1; id <= n; id++ {
		job := store.Job{ID: strconv.Itoa(id), Status: store.Ready, Queue: "my-queue"}
		p.AddJob(job)
	}
}

func main() {
	mem := memory.NewMemoryStore()
	// producer := queue.NewProducer(mem)

	// addJobs(100_000, producer)

	// go func() {
	// 	for tick := range time.Tick(30 * time.Second) {
	// 		fmt.Println("More Jobs Incomming", tick)
	// 		addJobs(100_000, producer)
	// 	}
	// }()
	// jobs := make(chan store.Job)
	// consumer := queue.NewConsumer(mem, 10)
	// consumer.Run("test", 10)

	// f := queue.Fetcher{BatchSize: 100, MaxRetries: 3, MaxColdTimeout: 30_000}

	// for id := 1; id <= 100_000; id++ {
	// 	q.Enqueue(id)
	// }
	server := queue.NewServer(mem)
	client := queue.NewClient(mem)

	server.Run()

	func() {
		for tick := range time.Tick(6 * time.Second) {
			fmt.Println("More Jobs Incomming", tick)
			for id := 1; id <= 1000; id++ {
				client.Enqueue(id)
			}
		}
	}()

	// f.Fetch(mem)
}
