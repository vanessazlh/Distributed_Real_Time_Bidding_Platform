# Real-Time Surplus Auction Platform

A microservices-based platform where local stores auction surplus items in short 5-minute windows. Built for a distributed systems course, focusing on real-time bidding concurrency, horizontal scaling, and notification fan-out.

## Services

| Service | Status | Owner | Description |
|---------|--------|-------|-------------|
| User Service | ✅ Done | Vanessa | Registration, login, JWT auth, profile |
| Shop Service | ✅ Done | Vanessa | Shop + item CRUD, owner verification |
| Auction Service | ✅ Done | Lucy | Auction lifecycle, bid validation, concurrency control |
| Bid Service | ✅ Done | Lucy | Bid history storage, outbid tracking, user bid queries |
| Notification Service | 🚧 In Progress | Claire | WebSocket / SSE / polling fan-out |
| Payment Service | 🚧 In Progress | Wendy | Winner payment processing |

## Tech Stack

- **Language**: Go (gin framework)
- **Database**: DynamoDB (Local for dev, AWS for prod)
- **Cache / Concurrency**: Redis 7 (optimistic locking, pessimistic locking, pub/sub)
- **Auth**: JWT (golang-jwt) + bcrypt
- **Events**: Redis Pub/Sub (`bid_placed`, `auction_closed`)
- **Infra**: Docker Compose (local), ECS Fargate + ALB (prod)
- **Testing**: Go built-in testing, Locust (load testing)

## Quick Start

```bash
# 1. Start DynamoDB Local
docker-compose up -d

# 2. Create tables
go run scripts/init_tables.go

# 3. Start the server
go run cmd/server/main.go
```

Server runs on `localhost:8080` by default.

## API Overview

### User Service
- `POST /users` — Register
- `POST /auth/login` — Login (returns JWT)
- `GET /users/:user_id` — Get profile (auth required)

### Shop Service
- `POST /shops` — Create shop (auth required)
- `GET /shops/:shop_id` — Get shop
- `POST /shops/:shop_id/items` — Create item (auth required, owner only)
- `GET /shops/:shop_id/items` — List items

### Auction Service
- `POST /auctions` — Create auction (auth required)
- `GET /auctions` — List auctions (filterable by status)
- `GET /auctions/:id` — Get auction details
- `POST /auctions/:id/bid` — Place bid (auth required)
- `POST /auctions/:id/close` — Close auction (auth required)

### Bid Service
- `GET /auctions/:id/bids` — Get all bids for an auction
- `GET /users/:user_id/bids` — Get all bids by a user (auth required)

### Admin / Experiments
- `GET /admin/metrics` — Get bid metrics (latency, success/rejection counts, P95/P99)
- `POST /admin/metrics/reset` — Reset metrics counters
- `GET /admin/strategy` — Get current concurrency strategy
- `PUT /admin/strategy` — Switch strategy (`optimistic`, `pessimistic`, `queue`)

## Concurrency Strategies

The platform supports three pluggable bid-concurrency strategies, switchable at runtime via the admin endpoint:

| Strategy | How it works | Trade-off |
|----------|-------------|-----------|
| **Optimistic** | Redis `WATCH/MULTI/EXEC`, retry up to 3x with exponential backoff | Lowest latency, may fail under high contention |
| **Pessimistic** | Redis `SETNX` distributed lock (500ms TTL), retry up to 10x | Prevents conflicts, serializes writes |
| **Queue** | Go channel per auction, FIFO processing | Fully serialized, fairest ordering |

## Event-Driven Architecture

Domain events are published via Redis Pub/Sub for downstream services:

- **`bid_placed`** — auction_id, bid_id, user_id, amount, previous highest
- **`auction_closed`** — auction_id, winner_id, winning_bid, item_id, shop_id

A background goroutine auto-closes expired auctions every second.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | `secret` | JWT signing key |
| `DYNAMODB_ENDPOINT` | `http://localhost:8000` | DynamoDB endpoint |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `SERVER_ADDR` | `:8080` | Server listen address |

## Running Tests

```bash
go test ./...
```

## Experiments

1. **Bid contention** — Optimistic locking vs pessimistic locking vs serialized queue (use `/admin/strategy` to switch, `/admin/metrics` to compare)
2. **Horizontal scaling** — Auto-scaling under auction spike traffic
3. **Notification fan-out** — Push (WebSocket/SSE) vs pull (polling) performance