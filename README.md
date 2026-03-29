# SurpriseAuction

A platform where local stores post surplus or end-of-day items as rapid **5-minute auctions** instead of throwing them away.

For example, a bakery might post **3 mystery pastry boxes at 5pm**, and users have **5 minutes to bid** before the auction closes.

---

# Architecture

```
Client (Browser / Mobile)
        |
       HTTPS
        |
   ALB (Path-based routing)
        |
        └── Routes to ECS Fargate tasks
              |
    ┌─────────┬──────────┬─────────────┬──────────────┬─────────────┐
    │  User   │  Shop    │  Auction    │  Notification│  Payment    │
    │ Service │ Service  │  Service    │   Service    │  Service    │
    └─────────┴──────────┴──────┬──────┴──────┬───────┴──────┬──────┘
                                │             │              │
                         ───────┴─────────────┴──────────────┴───────
                                     Redis Pub/Sub (async events)
                                  bid_placed · auction_closed
                                  payment_processed · payment_failed · refund_processed
                         ───────────────────────────────────────────
                                │                          │
                         Redis (auction state)      DynamoDB
                         real-time bid data         payments, users,
                         + pub/sub bus              shops, bid history
```

**Key flows:**

- **Bid flow** — Client → ALB → Auction Service (validates + updates bid atomically in Redis) → publishes `bid_placed` → Notification Service fans out to connected clients
- **Auction close flow** — Auction Service publishes `auction_closed` → Payment Service creates payment record → publishes `payment_processed`

---

# Core Challenge: Real-Time Bidding Under Extreme Concurrency

When an auction is about to close, dozens of users may submit bids within the final seconds (the **"sniping" problem**).

Each bid must:

- Validate that the auction is still open
- Check that the new bid is higher than the current highest bid
- Update the highest bid **atomically**
- Notify other bidders that they have been outbid

All of these operations must happen **consistently under concurrent load**.

If two users submit bids at the exact same millisecond:

- Only **one bid should win**
- No user should observe **stale data**

At its core, this is a **concurrency and consistency problem**.

---

# Burst Traffic Characteristics

Auctions are **bursty by nature**.

Example scenario:

- A popular store posts **10 auctions at 5pm**
- Thousands of users flood the system simultaneously

Then at **5:05pm**, another spike occurs when:

- Auctions close
- Winners are processed
- Notifications are sent
- Payments are handled

The system must:

- **Scale horizontally** to absorb sudden spikes
- **Scale down** when traffic drops

---

# Services

Services communicate asynchronously via **Redis Pub/Sub**. Direct HTTP calls are only used for client-facing APIs.

## User Service

| Method | Path                   | Auth |
| ------ | ---------------------- | ---- |
| POST   | `/users`               | —    |
| POST   | `/auth/login`          | —    |
| GET    | `/users/:user_id`      | JWT  |
| GET    | `/users/:user_id/bids` | JWT  |

## Shop Service

| Method | Path                    | Auth |
| ------ | ----------------------- | ---- |
| POST   | `/shops`                | JWT  |
| GET    | `/shops/:shop_id`       | —    |
| POST   | `/shops/:shop_id/items` | JWT  |
| GET    | `/shops/:shop_id/items` | —    |
| GET    | `/items/:item_id`       | —    |

## Auction Service

| Method | Path                   | Auth |
| ------ | ---------------------- | ---- |
| POST   | `/auctions`            | JWT  |
| GET    | `/auctions`            | —    |
| GET    | `/auctions/:id`        | —    |
| POST   | `/auctions/:id/bid`    | JWT  |
| POST   | `/auctions/:id/close`  | JWT  |
| GET    | `/auctions/:id/bids`   | —    |
| GET    | `/admin/metrics`       | —    |
| POST   | `/admin/metrics/reset` | —    |
| GET    | `/admin/strategy`      | —    |
| PUT    | `/admin/strategy`      | —    |

Publishes: `bid_placed`, `auction_closed`

## Bid Service

_Bid history is currently stored within Auction Service. Standalone Bid Service — to be updated._

## Payment Service

| Method | Path                            | Auth |
| ------ | ------------------------------- | ---- |
| GET    | `/payments/:id`                 | JWT  |
| GET    | `/users/:user_id/payments`      | JWT  |
| GET    | `/auctions/:auction_id/payment` | JWT  |
| POST   | `/admin/payments/:id/process`   | —    |
| POST   | `/admin/payments/:id/refund`    | —    |

Subscribes: `auction_closed` — Publishes: `payment_processed`, `payment_failed`, `refund_processed`

## Notification Service

| Method | Path                                  | Auth |
| ------ | ------------------------------------- | ---- |
| GET    | `/auctions/:auction_id/subscribe`     | —    |
| GET    | `/auctions/:auction_id/subscribe/sse` | —    |
| GET    | `/metrics`                            | —    |

Subscribes: `bid_placed` — fans out to all connected WebSocket/SSE clients watching the auction

---

# Tech Stack

- **Go** — all backend services
- **Redis** — Pub/Sub event bus between services; auction state storage for fast reads/writes
- **DynamoDB** — persistent storage for payments and other records
- **ECS Fargate** with **ALB** and auto-scaling — deployment infrastructure
- **WebSockets / SSE** — real-time bid notifications to connected clients
- **Locust** — load testing and performance evaluation

---

# Scalability Experiments

## Experiment 1: Bid Contention Under Load

Simulate **500 concurrent users** bidding on the same auction during the **final 10 seconds**.

Compare different concurrency control strategies:

- Optimistic locking with retries
- Pessimistic locking
- Serialized bid queue

### Metrics

- Successful bid rate
- Rejected bid rate
- Average bid latency
- Consistency violations  
  (e.g., a lower bid winning over a higher bid)

---

## Experiment 2: Horizontal Scaling During Auction Spikes

Simulate a **rush-hour scenario**:

- **50 auctions** go live simultaneously
- Each attracts **100 bidders**

System starts with **2 ECS tasks** with auto-scaling enabled.

### Metrics

- Auto-scaling response time to the spike
- Latency during the scale-up window
- Throughput **before vs. after** new tasks join
- Whether any bids are lost during scaling transitions

---

## Experiment 3: Real-Time Notification Fan-Out

Whenever a **new highest bid** is placed, all other bidders watching the auction must be notified.

Simulate:

- **1000 connected clients**
- Watching a **single popular auction**
- With rapid bid updates

### Metrics

- Notification delivery latency  
  (time from bid acceptance to all clients being notified)

- System resource usage as the number of connected clients scales

- Performance comparison:
  - **Push model** (WebSockets)
  - **Pull model** (polling)
