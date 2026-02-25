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

## Job State Transition Matrix

### States

| State | Description |
|-------|-------------|
| `Ready` | Job is available to be claimed and processed |
| `InFlight` | Job is currently leased by a worker and being processed |
| `Failed` | Job exceeded max retries and will not be processed again |
| `Succeeded` | Job completed successfully |

### State Transitions

| From State | To State | Trigger | Description |
|------------|----------|---------|-------------|
| - | `Ready` | **Enqueue** | New job added to queue |
| `Ready` | `InFlight` | **Claim** | Worker successfully claims job |
| `InFlight` | `Succeeded` | **Ack** | Handler returns success |
| `InFlight` | `Ready` | **Nack (retry)** | Handler returns error, retries remain |
| `InFlight` | `Failed` | **Nack (exhausted)** | Handler returns error, max retries reached |
| `InFlight` | `Ready` | **Lease Timeout** | Lease expires, job becomes reclaimable |

### State Diagram

```
                    Enqueue
                       │
                       ▼
                ┌────────────┐
    ┌──────────││   Ready    │◄───────────────┐
    │          └────────────┘                │
    │                  │                     │
    │                  │ Claim               │
    │                  ▼                     │
    │          ┌────────────┐                │
    │          │  InFlight  │                │
    │          └─────┬──────┘                │
    │                │                        │
    │     ┌─────────┼─────────┐             │
    │     │         │         │             │
    │     ▼         ▼         ▼             │
    │ Ack/OK   Nack/Retry  Nack/Exhausted   │
    │     │         │         │             │
    │     ▼         │         ▼             │
    │ ┌────────┐    │    ┌────────┐         │
    └▶│Succeeded│    │    │ Failed │         │
      └────────┘    │    └────────┘         │
                    │                       │
                    └───────────────────────┘
                         (retry loop)
```

### Transition Triggers

#### Enqueue
- **Source**: Producer API
- **Precondition**: Job with unique ID does not exist
- **Action**: Insert job with Ready status
- **Postcondition**: Job is claimable

#### Claim
- **Source**: Fetcher component
- **Precondition**: Job status is Ready, retry_at <= now (if set)
- **Action**: Atomic update status to InFlight, assign lease token
- **Postcondition**: Job is leased to specific worker

#### Ack (Acknowledge)
- **Source**: Handler returns success
- **Precondition**: Job status is InFlight, lease token matches
- **Action**: Update status to Succeeded
- **Postcondition**: Job is complete, never reprocessed

#### Nack (Negative Acknowledge)
- **Source**: Handler returns error
- **Precondition**: Job status is InFlight, lease token matches
- **Action**: 
  - If retries < MaxRetries: Update to Ready with incremented retry count and computed retry_at
  - If retries >= MaxRetries: Update to Failed
- **Postcondition**: Job either scheduled for retry or permanently failed

#### Lease Timeout / Recovery
- **Source**: Lease expiry detection
- **Precondition**: Job status is InFlight, lease has expired
- **Action**: Implicit - job becomes claimable again with same ID
- **Postcondition**: Job is reclaimed by new worker

### Fencing and Safety

All state transitions from InFlight require a valid lease token:
- Stale lease tokens are rejected (preventing lost update problem)
- Each claim generates a new unique lease token
- Lease version incremented on each claim
- Prevents delayed acks/nacks from corrupting newer claims

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

## Retry and Backoff Semantics

### Retry Policy

**Retry Cap**: Jobs are retried at most `MaxRetries` times (default: 3)
- Initial attempt + 3 retries = 4 total attempts maximum
- When retries are exhausted, job transitions to `Failed` state
- Failed jobs require manual intervention or dead-letter handling

**Backoff Calculation**: Exponential backoff with full jitter
```
delay = random(0, min(base * 2^attempt, max))
```

Default values:
- Base delay: 500ms
- Maximum delay: 30s
- Jitter: Full decorrelated jitter to prevent thundering herds

**Retry Visibility**: After a nack
1. Retry count is incremented
2. `retry_at` is set to `now + calculated_delay`
3. Job returns to `Ready` state
4. Job is not claimable until `retry_at` time has passed

See [Retry Policy](./retry-policy.md) for complete details.

## Lease Semantics

### Lease Basics

A lease represents temporary exclusive ownership of a job by a worker:
- **Lease Token**: Unique identifier assigned at claim time
- **Lease Version**: Monotonically increasing on each claim
- **Lease Expiry**: `LeaseExpiresAt` timestamp after which lease is invalid

### Lease Duration (TTL)

- **LeaseTTL**: Duration from claim until lease expires (default: 5s)
- **Purpose**: Prevents stuck jobs when workers crash or disconnect
- **Trade-off**: Shorter TTL = faster recovery, more lease refresh overhead

### Lease Fencing

All updates to InFlight jobs require valid lease token:
- Ack/Nack must include matching lease token
- Stale tokens are rejected with `ErrStaleLease`
- Prevents delayed commits from corrupting newer claims

### Lease Extension (Heartbeat)

For long-running jobs, workers can extend leases:
- **Heartbeat Interval**: Typically LeaseTTL / 3 (e.g., 1.67s for 5s TTL)
- **Extension**: Each heartbeat extends `LeaseExpiresAt` by LeaseTTL
- **Stop Condition**: Heartbeat stops when job completes or context cancels

### Lease Expiry and Reclaim

When a lease expires:
1. Job remains in `InFlight` state in store
2. Store makes job available to new claims
3. New claim assigns new lease token (version incremented)
4. Old worker's commits are rejected (stale token)
5. Job may be processed by multiple workers (at-least-once guarantee)

### Separation of Concerns

**Lease TTL** vs **Execution Timeout**:
- Lease TTL: Storage-layer protection against stuck jobs
- Execution Timeout: Application-layer limit on handler runtime
- These are separate values to support long-running work with short leases

Example:
```
LeaseTTL: 30s          // Extendable via heartbeat
ExecutionTimeout: 5m   // Hard limit on handler runtime
```

This allows:
- 5-minute jobs that heartbeat every 10s
- Automatic reclaim if heartbeats stop (worker crash)
- Hard termination if handler runs longer than 5m
