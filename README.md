# SurpriseAuction — Real-Time Surplus Auction Platform

A microservices platform where local stores auction surplus items in short live windows. Buyers compete in real-time bidding; winners are charged automatically when the auction closes.

Built for a distributed systems course, with a focus on **concurrent bid processing**, **horizontal scaling**, and **real-time notification fan-out**.

---

## Architecture

```
Browser (React + Vite)
        │
        ▼
  nginx (port 3000)  ← SPA + reverse proxy
        │
        ├── /auth, /users          → User Service       :8082  (DynamoDB)
        ├── /shops, /sellers       → Shop Service       :8083  (DynamoDB)
        ├── /shops/:id/auctions    → Auction Service    :8081  (Redis)
        ├── /auctions/:id/bids     → Bid Service        :8084  (Redis + DynamoDB)
        ├── /auctions, /bids       → Auction Service    :8081  (Redis)
        ├── /auctions/:id/subscribe→ Notification Svc   :8080  (Redis Streams → WebSocket)
        ├── /notifications/subscribe→ Notification Svc  :8080  (per-user WebSocket)
        ├── /notifications          → Notification Svc  :8080  (REST — list/mark-read)
        ├── /bids                  → Bid Service        :8084  (Redis + DynamoDB)
        └── /payments              → Payment Service    :8085  (DynamoDB)
```

**Infrastructure:** DynamoDB Local (dev) · Redis 7 · Docker Compose

---

## Services

| Service | Port | Storage | Description |
|---|---|---|---|
| User | 8082 | DynamoDB | Registration, login, JWT auth, bid proxy |
| Shop | 8083 | DynamoDB | Shop + item CRUD, seller ownership checks |
| Auction | 8081 | Redis + DynamoDB | Auction lifecycle, bid validation, concurrency control |
| Bid | 8084 | Redis + DynamoDB | Bid history, outbid tracking, per-user bid queries |
| Notification | 8080 | Redis Streams + Redis | Per-auction WebSocket fan-out, per-user global WebSocket, persistent notification inbox |
| Payment | 8085 | DynamoDB | Winner charge processing, payment status tracking |
| Frontend | 3000 | — | React SPA + nginx reverse proxy |

---

## Quick Start

```bash
# Build and start all services
docker-compose up --build

# Open the app
open http://localhost:3000
```

On first run, the `init-tables` container automatically creates all DynamoDB tables before the dependent services start.

### Roles

Both buyers and sellers use the same login page at `http://localhost:3000/login`.
Select your role using the **Buyer / Seller toggle** before signing in.

| Role | How to access |
|---|---|
| Buyer | Go to `/login`, select **Buyer**, sign in → lands on auction feed |
| Seller | Go to `/login`, select **Seller**, sign in → lands on seller dashboard |
| New buyer | Go to `/register`, select **Buyer** |
| New seller | Go to `/register`, select **Seller** (or upgrade an existing buyer account) |

A seller account can also log in as a buyer to browse and bid on other shops' auctions.

---

## API Reference

All requests pass through nginx at `localhost:3000`. Protected routes require `Authorization: Bearer <jwt>`.

### Auth / Users — User Service

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/users` | — | Register (`role: buyer` or `seller`; existing buyer + seller role = upgrade) |
| `POST` | `/auth/login` | — | Login → `{ token }` (single account per email) |
| `GET` | `/users/:id` | ✓ | Get profile |
| `PUT` | `/users/:id` | ✓ | Update profile (username) — owner only |
| `GET` | `/users/:id/bids` | ✓ | List user's bids (proxied to Bid Service) |

### Shops + Items — Shop Service

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/shops` | ✓ seller | Create shop |
| `GET` | `/shops/:id` | — | Get shop |
| `GET` | `/sellers/:userId/shops` | ✓ | List shops owned by a seller |
| `POST` | `/shops/:id/items` | ✓ seller | Add item to shop |
| `GET` | `/shops/:id/items` | — | List items in a shop |

### Auctions — Auction Service

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/auctions` | ✓ | Create auction (optional: `scheduled_start`, `pickup_start`, `pickup_end`) |
| `GET` | `/auctions` | — | List auctions (optional `?status=OPEN`) |
| `GET` | `/auctions/:id` | — | Get auction details |
| `POST` | `/auctions/:id/bid` | ✓ | Place bid |
| `POST` | `/auctions/:id/close` | ✓ | Close auction early (owner only) |
| `GET` | `/shops/:id/auctions` | ✓ | List auctions for a shop |

### Bids — Bid Service

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/auctions/:id/bids` | — | Bid history for an auction |
| `GET` | `/users/:id/bids` | ✓ | All bids by a user |

### Payments — Payment Service

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/auctions/:id/payment` | ✓ | Payment for a specific auction |
| `GET` | `/users/:id/payments` | ✓ | All payments for a user |

### Notifications — Notification Service

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/auctions/:id/subscribe` | — | WebSocket — live bid/close events for an auction |
| `GET` | `/notifications/subscribe?token=<jwt>` | ✓ | WebSocket — global per-user notifications (outbid, won) |
| `GET` | `/notifications` | ✓ | List stored notifications + unread count |
| `POST` | `/notifications/read` | ✓ | Mark all notifications as read |

### Admin — Auction Service

| Method | Path | Description |
|---|---|---|
| `GET` | `/admin/metrics` | Bid metrics (latency, success/reject counts, P95/P99) |
| `POST` | `/admin/metrics/reset` | Reset metrics counters |
| `GET` | `/admin/strategy` | Current concurrency strategy |
| `PUT` | `/admin/strategy` | Switch strategy (`optimistic` / `pessimistic` / `queue`) |

---

## Concurrency Strategies

The platform supports three pluggable bid-concurrency strategies, switchable at runtime without restarting:

| Strategy | Mechanism | Trade-off | Status |
|---|---|---|---|
| **Pessimistic** | Lua script — atomic read/validate/write in a single Redis call | Zero-lock, zero-retry; supports quantity auctions | **Default (production)** |
| **Optimistic** | Redis `WATCH/MULTI/EXEC`, retry up to 3× with exponential backoff | Lowest latency; may fail under extreme contention | Experimental — does not work across multiple Redis nodes |
| **Queue** | Go channel per auction, FIFO processing | Fully serialized; fairest ordering; highest isolation | Experimental — per-process only, does not scale horizontally |

> **Note:** Optimistic and Queue strategies are in `concurrency/experimental/` and do **not** support quantity auctions (quantity > 1). They are kept for benchmarking and single-node development.

Switch strategies live:
```bash
curl -X PUT http://localhost:3000/admin/strategy \
  -H "Content-Type: application/json" \
  -d '{"strategy": "pessimistic"}'
```

---

## Event-Driven Flow

Services communicate through **Redis Streams** with consumer groups for durable, replayable, exactly-once delivery. Each consuming service has its own consumer group, so messages are processed independently and can scale horizontally without duplicate processing.

**`bid:placed`** stream
```
Auction Service → XADD bid:placed
    ├── Bid Service        [group: bid-service]        (records bid history)
    └── Notification Svc   [group: notification-service] (broadcasts to auction watchers + stores/pushes "outbid" to previous bidder)
```

**`auction:closed`** stream
```
Auction Service → XADD auction:closed
    ├── Payment Service    [group: payment-service]      (charges the winning bidder(s) — one payment per winner for quantity auctions)
    ├── Bid Service        [group: bid-service]           (marks winning bid(s) as WON)
    └── Notification Svc   [group: notification-service]  (broadcasts to auction watchers + stores/pushes "won" to all winners)
```

**Reliability features:**
- `XGroupCreateMkStream` — atomic stream + consumer group creation on cold start
- `XReadGroup` — blocking reads with configurable worker pools for concurrent processing
- `XAck` — messages are acknowledged only after successful processing (bid/payment services); notification service uses best-effort ACK since retrying failed WebSocket pushes is unlikely to help
- `XAutoClaim` — pending messages from dead consumer instances are automatically reclaimed (60s idle threshold, 30s reclaim interval)

### Reliable Close Sequence

The close flow is designed to prevent dead states where an auction is marked CLOSED but downstream services never receive the event:

1. **Atomic close + read winner(s)** — a Lua script atomically sets `status=CLOSED` and reads the winner(s) from a Redis Sorted Set (top-N for quantity auctions; fallback: hash field, then DynamoDB winners map)
2. **Publish event** — if this fails, the status is **rolled back to OPEN** so the closer retries on the next tick
3. **Persist to DynamoDB** — best-effort; the event has already been delivered
4. **Cleanup Redis** — delete hash, sorted set, remove from active set

Every bid also writes to a DynamoDB `winners` map asynchronously, so winner data survives a full Redis restart.

### Quantity Auctions (Multiple Winners)

Auctions support a `quantity` field (default 1) that determines how many buyers can win. For `quantity > 1`:

- The Redis Sorted Set tracks the top-N bidders by bid amount
- A Lua script handles slot management atomically: when all N slots are full, the lowest winner is evicted if a higher bid arrives
- `current_highest` reflects the **floor price** (lowest winning bid, or start_bid while slots remain open)
- On close, the top-N winners are read from the ZSET; the Payment Service creates one payment per winner
- The Notification and Bid services notify/mark all N winners

Quantity auctions require the **pessimistic** concurrency strategy (Lua script atomicity). The experimental strategies reject bids on `quantity > 1` auctions.

### Pickup Time Windows

Auctions support an optional **pickup window** (`pickup_start` and `pickup_end`, both RFC3339 timestamps) that tells buyers when they can physically collect their won item. Pickup is per-auction, not per-item — the same catalog item can have different pickup windows in different auctions.

- Sellers set the pickup window when creating an auction (or leave it empty to arrange separately)
- The homepage offers a second filter row — **Any Time / Morning / Afternoon / Evening** — that filters auctions by the hour of `pickup_start`. Auctions without a pickup window are hidden when a time filter is active.
- Auction cards show the pickup window below the bid info; the detail page displays it prominently with a clock icon

---

## Environment Variables

| Variable | Default | Used by |
|---|---|---|
| `JWT_SECRET` | `secret` | User, Shop, Auction, Bid, Payment |
| `DYNAMODB_ENDPOINT` | `http://localhost:8000` | User, Shop, Bid, Payment |
| `REDIS_ADDR` | `localhost:6379` | Auction, Bid, Notification, Payment |
| `SERVER_ADDR` | `:808x` | Each service (see ports above) |
| `BID_SERVICE_URL` | `http://bid:8084` | User (for bid proxy) |
| `CONCURRENCY_STRATEGY` | `pessimistic` | Auction |
| `BID_WORKERS` | `10` | Bid (stream consumer concurrency) |
| `NOTIF_WORKERS` | `10` | Notification (stream consumer concurrency) |
| `PAYMENT_WORKERS` | `10` | Payment (stream consumer concurrency) |

All defaults are pre-configured in `docker-compose.yml`.

---

## Running Tests

```bash
go test ./...
```

---

## Research Experiments

### 1. Bid Contention Under Load
Simulate 500 concurrent users bidding on the same auction in its final 10 seconds. Compare the three concurrency strategies on:
- Successful vs. rejected bid rate
- Average bid latency and P95/P99
- Consistency violations (lower bid winning)

### 2. Horizontal Scaling During Auction Spikes
Simulate a rush-hour scenario: 50 auctions go live simultaneously, each attracting 100 bidders, against 2 ECS tasks with auto-scaling enabled. Measure:
- Auto-scaling response time
- Latency during the scale-up window
- Bids lost during scaling transitions

### 3. Notification Fan-Out
Simulate 1000 clients watching a single popular auction with rapid bid updates. Compare:
- Push (WebSocket) vs. pull (polling) delivery latency
- Resource usage as connected clients scale
