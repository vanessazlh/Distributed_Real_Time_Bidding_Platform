"""
Experiment 1 – Bid Contention Under Load
=========================================
Goal: Compare three concurrency strategies (optimistic / pessimistic / queue)
under 500 concurrent bidders hitting a single auction.

Usage (run 3 times per strategy, then repeat for pessimistic and queue):

  # 1. Make sure services are up:  docker compose up -d
  # 2. Set env vars (or edit defaults below):
  #      AUCTION_ID   – ID of a pre-created OPEN auction
  #      BUYER_TOKEN  – JWT token with role=buyer
  #      STRATEGY     – optimistic | pessimistic | queue  (switches strategy before run)
  #      RUN          – run number: 1, 2, or 3

  # Run optimistic x3:
  for RUN in 1 2 3; do
    STRATEGY=optimistic RUN=$RUN AUCTION_ID=... BUYER_TOKEN=... \
      locust -f tests/loadtest/scenarios/exp1_bid_contention.py \
             --headless -u 500 -r 50 -t 60s \
             --host http://localhost:8081 \
             --csv tests/loadtest/results/exp1_optimistic_run${RUN}
    sleep 5
    curl -s -X POST http://localhost:8081/admin/metrics/reset
    sleep 2
  done

  # Repeat the loop with STRATEGY=pessimistic and STRATEGY=queue.

How it works:
  - on_start: each simulated user reads the current highest bid (GET /auctions/:id)
  - Each task: GET auction → read current_highest → POST bid with amount = current_highest + rand(1,100)
  - This creates real contention: 500 users read the same price and simultaneously
    try to outbid, so most will be rejected – exactly what we want to measure.
  - Rejected bids (400 bid-too-low, 409 closed, lock conflicts) are recorded as
    "bid_rejected" custom events so Locust counts them separately from errors.
"""

import os
import random
import json
import time
from locust import HttpUser, task, between, events

# ── Configuration ─────────────────────────────────────────────────────────────
AUCTION_ID  = os.getenv("AUCTION_ID", "")        # must be set before running
BUYER_TOKEN = os.getenv("BUYER_TOKEN", "")        # JWT with role=buyer
STRATEGY    = os.getenv("STRATEGY", "optimistic") # optimistic | pessimistic | queue
RUN         = os.getenv("RUN", "1")               # run number: 1, 2, or 3

AUCTION_HOST  = os.getenv("AUCTION_HOST", "http://localhost:8081")
# ──────────────────────────────────────────────────────────────────────────────


# Switch strategy once before the test begins
@events.test_start.add_listener
def switch_strategy(environment, **kwargs):
    if not AUCTION_ID:
        print("[WARN] AUCTION_ID is not set – bids will fail with 404")
    if STRATEGY:
        import urllib.request, urllib.error
        url = f"{AUCTION_HOST}/admin/strategy"
        payload = json.dumps({"strategy": STRATEGY}).encode()
        req = urllib.request.Request(url, data=payload, method="PUT",
                                     headers={"Content-Type": "application/json"})
        try:
            with urllib.request.urlopen(req, timeout=5) as resp:
                body = json.loads(resp.read())
                print(f"[INFO] Strategy set to: {body.get('strategy')}")
        except Exception as e:
            print(f"[WARN] Could not set strategy: {e}")


# Save metrics after the test ends
@events.test_stop.add_listener
def save_metrics(environment, **kwargs):
    import urllib.request
    url = f"{AUCTION_HOST}/admin/metrics"
    try:
        with urllib.request.urlopen(url, timeout=5) as resp:
            metrics = json.loads(resp.read())
        metrics["strategy"] = STRATEGY
        metrics["auction_id"] = AUCTION_ID
        metrics["timestamp"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        out_path = f"tests/loadtest/results/exp1_{STRATEGY}_run{RUN}_metrics.json"
        with open(out_path, "w") as f:
            json.dump(metrics, f, indent=2)
        print(f"[INFO] Metrics saved to {out_path}")
        print(json.dumps(metrics, indent=2))
    except Exception as e:
        print(f"[WARN] Could not fetch/save metrics: {e}")


class BidUser(HttpUser):
    """
    Simulates a buyer who continuously tries to place the highest bid.
    Wait 0.1–0.5 s between attempts (aggressive but realistic for a hot auction).
    """
    wait_time = between(0.1, 0.5)
    host = AUCTION_HOST

    def on_start(self):
        """Verify the auction exists and store the initial price."""
        self.auction_id = AUCTION_ID
        self.auth_header = {"Authorization": f"Bearer {BUYER_TOKEN}"}
        self.current_highest = self._get_current_highest()

    def _get_current_highest(self) -> int:
        """GET /auctions/:id and return current_highest (int, cents)."""
        if not self.auction_id:
            return 300  # fallback if no auction configured
        with self.client.get(
            f"/auctions/{self.auction_id}",
            headers=self.auth_header,
            name="/auctions/:id [GET]",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                try:
                    data = resp.json()
                    return int(data.get("current_highest_bid", data.get("current_highest", 300)))
                except Exception:
                    pass
            resp.failure(f"GET auction failed: {resp.status_code}")
            return 300

    @task
    def place_bid(self):
        """
        Step 1: Refresh current highest price.
        Step 2: Bid current_highest + random(1, 100) cents.
        A bid of +1–100¢ keeps the price realistic while creating genuine
        contention (many users read the same price simultaneously).
        """
        if not self.auction_id:
            return

        # Refresh price every bid to get the latest state
        self.current_highest = self._get_current_highest()
        bid_amount = self.current_highest + random.randint(1, 100)

        with self.client.post(
            f"/auctions/{self.auction_id}/bid",
            json={"amount": bid_amount},
            headers=self.auth_header,
            name="/auctions/:id/bid [POST]",
            catch_response=True,
        ) as resp:
            if resp.status_code == 201:
                # Bid accepted – update local price estimate
                try:
                    self.current_highest = resp.json().get("amount", bid_amount)
                except Exception:
                    pass
                resp.success()
            elif resp.status_code in (400, 409):
                # Bid rejected (too low, auction closed, lock conflict)
                # Mark as success so Locust doesn't count this as an error –
                # rejected bids are expected and are the core metric.
                resp.success()
                # Fire a custom event so we can track rejection rate separately
                events.request.fire(
                    request_type="BID",
                    name="bid_rejected",
                    response_time=resp.elapsed.total_seconds() * 1000,
                    response_length=len(resp.content),
                    exception=None,
                    context={},
                )
            else:
                resp.failure(f"Unexpected status {resp.status_code}: {resp.text[:200]}")
