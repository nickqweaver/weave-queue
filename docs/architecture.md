# Architecture Overview

This document describes the high-level architecture and core queue invariants.

## System Components

```
┌─────────────────────────────────────────────────────────────┐
│                         Producer                            │
│                   (EnqueueTask API)                         │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                      Store Interface                        │
│         (Memory / PostgreSQL / Future Adapters)            │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                   Jobs Table                        │   │
│  │  id | queue | status | payload | retries | lease   │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                      Server Runtime                         │
│                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │  Fetcher    │───▶│  Consumer   │───▶│  Committer  │     │
│  │  (Claims)   │    │  (Workers)  │    │  (Ack/Nack) │     │
│  └─────────────┘    └─────────────┘    └─────────────┘     │
│                                                             │
│  ┌─────────────┐    ┌─────────────┐                       │
│  │   Worker    │    │   Worker    │                       │
│  │  (Handler)  │    │  (Handler)  │                       │
│  └─────────────┘    └─────────────┘                       │
└─────────────────────────────────────────────────────────────┘
```

## Core Queue Invariants

### 1. Job State Machine
Jobs transition through states according to strict rules:

```
         ┌──────────────────────────────────────────┐
         │                                          │
         ▼                                          │
┌─────────────┐    Lease Expiry    ┌─────────────┐   │
│    Ready    │◄───────────────────│   InFlight  │   │
└──────┬──────┘                    └──────┬──────┘   │
       │                                  │          │
       │ Claim                            │ Ack      │
       ▼                                  │          │
  [Processing] ──────────────────────────┘          │
       │                                            │
       │ Nack (retry)                               │
       ▼                                            │
┌─────────────┐                                     │
│    Failed   │─────────────────────────────────────┘
│  (max retries)                                    
└─────────────┘
```

### 2. Ownership Flow
1. **Enqueue**: Producer creates job in Ready state
2. **Claim**: Fetcher atomically claims job, moving to InFlight
3. **Process**: Worker executes handler with leased job
4. **Commit**: Committer writes final state (Ack→Succeeded, Nack→Ready/Failed)

### 3. Lease Mechanics
- Each claim assigns a unique lease token and version
- Lease token must match for updates (fencing)
- Heartbeats extend lease for long-running work
- Expired leases allow reclaim by other workers

### 4. Concurrency Control
- Multiple fetchers can claim concurrently (row-level locking)
- Workers process jobs independently
- Committer serializes state updates
- Per-queue limits prevent starvation

## Data Flow

### Enqueue Path
1. Client calls EnqueueTask with queue name and payload
2. Store persists job with Ready status
3. Job becomes visible for claiming after commit

### Processing Path
1. Fetcher polls Store.ClaimAvailable with queue filter
2. Store returns leased jobs (InFlight status)
3. Fetcher pushes jobs to pending channel
4. Workers pull from pending and invoke handlers
5. Handlers return result (success/error/panic)
6. Workers send result to finished channel
7. Committer updates job state based on result

### Shutdown Path
1. Stop signal received
2. Fetcher stops claiming new jobs
3. Workers drain pending channel
4. Committer flushes in-flight updates
5. Graceful exit after timeout or completion

## Safety Properties

### Exactly-Once Lease
- Only one fetcher can claim a specific job at a time
- PostgreSQL: SELECT FOR UPDATE SKIP LOCKED
- Memory: Atomic compare-and-swap

### At-Least-Once Commit
- Ack/Nack are durable once committer returns success
- Retry on commit failure
- Duplicate commits are idempotent

### Bounded Concurrency
- Configurable max workers per queue
- Fetch rate limiting via batch size
- Backpressure via channel buffering

## Extensibility Points

- **Store Adapters**: Implement Store interface for new backends
- **Task Handlers**: Register handlers for task types
- **Middleware**: Wrap handlers for cross-cutting concerns
- **Metrics**: Implement metrics interface for observability
