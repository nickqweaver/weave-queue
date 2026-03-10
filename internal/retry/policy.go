package retry

import (
	"time"

	"github.com/nickqweaver/weave-queue/internal/store"
	"github.com/nickqweaver/weave-queue/internal/utils"
)

type Config struct {
	MaxRetries    int
	BackoffBaseMS int
	BackoffMaxMS  int
}

func FailureUpdate(job store.Job, now time.Time, cfg Config) store.JobUpdate {
	nextRetries := job.Retries + 1
	update := store.JobUpdate{
		Status:  store.Failed,
		Retries: &nextRetries,
	}

	if nextRetries > cfg.MaxRetries {
		return update
	}

	retryAt := now.Add(utils.RetryDelay(nextRetries, cfg.BackoffBaseMS, cfg.BackoffMaxMS))
	update.RetryAt = &retryAt
	return update
}
