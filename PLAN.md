# SurpriseAuction — Development Plan

## Status Overview

| Area | Status |
|---|---|
| Buyer auth + bidding flow | Working |
| Seller auth + shop/item creation | Working |
| Seller dashboard | Working |
| Real-time bid updates (WebSocket) | Working |
| Payment processing (simulated) | Working |
| My Bids page | Working |
| Auction enrichment fields | Working |
| Seller auction management UI | **Done** (per-shop management page at /seller/shops/:shopId with Items + Auctions tabs; dashboard simplified to overview cards with "Manage →") |
| Automatic auction expiry | **Basic version done** (closer.go polls every 1s, closes OPEN auctions past end_time) |
| Payment service Redis Streams migration | **Complete** |
| Auction/notification/bid Redis Streams migration | **Complete** |
| Bid service WON status on close | **Done** (consumer subscribes to auction_closed, marks winner bid as WON) |
| Per-auction WebSocket notifications | **Done** (broadcasts all bids + auction_closed, frontend WON/CLOSED banners) |
| Global notification system | **Done** (persistent Redis store, per-user WebSocket, bell + toast UI, capped at 20 with dedup) |
| Cache reliability (ensureRedisCached) | **Done** (DynamoDB backing store, write-through on create, fallback read with stampede lock, Redis cleanup on close) |

---

## Completed: Seller Auction Management

Implemented both inline dashboard preview and dedicated drill-down page. CloseAuction uses ownership check (seller_id comparison). See commit `7d3df14` for full details.

| Route | Component | Access |
|---|---|---|
| `/seller/dashboard` | `SellerDashboardPage` | Seller only |
| `/seller/shops/:shopId/auctions` | `SellerAuctionPage` | Seller only |
| `/shop/:id` | `ShopDetailPage` | Public (buyers) |
| `/auction/:id` | `AuctionDetailPage` | Public (buyers) |

---

## Known Issues (Prioritised)

### High

#### 1. ~~Auction expiry: PENDING status~~ **Fixed**
`closer.go` now handles both directions: closes OPEN auctions past `end_time` and opens PENDING auctions when `start_time` arrives. CreateAuction accepts optional `scheduled_start` (RFC3339); if set, auction starts as PENDING. Frontend includes optional schedule picker on create form and shows "Starting Soon" overlay for PENDING auctions.

#### 2. ~~`POST /auctions` unprotected by role~~ **Fixed**
CreateAuction handler now checks `callerRole(c) != "seller"` and returns 403.

#### 3. ~~Seller can bid on their own auction~~ **Fixed**
PlaceBid now compares `a.SellerID == userID` and returns ErrSelfBid (403).

#### 4. ~~Bid service: `ensureConsumerGroup()` MKSTREAM~~ **Fixed**
All services now use `XGroupCreateMkStream` which atomically creates the stream if it doesn't exist. Applied during the Streams migration.

#### 5. ~~Bid service: self-rebid produces multiple WON records~~ **Fixed**
RecordBid now calls MarkUserPreviousBids before creating the new bid, marking the caller's own previous ACCEPTED bids as OUTBID.

### Medium

#### 6. ~~Bid enrichment missing on My Bids page~~ **Fixed**
Bid model now stores `item_title` and `shop_name`. BidPlacedEvent carries `shop_name`. Consumer passes both fields at write time. Frontend `toUserBid` reads them from the response.

#### 7. ~~WebSocket notifications incomplete~~ **Fixed**
Notification hub now broadcasts all bid_placed events (not just outbids) and subscribes to auction_closed channel. Frontend handles both event types with appropriate banners (WON/CLOSED). Global per-user WebSocket added for cross-page notifications with persistent Redis storage, bell dropdown, and toast popups.

#### 8. ~~Bid WON status never set~~ **Fixed**
Bid service consumer now subscribes to `auction_closed` Pub/Sub channel. On close, marks the winner's ACCEPTED bid as WON via `MarkWon()`. Frontend maps WON status correctly.

### Low

#### 9. ~~`POST /auctions/:id/close` missing ownership check~~ **Fixed**
CloseAuction handler now compares `auction.SellerID` to `callerID(c)` and returns 403 on mismatch.

#### 10. ~~Auction creation missing input validation~~ **Fixed**
CreateAuction now validates: `start_bid >= 0`, `duration_minutes` between 1–10080, `scheduled_start` must be valid RFC3339 and in the future. Returns 400 with descriptive error messages.

#### 11. ~~Payment page 404~~ **Fixed**
nginx routed `/auctions/:id/payment` and `/users/:id/payments` to the auction/user services instead of the payment service. Added explicit regex routes for both paths before the generic prefix blocks. Removed the unused `/payments` prefix block.

#### 12. ~~Same email cannot register as both buyer and seller~~ **Fixed**
Replaced dual-account model with single-account model: one email = one account. Registering as seller with an existing buyer email upgrades the role (after password verification) instead of creating a duplicate. JWT now includes `username` and `email` claims. Login no longer takes a role parameter — role is determined by the account state.

---

## Backlog Features

### 1. ~~Auction lifecycle: maxPrice field~~ **Done**
Added `max_price` field (bid ceiling, 0 = no limit) to Auction model, CreateAuctionRequest, Redis hash, and DynamoDB. Lua script checks `max_price` upper bound atomically alongside `current_highest` lower bound. Frontend CreateAuctionPage has optional "Max Price" input. Backend returns 400 if bid exceeds max.

### 2. ~~Pessimistic strategy: Lua script atomicity~~ **Done**
Replaced multi-step SETNX lock + HGetAll + HSet in `PessimisticStrategy` with a single Lua script (`placeBidLua`). The script atomically reads status/current_highest/version → validates → writes current_highest/highest_bidder/version + increments bid_count. No external lock needed. Also fixed `bid_count` never being incremented in all three strategies (optimistic and queue got the same fix).

### 3. ~~Cache reliability: ensureRedisCached() + stampede protection~~ **Done**
Added DynamoDB as a durable backing store for the auction service (opt-in via `DYNAMODB_ENDPOINT` env var). Write-through on Create, fallback read with double-checked locking for stampede protection, Redis hash deleted on Close to reclaim memory. All auction fields including `seller_id` and `max_price` are persisted. Backward-compatible — runs Redis-only when DynamoDB is not configured.

### 4. ~~Close sequence reliability~~ **Done**
Reordered close sequence to: atomic close + read winner (Lua script) → publish event → persist DynamoDB → cleanup Redis. If event publish fails, status rolls back to OPEN so closer retries next tick. Added three-level winner fallback: Redis ZSET (updated on every bid via ZADD in all three strategies) → DynamoDB winners map (async write-through on each bid) → DynamoDB `highestBidder` field. Two new Lua scripts (`closeAndReadWinnerLua`, `rollbackCloseLua`) ensure atomicity.

### 5. ~~Multiple winners (quantity auctions)~~ **Done**
Added `quantity` field (default 1) to Auction model, CreateAuctionRequest, Redis hash, and DynamoDB. Lua script (`placeBidLua`) handles slot management: when all N slots are full, lowest winner is evicted if a higher bid arrives; `current_highest` reflects the floor winner price (or start_bid while slots remain open). `closeAndReadWinnerLua` returns top-N winners for multi-winner auctions. Payment service creates one payment per winner with per-winner idempotency. Notification and bid services updated to notify/mark all N winners. Frontend: quantity input on CreateAuctionPage, "X winners" badge on AuctionCard and BiddingPanel, "Minimum Bid to Win" label for quantity>1.

### 6. ~~Move optimistic and queue strategies to experimental/~~ **Done**
Moved `OptimisticStrategy` and `QueueStrategy` to `concurrency/experimental/` package with clear limitation comments. Both reject quantity>1 auctions with descriptive errors. `PessimisticStrategy` (Lua script) is now the default (`CONCURRENCY_STRATEGY` env var defaults to "pessimistic"). Service struct imports experimental package for benchmarking/dev use.

### 7. Geo support (buyer + seller)
Neither buyers nor sellers have location data. Add `lat`/`lng` or a structured address to the `Shop` model so shops can be surfaced by proximity.

- Seller: structured address on shop creation
- Buyer: location captured on registration or via browser geolocation
- Likely requires a geohash GSI in DynamoDB or a dedicated geo service

### 2. ~~Item categories~~ **Done**
Category field existed on Item model and frontend but was never saved by the shop service's `CreateItem`. Fixed: `req.Category` now passed through to the Item struct. Frontend `CreateItemPage` already had category dropdown, `HomePage` already filters by category, `CreateAuctionPage` inherits category from the selected item.

### 3. ~~Profile update endpoints~~ **Done**
`PUT /users/:user_id` implemented with username/avatar editing. `PUT /shops/:shop_id` added with name/location/logo editing and ownership check. Frontend: "Edit Shop" button on `SellerShopPage` opens inline edit form.

### 13. ~~Per-shop management page~~ **Done**
`SellerShopPage` at `/seller/shops/:shopId/:tab` with Items and Auctions tabs. Dashboard shop cards simplified to overview + "Manage →" link. Old `/seller/shops/:shopId/auctions` URL still works via the `:tab` param. `SellerAuctionPage` kept but no longer routed.

### 10. ~~Redis Pub/Sub → Redis Streams~~ **Done**
All services migrated from Redis Pub/Sub to Redis Streams with consumer groups.

**Streams:**
- `bid:placed` — published by Auction Service, consumed by Bid Service (`bid-service` group) and Notification Service (`notification-service` group)
- `auction:closed` — published by Auction Service, consumed by Payment Service (`payment-service` group), Bid Service (`bid-service` group), and Notification Service (`notification-service` group)

**Features:** `XGroupCreateMkStream` for atomic stream/group creation, `XReadGroup` with blocking reads, `XAck` on successful processing, `XAutoClaim` for pending message recovery (60s timeout, 30s reclaim interval), configurable worker pools via env vars (`BID_WORKERS`, `NOTIF_WORKERS`, `PAYMENT_WORKERS`).

### 11. Real payment gateway
Payment processing is currently simulated (90% success rate mock). Replace with Stripe or equivalent for production.

### 12. Shop settlement
The payment flow records `shop_id` but does not disburse funds to the shop owner. Settlement flow to be designed.

### 7. Image storage — file upload + S3
Currently `image_url` and `logo_url` are free-text URL fields. There is no file upload support — sellers must paste an external URL manually.

Plan:
- Add a file upload endpoint (e.g. `POST /uploads`) that accepts a multipart image, stores it in S3 (or compatible object storage like MinIO for local dev), and returns a public URL
- Replace the URL text inputs on `CreateItemPage` and `CreateShopPage` with a file picker that calls this endpoint
- In production, serve images via CloudFront in front of S3 for low-latency delivery


### 13. ~~Seller sees buyer-facing auction detail page~~ **Done**

In `SellerAuctionPage`, auction titles previously linked to `/auction/:id` (`AuctionDetailPage`), which is the buyer-facing real-time bidding view. Sellers clicking an auction in their management page were dropped into the buyer experience.

**Fix:** Added `SellerAuctionDetailPage` at `/seller/auctions/:auctionId` — a dedicated seller drill-down page with: auction metadata header (image, title, status badge, quantity badge), stats row (current bid, retail price, total bids, max price, time left), full bid history table (bidder, amount, status, time) with real-time WebSocket updates, item details sidebar, winners card for closed auctions, and payment status card. Auction rows in `SellerShopPage` (both active and closed) now link to this page. Close Auction button available for OPEN auctions. Nginx route added for `GET /auctions/:id/bids` → bid service.

### 14. ~~Pickup time window on auctions + buyer filter~~ **Done**

Added optional `pickup_start` and `pickup_end` (RFC3339) fields to the Auction model, Redis hash, DynamoDB, and API. Backend validates RFC3339 format and that end > start. Frontend: `CreateAuctionPage` has paired datetime-local inputs in a "Pickup Window" field. `AuctionCard` shows "Pickup {date}, {start} – {end}" below the bid info when present. `AuctionDetailPage` shows a prominent pickup window card with clock icon. `SellerAuctionDetailPage` shows pickup window in the Item Details sidebar. `HomePage` has a second filter row (pill buttons): Any Time / Morning (before 12pm) / Afternoon (12–5pm) / Evening (after 5pm), client-side filtering on `pickup_start` hour. Auctions without a pickup window are hidden when a time filter is active.

