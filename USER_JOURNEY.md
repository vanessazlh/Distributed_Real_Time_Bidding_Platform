# SurpriseAuction — User Journey

SurpriseAuction is a real-time surplus auction platform where local stores list unsold end-of-day items as short auctions. Buyers compete in live bidding; winners are charged automatically when the auction closes.

---

## Buyer Journey

### 1. Discovery
A buyer lands on the homepage and sees a live feed of active auctions, filterable by category (Bakery, Sushi, and more). Each card shows the item photo, the shop it comes from, the current highest bid, and a live countdown to closing.

### 2. Account
The buyer registers at `/login` with a username, email, and password — or logs in if they already have an account. Authentication is JWT-based; no session state is stored server-side. The buyer's `role` is stamped into the token as `buyer`.

### 3. Entering an Auction
The buyer clicks into an auction and sees:
- A full item photo and description
- Retail price and the originating shop (with a link to the shop profile)
- The current highest bid, updating in real time via WebSocket
- A live bid history feed showing who bid what and when
- A countdown timer to closing

### 4. Placing a Bid
The buyer enters a bid amount in the bidding panel. On submission:
- If accepted, the buyer's panel shows a **Winning** banner and the price updates across all connected clients instantly
- If outbid by someone else, the banner switches to **Outbid** and the new price is broadcast to all watchers

### 5. Auction Close
When the countdown reaches zero (or the seller closes the auction early), the system resolves the winner:
- The auction is marked **Closed**
- An `auction_closed` event is published internally
- The Payment Service automatically initiates a charge to the winning bidder
- The winner and outbid participants are notified

### 6. Payment & History
Payment is processed automatically — no action required from the buyer. The buyer can review their activity at any time:
- **My Bids** (`/my-bids`) — full bid history across all auctions, with status (Winning / Outbid / Won / Lost) and a link to the payment record for won auctions
- **My Payments** (`/my-payments`) — all payment records with status (Completed / Failed / Pending) and total spent
- **Payment Detail** (`/payment/auction/:id`) — status, amount, and timestamps for a specific auction's payment

---

## Seller Journey

### 1. Seller Account
The seller registers or logs in at `/shop/login` — a separate entry point from the buyer login page. Both hit the same `POST /auth/login` endpoint, but the seller's `role: seller` is encoded in the JWT, unlocking seller-only routes.

### 2. Seller Dashboard
After login the seller lands on `/seller/dashboard`. The dashboard lists all their shops. From here they can:
- Create a new shop
- Navigate into a shop to manage its items
- Publish a new auction for any listed item

### 3. Create a Shop
The seller fills in a shop name and location at `/shops/new`. The shop is saved in DynamoDB and immediately appears on the dashboard.

### 4. Add an Item
From a shop page, the seller adds a surplus item via `/shops/:id/items/new`, providing a title, description, retail value, and optional image URL. Items are saved to DynamoDB under the shop.

### 5. Publish an Auction
The seller navigates to `/auction/new?shopId=:id`. They select an item from the shop's inventory, set the duration (in minutes), and set a starting bid. On submission:
- Auction enrichment data (shop name, retail price, item image, description) is captured at creation time and stored in Redis
- The auction immediately goes live and is visible to all buyers on the homepage

### 6. Monitor & Close
During the auction window, the seller can close the auction early via `POST /auctions/:id/close` — for example if the item is no longer available. Full seller-facing auction management UI (per-shop auction list, close button, live status) is on the roadmap.

### 7. Settlement
When the auction closes, the payment is initiated automatically. The `shop_id` recorded at auction creation is included in the payment record. Full fund disbursement to the seller is planned for a future release.

---

## Real-Time Flow

```
Buyer places bid
      │
      ▼
Auction Service validates & updates highest bid atomically (Redis + optimistic locking)
      │
      ├──► Publishes bid_placed event (Redis Pub/Sub)
      │         │
      │         ▼
      │    Bid Service records bid history in Redis sorted set
      │         │
      │         ▼
      │    Notification Service broadcasts to all auction watchers (WebSocket)
      │
Auction closes (seller closes or timer expires)
      │
      ├──► Publishes auction_closed event (Redis Pub/Sub)
                │
                ▼
           Payment Service charges the winner (simulated gateway)
                │
                ├──► payment_processed → Notification Service notifies winner
                └──► payment_failed    → Notification Service notifies winner
```

---

## Current Limitations

| Area | Status |
|---|---|
| Payment gateway | Simulated (90% success rate mock) — no real Stripe integration yet |
| Shop settlement | Payment records `shop_id` but does not disburse funds to the seller |
| Seller auction management UI | Sellers cannot yet see or close their auctions from the dashboard |
| Automatic auction expiry | Auctions must be closed manually; no timer-based auto-close yet |
| Bid enrichment on My Bids | Item title and shop name are blank on the My Bids page |
| Message delivery guarantee | Redis Pub/Sub is fire-and-forget — migration to Redis Streams planned |
| Geo / location filtering | No geo-based search or proximity filtering |
| Item categories | Items have no category field; home page filtering is heuristic-based |
