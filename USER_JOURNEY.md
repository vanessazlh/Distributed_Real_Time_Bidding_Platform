# SurpriseAuction — User Journey

SurpriseAuction is a real-time surplus auction platform where local stores list unsold end-of-day items as short auctions. Buyers compete in live bidding; winners are charged automatically when the auction closes.

---

## Buyer Journey

### 1. Discovery
A buyer lands on the homepage and sees a live feed of active auctions, filterable by:
- **Category** — Bakery, Meals, Groceries, Others (tab bar)
- **Pickup time** — Any Time, Morning (before 12 pm), Afternoon (12–5 pm), Evening (after 5 pm)
- **Nearby** — Within 2 km / 5 km / 10 km. The browser requests location permission automatically when the homepage loads. Once granted, distance badges (`~X.Xkm`) appear on each auction card immediately. Selecting a distance filter re-fetches only auctions from shops within that radius.

Each card shows the item photo, the shop it comes from, the current highest bid, a live countdown to closing, and (if set) the pickup window. Scheduled auctions that haven't started yet appear with a "Starting Soon" overlay.

### 2. Account
The buyer registers or logs in at `/login` — the single entry point for all users. A **Buyer / Seller toggle** at the top of the page determines the session role; buyers leave it on **Buyer**. Authentication is JWT-based; no session state is stored server-side. The platform uses a **single-account model**: one email = one account. A buyer can later upgrade to a seller (see Seller Journey, step 1) without creating a second account — they simply re-register with the same email and Seller selected.

### 3. Entering an Auction
The buyer clicks into an auction and sees:
- A full item photo and description
- Retail price and the originating shop (with a link to the shop profile)
- If a pickup window is set, a prominent card with a clock icon showing the date and time range for collection
- The current highest bid, updating in real time via WebSocket
- A live bid history feed showing who bid what and when
- A countdown timer to closing
- If the auction is PENDING (scheduled but not yet started), bidding is disabled with an info banner

### 4. Placing a Bid
The buyer enters a bid amount in the bidding panel. On submission:
- If accepted, the buyer's panel shows a **Winning** banner and the price updates across all connected clients instantly
- If outbid by someone else, the banner switches to **Outbid** and the new price is broadcast to all watchers
- Sellers cannot bid on their own auctions (403 Forbidden)

For **quantity auctions** (multiple winners), the bidding panel shows a "X winners" badge and the label changes to "Minimum Bid to Win" — the floor price to secure a winning slot. When all slots are full, a new bid must exceed the lowest winner's amount; the evicted bidder is notified as outbid.

### 5. Notifications
Buyers receive real-time notifications regardless of which page they are on:
- **Outbid** — when another buyer places a higher bid on an auction the buyer previously bid on
- **Won** — when an auction the buyer is winning closes

Notifications are delivered two ways:
- **Toast popup** — appears in the bottom-right corner, auto-dismisses after 5 seconds, clickable to navigate to the auction
- **Bell icon** — in the navbar, shows an unread count badge. Clicking opens a dropdown listing up to 20 recent notifications. Each notification is clickable and navigates to the relevant auction. A "Mark all as read" button resets the unread count.

Notifications are stored in Redis (capped at 20 per user with same-auction dedup, 7-day TTL) so they persist across page refreshes and browser sessions.

### 6. Auction Close
When the countdown reaches zero (or the seller closes the auction early), the system resolves the winner(s) using a reliable close sequence:
- A Lua script **atomically** marks the auction as CLOSED and reads the winner(s) from a Redis Sorted Set — top-N for quantity auctions (with DynamoDB fallback if Redis data is unavailable)
- An `auction_closed` event is published to the Redis Stream — if publishing fails, the status is **rolled back to OPEN** and the closer retries on the next tick, preventing dead states
- The Payment Service automatically initiates a charge to each winning bidder (one payment per winner for quantity auctions)
- The Bid Service marks each winner's bid as **WON**
- Winners see a **Won** banner on the auction page and receive a global notification
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
The platform uses a **single-account model** — one email maps to one account, and one login page serves everyone. On the login and register pages, a **Buyer / Seller toggle** determines the session role:

- **New seller**: go to `/register`, select **Seller**, fill in username, email, and password
- **Existing buyer upgrading to seller**: go to `/register`, select **Seller**, enter the same email and password — the account is upgraded automatically. The `seller` role is a superset of `buyer`, so a seller can still log in as a buyer to browse and bid on other shops' auctions
- **Logging in**: go to `/login`, select **Seller** in the toggle. If the account has never been upgraded to seller, an error prompts the user to register as seller first

The session role is determined by the toggle at login time, not by the JWT alone. A seller-account holder who selects **Buyer** at login gets a full buyer session — they can browse and bid with no seller dashboard access.

### 2. Seller Dashboard
After login the seller lands on `/seller/dashboard`. The dashboard shows:
- Stats: total shops, active auctions, closed auctions, total bids
- A grid of all their shops, each showing auction count, total bids, and active count
- A "Manage →" link per shop leading to the per-shop management page

### 3. Create a Shop
The seller fills in a shop name and display address at `/shops/new`. A **Shop Location** field (required) provides two ways to set GPS coordinates:
- **📍 Use my location** — auto-detects via browser geolocation
- **🗺 Pin on map** — opens an interactive OpenStreetMap where the seller clicks or drags a pin to their exact location

Coordinates are required to submit the form and enable proximity filtering for buyers. They can be updated later via the Edit Shop form.

### 4. Add an Item
From a shop page, the seller adds a surplus item via `/shops/:id/items/new`, providing a title, description, retail value, and optional image URL. Items are saved to DynamoDB under the shop.

### 5. Publish an Auction
The seller navigates to `/auction/new?shopId=:id`. They select an item from the shop's inventory, set the duration (in minutes), a starting bid, optionally a max price (bid ceiling), a **quantity** (number of winners, default 1), optionally schedule a future start time, and optionally set a **pickup window** (start and end datetime for when winners can collect the item). On submission:
- Auction enrichment data (shop name, retail price, item image, description) is captured at creation time and stored in Redis
- If no schedule time is set, the auction immediately goes live (**OPEN**) and is visible to all buyers
- If a future start time is set, the auction is created as **PENDING** and automatically transitions to OPEN when the scheduled time arrives
- For **quantity > 1**, the auction becomes a multi-winner auction where the top N bidders each win a copy of the item
- Input validation enforces: start_bid >= 0, duration 1–10080 minutes, scheduled_start must be in the future

### 6. Monitor & Close
The seller manages each shop from `/seller/shops/:shopId`, a tabbed page with two sections:
- **Items tab** — lists all items in the shop's inventory with title, description, retail value, and image. Includes an "+ Add Item" button.
- **Auctions tab** — shows stats (active, closed, total bids, revenue), active auctions with live countdown timers, and closed auctions with final prices. Each auction row is clickable.

Clicking an auction row opens the **Seller Auction Detail** page at `/seller/auctions/:auctionId`, a dedicated drill-down with:
- **Header** — item image, title, status badge, quantity badge (for multi-winner auctions), and a "Close Auction" button for OPEN auctions
- **Stats row** — current bid, retail price, total bids, max price (if set), and a live countdown timer
- **Bid history table** — every bid placed, showing masked bidder ID, amount, status (Winning / Outbid / Won), and relative time. Updates in real time via WebSocket as new bids arrive.
- **Item details sidebar** — description, category, quantity, bid ceiling, and pickup window (if set)
- **Winners card** (closed auctions) — lists each winner with their winning bid amount
- **Payment status card** (closed auctions) — shows payment status (Pending / Completed / Failed / Refunded) and amount

This page gives sellers full visibility into individual auctions without exposing the buyer-facing bidding UI.

### 7. Settlement
When the auction closes, the payment is initiated automatically. The `shop_id` recorded at auction creation is included in the payment record. Full fund disbursement to the seller is planned for a future release.

---

## Real-Time Flow

All inter-service events use **Redis Streams** with consumer groups. Each consumer group processes messages independently, enabling horizontal scaling and durable delivery with automatic pending-message recovery.

```
Buyer places bid
      │
      ▼
Auction Service validates & updates highest bid atomically (Redis + concurrency strategy)
      │
      ├──► XADD bid:placed (Redis Stream)
      │         │
      │         ├── [bid-service group] Bid Service records bid history (marks previous same-user bids as OUTBID)
      │         │
      │         ├── [notification-service group] Notification Service broadcasts to all auction watchers (per-auction WebSocket)
      │         │
      │         └── [notification-service group] Notification Service stores + pushes "outbid" to previous bidder (per-user WebSocket)
      │
Auction closes (seller closes or timer expires via closer.go)
      │
      ├──► XADD auction:closed (Redis Stream)
                │
                ├── [bid-service group] Bid Service marks winner(s)' bids as WON
                │
                ├── [payment-service group] Payment Service charges each winner (one payment per winner for quantity auctions)
                │       ├── payment:processed stream → records success
                │       └── payment:failed stream    → records failure
                │
                ├── [notification-service group] Notification Service broadcasts close to auction watchers (per-auction WebSocket)
                │
                └── [notification-service group] Notification Service stores + pushes "won" to all winners (per-user WebSocket)
```

---

## Current Limitations

| Area | Status |
|---|---|
| Payment gateway | Simulated (90% success rate mock) — no real Stripe integration yet |
| Shop settlement | Payment records `shop_id` but does not disburse funds to the seller |
| Message delivery guarantee | Redis Streams with consumer groups — durable, replayable, with automatic pending-message recovery via XAutoClaim |
| Geo / location filtering | **Done** — Redis GEOSEARCH proximity filter, browser geolocation, distance badges |
| Image upload | **Done** — MinIO/S3 file upload via `POST /uploads`, `ImageUpload` component with drag-and-drop and URL-paste fallback |
