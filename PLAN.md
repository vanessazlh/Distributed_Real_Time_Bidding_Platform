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
| Auction/notification Redis Streams migration | **Not started** (still on Pub/Sub) |
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

#### 4. Bid service: `ensureConsumerGroup()` MKSTREAM
Bid service currently uses Pub/Sub, not Streams. When migrated to Streams, `XGROUP CREATE` will fail on cold start if the stream key doesn't exist yet.

Fix: pass `MKSTREAM=true` when creating the consumer group so the stream is created atomically. Apply this fix during the Streams migration, not before.

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

### 5. Multiple winners (quantity auctions)
Add `quantity` field to support auctions where N buyers can win.

- Redis Sorted Set maintains top-N winners by bid amount
- Lua script handles slot management: when full, lowest winner is evicted if a higher bid arrives; `current_highest` always reflects current floor winner price
- Payment service triggers N payment records on close

### 6. Move optimistic and queue strategies to experimental/
Both strategies have deployment limitations:
- `OptimisticStrategy` (Redis WATCH) does not work correctly across multiple Redis nodes without careful sharding
- `QueueStrategy` (Go channel) is per-process only; does not work with multiple auction service instances

Move both to `concurrency/experimental/` with clear comments. `PessimisticStrategy` (upgraded with Lua) becomes the default.

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

### 10. Redis Pub/Sub → Redis Streams
The auction, notification, and payment services all use Redis Pub/Sub, which is fire-and-forget.

Problems:
- Messages lost if a consumer is offline at publish time
- No consumer group support — cannot scale horizontally without duplicate processing
- No replay or audit trail

Plan: migrate all three services to Redis Streams with consumer groups for durable, replayable, exactly-once delivery.

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


### 13. Seller sees buyer-facing auction detail page (Bug)

In `SellerAuctionPage`, auction titles previously linked to `/auction/:id` (`AuctionDetailPage`), which is the buyer-facing real-time bidding view. Sellers clicking an auction in their management page were dropped into the buyer experience — live countdown, bid input, WebSocket feed — with no seller controls.

**Fix applied (partial):** Removed the `<Link to="/auction/:id">` from auction rows in `SellerAuctionPage` so sellers can no longer accidentally navigate there from their management page.

**Remaining work:** Sellers now have a per-shop management page with an Auctions tab (`/seller/shops/:shopId/auctions`) that lists active/closed auctions with close buttons. A dedicated single-auction drill-down (bid history, current winner, item metadata) is still missing.
