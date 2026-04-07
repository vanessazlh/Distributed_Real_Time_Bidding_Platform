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
| Seller auction management UI | **Done** (dashboard inline preview + dedicated /seller/shops/:shopId/auctions page with close) |
| Automatic auction expiry | **Basic version done** (closer.go polls every 1s, closes OPEN auctions past end_time) |
| Payment service Redis Streams migration | **Complete** |
| Auction/notification Redis Streams migration | **Not started** (still on Pub/Sub) |
| Bid service WON status on close | **Done** (consumer subscribes to auction_closed, marks winner bid as WON) |
| Per-auction WebSocket notifications | **Done** (broadcasts all bids + auction_closed, frontend WON/CLOSED banners) |
| Global notification system | **Done** (persistent Redis store, per-user WebSocket, bell + toast UI, capped at 20 with dedup) |
| Cache reliability (ensureRedisCached) | **Not started** |

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

#### 1. Auction expiry: basic done, PENDING status not yet added
`closer.go` polls every 1 second and closes OPEN auctions past `end_time` — this works.

Missing: PENDING status for pre-scheduled auctions. Currently auctions go OPEN immediately on creation. To support a future `start_time`, add a PENDING state and an AuctionOpener goroutine that transitions `PENDING → OPEN` at `start_time`.

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

#### 10. Auction creation missing input validation
No validation on `startTime < endTime`, `endTime` in the future, or `maxPrice > startingBid`.

Fix: add these checks in `CreateAuction` handler before writing to Redis.

---

## Backlog Features

### 1. Auction lifecycle: maxPrice field
Replace the unused `reservePrice` field with `maxPrice` as a bid ceiling. Lua script in pessimistic strategy handles both `maxPrice` upper bound and `startingBid` lower bound in a single atomic operation.

### 2. Pessimistic strategy: Lua script atomicity
Replace the current multi-step HSET in `PessimisticStrategy` with a single Lua script. The script handles read → validate (`startingBid`, `maxPrice`, `status`) → write atomically, eliminating the gap between reads and writes.

### 3. Cache reliability: ensureRedisCached() + stampede protection
Redis is the primary store for auction state. If a key is evicted or Redis restarts, bids fail with "auction not found".

- `ensureRedisCached()`: on cache miss, rebuild from DynamoDB before proceeding
- Double-checked locking via Redisson rebuild lock: prevents thundering herd when many requests hit the same missing key simultaneously
- `seedRedisCache()`: add `seller_id`, `quantity`, `max_price` fields to reduce per-bid DynamoDB reads
- On auction close: explicitly delete Redis hash and ZSET to avoid memory leaks

### 4. Close sequence reliability
Current order risks a dead state if event publishing fails after DynamoDB write.

New order: read winners → publish `auction:closed` event → write CLOSED to DynamoDB → delete Redis keys. If event publish fails, auction stays OPEN and close can be retried.

Also add three-level fallback for winner resolution: Redis ZSET → DynamoDB winners map → DynamoDB `highestBidder`. Write full winners map to DynamoDB on each successful bid so recovery is possible after Redis restart.

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

### 2. Item categories
Items have no category field, blocking real filtering on the buyer home page.

- Add `category` to the `Item` model (shop service)
- Pass through to the `Auction` model
- Update `CreateItemPage` with a category selector
- Update `HomePage` tabs to filter by real category data

### 3. Profile update endpoints
Users and sellers can register and log in but cannot update their details.

- `PUT /users/:user_id` — update username, email, password (ownership check required)
- `PUT /shops/:shop_id` — edit shop name, location, logo URL (ownership check required)
- Frontend: "Edit Profile" page for buyers; "Edit Shop" button on the seller dashboard

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

