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
| B16 | Ratings & reviews      | `POST /shops/:id/reviews`, list, seller reply, duplicate guard, auth | Review stored; average shown; reply protected  |

**Last run: 2026-03-29 — 20/20 passed ✅** (B16 added 2026-04-16, not yet run against live stack)

**Unit tests:** `go test ./...` — all packages pass. Shop service has 19 unit tests covering shop/item CRUD, upload validation, review creation, duplicate enforcement, average rating, and seller reply access control.

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
- Terraform defaults remain `2` ECS tasks and `3000` req/target/min for initial bring-up
- Validated experiment configuration uses `4` ECS tasks and
  `ALBRequestCountPerTarget` target tracking with threshold `2000` req/target/min
- Locust ramps from 0 → 5000 users over 60s

**Recommended next iteration (service-level prewarm):**

- Raise the steady baseline to `4` tasks
- Add a scheduled prewarm window for the whole auction service (for example `4 -> 8`)
- Start the prewarm `2-3` minutes before the planned load spike
- Return to baseline `5-10` minutes after the run
- Keep `ALBRequestCountPerTarget` target tracking enabled as the fallback signal

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
- [ ] Auto-scaling policy set on Auction Service (`ALBRequestCountPerTarget` target
      tracking, 60s scale-out cooldown)
- [ ] Optional scheduled prewarm configured for the auction service if testing a known spike window
- [ ] CloudWatch dashboard configured (`RequestCountPerTarget`, task count, ALB latency)
- [x] Locust script written: `loadtest_aws/scenarios/exp2_scaling_spike.py`
- [ ] At least 2 runs for consistency
- [x] Results saved to `loadtest_aws/results/exp2_*.csv`
- [ ] Screenshot CloudWatch scaling event timeline

Current status:

- [x] ECS Task Definitions created for all services
- [x] ALB routing rules configured (path-based)
- [x] Auto-scaling policy set on Auction Service (`ALBRequestCountPerTarget` target tracking, 60s scale-out cooldown)
- [x] Optional scheduled prewarm implemented with `aws_appautoscaling_scheduled_action`
- [ ] CloudWatch dashboard captured as final artifact
- [x] Distributed Locust scenario executed across multiple Experiment 2 runs
- [x] Results recorded in `loadtest_aws/exp2_run1_report.md` and `loadtest_aws/results/`
- [ ] Final screenshot set added to the external report / milestone package

> **Note:** Local Docker has no auto-scaling. This experiment is only meaningful on AWS. Bids lost during task transitions should be captured as Locust `failure` responses, not just latency spikes.

---

## Experiment 3 — Notification Fan-Out

**Goal:** Split notification fan-out evaluation into two clearer questions:

- **Experiment 3A:** Which push transport is better for one-way auction updates: `WebSocket` or `SSE`?
- **Experiment 3B:** What is the architectural trade-off between push delivery and pull polling?

**Infrastructure:** Local Docker is sufficient for this experiment.

**Setup:**

- 1 popular auction
- Locust simulates 1000 total users: 999 subscribers and 1 bidder
- `MODE=ws`: subscribers connect to `/auctions/:id/subscribe`
- `MODE=sse`: subscribers connect to `/auctions/:id/subscribe/sse`
- `MODE=pull`: subscribers poll `GET /auctions/:id` every second
- A separate Locust bidder places 1 bid every ~2 seconds
- For push modes, measure time from `bid_accepted_at` (stamped in the event) to when each client receives the update

**Metrics** (from `GET /metrics` on Notification Service `:8080` for push runs):

| Metric                | Field                     |
| --------------------- | ------------------------- |
| Connected clients     | `active_connections`      |
| Total broadcasts sent | `total_broadcasts`        |
| Avg delivery latency  | `avg_delivery_latency_ms` |
| Tail latency          | `p99_delivery_latency_ms` |

For Experiment 3A (`ws` vs `sse`), compare:

- server-side fan-out latency from `/metrics`
- client-side push latency from `bid_accepted_at`
- connection setup success / latency

For Experiment 3B (`push` vs `pull`), compare:

- auction-service HTTP request volume and latency
- polling stale-read risk analytically from the poll interval
- do **not** compare notification delivery latency for `pull`, because there are no push subscribers in that mode

**Checklist:**

- [x] Notification service `/metrics` endpoint working and returning all 4 fields
- [x] Notification service SSE endpoint working at `/auctions/:id/subscribe/sse`
- [x] Canonical local script updated: `tests/loadtest_local/scenarios/exp3_notification.py`
- [x] Experiment 3A WebSocket run completed
- [x] Experiment 3A SSE run completed
- [x] Experiment 3B pull baseline run completed
- [x] Results saved to `tests/loadtest_local/results/exp3_{ws,sse,pull}_run*.json`
- [x] Compare push transport latency on equal workload parameters
- [x] Compare push vs pull HTTP load without treating pull as notification delivery

Current 3-run snapshot:

- `ws`: avg connect `22.2 ms`; server-side fan-out avg `13.5 ms`, p99 `51.0 ms`; client-side push latency avg `47.2 ms`, p99 `107.9 ms`
- `sse`: avg connect `33.1 ms`; server-side fan-out avg `14.8 ms`, p99 `62.5 ms`; client-side push latency avg `27.4 ms`, p99 `92.5 ms`
- `pull`: about `163,800` poll requests per run over 180s; avg poll latency `27.7 ms`, p99 `96 ms`; notification-side values are stored only as internal processing overhead, not delivery latency

Per-run notes:

- `ws` remained consistent across all three runs and had the lowest connection setup cost
- `sse` remained competitive on end-to-end push latency, but one run produced a noticeable connection tail outlier
- `pull` consistently reproduced the high read amplification expected from 1-second polling across 999 subscribers

> **Note:** Delivery latency is calculated as `now - bid_accepted_at` where `bid_accepted_at` is the timestamp stamped by Auction Service in the `BidPlacedEvent` (already implemented in `shared/events/events.go`). This is meaningful for `ws` and `sse` push clients. In `pull` mode, users observe changes only on the next poll, so the comparable question is freshness lag rather than push delivery latency.

---

## Results Directory Structure

```text
tests/loadtest_local/
├── scenarios/
│   └── exp3_notification.py
└── results/
    ├── exp3_ws_run1_metrics.json
    ├── exp3_sse_run1_metrics.json
    └── exp3_pull_run1_metrics.json
```
