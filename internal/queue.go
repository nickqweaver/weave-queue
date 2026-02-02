package queue

import (
	"errors"
	"fmt"
	"strconv"
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
// type Status int
//
// const (
// 	Ready Status = iota
// 	InFlight
// 	Failed
// 	Succeeded
// )

/*
Producer creates jobs and assigns to a queue (string).
E.G -> Producer.Enqueue('image-queue', Job{})
*/

// var storage = []*Job{}

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

// Unknowns
// API not sure I love
// Can't shut down?
// Workers stay open forever?
// Better way to batch enqueu jobs?
// TODO: We need to revisit select patterns and really understand when stuff is blocking

// TODO:
// Offset/limit in memory

// Where can we fail/hang/stop?
/**
1. Job can take too long (exceed timeoutIn)
		- queue should update job status to retry, and increment retries
2. Job can panic/throw -> Worker should nack
		- worker should notify queue nack/reject
		- queue should update to failed or send to dlq
3. Worker never acks/Queue never recieves ack
		- Timeout will trigger, job we will re run (therefor jobs should be idempotent)
4. Process exits
		- No longer accept any more jobs in flight, gracefully finish remaining jobs in workers and shutdown all workers
5. Process crashes
		- Jobs should be persisted in flight status during crash. If this happens on reboot we should prioritize in flight jobs first
*/

type Status int

const (
	Ack Status = iota
	NAck
)

type Res struct {
	Status  Status
	ID      string
	Message string
	From    int
}

type Req struct {
	Job store.Job
}

type InFlight struct {
	Req chan Req
	Res chan Res
}

type Consumer struct {
	InFlight    InFlight
	concurrency int
	store       store.Store
}

func doWork(job store.Job) (bool, error) {
	result := 0
	for i := range 5_000 {
		result += i * i
	}
	n, err := strconv.Atoi(job.ID)
	if err != nil {
		return false, errors.New("Failed to convert string to int")
	}

	if n%2 == 0 {
		return false, errors.New("Failed job cause its even")
	}

	return true, nil
}

// Maybe we change this to a req/response multi channel
// Then worker can recieve jobs on the inflight channel and push responses out nack/ack
func worker(id int, req <-chan Req, res chan<- Res) {
	// TODO: do something with the worker ID
	for r := range req {
		j := r.Job
		// Handler placeholder, should return ok, err then we can ack/nack based on that
		if _, err := doWork(j); err != nil {
			response := Res{
				Status:  NAck,
				Message: err.Error(),
				ID:      j.ID,
				From:    id,
			}

			res <- response
		} else {

			response := Res{
				Status:  Ack,
				Message: fmt.Sprintf("Successfully completed Job %s", j.ID),
				ID:      j.ID,
				From:    id,
			}
			res <- response
		}

	}
}

// TODO: Experiment with having a few channels compared to just using a mutex (pull lock, update, release)
// TODO: Also think we should have queue and change API to have Task instead of this consumer/producer theoretical type api
func NewConsumer(s store.Store, concurrency int) Consumer {
	req := make(chan Req, 1000)
	res := make(chan Res, 50)

	var batch []Res

	batchWrite := func(res []Res) {
		for _, r := range res {
			if r.Status == Ack {
				s.UpdateJob(r.ID, store.JobUpdate{Status: store.Succeeded})
			} else {
				s.UpdateJob(r.ID, store.JobUpdate{Status: store.Failed})
			}
		}
	}

	go func() {
		for r := range res {
			batch = append(batch, r)

			if len(batch) == cap(res) {
				// batch update
				batchWrite(batch)
				fmt.Println("Writing a batch!", len(batch))
				batch = []Res{}
			}
		}
	}()

	return Consumer{
		InFlight:    InFlight{Req: req, Res: res},
		store:       s,
		concurrency: concurrency,
	}
}

func (c *Consumer) Run(queue string, concurrency int) {
	// Initialize the jobs channel
	// Spawn the workers...
	for w := 1; w <= concurrency; w++ {
		go worker(w, c.InFlight.Req, c.InFlight.Res)
	}

	for {
		ready := []store.Job{}
		// We only fetch new jobs when the buffer has >= 50% capacity
		if len(c.InFlight.Req) < 500 {
			for _, job := range c.store.FetchJobs(store.Ready, 100) {
				if job.Status == store.Ready {
					ready = append(ready, job)
				}
			}
			limit := min(len(ready), 100)
			// We would batch update the store here
			for _, job := range ready[:limit] {
				c.store.UpdateJob(job.ID, store.JobUpdate{Status: store.InFlight})
			}
			// Now we are safe to send these jobs
			for _, job := range ready[:limit] {
				c.InFlight.Req <- Req{Job: job}
			}

			// Prevent Store Hammer
			time.Sleep(10 * time.Millisecond)
		}
	}
}
