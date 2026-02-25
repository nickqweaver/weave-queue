package queue

import (
	"errors"
	"fmt"
	"time"
)

// Config defines the runtime configuration for the queue.
type Config struct {
	// MaxConcurrency is the maximum number of concurrent workers.
	// Default: 10
	MaxConcurrency int

	// MaxQueue is the maximum number of pending jobs in the queue buffer.
	// Default: 100
	MaxQueue int

	// BatchSize is the number of jobs to fetch in a single claim operation.
	// Default: 10
	BatchSize int

	// MaxRetries is the maximum number of retry attempts for failed jobs.
	// Default: 3
	MaxRetries int

	// LeaseTTL is the duration for which claimed jobs are leased.
	// Jobs must be completed or heartbeated within this time.
	// Default: 30s
	LeaseTTL time.Duration

	// ExecutionTimeout is the maximum time a handler can run before being cancelled.
	// This is separate from LeaseTTL to support long-running jobs with heartbeats.
	// Default: 5m
	ExecutionTimeout time.Duration

	// ShutdownTimeout is the maximum time to wait for graceful shutdown.
	// Default: 30s
	ShutdownTimeout time.Duration

	// RetryBackoffBaseMS is the base delay for retry backoff in milliseconds.
	// Default: 500
	RetryBackoffBaseMS int

	// RetryBackoffMaxMS is the maximum delay for retry backoff in milliseconds.
	// Default: 30000 (30s)
	RetryBackoffMaxMS int

	// RetryFetchRatio is the ratio of fetch capacity reserved for retry jobs.
	// 0.0 to 1.0, where 0.2 means 20% of claims can be retries.
	// Default: 0.2
	RetryFetchRatio float64

	// MaxPayloadBytes is the maximum size of job payloads in bytes.
	// Default: 1048576 (1MB)
	MaxPayloadBytes int64

	// MaxMetadataBytes is the maximum size of job metadata in bytes.
	// Default: 65536 (64KB)
	MaxMetadataBytes int64
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		MaxConcurrency:     10,
		MaxQueue:           100,
		BatchSize:          10,
		MaxRetries:         3,
		LeaseTTL:           30 * time.Second,
		ExecutionTimeout:   5 * time.Minute,
		ShutdownTimeout:    30 * time.Second,
		RetryBackoffBaseMS: 500,
		RetryBackoffMaxMS:  30000,
		RetryFetchRatio:    0.2,
		MaxPayloadBytes:    1048576, // 1MB
		MaxMetadataBytes:   65536,   // 64KB
	}
}

// Validate checks the configuration for errors and applies defaults.
// Returns an error if the configuration is invalid.
func (c *Config) Validate() error {
	// Apply defaults for zero values
	defaults := DefaultConfig()

	if c.MaxConcurrency <= 0 {
		c.MaxConcurrency = defaults.MaxConcurrency
	}

	if c.MaxQueue <= 0 {
		c.MaxQueue = defaults.MaxQueue
	}

	if c.BatchSize <= 0 {
		c.BatchSize = defaults.BatchSize
	}

	if c.MaxRetries < 0 {
		c.MaxRetries = defaults.MaxRetries
	}

	if c.LeaseTTL <= 0 {
		c.LeaseTTL = defaults.LeaseTTL
	}

	if c.ExecutionTimeout <= 0 {
		c.ExecutionTimeout = defaults.ExecutionTimeout
	}

	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = defaults.ShutdownTimeout
	}

	if c.RetryBackoffBaseMS <= 0 {
		c.RetryBackoffBaseMS = defaults.RetryBackoffBaseMS
	}

	if c.RetryBackoffMaxMS <= 0 {
		c.RetryBackoffMaxMS = defaults.RetryBackoffMaxMS
	}

	if c.RetryFetchRatio <= 0 {
		c.RetryFetchRatio = defaults.RetryFetchRatio
	}

	if c.RetryFetchRatio > 1 {
		c.RetryFetchRatio = 1
	}

	if c.MaxPayloadBytes <= 0 {
		c.MaxPayloadBytes = defaults.MaxPayloadBytes
	}

	if c.MaxMetadataBytes <= 0 {
		c.MaxMetadataBytes = defaults.MaxMetadataBytes
	}

	// Validation checks
	if c.BatchSize > c.MaxQueue {
		return fmt.Errorf("batch size (%d) cannot exceed max queue (%d)", c.BatchSize, c.MaxQueue)
	}

	if c.LeaseTTL > c.ExecutionTimeout {
		return fmt.Errorf("lease TTL (%v) should not exceed execution timeout (%v)", c.LeaseTTL, c.ExecutionTimeout)
	}

	if c.RetryBackoffMaxMS < c.RetryBackoffBaseMS {
		return fmt.Errorf("retry backoff max (%d) cannot be less than base (%d)", c.RetryBackoffMaxMS, c.RetryBackoffBaseMS)
	}

	return nil
}

// Errors returned by Validate.
var (
	ErrInvalidBatchSize = errors.New("batch size cannot exceed max queue")
	ErrInvalidLeaseTTL  = errors.New("lease TTL cannot exceed execution timeout")
	ErrInvalidBackoff   = errors.New("retry backoff max cannot be less than base")
)
