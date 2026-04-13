# SurpriseAuction — Testing Plan

---

## Baseline Smoke Tests (run first, locally)

Verify the full end-to-end flow works before running load experiments.

> **Setup:** `docker compose up --build`, then `go run scripts/init_tables.go`

**Automated script:** `tests/smoke_test.sh` — runs B1–B15 end-to-end against locally running Docker services.

```bash
chmod +x tests/smoke_test.sh
./tests/smoke_test.sh
```

| #   | Test                   | How                                                                 | Expected                                       |
| --- | ---------------------- | ------------------------------------------------------------------- | ---------------------------------------------- |
| B1  | Register buyer         | `POST /users` with `role: buyer`                                    | 200, returns `user_id`                         |
| B2  | Register seller        | `POST /users` with `role: seller`                                   | 200, returns `user_id`                         |
| B3  | Login both roles       | `POST /auth/login`                                                  | JWT with correct `role` claim (`sub` field)    |
| B4  | Seller creates shop    | `POST /shops` with seller token                                     | 200, returns `shop_id`                         |
| B5  | Seller adds item       | `POST /shops/:id/items` with `retail_value > 0`                     | 200, returns `item_id`                         |
| B6  | Seller creates auction | `POST /auctions` with `duration_minutes`, `start_bid`               | Auction in `OPEN` status                       |
| B7  | List auctions          | `GET /auctions`                                                     | Auction appears on homepage                    |
| B8  | Buyer places bid       | `POST /auctions/:id/bid` with `amount > current_highest_bid`        | Bid accepted, `current_highest_bid` updates    |
| B9  | Bid rejected (too low) | `POST /auctions/:id/bid` with `amount <= current_highest_bid`       | 4xx or rejected                                |
| B10 | WebSocket live update  | Connect to `/auctions/:id/subscribe`, place bid from another client | Connected client receives `bid_placed` event   |
| B11 | Bid history            | `GET /auctions/:id/bids`                                            | All placed bids visible                        |
| B12 | User bid history       | `GET /users/:id/bids`                                               | Buyer's bids returned (proxied to Bid Service) |
| B13 | Auction auto-close     | Wait for `end_time` to pass                                         | Status → `CLOSED`                              |
| B14 | Payment auto-triggered | `GET /auctions/:id/payment` after close                             | Payment record in `completed` or `failed`      |
| B15 | Strategy switch        | `PUT /admin/strategy` → `pessimistic`, place bid, switch back       | Bids work under all 3 strategies               |

**Last run: 2026-03-29 — 20/20 passed ✅**

---

## Experiment 1 — Bid Contention Under Load

**Goal:** Compare 3 concurrency strategies under 500 concurrent bidders on a single auction.

**Infrastructure:** Local Docker (strategy comparison is valid locally)

**Setup:**

- 1 auction, 10-minute window, starting bid $3.00
- 500 Locust users each continuously placing bids (random amount slightly above current)
- Ramp from 0 → 500 users over 30s
- Run once per strategy; switch via `PUT /admin/strategy`, reset via `POST /admin/metrics/reset`

**Metrics** (from `GET /admin/metrics` on Auction Service `:8081`):

| Metric                  | Field                                                |
| ----------------------- | ---------------------------------------------------- |
| Total bids attempted    | `total_bids`                                         |
| Successful bids         | `successful_bids`                                    |
| Rejected bids           | `rejected_bids`                                      |
| Avg / P95 / P99 latency | `avg_latency_ms`, `p95_latency_ms`, `p99_latency_ms` |
| Consistency violations  | `consistency_violations`                             |

**Script:** `loadtest/scenarios/exp1_bid_contention.py` — created 2026-03-29

**Bid logic:** each user does `GET /auctions/:id` first to read `current_highest`, then bids `current_highest + random(1, 100)` cents. This creates genuine contention (500 users read the same price simultaneously and race to outbid), rather than a fixed increment that would exhaust valid bid amounts after a few hundred bids.

**How to run (repeat 3 times, once per strategy):**

```bash
# Step 1 — create a long-lived auction and get its ID + a buyer token
#   (use smoke_test output or manually call POST /auctions)

# Step 2 — run one strategy (change STRATEGY= and --csv name for each)
export AUCTION_ID="<paste auction_id here>"
export BUYER_TOKEN="<paste buyer JWT here>"
export STRATEGY=optimistic   # or: pessimistic | queue

locust -f loadtest/scenarios/exp1_bid_contention.py \
       --headless -u 500 -r 50 -t 60s \
       --host http://localhost:8081 \
       --csv loadtest/results/exp1_optimistic

# Step 3 — reset metrics before next run
curl -s -X POST http://localhost:8081/admin/metrics/reset

# Step 4 — repeat with STRATEGY=pessimistic, then STRATEGY=queue
```

Metrics are auto-saved to `loadtest/results/exp1_<strategy>_metrics.json` at end of each run.

**Checklist:**

- [x] Locust script written: `loadtest/scenarios/exp1_bid_contention.py`
- [ ] Run 1: `optimistic` — collect and export metrics
- [ ] Run 2: `pessimistic` — reset metrics first, collect and export
- [ ] Run 3: `queue` — reset metrics first, collect and export
- [ ] Confirm `consistency_violations = 0` across all 3 runs
- [ ] Results saved to `loadtest/results/exp1_*_metrics.json`

> **Note:** Strategy switches take effect immediately with no restart. Admin endpoints require no auth. `consistency_violations` tracks cases where a lower bid was accepted over a higher one — should always be 0.

---

## Experiment 2 — Horizontal Scaling During Auction Spikes

**Goal:** Measure auto-scaling response time and bid loss during a rush-hour spike on AWS ECS.

**Infrastructure:** AWS ECS Fargate + ALB (cannot be run locally)

**Setup:**

- 50 simultaneous auctions, 100 bidders each (5000 total users)
- Start with 2 ECS tasks for Auction Service, auto-scaling enabled (target CPU > 60%)
- Locust ramps from 0 → 5000 users over 60s

**Metrics:**

| Metric                                | Source                                                 |
| ------------------------------------- | ------------------------------------------------------ |
| Auto-scaling trigger time             | CloudWatch — time from spike start to new task healthy |
| Latency during scale-up window        | Locust response time chart (mark the scale-up window)  |
| Throughput before vs. after           | Locust RPS chart                                       |
| Failed/dropped bids during transition | Locust failure rate                                    |

**Checklist:**

- [ ] ECS Task Definitions created for all services
- [ ] ALB routing rules configured (path-based)
- [ ] Auto-scaling policy set on Auction Service (CPU > 60%, scale up by 1, cooldown 60s)
- [ ] CloudWatch dashboard configured (CPU %, task count, ALB latency)
- [ ] Locust script written: `loadtest/scenarios/exp2_scaling_spike.py`
- [ ] At least 2 runs for consistency
- [ ] Results saved to `loadtest/results/exp2_*.csv`
- [ ] Screenshot CloudWatch scaling event timeline

> **Note:** Local Docker has no auto-scaling. This experiment is only meaningful on AWS. Bids lost during task transitions should be captured as Locust `failure` responses, not just latency spikes.

---

## Experiment 3 — Notification Fan-Out

**Goal:** Measure WebSocket push vs. polling pull latency as connected clients scale to 1000.

**Infrastructure:** Local Docker is sufficient for this experiment.

**Setup:**

- 1 popular auction
- Locust simulates 1000 clients connecting to `/auctions/:id/subscribe` (WebSocket)
- A separate Locust user places 1 bid per second (the bid source)
- Measure time from `bid_accepted_at` (stamped in the event) to when each client receives the notification

**Metrics** (from `GET /admin/metrics` on Notification Service `:8080`):

| Metric                | Field                     |
| --------------------- | ------------------------- |
| Connected clients     | `active_connections`      |
| Total broadcasts sent | `total_broadcasts`        |
| Avg delivery latency  | `avg_delivery_latency_ms` |
| Tail latency          | `p99_delivery_latency_ms` |

**Checklist:**

- [ ] Notification service `/admin/metrics` endpoint working and returning all 4 fields
- [ ] Locust WebSocket scenario written: `loadtest/scenarios/exp3_notification.py`
- [ ] Push run: 1000 WebSocket clients, 1 bid/sec, 5 minutes
- [ ] Pull run: 1000 polling clients, each polling `GET /auctions/:id` every 1s
- [ ] Compare P99 latency and server CPU between push and pull
- [ ] Results saved to `loadtest/results/exp3_ws_vs_poll.json`

> **Note:** Delivery latency is calculated as `now - bid_accepted_at` where `bid_accepted_at` is the timestamp stamped by Auction Service in the `BidPlacedEvent` (already implemented in `shared/events/events.go`). Notification Service must subtract this from receive time.

---

## Results Directory Structure

```
loadtest/
├── scenarios/
│   ├── exp1_bid_contention.py
│   ├── exp2_scaling_spike.py
│   └── exp3_notification.py
└── results/
    ├── exp1_optimistic.json
    ├── exp1_pessimistic.json
    ├── exp1_queue.json
    ├── exp2_run1.csv
    ├── exp2_run2.csv
    └── exp3_ws_vs_poll.json
```
