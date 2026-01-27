package queue

import (
	"fmt"
	"time"
)

/*
*
So we have an entity Queue
1. add job
2. assign job (lease out job to worker)
3. tracks delivery (worker hasn't ack since timeout we will re assign)
4. Removes job/marks done/delete when ack
5. Re asssign when noAck
*/
type Status int

const (
	Ready Status = iota
	InFlight
	Failed
	Succeeded
)

/*
Producer creates jobs and assigns to a queue (string).
E.G -> Producer.Enqueue('image-queue', Job{})
*/

var store = []*Job{}

type Producer struct{}

func (p Producer) Enqueue(queue string, id int) {
	// Create Job in Database with data
	job := Job{ID: id, Status: Ready, Queue: queue}
	store = append(store, &job)
}

/*
Has to start some loop process that continually checks for jobs. So maybe we have a simple
Stores active jobs on consumer struct. This way we poll/batch jobs in memory. W
*/
type Job struct {
	ID     int
	Queue  string
	Status Status
}

type Consumer struct {
	reserved    []*Job
	InFlight    chan *Job
	concurrency int
}

var isCached = false

func doWork(job *Job) {
	result := 0
	if isCached {
		job.Status = Succeeded
		fmt.Println("Cache hit", job.ID)
		return
	}
	for i := 0; i < 5_000_000; i++ {
		result += i * i
	}
	job.Status = Succeeded
	fmt.Println("Completed Job", job.ID, "result", result)
	// isCached = true
}

func worker(id int, jobs <-chan *Job) {
	for j := range jobs {
		doWork(j)
	}
}

func (c *Consumer) Run(queue string, concurrency int) {
	// Initialize the jobs channel
	// Spawn the workers...
	for w := 1; w <= concurrency; w++ {
		go worker(w, c.InFlight)
	}

	fmt.Println("Spawned ", concurrency, " Workers")
	for {
		ready := []*Job{}

		// This wouldn't pull every job that is ready ideally we would batch them up
		// And we woudn't want to query every loop would we
		for _, job := range store {
			if job.Status == Ready {
				ready = append(ready, job)
			}
		}
		// If the inflight channel is empty queue up more, but also store would have to have some
		if len(c.InFlight) == 0 && len(ready) > 0 {
			limit := 10000
			if len(ready) < limit {
				limit = len(ready)
			}
			for _, job := range ready[:limit] {
				// Set io
				job.Status = InFlight
				c.InFlight <- job
			}
		} else {
			// Wait to re check
			fmt.Println("No jobs left waiting...")
			time.Sleep(250 * time.Millisecond)
		}
	}
}

// Add worker pool per consumer to pick jobs off concurrently
// So maybe you have your concurrency and options and shit in here
// func (c *Consumer) Run(queue string) {
// 	for {
// 		if len(c.reserved) > 0 {
// 			// Drain the queue, each iter we shift off the front, not great
// 			for len(c.reserved) > 0 {
// 				time.Sleep(250 * time.Millisecond)
// 				// This updates in the store cause we have a pointer
// 				c.reserved[0].Status = Succeeded
// 				fmt.Println("Completed Job", c.reserved[0].ID)
//
// 				snapshot := []Status{}
//
// 				for _, j := range store {
// 					snapshot = append(snapshot, j.Status)
// 				}
// 				fmt.Println("Snapshot", snapshot)
// 				// Remove from reserved, maintained in the store
// 				c.reserved = c.reserved[1:]
// 			}
// 		} else {
// 			// fetch jobs set them to pending/reserved
// 			ready := []*Job{}
//
// 			for _, job := range store {
// 				if job.Status == Ready {
// 					ready = append(ready, job)
// 				}
// 			}
//
// 			// Update all status to InFlight and send batch to consumer
// 			if len(ready) > 0 {
// 				for _, job := range ready {
// 					job.Status = InFlight
// 				}
// 				c.reserved = append(c.reserved, ready...)
// 			} else {
// 				// No jobs ready in the store, timeout
// 				// sleep 1000 to avoid query per loop
// 				time.Sleep(1000 * time.Millisecond)
// 				fmt.Println("No more jobs, waiting...")
// 			}
// 		}
// 	}
// }
