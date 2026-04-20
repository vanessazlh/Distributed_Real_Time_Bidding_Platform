# Project Management — SurpriseAuction

## 1. How We Broke Down the Problem

We decomposed the platform into **6 microservices** based on domain boundaries, following Parnas's information-hiding principle — each service owns its data and exposes only an API contract:

| Service              | Owner               | Storage          | Responsibility                                                          |
| -------------------- | ------------------- | ---------------- | ----------------------------------------------------------------------- |
| User Service         | Vanessa             | DynamoDB         | Registration, JWT auth, watchlist, profiles                             |
| Shop Service         | Vanessa             | DynamoDB + MinIO | Shop/item CRUD, image uploads, categories                               |
| Auction Service      | Lucy                | Redis + DynamoDB | Auction lifecycle, bid validation, concurrency strategies (Lua scripts) |
| Bid Service          | Lucy                | Redis + DynamoDB | Bid history, outbid tracking, WON status                                |
| Notification Service | Claire              | Redis Streams    | Per-auction WebSocket fan-out, global notification inbox                |
| Payment Service      | Wendy               | DynamoDB         | Winner charge processing, payment status                                |
| Frontend             | Claire (lead) + all | —                | React SPA, buyer/seller flows, real-time UI                             |

This split meant each pair of team members could work in parallel without stepping on each other's code — services communicate only through REST APIs and Redis Streams events.

## 2. Development Phases

### Phase 1 — Core Foundation (Week 1)

**Goal:** Get the basic auction loop working end-to-end.

- Set up monorepo, Docker Compose, shared event schemas, DynamoDB table init
- User + Shop services: registration, JWT auth, shop/item CRUD
- Auction + Bid services: create auction, place bid (optimistic locking v1), bid history
- Notification service: basic WebSocket per-auction broadcast
- Payment service: simulated charge on auction close
- Frontend MVP: buyer auction feed, bidding panel, seller dashboard
- **Deliverable:** Milestone 1 report + working local demo

### Phase 2 — Correctness & Reliability (Weeks 2–3)

**Goal:** Fix all known bugs, harden the bid path, migrate to Streams.

- Fixed 12 known issues (prioritized High → Medium → Low from PLAN.md):
  - **High:** Auction expiry (closer.go), role-based route protection, seller self-bid prevention, consumer group MKSTREAM, self-rebid duplicate WON records
  - **Medium:** Bid enrichment for My Bids, WebSocket notification completeness, WON status propagation
  - **Low:** Ownership checks on close, input validation, nginx routing for payments, single-account model
- Migrated all inter-service communication from Redis Pub/Sub → **Redis Streams** with consumer groups, XAck, and XAutoClaim for pending message recovery
- Replaced naive pessimistic lock (SETNX + HGetAll + HSet) with **atomic Lua scripts** — single round-trip for bid validation + state update
- Added DynamoDB as durable backing store with write-through caching and stampede protection

### Phase 3 — Feature Expansion (Weeks 3–4)

**Goal:** Build out buyer and seller experience, prepare for load testing.

- Quantity auctions (top-N winners via Redis Sorted Set)
- Geo support (GEOADD/GEOSEARCH, nearby filter, distance badges)
- Image uploads (MinIO/S3, multipart upload, drag-and-drop UI)
- Pickup time windows, item categories, search, watchlist/favorites
- Seller auction detail page with live bid history, winners, payment status
- Minimum bid increment (Lua enforcement)
- Ratings & reviews system
- Mobile responsive polish across all pages

### Phase 4 — Load Testing & Experiments (Week 4)

**Goal:** Validate the system under the three proposed experiments on AWS.

- Experiment 1: 500 concurrent bidders on one auction — compared optimistic, pessimistic, and queue strategies
- Experiment 2: 50 simultaneous auctions × 100 bidders — tested ECS auto-scaling from 2→N tasks
- Experiment 3: 1000 WebSocket clients — measured notification fan-out latency

## 3. Who Worked on What

| Team Member | Primary Services       | Key Contributions                                                                                                                                                                                                                                                                             |
| ----------- | ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Vanessa** | User, Shop             | JWT auth system, single-account model (buyer→seller upgrade), shop/item CRUD, image uploads (MinIO integration), profile/shop editing, watchlist (DynamoDB set operations), ratings & reviews, categories                                                                                     |
| **Lucy**    | Auction, Bid           | Three concurrency strategies (optimistic/pessimistic/queue), Lua script atomicity for bid validation, auction closer (PENDING→OPEN→CLOSED lifecycle), quantity auctions (top-N via ZADD), close sequence reliability with 3-level winner fallback, min bid increment, Redis Streams migration |
| **Claire**  | Notification, Frontend | WebSocket per-auction + per-user global notifications, persistent notification inbox (Redis), React frontend (all pages), real-time bid updates, geo support (GEOSEARCH + frontend map/filter), search, pickup time filters, mobile responsive polish                                         |
| **Wendy**   | Payment, Load Testing  | Payment processing (simulated), per-winner idempotent payments for quantity auctions, Locust load test scripts for all 3 experiments, AWS ECS deployment & auto-scaling configuration                                                                                                         |

## 4. Problems Encountered & How We Solved Them

### Problem 1: Bid Race Conditions

**What happened:** Under load testing (Experiment 1), the optimistic locking strategy showed high reject rates (~40%) as concurrent bids collided and retried.
**Solution:** Replaced with atomic Lua scripts that read-validate-write in a single Redis command. Eliminated all race conditions and reduced bid latency by removing retry loops.

### Problem 2: Redis Pub/Sub Message Loss

**What happened:** When a service restarted, any messages published during the downtime were lost — Pub/Sub has no persistence.
**Solution:** Migrated to Redis Streams with consumer groups. Streams persist messages, XAck tracks processing, and XAutoClaim recovers messages that were read but not acknowledged (e.g., if a consumer crashed mid-processing).

### Problem 3: Cache Stampede on Popular Auctions

**What happened:** When a popular auction's Redis cache expired, dozens of concurrent requests all hit DynamoDB simultaneously to rebuild it.
**Solution:** Implemented double-checked locking — first request acquires a Redis lock and rebuilds the cache, all others wait and read the freshly cached result. Write-through on create ensures the cache is warm from the start.

### Problem 4: Auction Close Reliability

**What happened:** If the event publish step failed after marking an auction CLOSED, downstream services (payment, bid, notification) never learned about the close — resulting in no winner, no payment, no notification.
**Solution:** Reordered the close sequence: atomically close + read winner (Lua script) → publish event → persist to DynamoDB. If the event publish fails, the Lua rollback script reverts status to OPEN so the closer retries on the next tick. Added a three-level winner fallback (Redis ZSET → DynamoDB winners map → DynamoDB highestBidder field).

### Problem 5: Dual-Account Confusion

**What happened:** Users who registered as buyers couldn't become sellers without creating a second account with a different email. This caused confusion and duplicate data.
**Solution:** Switched to a single-account model — one email = one account. If a buyer registers as a seller, it upgrades their existing account (with password verification) rather than creating a duplicate. JWT now carries role, username, and email.

## 5. Initial Design → Final State

```
Initial Design (Week 1)                    Final State (Week 4)
─────────────────────                      ─────────────────────
6 services, REST only                  →   6 services + Redis Streams event bus
Redis Pub/Sub for events               →   Redis Streams with consumer groups + XAutoClaim
Naive SETNX lock for bids              →   Atomic Lua scripts (single round-trip)
Redis-only auction state               →   Redis + DynamoDB (write-through, stampede protection)
Single-winner auctions only            →   Quantity auctions (top-N via Sorted Sets)
No image support                       →   MinIO/S3 with multipart upload
No geo features                        →   GEOADD/GEOSEARCH, nearby filter, distance badges
Dual buyer/seller accounts             →   Single-account with role upgrade
Basic WebSocket per auction            →   Per-auction + global per-user WebSocket + persistent inbox
Manual auction close only              →   Automatic closer (PENDING→OPEN→CLOSED lifecycle)
No input validation                    →   Full validation on all create/update endpoints
Desktop-only UI                        →   Mobile responsive across all pages
3 untested concurrency strategies      →   3 strategies benchmarked under 500-user load test
```

The PLAN.md file served as our project tracker — every feature and bug was listed, prioritized, and marked Done with implementation details as we completed them. Git commits reference the plan items for traceability.
