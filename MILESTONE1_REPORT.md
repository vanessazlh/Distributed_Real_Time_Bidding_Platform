# Milestone 1 Report — SurpriseAuction

---

## 1. Problem, Team, and Overview of Experiments

### Problem Statement

Food waste is a systemic problem for local retailers. Bakeries, sushi restaurants, and similar shops routinely discard unsold end-of-day inventory because there is no efficient channel to move it quickly and at market-clearing prices. At the same time, consumers are willing to pay discounted prices for high-quality surplus goods if they can be discovered and purchased in time.

**SurpriseAuction** addresses this by providing a real-time auction platform where local shops can list surplus items as short 5-minute auctions. Buyers compete live, and winners are charged automatically when the auction closes. The platform reduces food waste, gives shops a monetization channel for goods that would otherwise be thrown away, and delivers value to consumers.

The core technical challenge is not the concept itself, but the **distributed systems problem it creates**: when an auction is about to close, dozens of users may submit bids within the final seconds — the classic *sniping* problem. Each bid must atomically validate the auction is open, verify the bid exceeds the current highest, update the state, and notify all other connected clients. Under concurrent load, naive implementations will produce inconsistency violations (a lower bid overwriting a higher one, duplicate winners, stale reads). This is a concurrency and consistency problem at its core, compounded by the burst-traffic nature of auctions.

### Team

| Name | Role | Contribution |
|------|------|-------------|
| Vanessa | User Service + Shop Service | User registration, authentication (JWT), shop and item CRUD, owner verification |
| Lucy | Auction Service + Bid Service | Auction lifecycle, bid validation, three concurrency strategies (optimistic locking, pessimistic locking, serialized queue), bid history |
| Claire | Notification Service + Frontend | Real-time fan-out via WebSocket / SSE / polling, React frontend |
| Wendy | Payment Service + Load Testing | Winner payment processing, event-driven charge flow, Locust load test scripts for all three experiments |

### Overview of Experiments

Three experiments are designed to evaluate distinct distributed systems challenges:

1. **Experiment 1 — Bid Contention Under Load**: 500 concurrent users bidding on the same auction in a 10-second window. Evaluates three concurrency control strategies — optimistic locking, pessimistic locking, and serialized queue — across metrics including successful bid rate, rejected bid rate, average latency, and consistency violations.

2. **Experiment 2 — Horizontal Scaling Under Auction Spikes**: 50 simultaneous auctions each attracting 100 active bidders, simulating a rush-hour scenario. Starting from 2 ECS Fargate tasks with auto-scaling enabled, measures scale-up response time, latency during the transition window, throughput before and after new tasks join, and bid loss rate.

3. **Experiment 3 — Real-Time Notification Fan-Out**: 1,000 connected clients watching a single high-activity auction. Compares push (WebSocket / SSE) vs. pull (polling) delivery models on notification delivery latency and resource consumption as the connection count scales.

### Role of AI

Claude (Claude Sonnet) is used throughout the project as a development accelerator: generating boilerplate across all six microservices, resolving cross-service integration issues (event schema alignment, JWT claim key consistency), architecting data models and API contracts, and drafting documentation. AI is explicitly **not** used as a decision-making component within the system itself — all routing, bidding logic, and payment processing are deterministic code.

### Observability

The system exposes a `/admin/metrics` endpoint on the Auction Service that tracks, in real-time: successful bid count, rejected bid count, P95/P99 bid latency, and the active concurrency strategy. Metrics can be reset between experiment runs via `POST /admin/metrics/reset`. The Notification Service exposes a `/metrics` endpoint tracking active connection count, total broadcasts sent, and average/P99 delivery latency for Experiment 3. All services emit structured logs to stdout, compatible with CloudWatch when deployed on ECS.

---

## 2. Project Plan and Recent Progress

### Timeline

| Week | Milestone |
|------|-----------|
| Week 1 *(complete)* | Architecture design, monorepo setup, shared event schema; User, Shop, Auction, Bid services complete; Notification and Payment services core logic; Frontend MVP; Milestone 1 report |
| Week 2 | Full-stack docker-compose integration; Locust scripts for all three experiments; Experiment 1 data collection (all three concurrency strategies); Experiment 3 data collection (WebSocket vs SSE vs polling) |
| Week 3 | Experiment 2 data collection on ECS Fargate with auto-scaling; Redis Streams migration; cross-service integration hardening |
| Week 4 *(final)* | Results analysis, report writeup, demo polish, presentation |

### Recent Progress

- **Monorepo restructured**: All six services migrated to `services/<name>/` under a single root `go.mod` (`module rtb`, Go 1.25).
- **Auction + Bid Services**: Complete. Three concurrency strategies are implemented and switchable at runtime via `PUT /admin/strategy`. Auto-close background goroutine running.
- **User + Shop Services**: Complete. JWT issuance using `"sub"` claim. Owner-only endpoints enforced.
- **Notification Service**: WebSocket and SSE push endpoints implemented for per-auction fan-out, plus polling baseline support via the Auction Service. Redis Streams subscription to bid events is live.
- **Payment Service**: Event consumer subscribing to `auction_closed`. Full payment lifecycle (PENDING → PROCESSING → COMPLETED / FAILED → REFUNDED) backed by DynamoDB. Payment events published to Redis for downstream consumption.
- **Frontend**: React + Vite. Live auction browsing, real-time bid updates via WebSocket, authentication, My Bids page.
- **Shared event schema**: `shared/events/events.go` defines canonical types for all cross-service events, eliminating duplication.

### Division of Work

- **Vanessa**: User + Shop services, shared middleware, frontend auth context
- **Lucy**: Auction + Bid services, concurrency strategies, `/admin/metrics` endpoint, auto-close goroutine
- **Claire**: Notification service (WebSocket / SSE / polling), frontend (auction detail page, bid history feed, real-time hooks)
- **Wendy**: Payment service (DynamoDB-backed, event-driven), Locust load test scripts, docker-compose integration, this report

### AI in the Development Process

**Benefits observed:**
- Significant reduction in boilerplate time across six services (model/repository/service/handler layers)
- Reliable pattern consistency — all services follow the same architectural conventions without manual coordination
- Fast resolution of cross-service issues (e.g., identifying `time.Time` vs `string` mismatch in event structs, aligning JWT claim keys across services)
- Documentation (README, user journey, this report) drafted at near-zero marginal cost

**Costs and risks:**
- AI-generated code requires careful review — incorrect assumptions about library APIs (e.g., AWS SDK v2 call signatures) were caught during testing
- Over-reliance risk: team members must understand the code, not just accept it
- Context limits mean complex cross-file refactors sometimes require multiple iterations

---

## 3. Objectives

### Short-Term (within this course)

- Complete all six microservices with working end-to-end integration
- Run all three experiments on ECS Fargate and collect quantitative results
- Demonstrate that the choice of concurrency strategy has a measurable, statistically significant effect on bid success rate and latency
- Demonstrate WebSocket outperforms polling for notification fan-out under load

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

### Course Readings

- **Fischer, Lynch, and Paterson (FLP Impossibility)**: Informs our understanding of why strong consistency under network partition is unachievable; we accept eventual consistency for bid notifications while requiring strict consistency for the winning bid state.
- **Lamport Clocks / Vector Clocks**: Relevant to ordering bid events across distributed nodes; our Redis-based optimistic locking (WATCH/MULTI/EXEC) provides a single-node serialization point that sidesteps the distributed ordering problem at the cost of scalability.
- **Amazon Dynamo (DeCandia et al.)**: Informed our choice of DynamoDB for the payment ledger — high availability, tunable consistency, and predictable latency at scale.
- **CAP Theorem (Brewer)**: Our architecture makes an explicit CAP tradeoff: Redis (CP) for auction state, DynamoDB (AP with strong reads) for payment records.
- **The Google File System / MapReduce**: Background on how large-scale distributed systems handle failure and replication; applied to our thinking on ECS task recovery and Redis persistence.

### Related Projects on Piazza

> **[PLACEHOLDER — identify the 3 related projects from Piazza and fill in below]**

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

| Strategy | Mechanism | Expected behavior |
|----------|-----------|------------------|
| **Optimistic** | Redis `WATCH/MULTI/EXEC`, up to 3 retries with exponential backoff | Low latency under low contention; retry storms under high contention |
| **Pessimistic** | Redis `SETNX` distributed lock (500ms TTL), up to 10 retries | Eliminates conflicts; serializes writes; higher latency |
| **Queue** | Go buffered channel per auction, FIFO | Fully serialized, zero conflicts, highest latency, fairest ordering |

All three strategies guarantee that only one bid wins when two arrive simultaneously, and that no lower bid can overwrite a higher one. The tradeoff is latency vs. throughput vs. conflict rate.

### Load Testing (Locust)

Locust scripts in `loadtest/scenarios/` will simulate each experiment. For Experiment 1, 500 users ramp up over 10 seconds, each sending `POST /auctions/:id/bid` with incrementing amounts. The `/admin/strategy` endpoint is used to switch strategies between runs; `/admin/metrics/reset` clears counters. Results are exported to `loadtest/results/`.

### AI Usage in Methodology

Claude is used to generate Locust scenario scripts, review test plans for coverage gaps, and assist in interpreting ambiguous results. All Locust scripts are reviewed and validated by the team before use. AI is not used to generate or modify experimental data.

### Observability

Each experiment run is bracketed by a `POST /admin/metrics/reset` call. During the run, `/admin/metrics` is polled every second by the Locust master and results written to a timestamped JSON file. For Experiment 3, the Notification Service `/metrics` endpoint is similarly polled. ECS CloudWatch metrics (CPU, memory, task count) are captured for Experiment 2.

---

## 6. Preliminary Results

> **Note: Experiments are in setup phase. Data collection begins in Week 7. The following describes expected results and preliminary local observations.**

### Experiment 1 — Bid Contention

**Status**: Concurrency strategies implemented and unit-tested. Locust script scaffolded. Awaiting full 500-user run on ECS.

**Local preliminary observation** (informal, single machine):
- All three strategies correctly prevent consistency violations in unit tests
- Optimistic strategy shows retry storm behavior when artificially increasing contention in tests
- Queue strategy produces expected FIFO ordering

**Expected results**: Under 500 concurrent users, optimistic locking will show the highest throughput but elevated retry/rejection rates. Pessimistic locking will show lower throughput but near-zero consistency violations. Queue will show the lowest throughput but perfect ordering.

**[PLACEHOLDER — insert actual latency numbers, success/rejection rates once Experiment 1 is run]**

### Experiment 2 — Horizontal Scaling

**Status**: ECS Fargate infrastructure setup in progress. Auto-scaling policy to be configured.

**Expected results**: Initial latency spike during scale-up window (before new tasks are healthy). Throughput recovery within [PLACEHOLDER] seconds. Zero bid loss expected with connection draining enabled on ALB.

**[PLACEHOLDER — insert CloudWatch metrics, latency graphs, task count timeline once Experiment 2 is run]**

### Experiment 3 — Notification Fan-Out

**Status**: The refined local rerun has completed three runs per mode. Experiment 3A compares `WebSocket` vs `SSE` for one-way auction updates; Experiment 3B compares push delivery vs polling as an architectural trade-off.

**Current local observations (3 runs per mode, 999 subscribers + 1 bidder, 180s each):**

| Variant | Connection result | Server-side fan-out / processing | Client-side push latency | Key takeaway |
| --- | --- | --- | --- | --- |
| `WebSocket` | 999 / 999 connected in every run, 0 failures; avg connect `22.2 ms` | avg `13.5 ms`, p99 `51.0 ms` | avg `47.2 ms`, p99 `107.9 ms` | Faster connection setup and consistently low server-side fan-out cost |
| `SSE` | 999 / 999 connected in every run, 0 failures; avg connect `33.1 ms` | avg `14.8 ms`, p99 `62.5 ms` | avg `27.4 ms`, p99 `92.5 ms` | Competitive push transport with lower average client-observed latency, but less stable connection tail behavior |
| `Pull` | no persistent push connections | internal processing avg `11.8 ms`, p99 `55.3 ms` | not comparable | generated about `163,800` poll requests per 180-second run; avg poll latency `27.7 ms`, p99 `96 ms` |

**Interpretation:** The full rerun validates the new split design. `WebSocket` and `SSE` can now be compared directly as push transports under the same workload, while `pull` is treated as a high-load baseline rather than a fake delivery-latency peer. The clearest pattern across all three runs is:

- `WebSocket` established connections faster and kept lower server-side fan-out latency
- `SSE` showed lower average client-observed push latency, but its connection tail was less stable in one run because of a startup outlier
- `Pull` repeatedly generated roughly `164k` extra HTTP reads in the same 180-second window, confirming the cost difference between push and polling

### Pathological Worst Case

The worst-case workload is **Experiment 1 with optimistic locking**: 500 users targeting a single auction in the final 10 seconds. Under this scenario, every bid update invalidates all concurrent `WATCH` transactions, causing cascading retries. If retry depth exceeds the 3-retry cap, bids are rejected — the system remains consistent but throughput collapses. This is the design tradeoff we intend to quantify.

---

## 7. Impact

### Why This Work Matters

**For distributed systems practitioners**: The three experiments directly evaluate tradeoffs that every real-time, high-concurrency system faces — and for which there is no universal answer. Optimistic vs. pessimistic locking is a recurring design decision in any system with contended shared state (databases, caches, auction platforms, ticketing systems). Our results will provide concrete, reproducible numbers to inform that decision under a specific workload profile.

**For the food waste problem**: If SurpriseAuction were deployed at scale, it would provide a market mechanism that currently doesn't exist for micro-retailers. Unlike large platforms (Too Good To Go, for example), SurpriseAuction is designed from the ground up for real-time competitive pricing rather than fixed-discount sales.

**For the class**: The platform is fully functional and open to external load. We welcome classmates to:
- **Register accounts and place bids** against our running system to stress-test real user behavior
- **Use our Locust scripts** as a reference for their own load testing setups
- **Stress-test the notification fan-out** by connecting multiple WebSocket clients to an active auction

If other teams want to use SurpriseAuction as a load target for their own experiments (e.g., testing an API gateway, a monitoring system, or a circuit breaker), we are happy to coordinate.

### Limitations and Honest Scope

- The payment gateway is simulated. Real-money flows are out of scope for this course project.
- Redis Pub/Sub does not guarantee delivery; a production system would require Redis Streams or a dedicated message broker. This is planned as a post-milestone improvement.
- Experiment 2 (auto-scaling) requires AWS ECS and cannot be fully reproduced locally — classmates wishing to replicate it will need an AWS account.
