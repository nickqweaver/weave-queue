// Package queue provides a durable job queue system with at-least-once delivery guarantees.
//
// The queue supports multiple storage backends (PostgreSQL, in-memory) and provides
// features like automatic retries with exponential backoff, lease management for
// fault tolerance, and per-queue concurrency controls.
//
// Basic usage:
//
//	// Create a store (PostgreSQL example)
//	store, err := postgres.New(db)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create runtime
//	runtime := queue.NewRuntime(store, queue.Config{
//	    MaxConcurrency: 10,
//	    MaxRetries:     3,
//	})
//
//	// Register task handlers
//	runtime.Register("email.send", emailHandler)
//	runtime.Register("report.generate", reportHandler)
//
//	// Enqueue tasks
//	client := queue.NewClient(store)
//	client.EnqueueTask(ctx, "email.send", payload)
//
//	// Run the queue
//	if err := runtime.Run(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// The queue provides at-least-once delivery guarantees. Tasks may be processed
// multiple times in failure scenarios, so handlers must be idempotent.
//
// For more details, see the documentation at:
// https://github.com/nickqweaver/weave-queue/docs
package queue
