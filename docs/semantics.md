# Queue Semantics and Invariants

This document defines the core guarantees and non-guarantees of the job queue system.

## System Guarantees

### At-Least-Once Delivery
- Every enqueued job will be processed at least once
- Jobs may be processed multiple times in failure scenarios (consumers must be idempotent)
- No silent loss of jobs once successfully enqueued

### Durability
- Successfully enqueued jobs survive process restarts
- PostgreSQL store provides ACID guarantees for job state
- Memory store is ephemeral and for testing only

### Ordering
- Jobs within a single queue are processed in FIFO order by default
- No cross-queue ordering guarantees
- Priority settings may alter ordering within a queue

### Visibility
- Jobs are invisible to other workers while leased (in-flight)
- Failed jobs become visible again after retry_at time
- Successful jobs are never re-processed

## Non-Guarantees

### Exactly-Once Processing
- We do not guarantee exactly-once semantics
- Network partitions and timeouts may cause duplicate processing
- Consumers must handle idempotency

### Real-Time Ordering
- Strict FIFO is not maintained across retries
- High-priority jobs may skip ahead
- Concurrent workers may process jobs out of enqueue order

### Immediate Visibility
- Enqueued jobs may not be immediately claimable
- Store implementations may batch or delay visibility

## Core Invariants

1. **State Consistency**: A job is always in exactly one state (Ready, InFlight, Failed, or Succeeded)
2. **Lease Safety**: Only one worker can hold a lease on a job at any time
3. **Retry Bound**: Jobs are retried at most MaxRetries times before being permanently failed
4. **TTL Enforcement**: Leases expire and jobs become reclaimable after LeaseTTL duration

## Failure Modes

### Worker Crash
- Leased jobs become reclaimable after lease expiry
- No in-flight work is lost

### Store Unavailability
- Fetchers retry with backoff
- Committed work is preserved

### Network Partition
- Leases may expire and cause duplicate processing
- At-least-once delivery is still maintained
