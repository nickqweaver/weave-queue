// Basic example demonstrating how to use the queue library.
//
// This example shows:
// - Creating a runtime with configuration
// - Registering task handlers
// - Running the queue server
// - Graceful shutdown on interrupt
//
// Run with: go run examples/basic/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nickqweaver/weave-queue/pkg/queue"
)

func main() {
	// Create a context that can be cancelled on interrupt
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// For this example, we use a mock in-memory store
	// In production, you would use a PostgreSQL store
	store := &memoryStore{
		jobs: make(map[string]queue.Job),
	}

	// Configure the runtime
	config := queue.Config{
		MaxConcurrency:     5,
		MaxQueue:           100,
		BatchSize:          10,
		MaxRetries:         3,
		LeaseTTL:           30 * time.Second,
		ExecutionTimeout:   5 * time.Minute,
		ShutdownTimeout:    30 * time.Second,
		RetryBackoffBaseMS: 500,
		RetryBackoffMaxMS:  30000,
	}

	// Create the runtime
	runtime, err := queue.NewRuntime(store, config)
	if err != nil {
		log.Fatalf("Failed to create runtime: %v", err)
	}

	// Register task handlers
	if err := runtime.RegisterFunc("email.send", handleEmailSend); err != nil {
		log.Fatalf("Failed to register handler: %v", err)
	}

	if err := runtime.RegisterFunc("report.generate", handleReportGenerate); err != nil {
		log.Fatalf("Failed to register handler: %v", err)
	}

	// Start the runtime
	log.Println("Starting queue server...")
	log.Println("Press Ctrl+C to stop")

	if err := runtime.Run(ctx); err != nil {
		log.Fatalf("Runtime error: %v", err)
	}

	log.Println("Server stopped gracefully")
}

// handleEmailSend processes email.send tasks.
func handleEmailSend(ctx context.Context, job queue.Job) error {
	fmt.Printf("Sending email: job=%s\n", job.ID)
	// Simulate work
	select {
	case <-time.After(100 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// handleReportGenerate processes report.generate tasks.
func handleReportGenerate(ctx context.Context, job queue.Job) error {
	fmt.Printf("Generating report: job=%s\n", job.ID)
	// Simulate work
	select {
	case <-time.After(500 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// memoryStore is a simple in-memory implementation for demonstration.
// In production, use a persistent store like PostgreSQL.
type memoryStore struct {
	mu   sync.RWMutex
	jobs map[string]queue.Job
}

func (m *memoryStore) AddJob(ctx context.Context, job queue.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
	return nil
}

func (m *memoryStore) ClaimAvailable(ctx context.Context, opts queue.ClaimOptions) ([]queue.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var claimed []queue.Job
	for _, job := range m.jobs {
		if job.Status == queue.JobStatusReady {
			claimed = append(claimed, job)
			if len(claimed) >= opts.Limit {
				break
			}
		}
	}
	return claimed, nil
}

func (m *memoryStore) UpdateJob(ctx context.Context, id string, update queue.JobUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job, ok := m.jobs[id]; ok {
		job.Status = update.Status
		if update.Retries != nil {
			job.Retries = *update.Retries
		}
		m.jobs[id] = job
	}
	return nil
}

func (m *memoryStore) GetJob(ctx context.Context, id string) (queue.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if job, ok := m.jobs[id]; ok {
		return job, nil
	}
	return queue.Job{}, fmt.Errorf("job not found: %s", id)
}
