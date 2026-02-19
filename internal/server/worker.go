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
	req <-chan Req
	res chan<- Res
	ID  int
}

func NewWorker(id int, req <-chan Req, res chan<- Res) *Worker {
	return &Worker{
		ID:  id,
		res: res,
		req: req,
	}
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

func (w *Worker) Run(ctx context.Context) {
	for r := range w.req {
		j := r.Job

		if j.LeasedAt == nil {
			response := Res{
				Status:  NAck,
				Message: "Job has not been leased",
				ID:      j.ID,
				Job:     j,
				From:    w.ID,
			}

			w.res <- response
		} else {
			deadline := j.LeasedAt.Add(time.Millisecond * time.Duration(j.Timeout))
			// Handler placeholder, should return ok, err then we can ack/nack based on that
			jobCtx, cancel := context.WithDeadline(ctx, deadline)
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
