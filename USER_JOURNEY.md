# SurpriseAuction — User Journey

SurpriseAuction is a real-time surplus auction platform where local stores list unsold end-of-day items as short auctions. Buyers compete in live bidding; winners are charged automatically when the auction closes.

---

## Buyer Journey

### 1. Discovery
A buyer lands on the homepage and sees a live feed of active auctions, filterable by category (Bakery, Sushi, and more). Each card shows the item photo, the shop it comes from, the current highest bid, and a live countdown to closing. Scheduled auctions that haven't started yet appear with a "Starting Soon" overlay.

### 2. Account
The buyer registers at `/login` with a username, email, and password — or logs in if they already have an account. Authentication is JWT-based; no session state is stored server-side. The account's `role` is stamped into the JWT along with `username` and `email`. The platform uses a **single-account model**: one email = one account. A buyer can later upgrade to a seller (see Seller Journey below) without creating a second account.

### 3. Entering an Auction
The buyer clicks into an auction and sees:
- A full item photo and description
- Retail price and the originating shop (with a link to the shop profile)
- The current highest bid, updating in real time via WebSocket
- A live bid history feed showing who bid what and when
- A countdown timer to closing
- If the auction is PENDING (scheduled but not yet started), bidding is disabled with an info banner

### 4. Placing a Bid
The buyer enters a bid amount in the bidding panel. On submission:
- If accepted, the buyer's panel shows a **Winning** banner and the price updates across all connected clients instantly
- If outbid by someone else, the banner switches to **Outbid** and the new price is broadcast to all watchers
- Sellers cannot bid on their own auctions (403 Forbidden)

### 5. Notifications
Buyers receive real-time notifications regardless of which page they are on:
- **Outbid** — when another buyer places a higher bid on an auction the buyer previously bid on
- **Won** — when an auction the buyer is winning closes

Notifications are delivered two ways:
- **Toast popup** — appears in the bottom-right corner, auto-dismisses after 5 seconds, clickable to navigate to the auction
- **Bell icon** — in the navbar, shows an unread count badge. Clicking opens a dropdown listing up to 20 recent notifications. Each notification is clickable and navigates to the relevant auction. A "Mark all as read" button resets the unread count.

Notifications are stored in Redis (capped at 20 per user with same-auction dedup, 7-day TTL) so they persist across page refreshes and browser sessions.

### 6. Auction Close
When the countdown reaches zero (or the seller closes the auction early), the system resolves the winner:
- The auction is marked **Closed**
- An `auction_closed` event is published internally
- The Payment Service automatically initiates a charge to the winning bidder
- The Bid Service marks the winner's bid as **WON**
- The winner sees a **Won** banner on the auction page and receives a global notification
- Other participants see a **Closed** banner

### 7. Profile & History
The buyer's profile page (`/profile`) is a sidebar-tabbed account hub with three sections:
- **Account** — view and edit username (inline save), see email and role badge. Buyers see an "Upgrade to Seller" CTA; sellers see a link to the Seller Dashboard.
- **My Bids** (`/profile/bids`) — full bid history across all auctions, with status (Winning / Outbid / Won / Lost), item title, shop name, and a link to the payment record for won auctions
- **Payments** (`/profile/payments`) — all payment records with status (Completed / Failed / Pending) and total spent

The profile is accessible by clicking the username in the navbar. Individual payment details are at `/payment/auction/:id`.

---

## Seller Journey

### 1. Seller Account
The platform uses a **single-account model** — one email maps to one account. A seller registers at `/shop/register`:
- **New user**: creates an account with the `seller` role directly
- **Existing buyer**: entering the same email and password upgrades the existing account to `seller`. The `seller` role is a superset of `buyer` — a seller can still browse and bid on other shops' auctions

The seller logs in at `/shop/login`. If the account's role is not `seller`, the login page shows an error prompting the user to register as a seller first. The JWT carries `role: seller`, unlocking seller-only routes.

### 2. Seller Dashboard
After login the seller lands on `/seller/dashboard`. The dashboard shows:
- Stats: total shops, active auctions, closed auctions, total bids
- A grid of all their shops, each showing auction count, total bids, and active count
- A "Manage →" link per shop leading to the per-shop management page

### 3. Create a Shop
The seller fills in a shop name and location at `/shops/new`. The shop is saved in DynamoDB and immediately appears on the dashboard.

### 4. Add an Item
From a shop page, the seller adds a surplus item via `/shops/:id/items/new`, providing a title, description, retail value, and optional image URL. Items are saved to DynamoDB under the shop.

### 5. Publish an Auction
The seller navigates to `/auction/new?shopId=:id`. They select an item from the shop's inventory, set the duration (in minutes), a starting bid, and optionally schedule a future start time. On submission:
- Auction enrichment data (shop name, retail price, item image, description) is captured at creation time and stored in Redis
- If no schedule time is set, the auction immediately goes live (**OPEN**) and is visible to all buyers
- If a future start time is set, the auction is created as **PENDING** and automatically transitions to OPEN when the scheduled time arrives
- Input validation enforces: start_bid >= 0, duration 1–10080 minutes, scheduled_start must be in the future

### 6. Monitor & Close
The seller manages each shop from `/seller/shops/:shopId`, a tabbed page with two sections:
- **Items tab** — lists all items in the shop's inventory with title, description, retail value, and image. Includes an "+ Add Item" button.
- **Auctions tab** — shows stats (active, closed, total bids, revenue), active auctions with live countdown timers and a close button, and closed auctions with final prices. The close button triggers `POST /auctions/:id/close` with ownership verification (only the auction's seller can close it).

### 7. Settlement
When the auction closes, the payment is initiated automatically. The `shop_id` recorded at auction creation is included in the payment record. Full fund disbursement to the seller is planned for a future release.

---

## Real-Time Flow

```
Buyer places bid
      │
      ▼
Auction Service validates & updates highest bid atomically (Redis + concurrency strategy)
      │
      ├──► Publishes bid_placed event (Redis Pub/Sub)
      │         │
      │         ├── Bid Service records bid history (marks previous same-user bids as OUTBID)
      │         │
      │         ├── Notification Service broadcasts to all auction watchers (per-auction WebSocket)
      │         │
      │         └── Notification Service stores + pushes "outbid" to previous bidder (per-user WebSocket)
      │
Auction closes (seller closes or timer expires via closer.go)
      │
      ├──► Publishes auction_closed event (Redis Pub/Sub)
                │
                ├── Bid Service marks winner's bid as WON
                │
                ├── Payment Service charges the winner (simulated gateway)
                │       ├── payment_processed → records success
                │       └── payment_failed    → records failure
                │
                ├── Notification Service broadcasts close to auction watchers (per-auction WebSocket)
                │
                └── Notification Service stores + pushes "won" to winner (per-user WebSocket)
```

---

## Current Limitations

| Area | Status |
|---|---|
| Payment gateway | Simulated (90% success rate mock) — no real Stripe integration yet |
| Shop settlement | Payment records `shop_id` but does not disburse funds to the seller |
| Message delivery guarantee | Redis Pub/Sub is fire-and-forget — migration to Redis Streams planned |
| Geo / location filtering | No geo-based search or proximity filtering |
| Item categories | Items have no category field; home page filtering is heuristic-based |
| Profile updates | Users can edit their username; shop detail editing not yet implemented |
| Image upload | No file upload — sellers must paste external image URLs manually |
| Cache reliability | If a Redis key is evicted, bids fail with "auction not found" — DynamoDB fallback not yet implemented |
