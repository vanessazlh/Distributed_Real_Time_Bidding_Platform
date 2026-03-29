# SurpriseAuction — User Journey

SurpriseAuction is a real-time surplus auction platform where local stores list unsold end-of-day items as short 5-minute auctions. Buyers compete in live bidding; winners are charged automatically when the auction closes.

---

## Buyer Journey

### 1. Discovery
A buyer lands on the homepage and sees a live feed of active auctions, filterable by category (Bakery, Sushi, and more). Each card shows the item, the shop it comes from, the current highest bid, and a live countdown to closing.

### 2. Account
To place a bid, the buyer registers with a username, email, and password, or logs in if they already have an account. Authentication is handled via JWT — no session state is stored server-side.

### 3. Entering an Auction
The buyer clicks into an auction and sees:
- A full item photo and description
- The originating shop (with a link to the shop profile)
- The current highest bid, updating in real time via WebSocket
- A live bid history feed showing who bid what and when
- A countdown timer

### 4. Placing a Bid
The buyer enters a bid amount in the bidding panel. The platform enforces a minimum increment of $0.50 above the current highest bid. On submission:
- If accepted, the buyer's panel shows a **Winning** banner and the price updates across all connected clients instantly
- If outbid by someone else, the banner switches to **Outbid** and the new price is broadcast to all watchers

### 5. Auction Close
When the countdown reaches zero (or a shop owner manually closes the auction), the system resolves the winner:
- The auction is marked **Closed**
- A `auction_closed` event is published internally
- The Payment Service automatically initiates a charge to the winning bidder
- The winner receives a notification; outbid participants are notified the auction has ended

### 6. Payment
Payment is processed automatically in the background — no action required from the buyer. The charge resolves to either **Completed** or **Failed** (simulated gateway, 90% success rate in the current build). The buyer can view their payment status and bid history on the **My Bids** page.

---

## Shop Owner Journey

### 1. Register a Shop
The shop owner registers an account and creates a shop profile with a name and location via `POST /shops`.

### 2. List an Item
The owner adds a surplus item to their shop — title, description, and any relevant metadata — via `POST /shops/:id/items`.

### 3. Create an Auction
The owner schedules an auction for the item, specifying a start time, end time, and starting bid via `POST /auctions`. Once the start time passes, the auction goes live and is visible to all buyers.

### 4. Live Auction
During the auction window, the owner can monitor activity. If needed, they can close the auction early via `POST /auctions/:id/close` — for example, if the item sells out or the window needs to be cut short.

### 5. Settlement
When the auction closes, the payment is routed using the `shop_id` recorded at auction creation. The shop owner receives the proceeds from the winning bid (settlement flow to be implemented in a future release).

---

## Real-Time Flow Summary

```
Buyer places bid
      │
      ▼
Auction Service validates & updates highest bid atomically
      │
      ├──► Publishes bid_placed event
      │         │
      │         ▼
      │    Notification Service broadcasts to all watchers (WebSocket / SSE)
      │
Auction closes (timeout or manual)
      │
      ├──► Publishes auction_closed event
      │         │
      │         ▼
      │    Payment Service charges the winner
      │         │
      │         ├──► payment_processed → Notification Service notifies winner
      │         └──► payment_failed    → Notification Service notifies winner
```

---

## Current Limitations

- **Payment gateway is simulated** — a real Stripe or equivalent integration would replace the current mock.
- **Shop settlement** — the payment flow records the seller's `shop_id` but does not yet disburse funds to the shop owner.
- **Shop owner UI** — auction creation and item management are currently API-only; a shop dashboard is planned.
- **Message delivery guarantee** — the platform currently uses Redis Pub/Sub (fire-and-forget). A migration to Redis Streams for guaranteed delivery is planned for the next release.
