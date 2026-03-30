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
        ├── /auctions, /bids       → Auction Service    :8081  (Redis)
        ├── /auctions/:id/subscribe→ Notification Svc   :8080  (Redis Pub/Sub → WebSocket)
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
| Auction | 8081 | Redis | Auction lifecycle, bid validation, concurrency control |
| Bid | 8084 | Redis + DynamoDB | Bid history, outbid tracking, per-user bid queries |
| Notification | 8080 | Redis Pub/Sub | WebSocket fan-out of bid events to watching clients |
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

| Entry Point | Role |
|---|---|
| `http://localhost:3000/login` | Buyer |
| `http://localhost:3000/shop/login` | Seller |

---

## API Reference

All requests pass through nginx at `localhost:3000`. Protected routes require `Authorization: Bearer <jwt>`.

### Auth / Users — User Service

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/users` | — | Register (role: `buyer` or `seller`) |
| `POST` | `/auth/login` | — | Login → `{ token }` |
| `GET` | `/users/:id` | ✓ | Get profile |
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
| `POST` | `/auctions` | ✓ | Create auction |
| `GET` | `/auctions` | — | List auctions (optional `?status=OPEN`) |
| `GET` | `/auctions/:id` | — | Get auction details |
| `POST` | `/auctions/:id/bid` | ✓ | Place bid |
| `POST` | `/auctions/:id/close` | ✓ | Close auction early |

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

| Method | Path | Description |
|---|---|---|
| `GET` | `/auctions/:id/subscribe` | WebSocket — live bid events for an auction |

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

| Strategy | Mechanism | Trade-off |
|---|---|---|
| **Optimistic** | Redis `WATCH/MULTI/EXEC`, retry up to 3× with exponential backoff | Lowest latency; may fail under extreme contention |
| **Pessimistic** | Redis `SETNX` distributed lock (500ms TTL), retry up to 10× | Prevents all conflicts; serializes writes per auction |
| **Queue** | Go channel per auction, FIFO processing | Fully serialized; fairest ordering; highest isolation |

Switch strategies live:
```bash
curl -X PUT http://localhost:3000/admin/strategy \
  -H "Content-Type: application/json" \
  -d '{"strategy": "pessimistic"}'
```

---

## Event-Driven Flow

Services communicate through Redis Pub/Sub. Two domain events are published:

**`bid_placed`**
```
Auction Service → Redis Pub/Sub
    ├── Bid Service        (records bid history)
    └── Notification Svc   (broadcasts to WebSocket watchers)
```

**`auction_closed`**
```
Auction Service → Redis Pub/Sub
    ├── Payment Service    (charges the winning bidder)
    └── Notification Svc   (notifies winner and losing bidders)
```

---

## Environment Variables

| Variable | Default | Used by |
|---|---|---|
| `JWT_SECRET` | `secret` | User, Shop, Auction, Bid, Payment |
| `DYNAMODB_ENDPOINT` | `http://localhost:8000` | User, Shop, Bid, Payment |
| `REDIS_ADDR` | `localhost:6379` | Auction, Bid, Notification, Payment |
| `SERVER_ADDR` | `:808x` | Each service (see ports above) |
| `BID_SERVICE_URL` | `http://bid:8084` | User (for bid proxy) |
| `CONCURRENCY_STRATEGY` | `optimistic` | Auction |

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
