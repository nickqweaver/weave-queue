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

func doWork(job *Job) {
	result := 0
	job.Status = Succeeded

	for i := range 5_000 {
		result += i * i
	}
	job.Status = Succeeded
	fmt.Println("Completed Job", job.ID, "result", result)
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
			limit := min(len(ready), 10000)
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
