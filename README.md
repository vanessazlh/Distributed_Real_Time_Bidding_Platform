# Real-Time Surplus Auction Platform

A microservices-based platform where local stores auction surplus items in short 5-minute windows. Built for a distributed systems course, focusing on real-time bidding concurrency, horizontal scaling, and notification fan-out.

## Services

| Service | Status | Owner | Description |
|---------|--------|-------|-------------|
| User Service | ✅ Done | Vanessa | Registration, login, JWT auth, profile |
| Shop Service | ✅ Done | Vanessa | Shop + item CRUD, owner verification |
| Auction Service | 🚧 In Progress | Lucy | Auction lifecycle, bid validation, concurrency control |
| Bid Service | 🚧 In Progress | Lucy | Bid history storage and queries |
| Notification Service | 🚧 In Progress | Claire | WebSocket / SSE / polling fan-out |
| Payment Service | 🚧 In Progress | Wendy | Winner payment processing |

## Tech Stack

- **Language**: Go (gin framework)
- **Database**: DynamoDB (Local for dev, AWS for prod)
- **Auth**: JWT (golang-jwt) + bcrypt
- **Infra**: ECS Fargate, ALB, auto-scaling
- **Testing**: Locust (load testing), Go built-in testing

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

### Auction Service (coming soon)
- `POST /auctions` — Create auction
- `GET /auctions/:auction_id` — Get auction
- `POST /auctions/:auction_id/bid` — Place bid
- `POST /auctions/:auction_id/close` — Close auction

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | `secret` | JWT signing key |
| `DYNAMODB_ENDPOINT` | `http://localhost:8000` | DynamoDB endpoint |
| `SERVER_ADDR` | `:8080` | Server listen address |

## Running Tests

```bash
go test ./...
```

## Experiments (Planned)

1. **Bid contention** — Optimistic locking vs pessimistic locking vs serialized queue
2. **Horizontal scaling** — Auto-scaling under auction spike traffic
3. **Notification fan-out** — Push (WebSocket/SSE) vs pull (polling) performance