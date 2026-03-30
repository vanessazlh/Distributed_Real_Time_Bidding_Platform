# Milestone 1 Report — SurpriseAuction

## 1. Problem, Team, and Overview of Experiments

### Problem Statement

Local retailers — bakeries, sushi restaurants, convenience stores — discard unsold end-of-day inventory daily because there is no fast, competitive channel to move it. Buyers would pay discounted prices for surplus goods if they could find them in time.

**SurpriseAuction** is a real-time auction platform where shops list surplus items as short 5-minute auctions. Buyers bid live; the winner is charged automatically when the auction closes.

The core technical challenge is a **concurrency problem**: in the final seconds of a hot auction, hundreds of users submit bids simultaneously. Each bid must atomically check the auction is open, verify the amount exceeds the current highest, update state, and notify all connected clients — all without race conditions. This is compounded by burst traffic: auctions attract spikes at open and close, then go quiet.

### Team

| Name    | Role                            | Contribution                                                                                                                             |
| ------- | ------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| Vanessa | User Service + Shop Service     | User registration, authentication (JWT), shop and item CRUD, owner verification                                                          |
| Lucy    | Auction Service + Bid Service   | Auction lifecycle, bid validation, three concurrency strategies (optimistic locking, pessimistic locking, serialized queue), bid history |
| Claire  | Notification Service + Frontend | Real-time fan-out via WebSocket / SSE / polling, React frontend                                                                          |
| Wendy   | Payment Service + Load Testing  | Winner payment processing, event-driven charge flow, Locust load test scripts for all three experiments                                  |

### Overview of Experiments

Three experiments are designed to evaluate distinct distributed systems challenges:

1. **Experiment 1 — Bid Contention Under Load**: 500 concurrent users bidding on the same auction in a 10-second window. Evaluates three concurrency control strategies — optimistic locking, pessimistic locking, and serialized queue — across metrics including successful bid rate, rejected bid rate, and average/tail latency.

2. **Experiment 2 — Horizontal Scaling Under Auction Spikes**: 50 simultaneous auctions each attracting 100 active bidders, simulating a rush-hour scenario. Starting from 2 ECS Fargate tasks with auto-scaling enabled, measures scale-up response time, latency during the transition window, throughput before and after new tasks join, and bid loss rate.

3. **Experiment 3 — Real-Time Notification Fan-Out**: 1,000 connected clients watching a single high-activity auction. Compares push (WebSocket / SSE) vs. pull (polling) delivery models on notification delivery latency and resource consumption as the connection count scales.

### Role of AI

We use Claude (Sonnet) as a development aid: generating service boilerplate, resolving cross-service integration issues, and drafting documentation. All business logic — routing, bidding, payment processing — is deterministic code written and reviewed by the team. AI is not a runtime component of the system.

### Observability

The system exposes a `/admin/metrics` endpoint on the Auction Service that tracks, in real-time: successful bid count, rejected bid count, P95/P99 bid latency, and the active concurrency strategy. Metrics can be reset between experiment runs via `POST /admin/metrics/reset`. The Notification Service exposes a `/metrics` endpoint tracking active connection count, total broadcasts sent, and average/P99 delivery latency for Experiment 3. All services emit structured logs to stdout, compatible with CloudWatch when deployed on ECS.

---

## 2. Project Plan and Recent Progress

### Timeline

> Detailed task breakdown, implementation notes, and backlog are tracked in [`PLAN.md`](PLAN.md).

| Week                                    | Milestone                                                                                                                                                                                                                                                                                                                                                                         |
| --------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Week 1 _(complete — ending 2026-03-29)_ | Architecture design, monorepo setup, shared event schema; all six services implemented; frontend MVP; docker-compose full-stack integration; Locust scripts for Experiments 1 and 3; Experiment 1 data collection (all three concurrency strategies, 3 runs each); Experiment 3 data collection (WebSocket push vs polling pull, 3 runs each); Milestone 1 report                 |
| Week 2                                  | Feature completion: seller auction dashboard (backend `GET /shops/:id/auctions` + frontend), automatic auction expiry (background goroutine), My Bids enrichment (item title + shop name stored at bid write time), WebSocket outbid and close notifications, role/ownership checks on auction endpoints; Experiment 2 preparation (ECS Fargate infra setup, auto-scaling policy) |
| Week 3                                  | Deployment and DevOps: ECS Fargate task definitions, ALB path-based routing, CloudWatch logging, auto-scaling configuration; Experiment 2 data collection; final results analysis and report writeup                                                                                                                                                                              |
| Week 4 _(final)_                        | Demo polish, presentation prep, submission                                                                                                                                                                                                                                                                                                                                        |

### Recent Progress

- **Monorepo restructured**: All six services migrated to `services/<name>/` under a single root `go.mod` (`module rtb`, Go 1.25).
- **Auction + Bid Services**: Complete. Three concurrency strategies are implemented and switchable at runtime via `PUT /admin/strategy`. Auto-close background goroutine running.
- **User + Shop Services**: Complete. JWT issuance using `"sub"` claim. Owner-only endpoints enforced.
- **Notification Service**: WebSocket, SSE, and polling fan-out implemented. Redis Pub/Sub subscription to `bid_placed` live.
- **Payment Service**: Event consumer subscribing to `auction_closed`. Full payment lifecycle (PENDING → PROCESSING → COMPLETED / FAILED → REFUNDED) backed by DynamoDB. Payment events published to Redis for downstream consumption.
- **Frontend**: React + Vite. Live auction browsing, real-time bid updates via WebSocket, authentication, My Bids page.
- **Shared event schema**: `shared/events/events.go` defines canonical types for all cross-service events, eliminating duplication.

### Division of Work

- **Vanessa**: User + Shop services, shared middleware, frontend auth context
- **Lucy**: Auction + Bid services, concurrency strategies, `/admin/metrics` endpoint, auto-close goroutine
- **Claire**: Notification service (WebSocket / SSE / polling), frontend (auction detail page, bid history feed, real-time hooks)
- **Wendy**: Payment service (DynamoDB-backed, event-driven), Locust load test scripts, docker-compose integration, this report

### Known Issues and Immediate Next Steps

The following issues are tracked and prioritized for Week 2:

| Priority | Issue                                                                                                                                    | Impact                                                            |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| High     | No automatic auction expiry — auctions remain `OPEN` past `end_time` unless closed manually                                              | Breaks the full end-to-end flow (auction close → payment trigger) |
| High     | Seller auction dashboard missing — no `GET /shops/:id/auctions` endpoint or UI                                                           | Sellers cannot view or manage their own auctions                  |
| Medium   | My Bids page shows blank item titles and shop names — bid records do not store `item_title`/`shop_name` at write time                    | Degraded buyer experience                                         |
| Medium   | WebSocket notifications incomplete — `bid_placed` is broadcast but "you've been outbid" and auction-close notifications are not yet sent | Real-time UX incomplete                                           |
| Low      | `POST /auctions` and `POST /auctions/:id/close` lack role and ownership checks                                                           | Any authenticated user can create or close any auction            |

### AI in the Development Process

**Benefits observed:**

- Significant reduction in boilerplate time across six service
- Reliable pattern consistency
- Fast resolution of cross-service issues (e.g., identifying `time.Time` vs `string` mismatch in event structs, aligning JWT claim keys across services)
- Fast Documentation

**Costs and risks:**

- AI-generated code requires careful review
- Context limits mean complex cross-file refactors sometimes require multiple iterations

---

## 3. Objectives

### Short-Term (within this course)

- Complete all six microservices with working end-to-end integration
- Run all three experiments on ECS Fargate and collect quantitative results
- Demonstrate that the choice of concurrency strategy has a measurable, statistically significant effect on bid success rate and latency

### Long-Term (beyond the course)

- **Redis Streams migration**: Replace Redis Pub/Sub (fire-and-forget) with Redis Streams for guaranteed event delivery and consumer acknowledgment — critical for production payment reliability
- **Real payment gateway**: Replace the simulated payment gateway (currently 90% success rate mock) with Stripe or equivalent
- **Shop settlement**: Implement payout routing to shop owners using the `shop_id` recorded at auction creation
- **Shop owner dashboard**: UI for auction creation, item management, and revenue tracking
- **Fraud and abuse prevention**: Rate limiting on bids, anomaly detection on bidding patterns
- **Multi-region deployment**: Reduce latency for geographically distributed users; introduce conflict resolution for cross-region bid state

### Observability Plan

The current observability surface:

- `/admin/metrics` — bid-level metrics (success rate, latency percentiles, strategy)
- `/metrics` on Notification Service — connection count, broadcast latency
- Structured stdout logs on all services (CloudWatch-compatible)

Future observability work:

- Integrate Prometheus + Grafana for time-series dashboards
- Distributed tracing (OpenTelemetry) across the bid → notification → payment chain
- Alerting on P99 latency spikes and error rate thresholds

---

## 4. Related Work

### Related Projects on Piazza

**1. Multiplayer Matchmaking and Player State Engine** (Jassem Alabdulrazaq & Dheepa Maharaji Sankara Subramanian)

Both systems handle high-concurrency spikes on shared mutable state. Their Queue Overload experiment parallels our Experiment 2 (auction spike), and their State Desynchronization experiment (player leaving mid-queue) is directly analogous to our race condition where a bid arrives after an auction closes. The key difference is contention type: their system cooperatively groups players, while ours is adversarial — only one bid wins.

---

**2. Distributed Ticket Reservation System** (Akanbi Jubril Adeyemi)

The most structurally similar project to ours. Both prevent two users from claiming the same resource under concurrent load, and both use Locust to measure success rate, latency, and consistency violations. The difference is that ticket reservation is binary (reserved or not), while our system requires total ordering — a lower bid must never beat a higher one. Their fault-tolerance experiment (killing instances during live traffic) is an angle we did not scope but raises a valid question for our own system.

---

**3. Smart Grocery Assistant — AI-Powered Shopping with Tiered Inference** (Kaiyue Wei, William Gao, Qi Wei)

The overlap is narrower: both projects run async pipelines under concurrent load. Their Experiment 3 (RabbitMQ → AI worker → Redis under load) maps conceptually to our notification fan-out experiment. The difference is significant — their core challenge is tiered inference routing and offline viability, while ours is a concurrency and consistency problem on a real-time competitive resource.

---

## 5. Methodology

### System Architecture

SurpriseAuction is a six-service microservices system deployed on ECS Fargate behind an Application Load Balancer. Services communicate via REST for synchronous operations and Redis Pub/Sub for asynchronous events.

```
Client (Browser)
      │ HTTPS
      ▼
ALB (path-based routing)
      │
      ├── User Service      — registration, auth (PostgreSQL / DynamoDB)
      ├── Shop Service      — shops, items (DynamoDB)
      ├── Auction Service   — bid processing, concurrency control (Redis)
      ├── Bid Service       — bid history (DynamoDB)
      ├── Notification Svc  — WebSocket / SSE / polling fan-out
      └── Payment Service   — winner charging (DynamoDB)

         Redis Pub/Sub (async event bus)
         ├── bid_placed       → Notification Service
         └── auction_closed   → Payment Service
```

### Concurrency Strategies (Experiment 1)

The Auction Service implements three pluggable strategies, switchable at runtime via `PUT /admin/strategy`:

| Strategy        | Mechanism                                                          | Expected behavior                                                    |
| --------------- | ------------------------------------------------------------------ | -------------------------------------------------------------------- |
| **Optimistic**  | Redis `WATCH/MULTI/EXEC`, up to 3 retries with exponential backoff | Low latency under low contention; retry storms under high contention |
| **Pessimistic** | Redis `SETNX` distributed lock (500ms TTL), up to 10 retries       | Eliminates conflicts; serializes writes; higher latency              |
| **Queue**       | Go buffered channel per auction, FIFO                              | Fully serialized, zero conflicts, highest latency, fairest ordering  |

All three strategies guarantee that only one bid wins when two arrive simultaneously, and that no lower bid can overwrite a higher one. The tradeoff is latency vs. throughput vs. conflict rate.

### Load Testing (Locust)

Locust scripts in `loadtest/scenarios/` will simulate each experiment. For Experiment 1, 500 users ramp up over 10 seconds, each sending `POST /auctions/:id/bid` with incrementing amounts. The `/admin/strategy` endpoint is used to switch strategies between runs; `/admin/metrics/reset` clears counters. Results are exported to `loadtest/results/`.

### AI Usage in Methodology

Claude is used to generate Locust scripts, review test plans, interpret ambiguous results, and draft cross-team communication (e.g., integration alignment messages for JWT claim keys and event schema changes). AI is not used to generate or modify experimental data.

### Observability

Each experiment run is bracketed by a `POST /admin/metrics/reset` call. During the run, `/admin/metrics` is polled every second by the Locust master and results written to a timestamped JSON file. For Experiment 3, the Notification Service `/metrics` endpoint is similarly polled. ECS CloudWatch metrics (CPU, memory, task count) are captured for Experiment 2.

---

## 6. Preliminary Results

> Experiments 1 and 3 were run locally (Docker Compose, single machine) on 2026-03-29. Each experiment was run 3 times per strategy/mode; figures below are averages across 3 runs. Experiment 2 (ECS auto-scaling) requires AWS infrastructure and is deferred to Week 3.

### Experiment 1 — Bid Contention Under Load

> Full raw data, per-run breakdown, and analysis are in [`LOAD_TEST_REPORT.md`](LOAD_TEST_REPORT.md).

500 Locust users, 60-second runs, 3 runs per strategy. Summary (averages):

| Strategy    | Avg Successful | Success Rate | Avg Latency | P99 Latency |
| ----------- | -------------- | ------------ | ----------- | ----------- |
| Optimistic  | 16,005         | 19.6%        | 1.15 ms     | 12.6 ms     |
| Pessimistic | 19,733         | **29.3%**    | **0.54 ms** | **1.2 ms**  |
| Queue       | 17,793         | 22.3%        | 0.68 ms     | 2.6 ms      |

Pessimistic locking had the highest success rate and the most stable latency. Optimistic locking's P99 (12.6 ms) was 10x higher than pessimistic due to retry backoff under concurrent WATCH conflicts.

---

### Experiment 2 — Horizontal Scaling

**Status**: Deferred. Requires AWS ECS Fargate with ALB and auto-scaling policy. Infrastructure setup planned for Week 3.

---

### Experiment 3 — Real-Time Notification Fan-Out

> Full raw data and analysis are in [`LOAD_TEST_REPORT.md`](LOAD_TEST_REPORT.md).

999 subscribers + 1 bidder, 180-second runs, 3 runs per mode. Summary:

| Mode             | Auction Service Load | WS Connect Failures | Avg Fan-Out Latency  |
| ---------------- | -------------------- | ------------------- | -------------------- |
| Push (WebSocket) | **~7 req/s**         | 0%                  | 0.7 ms               |
| Pull (Polling)   | ~934 req/s           | —                   | ~1 s (poll interval) |

Push generated 133x less traffic on the auction service. All 999 WebSocket connections were established with zero failures. Pull clients have up to 1 second of inherent update delay vs sub-millisecond for WebSocket push.

---

## 7. Impact

### Why This Work Matters

On the product side, SurpriseAuction targets a gap that large platforms like Too Good To Go don't fill: real-time competitive pricing for micro-retailers with end-of-day surplus.

### Limitations

- **Target audience may not be large.** The platform assumes buyers are nearby and available at the exact time a short auction runs. In practice, coordinating buyer attention around a 5-minute window is a cold-start and discovery problem we have not addressed.
- **Geographic scope is assumed but not implemented.** The platform currently has no location-based discovery, so buyers have no way to find nearby shops.
