package utils

import "time"

const (
	DefaultRetryBackoffBaseMS = 500
	DefaultRetryBackoffMaxMS  = 30_000
)

func RetryDelay(attempt int, baseMS int, maxMS int) time.Duration {
	timeout := baseMS
	delay := time.Duration(timeout)

	for range max(1, attempt) {
		timeout, delay = Backoff(timeout, maxMS, false)
	}

	return time.Millisecond * delay
}
