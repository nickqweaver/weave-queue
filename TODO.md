# MVP TODO

This document defines the work required for the queue to be considered MVP-complete and blog-worthy.

## MVP Definition

- At-least-once job delivery works end-to-end
- Long-running jobs remain leased via heartbeat
- Expired or abandoned jobs are safely recovered and retried
- Stale workers cannot ack, nack, or renew a reclaimed job
- Jobs execute real registered tasks instead of demo logic
- Public client/server APIs are shippable from `pkg/`
- Redis is available as a working backing store adapter
- The repo includes enough docs/tests/examples to explain the design clearly

---

## 1. Core Runtime Cleanup

### 1.1 Config and constructor cleanup

- [x] Replace long constructor arg chains with focused config/option structs
- [x] Centralize config validation/defaulting in one place
- [x] Remove duplicated retry/lease/backoff config plumbing across runtime components
- [x] Make invalid config fail fast with clear errors

Notes:

- Avoid hidden defaults spread across `server`, `fetcher`, `consumer`, and `committer`.
- Nil or zero-value config should not panic.
- Done: `Fetcher`, `Consumer`, and `Committer` now receive focused configs, and normalization/defaulting is centralized in server construction.
- Done: invalid top-level and claim config now returns explicit construction errors instead of being silently clamped or panicking later.

### 1.2 Runtime flow cleanup

- [x] Make fetcher, consumer, worker, and committer responsibilities explicit and non-overlapping
- [ ] Remove demo-only logic that does not belong in the runtime path
- [ ] Normalize naming for leases, retries, queue names, and task execution
- [ ] Remove resolved TODOs and stale comments from runtime code

Notes:

- Done: runtime config is split by component, worker-specific settings are grouped under `WorkerConfig`, and retry state transitions are centralized in a shared retry module instead of being duplicated across committer and lease recovery.

### 1.3 Shutdown and lifecycle

- [ ] Ensure clean shutdown across fetcher, workers, and committer
- [ ] Ensure channels are closed exactly once and in the correct order
- [ ] Ensure in-flight work exits predictably on context cancellation
- [ ] Add tests for shutdown with active work and idle workers

---

## 2. Fault Tolerance and Lease Correctness

### 2.1 Lease lifecycle

- [ ] Define the full lease lifecycle: claim, heartbeat renew, ack/nack, expiry, reclaim
- [x] Separate job execution timeout from lease TTL semantics
- [x] Ensure claimed jobs always receive lease metadata
- [ ] Ensure ack/nack paths clear lease state correctly

Notes:

- Execution timeout answers "how long may this task run?"
- Lease TTL answers "how long do we trust this worker without hearing from it?"

### 2.2 Heartbeat completion

- [x] Finish heartbeat renewal for long-running jobs
- [x] Set a safe heartbeat interval relative to lease TTL
- [x] Prevent invalid ticker behavior when heartbeat/lease config is missing or zero
- [x] Stop heartbeat goroutines promptly when work completes or is canceled
- [x] Add tests proving healthy long-running jobs are not reclaimed

Notes:

- Done: heartbeat renewals extend lease expiry through the committer, worker heartbeat goroutines are shut down per job, and tests now prove active heartbeats prevent false reclaim until heartbeats stop.
- Remaining: add a defensive worker-side guard for invalid heartbeat intervals and later thread lease fencing through heartbeat renewals.

### 2.3 Expired lease recovery

- [x] Recover expired `InFlight` jobs during normal runtime
- [x] Route expired-lease recovery through the same retry policy used by `NAck`
- [x] Increment retry count on lease expiry
- [x] Mark jobs terminal when retry limit is exceeded
- [x] Add tests for crash-after-claim-before-ack recovery

### 2.4 Lease fencing

- [x] Add lease token/version to the job model
- [x] Increment the token every time a job is claimed or re-claimed
- [x] Require ack operations to include the active lease token
- [x] Require nack operations to include the active lease token
- [x] Require heartbeat renewals to include the active lease token
- [x] Reject stale writes when the token no longer matches store state
- [x] Add tests covering stale ack, stale nack, and stale heartbeat rejection

Notes:

- This is required for the fault-tolerance story to be correct.
- Reclaimed jobs must not be writable by the old worker.
- Done: lease tokens are now persisted on jobs, minted on claim/reclaim paths, carried through worker responses and heartbeats, and enforced on updates with stale-write coverage in tests.

### 2.5 Retry policy consistency

- [x] Use one retry decision path for worker failures and lease-expiry recovery
- [x] Ensure retry scheduling always applies the same backoff rules
- [x] Ensure terminal failures are represented consistently
- [x] Add tests for retry count, retry scheduling, and terminal failure behavior

Notes:

- Done: retry transition rules now live in `internal/retry/` and are reused by both committer writes and expired-lease recovery.

---

## 3. Queue Model and Public Behavior

### 3.1 Queue semantics

- [ ] Decide and document MVP queue behavior
- [ ] Support consuming a specific named queue
- [ ] Ensure claiming/filtering respects queue name
- [ ] Add tests proving jobs from other queues are not consumed accidentally

Notes:

- MVP does not need advanced multi-queue scheduling.
- Simple and explicit is enough: producer writes to a queue, worker/server consumes a queue.

### 3.2 Job model cleanup

- [ ] Finalize the minimal MVP job fields
- [ ] Distinguish store-managed fields from user-supplied fields
- [ ] Add payload/task metadata needed for real execution
- [ ] Remove fields that only exist for the current demo path

---

## 4. Tasks and Task Registry

### 4.1 Task execution model

- [ ] Replace `doWork` demo logic with real task execution
- [ ] Define the task handler interface
- [ ] Define how job payload data is passed into handlers
- [ ] Define how handlers return success vs retryable/non-retryable failure

### 4.2 Task registry

- [ ] Add a task registry keyed by task name
- [ ] Support registering handlers before server startup
- [ ] Validate that enqueued jobs reference known task names
- [ ] Return clear errors for unknown or duplicate task registration

### 4.3 Enqueue API for real jobs

- [ ] Add enqueue support for task name + payload + queue name
- [ ] Validate required enqueue fields
- [ ] Preserve job metadata needed for retries and execution
- [ ] Add tests for successful enqueue and invalid task/job input

### 4.4 Worker integration

- [ ] Make workers resolve task handlers from the registry
- [ ] Execute handlers under job timeout constraints
- [ ] Convert handler results into ack/nack behavior
- [ ] Preserve fault-tolerance behavior with real handlers in place

Notes:

- This is the step that turns the project from a queue prototype into a usable library.

---

## 5. Public Package Layout

### 5.1 Exported package structure

- [ ] Move shippable public APIs from `internal/` to `pkg/`
- [ ] Keep store internals/adapters private unless intentionally supported as public API
- [ ] Expose a clean producer/client package
- [ ] Expose a clean worker/server package

### 5.2 Public API cleanup

- [ ] Finalize exported types and method names
- [ ] Remove demo-oriented API surfaces from the public package layout
- [ ] Keep the public API small enough to support without churn
- [ ] Add package-level examples for the intended usage flow

Notes:

- Do this after the task execution model is stable enough that exported APIs will not immediately change again.

---

## 6. Redis Backing Store

### 6.1 Redis adapter design

- [ ] Define the Redis data model for ready, in-flight, retrying, failed, and succeeded jobs
- [ ] Define how leases and lease tokens are stored atomically
- [ ] Define how delayed retries become visible again
- [ ] Define the minimum Redis commands/scripts needed for safe claim/update flows

### 6.2 Redis adapter implementation

- [ ] Implement the store interface with Redis
- [ ] Implement atomic claim + lease assignment
- [ ] Implement atomic ack/nack guarded by lease token
- [ ] Implement heartbeat lease renewal guarded by lease token
- [ ] Implement expired-lease recovery behavior
- [ ] Implement retry scheduling with backoff

### 6.3 Redis tests

- [ ] Add adapter-level tests for claim, ack, nack, retry, and recovery
- [ ] Add tests for stale token rejection
- [ ] Add tests for process-restart durability expectations
- [ ] Add end-to-end tests using the Redis adapter

Notes:

- For MVP, "durable enough" means jobs survive process restart and runtime coordination works correctly against Redis.

---

## 7. Testing and Verification

### 7.1 Runtime correctness tests

- [ ] Add end-to-end tests for successful task execution
- [ ] Add end-to-end tests for retryable task failure
- [ ] Add end-to-end tests for terminal failure after max retries
- [x] Add end-to-end tests for lease expiry and reclaim
- [x] Add end-to-end tests for heartbeat keeping a job alive

### 7.2 Concurrency and failure tests

- [ ] Add tests for concurrent workers processing the same queue safely
- [x] Add tests for duplicate/stale worker responses being rejected
- [ ] Add tests for shutdown during active processing
- [ ] Add tests for Redis-backed recovery after server restart

### 7.3 MVP acceptance checks

- [x] Verify all adapters/tests pass through `go test ./...`
- [ ] Verify the example app runs with real registered tasks
- [ ] Verify one documented crash/recovery scenario manually
- [ ] Verify one documented long-running heartbeat scenario manually

---

## 8. Docs and Blog Readiness

### 8.1 README and usage docs

- [ ] Rewrite `README.md` to describe what the queue does today
- [ ] Document delivery guarantees and non-goals
- [ ] Document queues, retries, leases, heartbeat, and lease fencing
- [ ] Document how to register tasks and enqueue jobs
- [ ] Document how to run the project with Redis locally

### 8.2 Example application

- [ ] Replace the current demo with a realistic example using registered tasks
- [ ] Show one success path and one retry path
- [ ] Show configuration for queue name, concurrency, and Redis
- [ ] Keep the example simple enough to support the blog write-up

### 8.3 Blog support material

- [ ] Capture the architecture in one diagram or concise section
- [ ] Capture one crash-recovery walkthrough
- [ ] Capture one stale-worker / lease-fencing walkthrough
- [ ] Capture the tradeoff: Redis-backed MVP, not production-grade distributed queue

---

## Exit Criteria

The MVP is done when all of the following are true:

- [ ] A user can register a task, enqueue a job, and process it successfully
- [ ] Failed jobs retry with backoff and stop at the configured retry limit
- [ ] Long-running jobs stay leased via heartbeat
- [ ] Expired jobs are reclaimed safely
- [x] Stale workers cannot mutate reclaimed jobs because lease fencing is enforced
- [ ] The public API lives in `pkg/` and is usable without depending on `internal/`
- [ ] Redis works as a backing store for the MVP flow
- [ ] The docs/example are strong enough to support the blog post
