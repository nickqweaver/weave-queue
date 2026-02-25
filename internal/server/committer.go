package server

import (
	"context"
	"fmt"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
	"github.com/nickqweaver/weave-queue/internal/utils"
)

type Committer struct {
	store              store.Store
	res                <-chan Res
	heartbeat          <-chan HeartBeat
	maxRetries         int
	retryBackoffBaseMS int
	retryBackoffMaxMS  int
}

func NewCommitter(
	s store.Store,
	res chan Res,
	maxRetries int,
	retryBackoffBaseMS int,
	retryBackoffMaxMS int,
	heartbeat chan HeartBeat,
) *Committer {
	return &Committer{
		store:              s,
		res:                res,
		heartbeat:          heartbeat,
		maxRetries:         max(0, maxRetries),
		retryBackoffBaseMS: retryBackoffBaseMS,
		retryBackoffMaxMS:  retryBackoffMaxMS,
	}
}

func (c *Committer) Cleanup() {
	fmt.Println("Shutting down committer...")
}

func (c *Committer) batchWrite(batch []Res) {
	for _, r := range batch {
		jobID := r.ID
		if r.Job.ID != "" {
			jobID = r.Job.ID
		}

		if jobID == "" {
			fmt.Printf("Error updating job: missing job id in response %+v\n", r)
			continue
		}

		update := store.JobUpdate{Status: store.Succeeded}

		if r.Status != Ack {
			nextRetries := r.Job.Retries + 1

			if nextRetries <= c.maxRetries {
				retryAt := time.Now().
					UTC().
					Add(utils.RetryDelay(nextRetries, c.retryBackoffBaseMS, c.retryBackoffMaxMS))
				update = store.JobUpdate{
					Status:  store.Failed,
					Retries: &nextRetries,
					RetryAt: &retryAt,
				}
			} else {
				update = store.JobUpdate{Status: store.Failed}
			}
		}

		var err error
		err = c.store.UpdateJob(jobID, update)
		if err != nil {
			fmt.Printf("Error updating job %s: %v\n", jobID, err)
		}
	}
}

func (c *Committer) run(ctx context.Context) {
	defer c.Cleanup()
	batchSize := max(1, cap(c.res))
	batch := make([]Res, 0, batchSize)

	activeRes := c.res
	activeHeartbeat := c.heartbeat

	for activeRes != nil || activeHeartbeat != nil {
		select {
		case r, ok := <-activeRes:
			if !ok {
				activeRes = nil
				activeHeartbeat = c.heartbeat
				continue
			}

			batch = append(batch, r)

			if len(batch) == batchSize {
				c.batchWrite(batch)
				fmt.Println("Writing a batch!", len(batch))
				batch = batch[:0]
			}

		case hb, ok := <-activeHeartbeat:
			if !ok {
				activeHeartbeat = nil
				continue
			}

			ttl := defaultLeaseTTL // this should be from config
			now := time.Now().UTC()
			leaseExpiresAt := now.Add(ttl)
			jobID := fmt.Sprintf("%d", hb.jobId)
			if err := c.store.UpdateJob(
				jobID,
				store.JobUpdate{Status: store.InFlight, LeaseExpiresAt: &leaseExpiresAt},
			); err != nil {
				fmt.Printf("Error updating heartbeat lease for job %s: %v\n", jobID, err)
			}
		}
	}

	if len(batch) > 0 {
		c.batchWrite(batch)
		fmt.Println("Flushing final batch!", len(batch))
	}

	<-ctx.Done()
	fmt.Println("Committer stopped")
}
