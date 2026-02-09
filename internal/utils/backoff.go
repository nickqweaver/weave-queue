package utils

import (
	"math/rand/v2"
	"time"
)

func Backoff(timeout int, maximum int, jitter bool) (int, time.Duration) {
	exponential := min(timeout*2, maximum)

	// Full jitter
	if jitter {
		rnd := rand.IntN(exponential)
		return exponential, time.Duration(exponential - rnd)
	}

	return exponential, time.Duration(exponential)
}
