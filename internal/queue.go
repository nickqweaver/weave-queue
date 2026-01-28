package queue

import (
	"fmt"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
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

var storage = []*Job{}

type Producer struct {
	store store.Store
}

func (p Producer) Enqueue(queue string, id string) {
	// Create Job in Database with data
	job := store.Job{ID: id, Status: store.Ready, Queue: queue}
	p.store.AddJob(job)
}

func NewProducer(store store.Store) Producer {
	return Producer{
		store: store,
	}
}

type Job struct {
	ID     int
	Queue  string
	Status Status
}

type Consumer struct {
	InFlight    chan store.Job
	concurrency int
	store       store.Store
}

func doWork(job store.Job) int {
	result := 0
	for i := range 5_000 {
		result += i * i
	}
	return result
}

func worker(id int, c *Consumer) {
	for j := range c.InFlight {
		// Pass result from callback here (or not actually )
		doWork(j)
		c.store.UpdateJob(j.ID, store.UpdateJob{Status: store.Succeeded})
	}
}

func NewConsumer(s store.Store, concurrency int) Consumer {
	jobs := make(chan store.Job)
	return Consumer{
		InFlight:    jobs,
		store:       s,
		concurrency: concurrency,
	}
}

func (c *Consumer) Run(queue string, concurrency int) {
	// Initialize the jobs channel
	// Spawn the workers...
	for w := 1; w <= concurrency; w++ {
		go worker(w, c)
	}

	for {
		ready := []*Job{}

		// This wouldn't pull every job that is ready ideally we would batch them up
		// And we woudn't want to query every loop would we
		for _, job := range storage {
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
