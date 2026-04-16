#!/bin/bash
# SurpriseAuction — Smoke Test Suite
# Requires: docker compose up --build && go run scripts/init_tables.go
#
# Usage:
#   chmod +x tests/smoke_test.sh
#   ./tests/smoke_test.sh

set -uo pipefail

# Random suffix to avoid email conflicts on repeated runs
RUN_ID=$(date +%s)

USER_SVC="http://localhost:8082"
SHOP_SVC="http://localhost:8083"
AUCTION_SVC="http://localhost:8081"
BID_SVC="http://localhost:8084"
PAYMENT_SVC="http://localhost:8085"

PASS=0
FAIL=0

# ── Helpers ────────────────────────────────────────────────────────────────────

ok() { echo "  ✅ PASS: $1"; ((PASS++)); }
fail() { echo "  ❌ FAIL: $1 — $2"; ((FAIL++)); }

# Extract a JSON field: json_field '{"foo":"bar"}' foo → bar
json_field() { echo "$1" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('$2',''))" 2>/dev/null; }

assert_field() {
  local label=$1 body=$2 field=$3
  local val; val=$(json_field "$body" "$field")
  if [ -n "$val" ] && [ "$val" != "None" ] && [ "$val" != "null" ]; then
    ok "$label (${field}=${val})"
  else
    fail "$label" "field '$field' missing or empty in: $body"
  fi
}

assert_status() {
  local label=$1 body=$2 expected=$3
  local val; val=$(json_field "$body" "status")
  if [ "$val" = "$expected" ]; then
    ok "$label (status=${val})"
  else
    fail "$label" "expected status='$expected', got '$val' in: $body"
  fi
}

# ── B1: Register buyer ─────────────────────────────────────────────────────────
echo ""
echo "── B1: Register buyer"
BUYER=$(curl -sf -X POST "$USER_SVC/users" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"smokebuyer_${RUN_ID}\",\"email\":\"smokebuyer_${RUN_ID}@test.com\",\"password\":\"password123\",\"role\":\"buyer\"}")
BUYER_ID=$(json_field "$BUYER" "user_id")
assert_field "B1 register buyer" "$BUYER" "user_id"

# ── B2: Register seller ────────────────────────────────────────────────────────
echo ""
echo "── B2: Register seller"
SELLER=$(curl -sf -X POST "$USER_SVC/users" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"smokeseller_${RUN_ID}\",\"email\":\"smokeseller_${RUN_ID}@test.com\",\"password\":\"password123\",\"role\":\"seller\"}")
SELLER_ID=$(json_field "$SELLER" "user_id")
assert_field "B2 register seller" "$SELLER" "user_id"

# ── B3: Login both roles ───────────────────────────────────────────────────────
echo ""
echo "── B3: Login"
BUYER_LOGIN=$(curl -sf -X POST "$USER_SVC/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"smokebuyer_${RUN_ID}@test.com\",\"password\":\"password123\"}")
BUYER_TOKEN=$(json_field "$BUYER_LOGIN" "token")
assert_field "B3 buyer login" "$BUYER_LOGIN" "token"

SELLER_LOGIN=$(curl -sf -X POST "$USER_SVC/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"smokeseller_${RUN_ID}@test.com\",\"password\":\"password123\"}")
SELLER_TOKEN=$(json_field "$SELLER_LOGIN" "token")
assert_field "B3 seller login" "$SELLER_LOGIN" "token"

# Verify seller JWT has role=seller
ROLE=$(echo "$SELLER_TOKEN" | python3 -c "
import sys,json,base64
t=sys.stdin.read().strip()
p=t.split('.')[1]
p+='='*(4-len(p)%4)
print(json.loads(base64.b64decode(p)).get('role',''))
")
if [ "$ROLE" = "seller" ]; then
  ok "B3 seller JWT role=seller"
else
  fail "B3 seller JWT role" "expected 'seller', got '$ROLE'"
fi

# ── B4: Seller creates shop ────────────────────────────────────────────────────
echo ""
echo "── B4: Create shop"
SHOP=$(curl -sf -X POST "$SHOP_SVC/shops" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d '{"name":"Smoke Bakery","location":"1 Test Ave"}')
SHOP_ID=$(json_field "$SHOP" "shop_id")
assert_field "B4 create shop" "$SHOP" "shop_id"

# ── B5: Seller adds item ───────────────────────────────────────────────────────
echo ""
echo "── B5: Add item"
ITEM=$(curl -sf -X POST "$SHOP_SVC/shops/$SHOP_ID/items" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d '{"title":"Smoke Pastry Box","description":"Test item","retail_value":1000}')
ITEM_ID=$(json_field "$ITEM" "item_id")
assert_field "B5 add item" "$ITEM" "item_id"

# ── B6: Seller creates auction ─────────────────────────────────────────────────
echo ""
echo "── B6: Create auction"
# pickup window: 1 day from now → 2 days from now (required fields added in service validation)
PICKUP_START=$(python3 -c "from datetime import datetime, timedelta, timezone; print((datetime.now(timezone.utc)+timedelta(days=1)).strftime('%Y-%m-%dT%H:%M:%SZ'))")
PICKUP_END=$(python3 -c "from datetime import datetime, timedelta, timezone; print((datetime.now(timezone.utc)+timedelta(days=2)).strftime('%Y-%m-%dT%H:%M:%SZ'))")
AUCTION=$(curl -sf -X POST "$AUCTION_SVC/auctions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d "{
    \"item_id\": \"$ITEM_ID\",
    \"item_title\": \"Smoke Pastry Box\",
    \"shop_id\": \"$SHOP_ID\",
    \"shop_name\": \"Smoke Bakery\",
    \"retail_price\": 1000,
    \"description\": \"Test auction\",
    \"image_url\": \"https://example.com/img.jpg\",
    \"shop_logo_url\": \"https://example.com/logo.jpg\",
    \"duration_minutes\": 1,
    \"start_bid\": 200,
    \"pickup_start\": \"$PICKUP_START\",
    \"pickup_end\": \"$PICKUP_END\"
  }")
AUCTION_ID=$(json_field "$AUCTION" "auction_id")
assert_field "B6 create auction" "$AUCTION" "auction_id"
assert_status "B6 auction status=OPEN" "$AUCTION" "OPEN"

# ── B7: List auctions ──────────────────────────────────────────────────────────
echo ""
echo "── B7: List auctions"
AUCTIONS=$(curl -sf "$AUCTION_SVC/auctions")
COUNT=$(echo "$AUCTIONS" | python3 -c "import sys,json; d=json.load(sys.stdin); a=d.get('auctions',d) if isinstance(d,dict) else d; print(len(a))" 2>/dev/null)
if [ "$COUNT" -gt 0 ]; then
  ok "B7 list auctions (count=$COUNT)"
else
  fail "B7 list auctions" "empty list or unexpected shape: $AUCTIONS"
fi

# ── B8: Buyer places bid ───────────────────────────────────────────────────────
echo ""
echo "── B8: Place bid"
BID=$(curl -sf -X POST "$AUCTION_SVC/auctions/$AUCTION_ID/bid" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $BUYER_TOKEN" \
  -d '{"amount": 500}')
BID_STATUS=$(json_field "$BID" "status")
if [ "$BID_STATUS" = "ACCEPTED" ]; then
  ok "B8 place bid (status=ACCEPTED)"
else
  fail "B8 place bid" "expected ACCEPTED, got: $BID"
fi

# ── B9: Bid rejected (too low) ────────────────────────────────────────────────
echo ""
echo "── B9: Bid rejected if too low"
LOW_BID=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$AUCTION_SVC/auctions/$AUCTION_ID/bid" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $BUYER_TOKEN" \
  -d '{"amount": 100}')
if [ "$LOW_BID" = "400" ] || [ "$LOW_BID" = "409" ] || [ "$LOW_BID" = "422" ]; then
  ok "B9 low bid rejected (HTTP $LOW_BID)"
else
  fail "B9 low bid rejected" "expected 4xx, got HTTP $LOW_BID"
fi

# ── Auth negative cases ───────────────────────────────────────────────────────
echo ""
echo "── N1: Buyer cannot create auction (expect 403)"
N1=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$AUCTION_SVC/auctions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $BUYER_TOKEN" \
  -d "{\"item_id\":\"$ITEM_ID\",\"shop_id\":\"$SHOP_ID\",\"duration_minutes\":1,\"pickup_start\":\"$PICKUP_START\",\"pickup_end\":\"$PICKUP_END\"}")
if [ "$N1" = "403" ]; then
  ok "N1 buyer cannot create auction (HTTP 403)"
else
  fail "N1 buyer cannot create auction" "expected 403, got HTTP $N1"
fi

echo ""
echo "── N2: Seller cannot self-bid (expect 403)"
N2=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$AUCTION_SVC/auctions/$AUCTION_ID/bid" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SELLER_TOKEN" \
  -d '{"amount": 600}')
if [ "$N2" = "403" ]; then
  ok "N2 seller cannot self-bid (HTTP 403)"
else
  fail "N2 seller cannot self-bid" "expected 403, got HTTP $N2"
fi

echo ""
echo "── N3: Buyer cannot close auction (expect 403)"
N3=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$AUCTION_SVC/auctions/$AUCTION_ID/close" \
  -H "Authorization: Bearer $BUYER_TOKEN")
if [ "$N3" = "403" ]; then
  ok "N3 buyer cannot close auction (HTTP 403)"
else
  fail "N3 buyer cannot close auction" "expected 403, got HTTP $N3"
fi

# ── B10: Bid history for auction ──────────────────────────────────────────────
echo ""
echo "── B10: Bid history (auction)"
BIDS=$(curl -sf "$BID_SVC/auctions/$AUCTION_ID/bids")
BID_COUNT=$(echo "$BIDS" | python3 -c "import sys,json; d=json.load(sys.stdin); a=d.get('bids',d) if isinstance(d,dict) else d; print(len(a))" 2>/dev/null)
if [ "$BID_COUNT" -gt 0 ]; then
  ok "B10 bid history (count=$BID_COUNT)"
else
  fail "B10 bid history" "expected bids, got: $BIDS"
fi

# ── B11: User bid history ─────────────────────────────────────────────────────
echo ""
echo "── B11: User bid history"
USER_BIDS=$(curl -sf "$USER_SVC/users/$BUYER_ID/bids" \
  -H "Authorization: Bearer $BUYER_TOKEN")
USER_BID_COUNT=$(echo "$USER_BIDS" | python3 -c "import sys,json; d=json.load(sys.stdin); a=d.get('bids',d) if isinstance(d,dict) else d; print(len(a))" 2>/dev/null)
if [ "$USER_BID_COUNT" -gt 0 ]; then
  ok "B11 user bid history (count=$USER_BID_COUNT)"
else
  fail "B11 user bid history" "expected bids, got: $USER_BIDS"
fi

# ── B12: Close auction + payment trigger ──────────────────────────────────────
echo ""
echo "── B12–B14: Close auction and verify payment"
CLOSE=$(curl -sf -X POST "$AUCTION_SVC/auctions/$AUCTION_ID/close" \
  -H "Authorization: Bearer $SELLER_TOKEN")
CLOSE_MSG=$(json_field "$CLOSE" "message")
CLOSE_STATUS=$(json_field "$CLOSE" "status")
if [ "$CLOSE_MSG" = "auction closed" ] || [ "$CLOSE_STATUS" = "CLOSED" ]; then
  ok "B12 close auction"
else
  fail "B12 close auction" "got: $CLOSE"
fi

# Wait for payment consumer to process (poll until status settles, up to 10s)
echo "  ⏳ Waiting for payment consumer (up to 10s)..."
PAYMENT=""
PAYMENT_STATUS=""
for i in $(seq 1 10); do
  PAYMENT=$(curl -s "$PAYMENT_SVC/auctions/$AUCTION_ID/payment" \
    -H "Authorization: Bearer $BUYER_TOKEN") || PAYMENT=""
  PAYMENT_STATUS=$(json_field "$PAYMENT" "status")
  if [ "$PAYMENT_STATUS" = "completed" ] || [ "$PAYMENT_STATUS" = "failed" ]; then
    break
  fi
  sleep 1
done

# B13: Payment triggered
if [ "$PAYMENT_STATUS" = "completed" ] || [ "$PAYMENT_STATUS" = "failed" ]; then
  ok "B13 payment triggered (status=$PAYMENT_STATUS)"
else
  fail "B13 payment triggered" "expected completed/failed after 10s, got: $PAYMENT"
fi

# B14: User payment history
USER_PAYMENTS=$(curl -sf "$PAYMENT_SVC/users/$BUYER_ID/payments" \
  -H "Authorization: Bearer $BUYER_TOKEN")
PAYMENT_COUNT=$(echo "$USER_PAYMENTS" | python3 -c "import sys,json; a=json.load(sys.stdin); print(len(a) if isinstance(a,list) else 0)" 2>/dev/null)
if [ "$PAYMENT_COUNT" -gt 0 ]; then
  ok "B14 user payment history (count=$PAYMENT_COUNT)"
else
  fail "B14 user payment history" "expected payments, got: $USER_PAYMENTS"
fi

# ── B15: Strategy switch ──────────────────────────────────────────────────────
echo ""
echo "── B15: Concurrency strategy switch"
for STRATEGY in pessimistic queue optimistic; do
  SWITCH=$(curl -sf -X PUT "$AUCTION_SVC/admin/strategy" \
    -H "Content-Type: application/json" \
    -d "{\"strategy\": \"$STRATEGY\"}")
  CURRENT=$(json_field "$SWITCH" "strategy")
  if [ "$CURRENT" = "$STRATEGY" ]; then
    ok "B15 strategy=$STRATEGY"
  else
    fail "B15 strategy switch to $STRATEGY" "got: $SWITCH"
  fi
done

# ── B16: Reviews ──────────────────────────────────────────────────────────────
echo ""
echo "── B16: Reviews"

# B16a: List reviews for a shop (public, initially empty)
REVIEWS_EMPTY=$(curl -sf "$SHOP_SVC/shops/$SHOP_ID/reviews")
REV_COUNT=$(json_field "$REVIEWS_EMPTY" "total_reviews")
if [ "$REV_COUNT" = "0" ]; then
  ok "B16a list reviews (empty, total_reviews=0)"
else
  fail "B16a list reviews empty" "expected total_reviews=0, got: $REVIEWS_EMPTY"
fi

# B16b: Buyer submits a review (only possible if payment completed — smoke test
#        uses the auction_id from the payment we triggered above)
REV_BODY=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$SHOP_SVC/shops/$SHOP_ID/reviews" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $BUYER_TOKEN" \
  -d "{\"auction_id\":\"$AUCTION_ID\",\"rating\":5,\"comment\":\"Great pickup!\"}")
# Accept 201 (created) or 403 (payment check configured and auction still simulated).
# In local dev PAYMENT_SERVICE_URL is configured so the payment service is hit;
# payment status may be completed or failed — 403 is valid when not completed.
if [ "$REV_BODY" = "201" ] || [ "$REV_BODY" = "403" ]; then
  ok "B16b create review (HTTP $REV_BODY)"
else
  fail "B16b create review" "expected 201 or 403, got HTTP $REV_BODY"
fi

# B16c: If the review was created (201), verify it appears in the list and
#        that the seller can reply.
if [ "$REV_BODY" = "201" ]; then
  REVIEW_RESP=$(curl -sf -X POST "$SHOP_SVC/shops/$SHOP_ID/reviews" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $BUYER_TOKEN" \
    -d "{\"auction_id\":\"$AUCTION_ID\",\"rating\":5,\"comment\":\"Great pickup!\"}" 2>/dev/null || echo "{}")
  REVIEW_ID=$(json_field "$REVIEW_RESP" "review_id")

  # List should now have ≥1 review
  REVIEWS_LIST=$(curl -sf "$SHOP_SVC/shops/$SHOP_ID/reviews")
  TOTAL=$(json_field "$REVIEWS_LIST" "total_reviews")
  if [ "$TOTAL" -ge 1 ] 2>/dev/null; then
    ok "B16c review appears in list (total_reviews=$TOTAL)"
  else
    fail "B16c review in list" "expected ≥1, got: $REVIEWS_LIST"
  fi

  # Seller cannot duplicate review (second POST same auction should be 409)
  DUP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$SHOP_SVC/shops/$SHOP_ID/reviews" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $BUYER_TOKEN" \
    -d "{\"auction_id\":\"$AUCTION_ID\",\"rating\":3}")
  if [ "$DUP" = "409" ]; then
    ok "B16d duplicate review rejected (HTTP 409)"
  else
    # 403 also acceptable (payment check fires first)
    if [ "$DUP" = "403" ]; then
      ok "B16d duplicate review rejected (HTTP 403)"
    else
      fail "B16d duplicate review" "expected 409 or 403, got HTTP $DUP"
    fi
  fi

  # Seller replies to the review
  if [ -n "$REVIEW_ID" ]; then
    REPLY=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$SHOP_SVC/shops/$SHOP_ID/reviews/$REVIEW_ID/reply" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $SELLER_TOKEN" \
      -d '{"reply":"Thanks for the kind words!"}')
    if [ "$REPLY" = "200" ]; then
      ok "B16e seller reply (HTTP 200)"
    else
      fail "B16e seller reply" "expected 200, got HTTP $REPLY"
    fi

    # Buyer cannot reply
    BUYER_REPLY=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$SHOP_SVC/shops/$SHOP_ID/reviews/$REVIEW_ID/reply" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $BUYER_TOKEN" \
      -d '{"reply":"I should not be able to do this"}')
    if [ "$BUYER_REPLY" = "403" ]; then
      ok "B16f buyer cannot reply (HTTP 403)"
    else
      fail "B16f buyer reply guard" "expected 403, got HTTP $BUYER_REPLY"
    fi
  fi
fi

# B16g: Unauthenticated user cannot submit a review
UNAUTH=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$SHOP_SVC/shops/$SHOP_ID/reviews" \
  -H "Content-Type: application/json" \
  -d "{\"auction_id\":\"$AUCTION_ID\",\"rating\":4}")
if [ "$UNAUTH" = "401" ]; then
  ok "B16g unauthenticated review rejected (HTTP 401)"
else
  fail "B16g unauthenticated review" "expected 401, got HTTP $UNAUTH"
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════"
echo "  Results: $PASS passed, $FAIL failed"
echo "══════════════════════════════════════"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
