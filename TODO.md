# Job Queue Fault Tolerance TODO

## Context

- Jobs can get stuck in `InFlight` when a process crashes after claim and before `Ack`/`NAck`.
- This is not just a startup issue; it can happen any time between lease claim and commit.
- Current flow has retries for `NAck`, but no recovery path for expired `InFlight` leases.

## Key Decisions From Discussion

- Add lease-expiry recovery as a core runtime behavior (not only on startup).
- Keep weighted fetch logic focused on fairness/starvation, not correctness recovery.
- Count lease expiry as a retry (recommended), same as `NAck`.
- Keep worker timeout and heartbeat as separate concerns:
  - Worker timeout = max execution policy.
  - Heartbeat = lease ownership signal to prevent false reclaims for healthy long-running jobs.

## Why Heartbeat Matters (Example)

- Long job runtime (8-12 min), short lease TTL (30s) for quick crash detection.
- Without heartbeat: lease expires while worker is healthy -> job reclaimed -> duplicate processing.
- With heartbeat: worker extends lease periodically while active; reclaim only happens when heartbeats stop.

## Recommended Implementation Order

1. Implement expired-lease recovery (must-have first).
2. Reuse a single retry policy path for both `NAck` and lease-expiry recovery.
3. Add lease fencing token/version so stale `Ack`/`NAck` cannot overwrite newer lease state.
4. Split execution timeout from lease timeout (separate semantics).
5. Add heartbeat lease extension.
6. Add observability metrics/logging for recovery and stale-token behavior.

## Phase 1: Expired-Lease Recovery (Immediate Next Step)

### Store API changes

- Add a recovery operation, e.g.:
  - `RecoverExpiredLeases(now time.Time, limit int) []Job`
  - or fold recovery into `FetchAndClaim` internals.

### Recovery behavior

- For `InFlight` jobs where `leased_at + timeout <= now`:
  - Increment retries.
  - If retries `<= maxRetries`: set `Status=Failed`, schedule `RetryAt=now+backoff`, clear lease fields.
  - Else: terminal `Failed` with no `RetryAt`.

### Runtime integration

- Run recovery in two places:
  1. On startup: drain expired leases until no more are returned.
  2. In normal fetch loop: bounded periodic recovery batches.

## Worker Timeout vs Heartbeat

- Do not replace worker timeout with heartbeat.
- Keep both:
  - Timeout cancels jobs that exceed max allowed runtime.
  - Heartbeat extends lease while worker is healthy.
- Suggested heartbeat interval: around `leaseTTL / 3`.

## Testing Plan

- Expired in-flight jobs are reclaimed and retried (startup and periodic flows).
- Crash-after-claim-before-ack scenario no longer leaves jobs stuck forever.
- Stale `Ack`/`NAck` rejected after lease is re-claimed (once fencing is added).
- Heartbeat keeps lease alive during long processing.
- Worker timeout still `NAck`s when execution deadline is exceeded.

## Files Likely Involved

- `internal/store/store.go`
- `internal/store/adapters/memory/memory.go`
- `internal/server/fetcher.go`
- `internal/server/worker.go`
- `internal/server/committer.go`
- `internal/server/*_test.go`
- `internal/store/adapters/memory/memory_test.go`
