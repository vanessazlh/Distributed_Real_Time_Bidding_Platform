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
| Seller auction management UI | **Basic dashboard exists; auction list + close management not done** |
| Automatic auction expiry | **Basic version done** (closer.go polls every 1s, closes OPEN auctions past end_time) |
| Payment service Redis Streams migration | **Complete** |
| Auction/notification Redis Streams migration | **Not started** (still on Pub/Sub) |
| Bid service WON status on close | **Not started** |
| Cache reliability (ensureRedisCached) | **Not started** |

---

## Next Feature: Seller Auction Management

After a seller publishes an auction there is no dedicated way to see its state, monitor bids, or close it early. The public shop page (`/shop/:id`) is buyer-facing and must not be the seller's management surface.

### Backend

#### 1. Per-shop Redis set — `services/auction/internal/auction/repository.go`

When an auction is created, write its ID into a Redis set keyed by shop:

```
shop:{shop_id}:auctions  →  SET of auction_id strings
```

- `Create()`: add `pipe.SAdd(ctx, shopAuctionsKey(a.ShopID), a.AuctionID)`
- Helper: `func shopAuctionsKey(shopID string) string { return "shop:" + shopID + ":auctions" }`

#### 2. `ListByShop` method — `repository.go` + `service.go`

```go
// repository
func (r *Repository) ListByShop(ctx context.Context, shopID string) ([]*Auction, error)

// service
func (s *Service) ListAuctionsByShop(ctx context.Context, shopID string) ([]*Auction, error)
```

Reads IDs from `shop:{shop_id}:auctions`, fetches each auction by ID.

#### 3. New route — `services/auction/api/router.go`

```
GET  /shops/:shop_id/auctions   → h.ListShopAuctions   (protected)
POST /auctions/:id/close        → h.CloseAuction        (protected — move from public group)
```

#### 4. nginx — route auction sub-path correctly — `services/frontend/nginx.conf`

`GET /shops/:id/auctions` must reach the **auction** service, not the shop service. Add a more specific location block before the existing `/shops` block:

```nginx
location ~ ^/shops/[^/]+/auctions {
    proxy_pass http://auction:8081;
}
```

### Frontend

#### 1. API client — `frontend/src/lib/api.ts`

```ts
api.auctions.listByShop(shopId: string, token: string): Promise<Auction[]>
api.auctions.close(id: string, token: string): Promise<void>
```

#### 2. Seller Dashboard — `frontend/src/pages/SellerDashboardPage.tsx`

Extend each shop card to show its auctions (inline or drill-down). Display:
- Auction title + status badge (OPEN / CLOSED)
- Current highest bid and bid count
- "Close" button for OPEN auctions

#### 3. Seller Auction List page *(optional drill-down)* — `frontend/src/pages/SellerAuctionPage.tsx`

Route: `/seller/shops/:shopId/auctions`

A dedicated page listing all auctions for one shop with full management controls.

### Routing Summary

| Route | Component | Access |
|---|---|---|
| `/seller/dashboard` | `SellerDashboardPage` | Seller only |
| `/seller/shops/:shopId/auctions` | `SellerAuctionPage` | Seller only |
| `/shop/:id` | `ShopDetailPage` | Public (buyers) |
| `/auction/:id` | `AuctionDetailPage` | Public (buyers) |

### Open Questions

1. **Inline vs drill-down** — auctions inline on the dashboard shop card, or on a separate `/seller/shops/:shopId/auctions` page?
2. **CloseAuction auth** — verify the caller owns the shop the auction belongs to, or just require any valid seller JWT?

---

## Known Issues (Prioritised)

### High

#### 1. Auction expiry: basic done, PENDING status not yet added
`closer.go` polls every 1 second and closes OPEN auctions past `end_time` — this works.

Missing: PENDING status for pre-scheduled auctions. Currently auctions go OPEN immediately on creation. To support a future `start_time`, add a PENDING state and an AuctionOpener goroutine that transitions `PENDING → OPEN` at `start_time`.

#### 2. `POST /auctions` unprotected by role
Any authenticated user (buyer or seller) can create an auction. The auction handler has no role check.

Fix: add `callerRole` check to the `CreateAuction` handler, same pattern as the shop service.

#### 3. Seller can bid on their own auction
`placeBid()` does not check whether the bidder is the auction's seller. `seller_id` is stored in the Redis hash and can be read without extra calls.

Fix: read `seller_id` from Redis hash in `placeBid()`, reject if `bidderID == sellerID`.

#### 4. Bid service: `ensureConsumerGroup()` MKSTREAM
Bid service currently uses Pub/Sub, not Streams. When migrated to Streams, `XGROUP CREATE` will fail on cold start if the stream key doesn't exist yet.

Fix: pass `MKSTREAM=true` when creating the consumer group so the stream is created atomically. Apply this fix during the Streams migration, not before.

#### 5. Bid service: self-rebid produces multiple WON records
`recordBid()` does not mark the current bidder's own previous `ACTIVE` bid as `OUTBID` before writing the new one. When a user raises their own bid, both records end up as `WON` after close.

Fix: in `recordBid()`, query and mark the caller's own previous `ACTIVE` bid for this auction as `OUTBID` before inserting the new record.

### Medium

#### 6. Bid enrichment missing on My Bids page
`GET /users/:id/bids` returns bid records with no `item_title` or `shop_name`. The bid service stores `auction_id` but not the item title at write time. My Bids page shows blank titles and shop names.

Options:
- Store `item_title` in the bid record at write time (same denormalization as auctions)
- Enrich in the user service proxy before returning to the client

#### 7. WebSocket notifications incomplete
The notification service broadcasts `bid_placed` events. Missing:
- "You've been outbid" push to the previous highest bidder
- Auction close notification to winner and losing bidders

#### 8. Bid WON status never set
The bid service has no consumer for `auction:closed` stream. After an auction closes, winning bids remain in `ACTIVE` status forever — the `WON` state is never written.

Fix: add `AuctionClosedConsumer` in bid service that consumes `auction:closed` stream and marks the winner's bid as `WON`.

### Low

#### 9. `POST /auctions/:id/close` missing ownership check
The handler calls `callerID()` but does not verify the caller owns the shop the auction belongs to. Any authenticated user can close any auction.

Fix: read `seller_id` from the auction Redis hash and compare to `callerID(c)` before proceeding.

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

---

## Completed

| # | Fix | Date |
|---|---|---|
| 1 | My Bids page was a stub — user service now proxies `/users/:id/bids` to the bid service | 2026-03 |
| 2 | `bid_count` always 0 — Redis hash now increments on each bid; `parseAuction` reads it back | 2026-03 |
| 3 | Auction enrichment fields empty — stored at creation, parsed back, passed through frontend transform | 2026-03 |
| 4 | `payments` table missing from `init_tables.go` — added `createPaymentsTable()` | 2026-03 |
| 5 | Seller auth UX — separate `/shop/login` entry point, `role` field in JWT, seller dashboard page | 2026-03 |
| 6 | Payment pages missing — added `PaymentPage` and `MyPaymentsPage` with API client | 2026-03 |
| 7 | Chrome DevTools blank — nginx `/.well-known/` returning `index.html` with 200; fixed with `return 404` | 2026-03 |
| 8 | Auction item crash — `auction.item` undefined; added `BackendAuction` → `Auction` transform in `api.ts` | 2026-03 |
