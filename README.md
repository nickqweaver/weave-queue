# Go Durable Job Queue

A production-ready, durable job queue library for Go with at-least-once delivery guarantees.

## Features

- **At-Least-Once Delivery**: Jobs are guaranteed to be processed at least once
- **Durable Storage**: PostgreSQL backend with ACID guarantees
- **Automatic Retries**: Configurable retry policies with exponential backoff
- **Lease Management**: Prevents stuck jobs when workers crash
- **Task Registry**: Clean API for registering task handlers
- **Context-First**: Full context support for cancellation and timeouts
- **Observable**: Built-in logging and metrics hooks

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os/signal"
    "syscall"
    "time"

    "github.com/nickqweaver/weave-queue/pkg/queue"
)

func main() {
    // Setup signal handling
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    // Create store (PostgreSQL in production)
    store := createStore()

    // Configure and create runtime
    config := queue.Config{
        MaxConcurrency: 10,
        MaxRetries:     3,
        LeaseTTL:       30 * time.Second,
    }

    runtime, err := queue.NewRuntime(store, config)
    if err != nil {
        log.Fatal(err)
    }

    // Register task handlers
    runtime.RegisterFunc("email.send", handleEmail)

    // Run until interrupted
    runtime.Run(ctx)
}

func handleEmail(ctx context.Context, job queue.Job) error {
    // Process the job
    return nil
}
```

See [examples/basic](examples/basic/main.go) for a complete working example.

## Development

Use the Makefile for common development tasks:

```bash
make help      # Show all available targets
make test      # Run tests
make race      # Run tests with race detector
make vet       # Run go vet
make lint      # Run golangci-lint
make check     # Run all quality checks
make ci        # Run CI checks locally
```

## Documentation

- [Architecture Overview](docs/architecture.md) - System design and components
- [Queue Semantics](docs/semantics.md) - Guarantees, invariants, and state transitions
- [Retry Policy](docs/retry-policy.md) - Retry and backoff configuration
- [API Reference](docs/api.md) - Complete API documentation

## Status

This project is under active development. See [PROD_TASKS.json](PROD_TASKS.json) for the current roadmap.

## License

MIT License - see LICENSE file for details.
