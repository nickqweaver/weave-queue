package server

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
)

type Worker struct {
	req       <-chan Req
	res       chan<- Res
	ID        int
	heartbeat chan HeartBeat
	beatEvery time.Duration
}

func doWork(ctx context.Context, job store.Job) (bool, error) {
	result := 0
	for i := range 5_000 {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			result += i * i
		}
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

// In here we would set a tick that pings heartbeat every configurale second
func (w *Worker) Run(ctx context.Context) {
	for r := range w.req {
		j := r.Job

		if j.LeaseExpiresAt == nil {
			response := Res{
				Status:  NAck,
				Message: "Job has not been leased",
				ID:      j.ID,
				Job:     j,
				From:    w.ID,
			}

			w.res <- response
		} else {
			jobCtx, cancel := context.WithTimeout(ctx, j.Timeout)
			hbDone := make(chan struct{})

			// Spawn a new goroutine to run the ticker, make sure we kill the goroutine if ctx/heartbeat is done
			// This seems repetive/redundant I bet there is a better way look at refactor later
			go func(jobID string) {
				ticker := time.NewTicker(w.beatEvery)
				defer ticker.Stop()

				for {
					select {
					case <-jobCtx.Done():
						return
					case <-hbDone:
						return
					case <-ticker.C:
						select {
						case w.heartbeat <- HeartBeat{Worker: w.ID, Job: j.ID}:
						case <-jobCtx.Done():
							return
						case <-hbDone:
							return
						}
					}
				}
			}(j.ID)

			// Handler placeholder, should return ok, err then we can ack/nack based on that
			if _, err := doWork(jobCtx, j); err != nil {
				response := Res{
					Status:  NAck,
					Message: err.Error(),
					ID:      j.ID,
					Job:     j,
					From:    w.ID,
				}

				w.res <- response
			} else {
				response := Res{
					Status:  Ack,
					Message: fmt.Sprintf("Successfully completed Job %s", j.ID),
					ID:      j.ID,
					Job:     j,
					From:    w.ID,
				}
				w.res <- response
			}
			cancel()
		}

	}
}
