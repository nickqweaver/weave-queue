package queue

import (
	"context"
	"time"
)

// Job represents a task to be processed.
type Job struct {
	ID        string
	Queue     string
	Task      string
	Payload   []byte
	Metadata  map[string]string
	Status    JobStatus
	Retries   int
	CreatedAt time.Time
}

// JobStatus represents the current state of a job.
type JobStatus int

const (
	// JobStatusReady indicates the job is available to be claimed.
	JobStatusReady JobStatus = iota
	// JobStatusInFlight indicates the job is currently being processed.
	JobStatusInFlight
	// JobStatusFailed indicates the job has exhausted all retries.
	JobStatusFailed
	// JobStatusSucceeded indicates the job completed successfully.
	JobStatusSucceeded
)

func (s JobStatus) String() string {
	switch s {
	case JobStatusReady:
		return "ready"
	case JobStatusInFlight:
		return "inflight"
	case JobStatusFailed:
		return "failed"
	case JobStatusSucceeded:
		return "succeeded"
	default:
		return "unknown"
	}
}

// Handler is the interface for processing jobs.
type Handler interface {
	// Handle processes a job and returns an error if processing failed.
	// Returning nil indicates successful processing.
	// Returning an error will trigger retry according to the retry policy.
	Handle(ctx context.Context, job Job) error
}

// HandlerFunc is an adapter to allow the use of ordinary functions as handlers.
type HandlerFunc func(ctx context.Context, job Job) error

// Handle calls f(ctx, job).
func (f HandlerFunc) Handle(ctx context.Context, job Job) error {
	return f(ctx, job)
}

// Store is the interface for job storage backends.
type Store interface {
	// AddJob adds a new job to the store.
	AddJob(ctx context.Context, job Job) error

	// ClaimAvailable claims available jobs for processing.
	ClaimAvailable(ctx context.Context, opts ClaimOptions) ([]Job, error)

	// UpdateJob updates the status of a job.
	UpdateJob(ctx context.Context, id string, update JobUpdate) error

	// GetJob retrieves a job by ID.
	GetJob(ctx context.Context, id string) (Job, error)
}

// ClaimOptions configures how jobs are claimed.
type ClaimOptions struct {
	// Queue is the queue name to claim from. Empty means all queues.
	Queue string

	// Limit is the maximum number of jobs to claim.
	Limit int

	// LeaseTTL is the duration for which claimed jobs are leased.
	LeaseTTL time.Duration

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// RetryBackoffBaseMS is the base delay for retry backoff in milliseconds.
	RetryBackoffBaseMS int

	// RetryBackoffMaxMS is the maximum delay for retry backoff in milliseconds.
	RetryBackoffMaxMS int
}

// JobUpdate contains fields to update on a job.
type JobUpdate struct {
	// Status is the new job status.
	Status JobStatus

	// Retries is the updated retry count (optional).
	Retries *int

	// RetryAt is when the job should become visible again (optional).
	RetryAt *time.Time
}

// LeaseInfo contains lease metadata for a job.
type LeaseInfo struct {
	// Token is the unique lease identifier.
	Token string

	// Version is incremented each time the job is claimed.
	Version int

	// ExpiresAt is when the lease expires.
	ExpiresAt time.Time
}
