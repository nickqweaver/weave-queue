package server

import (
	"context"
	"fmt"
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
	"github.com/nickqweaver/weave-queue/internal/utils"
)

type Fetcher struct {
	BatchSize          int
	MaxRetries         int
	MaxColdTimeout     int
	LeaseDurationMS    int
	RetryFetchRatio    float64
	RetryBackoffBaseMS int
	RetryBackoffMaxMS  int
	pending            chan Req
}

func NewFetcher(pending chan Req, batchSize int, maxRetries int, maxColdTimeout int, leaseDurationMS int, retryFetchRatio float64, retryBackoffBaseMS int, retryBackoffMaxMS int) *Fetcher {
	return &Fetcher{
		BatchSize:          batchSize,
		MaxColdTimeout:     maxColdTimeout,
		MaxRetries:         maxRetries,
		LeaseDurationMS:    leaseDurationMS,
		RetryFetchRatio:    retryFetchRatio,
		RetryBackoffBaseMS: retryBackoffBaseMS,
		RetryBackoffMaxMS:  retryBackoffMaxMS,
		pending:            pending,
	}
}

func (f *Fetcher) Cleanup() {
	close(f.pending)
}

// TODO:
// Need to determine priority of how we are going to requeue failed jobs
func (f *Fetcher) fetch(ctx context.Context, s store.Store) {
	missed := 0
	wait := 100
	timeout := time.Duration(0)
	defer f.Cleanup()

	for {
		if ctx.Err() != nil {
			fmt.Println("Shutting Fetcher Down...")
			return
		}

		ready := s.ClaimAvailable(store.ClaimOptions{
			Limit:              f.BatchSize,
			LeaseDurationMS:    f.LeaseDurationMS,
			RetryFetchRatio:    f.RetryFetchRatio,
			MaxRetries:         f.MaxRetries,
			RetryBackoffBaseMS: f.RetryBackoffBaseMS,
			RetryBackoffMaxMS:  f.RetryBackoffMaxMS,
		})
		fmt.Println("Fetching More Jobs...")

		if len(ready) == 0 {
			missed++
		} else {
			// Send to pending Queue
			for _, j := range ready {
				select {
				case <-ctx.Done():
					fmt.Println("Shutting Fetcher Down...")
					return
				case f.pending <- Req{Job: j}:
				}
			}
			// Reset
			missed = 0
			wait = 100
			timeout = time.Duration(0)
		}

		if missed > 1 {
			wait, timeout = utils.Backoff(wait, f.MaxColdTimeout, true)
			t := time.NewTimer(time.Millisecond * timeout)
			select {
			case <-ctx.Done():
				t.Stop()
				fmt.Println("Shutting Fetcher Down...")
				return
			case <-t.C:
			}
		}

	}
}
