# Retry and Backoff Policy

This document defines the retry behavior, backoff calculation, and configuration options for the job queue system.

## Overview

When a job fails during processing (handler returns error or panics), the system automatically retries the job according to the configured retry policy. This document explains the semantics of that retry behavior.

## Retry Cap

### MaxRetries Configuration

- **Purpose**: Limits the total number of retry attempts for a job
- **Default**: 3 retries (4 total attempts including initial)
- **Range**: 0 (no retries) to N (system-defined upper limit)

### Retry Exhaustion

When a job fails and `retries >= MaxRetries`:
1. Job transitions to `Failed` state
2. Job is no longer eligible for claiming
3. Job remains in store for observability/debugging
4. Manual intervention or dead-letter handling required

## Backoff Behavior

### Exponential Backoff with Jitter

The system uses exponential backoff with decorrelated jitter to prevent thundering herd problems.

#### Calculation Formula

```
// Base exponential calculation
nextTimeout = min(currentTimeout * 2, maxTimeout)

// Full jitter decorrelation
retry_delay = random(0, nextTimeout)
```

#### Default Values

| Parameter | Value | Description |
|-----------|-------|-------------|
| `BaseMS` | 500ms | Initial retry delay |
| `MaxMS` | 30,000ms (30s) | Maximum retry delay cap |

#### Example Delays

| Attempt | Exponential | With Jitter (example) |
|---------|-------------|----------------------|
| 1 | 500ms | 0-500ms |
| 2 | 1,000ms | 0-1,000ms |
| 3 | 2,000ms | 0-2,000ms |
| 4 | 4,000ms | 0-4,000ms |
| 5 | 8,000ms | 0-8,000ms |
| 6+ | 30,000ms | 0-30,000ms |

### Why Jitter?

Without jitter, retries from multiple failed jobs can synchronize and create thundering herds. Full jitter provides:
- **Better distribution**: Spreads retries across the time window
- **Reduced contention**: Less simultaneous load on downstream services
- **Faster recovery**: Some retries happen immediately (delay = 0)

### Customization

Applications can configure:
- `RetryBackoffBaseMS`: Initial delay (default: 500ms)
- `RetryBackoffMaxMS`: Maximum delay cap (default: 30s)
- `MaxRetries`: Maximum retry count (default: 3)

## Retry Scheduling

### When Jobs Become Visible

After a failed attempt:
1. Handler returns error or panics
2. Committer updates job with incremented retry count
3. `retry_at` timestamp set to `now + calculated_delay`
4. Job transitions back to `Ready` state
5. Fetcher will not claim job until `retry_at <= now`

### Priority Consideration

Retried jobs maintain their original priority. High-priority jobs that fail will be retried before lower-priority jobs, subject to their `retry_at` time.

## Idempotency Requirements

### At-Least-Once Semantics

The queue provides at-least-once delivery guarantees. This means:
- Jobs may be processed multiple times
- Handlers must be idempotent
- Side effects should be guarded with idempotency keys

### Handling Duplicate Processing

Handlers should:
1. Check if work was already completed (idempotency key lookup)
2. Return success if work is already done
3. Only perform side effects if not previously completed

## Configuration Examples

### Conservative Retry (Financial Operations)

```go
config := queue.Config{
    MaxRetries:         5,        // More retries for critical ops
    RetryBackoffBaseMS: 1000,     // Start with 1s delay
    RetryBackoffMaxMS:  60000,    // Cap at 60s
}
```

### Aggressive Retry (Low-Latency Processing)

```go
config := queue.Config{
    MaxRetries:         2,        // Fewer retries
    RetryBackoffBaseMS: 100,     // Quick initial retry
    RetryBackoffMaxMS:  5000,    // Cap at 5s
}
```

### No Retry (One-Shot Operations)

```go
config := queue.Config{
    MaxRetries: 0,  // Fail immediately, no retries
}
```

## Monitoring and Alerting

### Retry Metrics

Monitor these indicators:
- `retries_total`: Total retry attempts across all jobs
- `retry_exhausted_total`: Jobs that reached MaxRetries
- `retry_latency_seconds`: Time spent in retry backoff

### Alerting Thresholds

Consider alerting when:
- Retry rate exceeds 10% of total jobs
- Many jobs reaching MaxRetries (indicating systemic issue)
- Retry delays causing unacceptable latency

## Best Practices

1. **Set appropriate MaxRetries**: Consider the cost of retries vs. value of eventual success
2. **Use idempotent handlers**: Always design for at-least-once execution
3. **Monitor retry patterns**: High retry rates indicate problems
4. **Configure timeouts properly**: Ensure execution timeout allows for meaningful work
5. **Consider dead-letter queues**: For jobs that exhaust retries
