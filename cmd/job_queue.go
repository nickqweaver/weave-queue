package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/nickqweaver/weave-queue/internal/client"
	"github.com/nickqweaver/weave-queue/internal/server"
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
		if err := p.AddJob(job); err != nil {
			log.Printf("Failed to add job %d: %v", id, err)
		}
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
	// server := queue.NewServer(mem)
	// client := queue.NewClient(mem)
	// for id := 1; id <= 100000; id++ {
	// 	client.Enqueue(id)
	// }
	//
	// server.Run()

	server := server.NewServer(mem, server.Config{
		MaxColdTimeout: 5000,
		BatchSize:      100,
		MaxQueue:       1000,
		MaxConcurrency: 4,
		MaxRetries:     3,
	})
	client := client.NewClient(mem)
	for id := 1; id <= 10000; id++ {
		if err := client.Enqueue("my_queue", strconv.Itoa(id)); err != nil {
			log.Printf("Failed to enqueue job %d: %v", id, err)
		}
	}
	nextID := 10001

	go func() {
		ticker := time.NewTicker(6 * time.Second)
		defer ticker.Stop()

		for tick := range ticker.C {
			fmt.Println("More Jobs Incoming", tick)
			for i := 0; i < 10_000; i++ {
				if err := client.Enqueue("my_queue", strconv.Itoa(nextID)); err != nil {
					log.Printf("Failed to enqueue job %d: %v", nextID, err)
				}
				nextID++
			}
		}
	}()

	server.Run()
	// f.Fetch(mem)
}

// TODO Next Steps
// config options for hard coded values
// stub out temperature states
// Determine if the for select default in fetcher is best practice
// Notify fetcher when more jobs get enqueued how?
// Lease stuff for retries

// Then next phase is tasks, how we are going to pass real tasks on the queue
// How we wanna shape our Task Registry, etc..
