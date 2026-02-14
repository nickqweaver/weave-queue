# Retry & Requeue Strategy

## Overview

When jobs fail (NAck from worker, lease expiry, committer failure), they need to be requeued for retry. The core challenge is avoiding **starvation** — where a growing pool of failing retry jobs crowds out fresh work.

## Per-Job Retry Tracking

Add `Retries int` and `RetryAt time.Time` fields to the `Job` struct. Each job tracks its own retry state independently.

When the committer processes a NAck:
- If `retries < maxRetries`: increment retries, compute `RetryAt`, set status back to `Ready`
- If `retries >= maxRetries`: mark as permanently `Failed` (or move to a dead letter queue)

The hard retry cap is the most important piece — it guarantees no job retries forever, which bounds the total number of retry-eligible jobs in the system.

## Exponential Backoff (Per-Job)

Backoff is tied to the individual job, not the fetcher or any global timer.

```
Retry 1 → wait 1s
Retry 2 → wait 4s
Retry 3 → wait 16s
Retry 4 → wait 64s
...
```

Implementation: set `RetryAt = now + backoff(retries)` on the job. The fetcher only picks up jobs where `RetryAt <= now` (or `RetryAt` is zero for fresh jobs). This is called a **visibility timeout** — the job exists in the store but is invisible to the fetcher until its backoff period elapses.

This is the approach used by SQS, Sidekiq, and most production queue systems.

## Preventing Starvation

Even with per-job backoff and retry caps, starvation can still occur if a large batch of jobs all fail and their retry windows align. Three standard solutions:

### 1. Weighted / Priority Fetching

The fetcher pulls jobs with a bias toward fresh work. For example: 80% of each batch is fresh jobs (`retries == 0`), 20% is retries. This guarantees fresh jobs always get worker capacity regardless of retry backlog.

### 2. Separate Retry Queue

Failed jobs go into a distinct retry queue with its own fetcher running at lower throughput/concurrency. Fresh jobs and retries don't compete for the same worker pool. This is a clean separation but adds operational complexity.

### 3. In-Flight Retry Cap

Limit the number of retry jobs that can be in-flight simultaneously. Even if 10,000 jobs are waiting to retry, only N can be claimed at once. Simple to implement — just a counter or semaphore in the fetcher.

## Why Starvation Is Bounded

The worst case scenario (fail 100 → requeue → fail 100 → requeue → fail 1000) is bounded by two factors working together:

1. **Hard retry cap**: A job can only fail N times before it's permanently dead. Maximum retry-eligible jobs at any moment = `total_jobs * maxRetries`.
2. **Exponential backoff**: Each retry takes exponentially longer to become visible. 100 jobs on retry 3 each wait ~16s+ before re-entering the pool. The retry backlog is self-draining — jobs either succeed or hit the cap and die.

In practice these two mechanisms mean the retry pool shrinks over time and cannot grow unbounded.

## Implementation Plan for This Codebase

### Store Layer

1. Add fields to `Job` struct in `internal/store/store.go`:
   - `Retries int`
   - `RetryAt time.Time`

2. Update `FetchAndClaim` to filter on `RetryAt <= now` (or zero value) in addition to status matching.

### Committer Layer

3. Update `batchWrite` in `internal/server/committer.go`:
   - On NAck: check retry count against max. If under limit, increment retries, compute `RetryAt`, update status back to `Ready`. If at limit, mark `Failed`.

### Fetcher Layer

4. (Optional) Add weighted fetching to `internal/server/fetcher.go`:
   - Split each batch: fetch N fresh jobs, then M retry jobs.
   - Or add an in-flight retry cap.

### Config

5. Add retry-related fields to `Config` in `internal/server/server.go`:
   - `MaxRetries int`
   - `BaseBackoff time.Duration`
   - `MaxBackoff time.Duration`
   - `RetryFetchRatio float64` (e.g., 0.2 = 20% of batch reserved for retries)
