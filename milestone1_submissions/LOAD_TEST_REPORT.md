# Load Test Report — Experiment 1: Bid Contention Under Load

**Date:** 2026-03-30

---

## 1. Experiment Setup

### Goal

Compare three concurrency strategies under 500 concurrent bidders competing on a single auction,
measuring throughput, latency, and correctness at extreme contention.

### Concurrency Strategies Under Test

| Strategy        | Mechanism                     | Description                                                                          |
| --------------- | ----------------------------- | ------------------------------------------------------------------------------------ |
| **Optimistic**  | Redis WATCH / MULTI / EXEC    | Read → validate → atomic commit; retry up to 3x on conflict with exponential backoff |
| **Pessimistic** | Redis SETNX distributed lock  | Acquire lock → update → release; fast-fail if lock held                              |
| **Queue**       | Go channel (single goroutine) | All bids enqueued; processed serially one at a time                                  |

### Test Parameters

| Parameter                 | Value                                         |
| ------------------------- | --------------------------------------------- |
| Concurrent users          | 500                                           |
| Ramp-up rate              | 50 users/second                               |
| Test duration             | 60 seconds per run                            |
| Runs per strategy         | 3                                             |
| Auction start bid         | $3.00 (300 cents)                             |
| Bid increment per attempt | current_highest + rand(1, 100) cents          |
| Infrastructure            | Local Docker Compose                          |
| Services                  | auction:8081, bid:8084, Redis, DynamoDB Local |

### Metrics Collected (from `GET /admin/metrics`)

- `total_bids` — all bid attempts processed
- `successful_bids` — bids accepted (amount > current highest, lock/transaction succeeded)
- `rejected_bids` — bids rejected (bid too low, lock conflict, or max retries exceeded)
- `avg_latency_ms` — average latency of **successful** bids only
- `p95_latency_ms` / `p99_latency_ms` — tail latency of successful bids

### Note on Auction State Per Strategy

Each strategy group ran against a fresh OPEN auction (start bid $3.00, version 0) to ensure
clean starting conditions. Metrics were reset between each run within a group.

---

## 2. Raw Results

### 2a. Optimistic Locking

Auction ID: `1a2d63ba-4d4f-4c67-88ea-4ac4b0c8364e`

| Run     | total_bids | successful_bids | rejected_bids | success rate | avg latency | p95 latency | p99 latency  |
| ------- | ---------- | --------------- | ------------- | ------------ | ----------- | ----------- | ------------ |
| 1       | 83,953     | 17,507          | 66,446        | 20.8%        | 1.07 ms     | 1.71 ms     | 12.39 ms     |
| 2       | 82,335     | 16,565          | 65,770        | 20.1%        | 1.08 ms     | 1.57 ms     | 12.50 ms     |
| 3       | 79,063     | 13,944          | 65,119        | 17.6%        | 1.29 ms     | 3.08 ms     | 12.85 ms     |
| **Avg** | **81,784** | **16,005**      | **65,778**    | **19.5%**    | **1.15 ms** | **2.12 ms** | **12.58 ms** |

### 2b. Pessimistic Locking

Auction ID: `7c899246-7203-46d9-980f-648945ed4d39`

| Run     | total_bids | successful_bids | rejected_bids | success rate | avg latency | p95 latency | p99 latency |
| ------- | ---------- | --------------- | ------------- | ------------ | ----------- | ----------- | ----------- |
| 1       | 68,348     | 20,737          | 47,611        | 30.3%        | 0.54 ms     | 0.84 ms     | 1.13 ms     |
| 2       | 67,537     | 19,810          | 47,727        | 29.3%        | 0.54 ms     | 0.82 ms     | 1.16 ms     |
| 3       | 66,419     | 18,651          | 47,768        | 28.1%        | 0.55 ms     | 0.84 ms     | 1.20 ms     |
| **Avg** | **67,435** | **19,733**      | **47,702**    | **29.2%**    | **0.54 ms** | **0.83 ms** | **1.16 ms** |

### 2c. Queue (Go Channel)

Auction ID: `7c899246-7203-46d9-980f-648945ed4d39`

| Run     | total_bids | successful_bids | rejected_bids | success rate | avg latency | p95 latency | p99 latency |
| ------- | ---------- | --------------- | ------------- | ------------ | ----------- | ----------- | ----------- |
| 1       | 82,568     | 17,523          | 65,045        | 21.2%        | 0.72 ms     | 1.69 ms     | 2.82 ms     |
| 2       | 79,555     | 17,944          | 61,611        | 22.6%        | 0.66 ms     | 1.53 ms     | 2.51 ms     |
| 3       | 77,706     | 17,912          | 59,794        | 23.1%        | 0.65 ms     | 1.49 ms     | 2.60 ms     |
| **Avg** | **79,943** | **17,793**      | **62,150**    | **22.3%**    | **0.68 ms** | **1.57 ms** | **2.64 ms** |

---

## 3. Comparative Summary

| Metric                        | Optimistic    | Pessimistic  | Queue             |
| ----------------------------- | ------------- | ------------ | ----------------- |
| Avg total_bids / run          | 81,784        | 67,435       | 79,943            |
| **Avg successful_bids / run** | **16,005**    | **19,733**   | **17,793**        |
| **Avg success rate**          | **19.5%**     | **29.2%**    | **22.3%**         |
| Avg latency (success)         | 1.15 ms       | **0.54 ms**  | 0.68 ms           |
| **p99 latency (success)**     | **12.58 ms**  | **1.16 ms**  | **2.64 ms**       |
| Run-to-run variance (success) | High (±1,782) | Low (±1,043) | **Lowest (±211)** |

---

## 4. Analysis

### 4.1 Optimistic — Competitive Throughput, High Tail Latency

Optimistic locking uses Redis `WATCH/MULTI/EXEC`. Each user watches the auction key, reads the
current highest bid, and attempts a transactional update. If another user commits between the
WATCH and EXEC, the transaction fails with `TxFailedErr` and retries up to 3 times with
exponential backoff (10ms → 20ms → 40ms).

**Result:** ~19.5% average success rate — competitive with queue (22.3%) but significantly
behind pessimistic (29.2%). The more striking finding is p99 latency: **12.58 ms**, roughly
10x higher than pessimistic (1.16 ms) and ~5x higher than queue (2.64 ms).

This tail latency spike is caused by the retry backoff. A user who exhausts all 3 retries has
waited up to 10 + 20 + 40 = 70 ms in backoff alone before returning failure. Even users who
succeed on the 2nd or 3rd retry accumulate significant wait time, pushing the p99 far above
the median. The p95 (2.12 ms) vs p99 (12.58 ms) gap confirms that most successes are fast but
a meaningful tail takes much longer.

The declining success rate across runs (20.8% → 20.1% → 17.6%) reflects rising auction price:
as `current_highest` increases, bids computed from a stale read are more frequently too low,
causing business-logic rejections on retry that do not trigger further retries.

**When optimistic locking works well:** Low-to-moderate concurrency where conflicts are rare.
The retries that hurt tail latency at 500 users would be invisible at 10–20 users, where
WATCH conflicts are infrequent and most transactions succeed on the first attempt.

### 4.2 Pessimistic — Best Throughput, Lowest and Most Stable Latency

Pessimistic locking uses Redis `SETNX` to acquire a per-auction distributed lock. Only one
request holds the lock at a time; all others receive an immediate rejection (fast-fail) without
waiting. The lock holder validates `amount > current_highest`, updates Redis atomically, then
releases the lock.

**Result:** Best success rate at 29.2% (~19,733 bids/run) with the lowest average latency
(0.54 ms) and tightly bounded p99 (1.16 ms). There are no retries and no backoff — a request
either acquires the lock and succeeds, or fails fast. This predictability is what makes
pessimistic locking attractive under high contention.

The total_bids per run (~67k) is lower than the other two strategies because lock-acquisition
failures return quickly but still consume a round trip, slightly reducing the total number of
requests each Locust user can issue in 60 seconds.

The declining success rate across runs (30.3% → 29.3% → 28.1%) is consistent with the rising
auction price across the three runs sharing the same auction — more bids fall below the
current highest.

### 4.3 Queue — Most Stable, Moderate Tail Latency

The queue strategy routes all bids through a Go channel consumed by a single goroutine per
auction. Processing is fully serial: no concurrent access, no lock contention, no transaction
conflicts of any kind.

**Result:** 22.3% average success rate with the lowest run-to-run variance (±211 bids between
runs 2 and 3). p99 latency (2.64 ms) sits between pessimistic and optimistic — higher than
pessimistic because requests must wait behind all others in the channel, but far lower than
optimistic because there is no retry backoff.

The queue's lower success rate vs pessimistic is explained by the channel bottleneck. The
goroutine processes bids at a fixed serial rate. Many bids arrive in the channel when
`current_highest` is X, but by the time the goroutine reaches them the price has moved to X+N,
making their amounts too low. Pessimistic allows concurrent reads and only serializes the
update, so more users hold a currently-valid bid when they acquire the lock.

The stability of queue results (17,523 → 17,944 → 17,912) is its key strength: output
throughput is bounded by the goroutine's processing speed, making it the most predictable
under load.

### 4.4 Correctness

All three strategies correctly enforce the invariant `new_bid > current_highest` before committing
any state change — guaranteed at the Redis level by atomic transactions (WATCH/MULTI/EXEC),
distributed lock (SETNX), and serial channel processing respectively. No lower bid displacing a
higher one was observed during testing.

---

## 5. Conclusions

|                      | Optimistic              | Pessimistic              | Queue                                  |
| -------------------- | ----------------------- | ------------------------ | -------------------------------------- |
| Avg success rate     | 19.5%                   | **29.2%**                | 22.3%                                  |
| Avg latency          | 1.15 ms                 | **0.54 ms**              | 0.68 ms                                |
| p99 latency          | 12.58 ms                | **1.16 ms**              | 2.64 ms                                |
| Run-to-run stability | Moderate                | Good                     | **Best**                               |
| Correctness          | ✓                       | ✓                        | ✓                                      |
| Best fit for         | Low-contention auctions | High-contention auctions | Audit-critical / predictable workloads |

**Finding 1 — Optimistic locking trades tail latency for moderate throughput.**
Under 500-user contention, optimistic locking achieves a ~19.5% success rate — usable, but at
the cost of a p99 latency of 12.58 ms, roughly 10x higher than pessimistic. The retry-with-backoff
mechanism is correct and avoids starvation, but it makes worst-case response times unpredictable.
Optimistic locking is the right choice when contention is low (< ~20 simultaneous bidders),
where conflicts are rare and the retry overhead is never incurred.

**Finding 2 — Pessimistic locking is the best strategy under high contention.**
SETNX serializes only the update, not the entire request. Fast-failing non-lock-holders keeps
latency low (p99 = 1.16 ms) while achieving the highest success rate (29.2%). For a real-time
auction with hundreds of simultaneous bidders on a single hot item, pessimistic locking provides
the best combination of throughput and latency.

**Finding 3 — Queue processing maximizes predictability at the cost of peak throughput.**
The single-goroutine queue achieves the most consistent output (lowest run-to-run variance) and
moderate tail latency (p99 = 2.64 ms), but its success rate (22.3%) is capped by the serial
processing bottleneck. It is best suited for scenarios where deterministic ordering and
auditability matter more than raw throughput — e.g., payment processing or compliance workflows.

**Recommendation:** A production auction system should apply strategy selection dynamically:
optimistic locking for auctions with few active bidders, switching to pessimistic when bid rate
exceeds a threshold. Pure queue processing is best reserved for downstream workflows (payment,
refund) rather than the hot path of bid acceptance — mirroring how platforms such as eBay
(proxy bidding) and Taobao (flash sale queues) handle load-dependent contention.

---

## 6. Result Files

All raw data is in `loadtest/results/`:

```
exp1_optimistic_run{1,2,3}_metrics.json   (auction 1a2d63ba)
exp1_optimistic_run{1,2,3}_stats.csv
exp1_pessimistic_run{1,2,3}_metrics.json  (auction 7c899246)
exp1_pessimistic_run{1,2,3}_stats.csv
exp1_queue_run{1,2,3}_metrics.json        (auction 7c899246)
exp1_queue_run{1,2,3}_stats.csv
```

Test script: `loadtest/scenarios/exp1_bid_contention.py`

---

# Experiment 2: Scaling Under Spike Load

> **Status: Pending.** This experiment requires a multi-instance deployment environment (e.g., AWS ECS or Kubernetes) to test horizontal scaling behavior. It will be conducted in a later phase when cloud infrastructure is available.

---

# Experiment 3: Notification Fan-Out

**Date:** 2026-04-18

---

## 1. Experiment Setup

### Goal

The original Experiment 3 question turned out to contain two different comparisons, so the final
local rerun split it into:

- **Experiment 3A:** compare two push transports for one-way auction updates: `WebSocket` vs `SSE`
- **Experiment 3B:** compare push delivery vs pull polling as an architectural trade-off

This produces cleaner conclusions than treating polling as if it were a direct peer of push
delivery latency.

### Delivery Models Under Test

| Variant | Mechanism | What it answers |
| --- | --- | --- |
| `ws` | `ws://notification:8080/auctions/:id/subscribe` | Is WebSocket a strong push transport for 1-to-many auction updates? |
| `sse` | `http://notification:8080/auctions/:id/subscribe/sse` | How does SSE compare with WebSocket for the same one-way push path? |
| `pull` | `GET http://auction:8081/auctions/:id` every ~1s | What is the HTTP cost of polling compared with push delivery? |

### Test Parameters

| Parameter | Value |
| --- | --- |
| Total Locust users | 1,000 |
| Subscriber / bidder ratio | 999 subscribers : 1 bidder |
| Ramp-up rate | 50 users / second |
| Test duration | 180 seconds per run |
| Runs per mode | 3 |
| Bid frequency | 1 bid every ~2 seconds |
| Infrastructure | Local Docker Compose |
| Services exercised | auction:8081, notification:8080, bid:8084, Redis |

### Metrics Collected

For **Experiment 3A** (`ws` vs `sse`):

- connection setup success / latency
- server-side fan-out latency from Notification Service `GET /metrics`
- client-observed push latency from `bid_accepted_at` to message receipt

For **Experiment 3B** (`push` vs `pull`):

- auction-service request count and response times
- polling read amplification
- theoretical stale-read risk from the 1-second poll interval

Important caveat:

- in `pull` mode there are no push subscribers, so Notification Service timing is stored only as
  **internal processing overhead**, not delivery latency

---

## 2. Raw Results

### 2a. Experiment 3A — WebSocket

| Run | Avg connect | Avg fan-out | p99 fan-out | Avg client push latency | p99 client push latency |
| --- | --- | --- | --- | --- | --- |
| 1 | 24.4 ms | 15.7 ms | 49.8 ms | 48.2 ms | 90.2 ms |
| 2 | 20.2 ms | 12.9 ms | 51.6 ms | 47.7 ms | 138.2 ms |
| 3 | 22.1 ms | 12.0 ms | 51.6 ms | 45.7 ms | 95.2 ms |
| **Avg** | **22.2 ms** | **13.5 ms** | **51.0 ms** | **47.2 ms** | **107.9 ms** |

Additional notes:

- all three runs established `999 / 999` WebSocket connections
- all three runs finished with `0` failures

### 2b. Experiment 3A — SSE

| Run | Avg connect | Avg fan-out | p99 fan-out | Avg client push latency | p99 client push latency |
| --- | --- | --- | --- | --- | --- |
| 1 | 27.9 ms | 17.3 ms | 60.3 ms | 29.0 ms | 87.3 ms |
| 2 | 33.7 ms | 14.0 ms | 60.3 ms | 26.0 ms | 81.6 ms |
| 3 | 37.6 ms | 13.1 ms | 66.8 ms | 27.2 ms | 108.7 ms |
| **Avg** | **33.1 ms** | **14.8 ms** | **62.5 ms** | **27.4 ms** | **92.5 ms** |

Additional notes:

- all three runs established `999 / 999` SSE connections
- all three runs finished with `0` failures
- run 3 contained a connection setup outlier, which widened the SSE tail metrics

### 2c. Experiment 3B — Pull Baseline

| Run | Poll requests | Avg poll latency | p99 poll latency | Avg internal processing | p99 internal processing |
| --- | --- | --- | --- | --- | --- |
| 1 | 164,184 | 24.9 ms | 85 ms | 11.8 ms | 51.6 ms |
| 2 | 164,024 | 25.9 ms | 93 ms | 11.9 ms | 51.6 ms |
| 3 | 163,191 | 32.2 ms | 110 ms | 11.7 ms | 62.6 ms |
| **Avg** | **163,800** | **27.7 ms** | **96 ms** | **11.8 ms** | **55.3 ms** |

Interpretation note:

- the internal processing numbers above are **not** delivery latency
- they reflect the notification service consuming bid events while iterating over zero subscribers

---

## 3. Comparative Summary

### 3A — WebSocket vs SSE

| Metric | WebSocket | SSE |
| --- | --- | --- |
| Connection success | 999 / 999 in every run | 999 / 999 in every run |
| Avg connect latency | **22.2 ms** | 33.1 ms |
| Avg server-side fan-out | **13.5 ms** | 14.8 ms |
| Avg fan-out p99 | **51.0 ms** | 62.5 ms |
| Avg client push latency | 47.2 ms | **27.4 ms** |
| Avg client push p99 | 107.9 ms | **92.5 ms** |

### 3B — Push vs Pull

| Metric | Push (`ws` / `sse`) | Pull (`poll`) |
| --- | --- | --- |
| Long-lived push connections | 999 | 0 |
| Auction-service read load | initial connection only, then bid traffic | about **163,800** extra reads / run |
| Avg subscriber request cost | negligible after connect | **27.7 ms** avg poll latency |
| Update staleness | event-driven | up to one poll interval |
| Notification delivery latency | meaningful | not directly comparable |

---

## 4. Analysis

### 4.1 Why the Experiment Was Split

The earlier version of Experiment 3 mixed two distinct questions:

1. which push transport is better for one-way updates?
2. why is push preferable to polling for a real-time auction?

Separating them made the results easier to interpret.

### 4.2 WebSocket vs SSE

Across three local runs, both push transports handled `999` subscribers with zero failures.

The main pattern was:

- **WebSocket** connected faster and had lower server-side fan-out latency
- **SSE** showed lower average client-observed push latency
- **SSE** also showed less stable connection tails in one run because of a startup outlier

That means both are viable push transports for this one-way update path, but they have different
operational trade-offs:

- WebSocket is stronger on connection setup and server-side control
- SSE is competitive when the workload is strictly one-way and browser-oriented

### 4.3 Push vs Polling

Polling repeatedly reproduced the same architectural penalty:

- around `164k` extra reads in the same `180s` window
- average poll latency in the high-20ms range
- p99 poll latency around `96 ms`

This is the more important architectural result. Even if each poll is individually cheap, the
aggregate read amplification is substantial compared with push delivery.

### 4.4 Why Pull Latency Was Not Treated as Delivery Latency

In `pull` mode there are no WebSocket or SSE subscribers. Notification Service still consumes the
bid event stream, but it does not actually deliver updates to push clients. Therefore its recorded
timing is only internal processing overhead and should not be compared directly against push
delivery latency.

The meaningful comparison for `pull` is:

- request volume on Auction Service
- poll latency
- the stale-read window introduced by the polling interval

---

## 5. Conclusions

**Finding 1 — Both WebSocket and SSE are viable push transports for one-way auction updates.**

Both modes connected `999 / 999` subscribers with zero failures in all three runs. WebSocket had
the faster connection setup and lower server-side fan-out latency, while SSE showed lower average
client-observed push latency.

**Finding 2 — Polling is the wrong primary architecture for real-time auction updates.**

The pull baseline generated about `163,800` extra reads per run in the same 180-second window,
which is the clearest evidence that polling scales poorly for this use case.

**Finding 3 — Experiment 3 is better expressed as two sub-experiments.**

`WebSocket vs SSE` answers the transport-selection question. `Push vs polling` answers the
architecture-selection question. Combining them into one table obscures the real trade-offs.

**Recommendation:** Keep `WebSocket` as the default real-time transport for the project, retain
`SSE` as a reasonable one-way push alternative for discussion and fallback design, and treat
polling only as a baseline / degradation path rather than the primary delivery mechanism.

---

## 6. Result Files

All raw data for the refined local rerun is in `tests/loadtest_local/results/`:

```text
exp3_ws_run{1,2,3}_metrics.json
exp3_ws_run{1,2,3}_stats.csv
exp3_sse_run{1,2,3}_metrics.json
exp3_sse_run{1,2,3}_stats.csv
exp3_pull_run{1,2,3}_metrics.json
exp3_pull_run{1,2,3}_stats.csv
```

Canonical script:

- `tests/loadtest_local/scenarios/exp3_notification.py`
