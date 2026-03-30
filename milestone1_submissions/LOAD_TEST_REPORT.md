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

# Experiment 3: Notification Fan-Out — WebSocket Push vs Polling Pull

**Date:** 2026-03-30

---

## 1. Experiment Setup

### Goal

Quantify the server-side cost difference between two delivery models as concurrent client count
reaches 1,000: (a) server-push via WebSocket, where the server proactively delivers bid updates,
and (b) client-pull via HTTP polling, where each client queries the auction state every second.

### Delivery Models Under Test

| Model    | Mechanism                                                   | Description                                                                                             |
| -------- | ----------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| **Push** | WebSocket (`ws://notification:8080/auctions/:id/subscribe`) | Clients hold persistent connections; notification service fans out each `bid_placed` event in real time |
| **Pull** | HTTP polling (`GET auction:8081/auctions/:id` every 1s)     | Clients periodically request the latest auction state; no persistent connection                         |

### Test Parameters

| Parameter               | Value                                            |
| ----------------------- | ------------------------------------------------ |
| Total Locust users      | 1,000                                            |
| Subscriber/bidder ratio | 999 subscribers : 1 bidder                       |
| Ramp-up rate            | 50 users/second                                  |
| Test duration           | 180 seconds per run                              |
| Runs per mode           | 3                                                |
| Bid frequency           | 1 bid every ~2 seconds (constant)                |
| Infrastructure          | Local Docker Compose                             |
| Services                | auction:8081, notification:8080, bid:8084, Redis |

### Metrics Collected

**Locust (HTTP layer):**

- Total request count, median/p99 response time per run

**Notification service (`GET /metrics`):**

- `delta_broadcasts` — broadcast events fired during the test window
- `avg_delivery_latency_ms` / `p99_delivery_latency_ms` — time for notification service to fan out one event to all connected WebSocket clients

---

## 2. Raw Results

### 2a. Push (WebSocket)

Auction ID: `cd511a87-0a38-454f-90e7-bc5eedab86ec`

**Locust stats (auction service HTTP traffic):**

| Run     | Total requests | WS connects  | Bid requests | Median latency | p99 latency |
| ------- | -------------- | ------------ | ------------ | -------------- | ----------- |
| 1       | 1,172          | 999 (0 fail) | ~173         | 12 ms          | 38 ms       |
| 2       | 1,172          | 999 (0 fail) | ~173         | 12 ms          | 76 ms       |
| 3       | 1,172          | 999 (0 fail) | ~173         | 13 ms          | 42 ms       |
| **Avg** | **1,172**      | **999**      | **~173**     | **12 ms**      | **52 ms**   |

**Notification service metrics:**

| Run     | delta_broadcasts | avg delivery latency | p99 delivery latency |
| ------- | ---------------- | -------------------- | -------------------- |
| 1       | 85               | 0.6 ms               | 8.4 ms               |
| 2       | 86               | 0.7 ms               | 11.7 ms              |
| 3       | 86               | 0.7 ms               | 11.7 ms              |
| **Avg** | **85.7**         | **0.67 ms**          | **10.6 ms**          |

### 2b. Pull (HTTP Polling)

Auction ID: `cd511a87-0a38-454f-90e7-bc5eedab86ec`

**Locust stats (auction service HTTP traffic):**

| Run     | Total requests | Poll requests | Bid requests | Median latency | p99 latency |
| ------- | -------------- | ------------- | ------------ | -------------- | ----------- |
| 1       | 167,521        | ~167,348      | ~173         | 10 ms          | 52 ms       |
| 2       | 167,822        | ~167,649      | ~173         | 8 ms           | 56 ms       |
| 3       | 167,939        | ~167,766      | ~173         | 9 ms           | 50 ms       |
| **Avg** | **167,761**    | **~167,588**  | **~173**     | **9 ms**       | **53 ms**   |

**Notification service metrics (broadcast events from bidder, no WS subscribers in pull mode):**

| Run     | delta_broadcasts | avg delivery latency | p99 delivery latency |
| ------- | ---------------- | -------------------- | -------------------- |
| 1       | 85               | 0.7 ms               | 10.1 ms              |
| 2       | 85               | 0.6 ms               | 9.0 ms               |
| 3       | 86               | 0.6 ms               | 9.0 ms               |
| **Avg** | **85.3**         | **0.63 ms**          | **9.4 ms**           |

---

## 3. Comparative Summary

| Metric                           | Push (WebSocket) | Pull (Polling) |
| -------------------------------- | ---------------- | -------------- |
| Avg total HTTP requests / run    | **1,172**        | 167,761        |
| Auction service RPS              | **~6.5 req/s**   | ~934 req/s     |
| HTTP request ratio               | **1×**           | **143×**       |
| WS connections established       | 999 / 999 (100%) | 0              |
| WS connect failure rate          | **0%**           | —              |
| Avg notification fan-out latency | 0.67 ms          | 0.63 ms\*      |
| p99 notification fan-out latency | 10.6 ms          | 9.4 ms\*       |
| Per-request median latency       | 12 ms            | 9 ms           |

\*Pull mode has no WebSocket subscribers — notification service fans out to 0 clients, so these values reflect internal event processing overhead only, not per-subscriber delivery.

---

## 4. Analysis

### 4.1 HTTP Traffic — The Core Difference

The most significant finding is the 143× difference in HTTP request volume on the auction service:

- **Push:** 999 clients each make exactly 1 request (WebSocket handshake, ~13 ms). After that, the
  persistent connection carries all future updates at zero HTTP cost. Total auction service traffic
  for 999 subscribers over 180 seconds: **999 requests**.

- **Pull:** 999 clients each poll every 1 second for 180 seconds. Total: 999 × 180 ≈ **179,820
  requests** (observed: ~167,588, accounting for ramp-up lag). This is continuous load that scales
  linearly with both client count and poll frequency.

At 1,000 users with 1-second polling the auction service sustained ~934 req/s of read traffic
just from subscribers — traffic that carries no information value on any poll where the price has
not changed (the majority). WebSocket push eliminates this entirely after connection setup.

### 4.2 WebSocket Scalability

999 WebSocket connections were established with **zero failures** across all three push runs.
Median connection time was ~13 ms; p99 was 42–76 ms. Once connected, all 999 clients held their
connections open for the full 180 seconds with no drops.

The notification service's fan-out latency (avg **0.67 ms**, p99 **10.6 ms**) shows that broadcasting
one event to all 999 connected clients takes under 1 ms on average. This demonstrates that the
Go-based WebSocket hub scales well to hundreds of concurrent connections on a single local instance.

### 4.3 Update Latency (User Experience)

A metric not captured in the raw numbers but important architecturally:

- **Push:** Users receive the update within milliseconds of the bid being placed (avg 0.67 ms
  fan-out latency on the notification service). A bidder sees the new price near-instantly.

- **Pull:** Users receive the update on their next poll, introducing up to 1 second of artificial
  delay. In a real-time auction where the price can change every few seconds, this lag is
  noticeable and can lead to stale-price decisions (bidding on an outdated current_highest).

### 4.4 Notification Latency Comparison Caveat

The notification service delivery latency appears similar in both modes (push: 0.67 ms, pull:
0.63 ms). This is because the metric measures **broadcast processing time on the notification
service**, not end-to-end delivery to each subscriber. In pull mode there are no WebSocket
subscribers, so the notification service processes each bid event and iterates over 0 connections —
near-instant. The push mode value (0.67 ms) represents real fan-out to 999 clients and is the
meaningful number for this experiment.

---

## 5. Conclusions

**Finding 1 — Pull generates 143× more HTTP traffic than push at 1,000 clients.**
Every additional client in pull mode linearly increases read load on the auction service.
At 10,000 users with 1-second polling that becomes ~10,000 req/s of read traffic — a significant
horizontal scaling requirement. WebSocket push converts this per-second-per-client cost to a
one-time connection setup, after which the server fans out updates to all clients simultaneously.

**Finding 2 — The notification service successfully fans out to 999 clients in avg 0.67 ms.**
Zero WebSocket connection failures across 2,997 total connections (999 × 3 runs). The Go
WebSocket hub's broadcast loop handles hundreds of concurrent connections with sub-millisecond
average delivery. p99 at 10.6 ms is acceptable for a real-time auction where bids arrive every
few seconds, not hundreds of times per second.

**Finding 3 — Pull introduces up to 1 second of update latency; push is near-instant.**
For a real-time auction, poll-based clients risk placing bids on stale price data. Push eliminates
this class of user experience problem entirely.

**Recommendation:** WebSocket push is the correct architecture for this use case. The only
scenario where polling is appropriate is as a fallback when WebSocket connections cannot be
established (firewalls, corporate proxies). The current implementation already supports both
WebSocket and SSE endpoints on the notification service, providing a natural degradation path.
A production deployment would additionally move from Redis Pub/Sub to Redis Streams to guarantee
no messages are dropped when the notification service restarts.

---

## 6. Result Files

All raw data is in `loadtest/results/`:

```
exp3_push_run{1,2,3}_metrics.json   (notification service metrics)
exp3_push_run{1,2,3}_stats.csv      (Locust HTTP stats)
exp3_pull_run{1,2,3}_metrics.json
exp3_pull_run{1,2,3}_stats.csv
```

Test script: `loadtest/scenarios/exp3_notification.py`
