# Payment Service

Handles post-auction payments for the Real-Time Surplus Auction Platform.

When an auction closes, the auction service publishes an `auction_closed` event. This service picks it up, charges the winner, and records the result.

---

## How It Works

```
auction_closed (Redis Pub/Sub)
        ↓
  create Payment (pending)
        ↓
  simulate charge
        ↓
  completed ──────────────→ payment_processed
  failed    ──→ [refund] ──→ payment_failed / refund_processed
```

Payments are stored in DynamoDB. Status transitions: `pending → processing → completed / failed → refunded`.

If `winner_id` is empty (no bids placed), payment is skipped.

---

## Running Locally

```bash
# Start DynamoDB local + Redis
docker compose up -d

# Run the service (port 8081)
go run ./cmd/server
```

The `payments` table is created automatically on startup.

---

## API

All routes except admin require a `Authorization: Bearer <jwt>` header.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/payments/:id` | Get payment by ID |
| GET | `/users/:user_id/payments` | List a user's payments |
| GET | `/auctions/:auction_id/payment` | Get the payment for an auction |
| POST | `/admin/payments/:id/process` | Manually trigger processing |
| POST | `/admin/payments/:id/refund` | Refund a payment |

---

## Testing the Flow

Manually publish an `auction_closed` event to trigger a payment:

```bash
redis-cli PUBLISH auction_closed '{
  "auction_id": "a1",
  "winner_id": "u1",
  "winning_bid": 5000,
  "item_id": "i1",
  "shop_id": "s1",
  "closed_at": "2026-03-28T10:00:00Z"
}'
```

Then query the result:

```bash
curl -H "Authorization: Bearer <token>" http://localhost:8081/auctions/a1/payment
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_ADDR` | `:8081` | HTTP listen address |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `DYNAMODB_ENDPOINT` | `http://localhost:8000` | DynamoDB endpoint |
| `AWS_REGION` | `us-east-1` | AWS region |
| `JWT_SECRET` | `dev-secret` | JWT signing secret |
