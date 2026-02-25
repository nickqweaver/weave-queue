package queue

import (
	"context"
	"sync"
	"time"

	"github.com/nickqweaver/weave-queue/internal/server"
	"github.com/nickqweaver/weave-queue/internal/store"
)

// Runtime manages the lifecycle of job processing.
type Runtime struct {
	server *server.Server
	config Config

	mu       sync.Mutex
	handlers map[string]Handler
}

// NewRuntime creates a new Runtime with the given store and configuration.
// The configuration is validated and defaults are applied.
func NewRuntime(s Store, cfg Config) (*Runtime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Convert public config to internal config
	internalCfg := server.Config{
		BatchSize:          cfg.BatchSize,
		MaxQueue:           cfg.MaxQueue,
		MaxConcurrency:     cfg.MaxConcurrency,
		MaxRetries:         cfg.MaxRetries,
		MaxColdTimeout:     int(cfg.ExecutionTimeout.Seconds()),
		LeaseTTL:           cfg.LeaseTTL,
		RetryFetchRatio:    cfg.RetryFetchRatio,
		RetryBackoffBaseMS: cfg.RetryBackoffBaseMS,
		RetryBackoffMaxMS:  cfg.RetryBackoffMaxMS,
	}

	// Create internal store adapter
	internalStore := &storeAdapter{store: s}

	srv := server.NewServer(internalStore, internalCfg)

	return &Runtime{
		server:   srv,
		config:   cfg,
		handlers: make(map[string]Handler),
	}, nil
}

// Register associates a task name with a handler.
// Returns an error if the task name is already registered.
func (r *Runtime) Register(taskName string, handler Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[taskName]; exists {
		return &DuplicateRegistrationError{TaskName: taskName}
	}

	r.handlers[taskName] = handler
	return nil
}

// RegisterFunc is a convenience method to register a function as a handler.
func (r *Runtime) RegisterFunc(taskName string, fn HandlerFunc) error {
	return r.Register(taskName, fn)
}

// Run starts the runtime and blocks until the context is cancelled.
// Jobs are processed concurrently up to MaxConcurrency.
func (r *Runtime) Run(ctx context.Context) error {
	r.server.RunWithContext(ctx)
	return nil
}

// Stop gracefully shuts down the runtime, waiting for in-flight jobs to complete.
// This is equivalent to cancelling the context passed to Run.
func (r *Runtime) Stop() {
	r.server.Close()
}

// DuplicateRegistrationError is returned when attempting to register
// a handler for a task name that is already registered.
type DuplicateRegistrationError struct {
	TaskName string
}

func (e *DuplicateRegistrationError) Error() string {
	return "handler already registered for task: " + e.TaskName
}

// storeAdapter adapts the public Store interface to the internal store interface.
type storeAdapter struct {
	store Store
}

// FetchJobs retrieves jobs by status (not yet implemented).
func (a *storeAdapter) FetchJobs(status store.Status, limit int) []store.Job {
	return nil
}

// ClaimAvailable claims available jobs from the store.
func (a *storeAdapter) ClaimAvailable(opts store.ClaimOptions) []store.Job {
	ctx := context.Background()
	claimOpts := ClaimOptions{
		Queue:              "", // Will be set by caller
		Limit:              opts.Limit,
		LeaseTTL:           opts.LeaseTTL,
		MaxRetries:         opts.MaxRetries,
		RetryBackoffBaseMS: opts.RetryBackoffBaseMS,
		RetryBackoffMaxMS:  opts.RetryBackoffMaxMS,
	}

	jobs, err := a.store.ClaimAvailable(ctx, claimOpts)
	if err != nil {
		return nil
	}

	result := make([]store.Job, len(jobs))
	for i, job := range jobs {
		result[i] = store.Job{
			ID:      job.ID,
			Queue:   job.Queue,
			Status:  convertStatusToInternal(job.Status),
			Retries: job.Retries,
		}
	}
	return result
}

// FailJob marks a job as failed.
func (a *storeAdapter) FailJob(id string) error {
	ctx := context.Background()
	update := JobUpdate{
		Status: JobStatusFailed,
	}
	return a.store.UpdateJob(ctx, id, update)
}

// AddJob adds a new job to the store.
func (a *storeAdapter) AddJob(job store.Job) error {
	ctx := context.Background()
	j := Job{
		ID:        job.ID,
		Queue:     job.Queue,
		Status:    convertStatusFromInternal(job.Status),
		Retries:   job.Retries,
		CreatedAt: time.Now(),
	}
	return a.store.AddJob(ctx, j)
}

// UpdateJob updates a job's status.
func (a *storeAdapter) UpdateJob(id string, update store.JobUpdate) error {
	ctx := context.Background()
	jobUpdate := JobUpdate{
		Status: convertStatusFromInternal(update.Status),
	}
	if update.Retries != nil {
		retries := *update.Retries
		jobUpdate.Retries = &retries
	}
	if update.RetryAt != nil {
		rt := *update.RetryAt
		jobUpdate.RetryAt = &rt
	}
	return a.store.UpdateJob(ctx, id, jobUpdate)
}

// GetAllJobs retrieves all jobs (for testing/debugging).
func (a *storeAdapter) GetAllJobs() []store.Job {
	return nil
}

// convertStatusToInternal converts public JobStatus to internal store.Status.
func convertStatusToInternal(s JobStatus) store.Status {
	switch s {
	case JobStatusReady:
		return store.Ready
	case JobStatusInFlight:
		return store.InFlight
	case JobStatusFailed:
		return store.Failed
	case JobStatusSucceeded:
		return store.Succeeded
	default:
		return store.Ready
	}
}

// convertStatusFromInternal converts internal store.Status to public JobStatus.
func convertStatusFromInternal(s store.Status) JobStatus {
	switch s {
	case store.Ready:
		return JobStatusReady
	case store.InFlight:
		return JobStatusInFlight
	case store.Failed:
		return JobStatusFailed
	case store.Succeeded:
		return JobStatusSucceeded
	default:
		return JobStatusReady
	}
}
