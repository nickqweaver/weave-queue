# ADR 0001: Idempotency and Ordering Guarantees

## Status

Accepted

## Context

The job queue system needs clear policies on:
1. Message delivery guarantees (at-least-once vs exactly-once)
2. Message ordering guarantees (FIFO vs best-effort)
3. Consumer expectations for handling duplicate messages

These decisions have significant implications for system complexity, performance, and consumer implementation requirements.

## Decision

### Delivery Guarantee: At-Least-Once

**Decision**: The system provides **at-least-once delivery** guarantees.

**Rationale**:
- Exactly-once delivery requires distributed transactions or two-phase commit
- At-least-once is simpler to implement and reason about
- Most business applications already require idempotency for other reasons
- Network partitions make exactly-once impossible without significant trade-offs

**Implications**:
- Jobs may be processed multiple times
- Handlers must be idempotent
- Consumers should track processed job IDs or use idempotency keys

### Ordering Guarantee: Per-Queue FIFO with Caveats

**Decision**: The system provides **best-effort FIFO ordering per queue** with the following caveats:

1. **Same-queue ordering**: Jobs within a single queue are generally processed in FIFO order
2. **Retry reordering**: Retried jobs may be processed out of original order
3. **Priority override**: Higher-priority jobs may skip ahead of lower-priority jobs
4. **Concurrent claimers**: Multiple workers may claim jobs in parallel, causing apparent out-of-order processing
5. **No cross-queue ordering**: No guarantees about relative ordering between different queues

**Rationale**:
- Strict FIFO is expensive to guarantee in a distributed system
- Most use cases don't require strict ordering
- Retries inherently break strict FIFO (a later job may succeed before an earlier retry)
- Priority is often more important than strict FIFO

**Implications**:
- Consumers should not depend on strict ordering
- If order matters, use a single queue and idempotency checks
- Consider sequence numbers in payload if strict ordering is required

## Consumer Idempotency Expectations

### Required: Handler Idempotency

**All handlers must be idempotent**.

An idempotent handler produces the same result when executed multiple times with the same input.

### Patterns for Idempotency

#### 1. Idempotency Key Check

Store completed job IDs or business-level idempotency keys:

```go
func Handler(ctx context.Context, job queue.Job) error {
    // Check if already processed
    if alreadyProcessed(job.ID) {
        return nil // Already done, success
    }
    
    // Do work
    result := process(job.Payload)
    
    // Record completion
    markProcessed(job.ID, result)
    return nil
}
```

#### 2. Upsert Operations

Use database upserts instead of inserts:

```go
// Instead of:
INSERT INTO results (job_id, data) VALUES (?, ?)

// Use:
INSERT INTO results (job_id, data) VALUES (?, ?)
ON CONFLICT (job_id) DO UPDATE SET data = EXCLUDED.data
```

#### 3. Conditional Updates

Use compare-and-swap or conditional updates:

```go
UPDATE orders 
SET status = 'shipped', shipped_at = ?
WHERE id = ? AND status = 'pending'
```

### State Tracking Recommendations

#### Option 1: Processed Job Registry

Store completed job IDs with TTL:

```sql
CREATE TABLE processed_jobs (
    job_id TEXT PRIMARY KEY,
    completed_at TIMESTAMP,
    result_hash TEXT
);

-- Clean up old records
DELETE FROM processed_jobs WHERE completed_at < NOW() - INTERVAL '7 days';
```

#### Option 2: Business Entity Versioning

Use entity-level versioning:

```go
type Order struct {
    ID      string
    Version int
    Status  string
}

// Only process if this is the expected version
func ProcessOrder(orderID string, expectedVersion int) error {
    // Check current version
    // Only proceed if versions match
}
```

## Consequences

### Positive

1. **Simpler system**: No distributed transaction complexity
2. **Better performance**: No need for global ordering locks
3. **Better availability**: System works during network partitions
4. **Clear contracts**: Consumers know exactly what to expect

### Negative

1. **Consumer burden**: All consumers must implement idempotency
2. **Duplicate work**: Some jobs may be processed multiple times
3. **Storage overhead**: Need to track processed jobs or use idempotency keys
4. **Ordering limitations**: Cannot guarantee strict global ordering

## Related Decisions

- Lease fencing prevents stale updates from corrupting state
- Exponential backoff with jitter reduces thundering herds from retries
- Dead-letter queues handle jobs that exhaust retries

## References

- [Queue Semantics](../semantics.md)
- [Retry Policy](../retry-policy.md)
- [Architecture Overview](../architecture.md)
