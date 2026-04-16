"""
Experiment 1 — Bid Contention Under Load
=========================================
500 concurrent bidders race on a single auction.
Run once per concurrency strategy (optimistic / pessimistic / queue).

Required env vars:
  AUCTION_ID    — ID of a long-lived OPEN auction
  BUYER_TOKEN   — valid buyer JWT
  STRATEGY      — optimistic | pessimistic | queue  (default: pessimistic)

Usage (repeat for each strategy, reset metrics between runs):
  export AUCTION_ID="<id>"
  export BUYER_TOKEN="<jwt>"
  export STRATEGY=optimistic

  locust -f loadtest/scenarios/exp1_bid_contention.py \\
         --headless -u 500 -r 50 -t 60s \\
         --host http://localhost:8081 \\
         --csv loadtest/results/exp1_optimistic

  curl -s -X POST http://localhost:8081/admin/metrics/reset
  # then repeat with STRATEGY=pessimistic, then queue
"""

import json
import os
import random
import time

import requests
from locust import HttpUser, between, events, task

AUCTION_ID = os.environ.get("AUCTION_ID", "")
BUYER_TOKEN = os.environ.get("BUYER_TOKEN", "")
STRATEGY = os.environ.get("STRATEGY", "pessimistic")

if not AUCTION_ID:
    raise SystemExit("ERROR: AUCTION_ID env var is required")
if not BUYER_TOKEN:
    raise SystemExit("ERROR: BUYER_TOKEN env var is required")


# ---------------------------------------------------------------------------
# Strategy switch + metrics save (run once before the test starts)
# ---------------------------------------------------------------------------

@events.test_start.add_listener
def on_test_start(environment, **kwargs):
    host = environment.host or "http://localhost:8081"
    # Switch strategy
    resp = requests.put(
        f"{host}/admin/strategy",
        json={"strategy": STRATEGY},
        timeout=5,
    )
    if resp.status_code != 200:
        raise SystemExit(f"Failed to set strategy '{STRATEGY}': {resp.text}")
    print(f"[exp1] Strategy set to: {STRATEGY}")

    # Reset metrics for a clean baseline
    requests.post(f"{host}/admin/metrics/reset", timeout=5)
    print("[exp1] Metrics reset")


@events.test_stop.add_listener
def on_test_stop(environment, **kwargs):
    host = environment.host or "http://localhost:8081"
    resp = requests.get(f"{host}/admin/metrics", timeout=5)
    if resp.status_code != 200:
        print(f"[exp1] WARNING: could not fetch metrics: {resp.text}")
        return

    metrics = resp.json()
    out_path = f"loadtest/results/exp1_{STRATEGY}_metrics.json"
    os.makedirs("loadtest/results", exist_ok=True)
    with open(out_path, "w") as f:
        json.dump({"strategy": STRATEGY, "metrics": metrics}, f, indent=2)
    print(f"[exp1] Metrics saved to {out_path}")
    print(json.dumps(metrics, indent=2))


# ---------------------------------------------------------------------------
# Locust user
# ---------------------------------------------------------------------------

class BidUser(HttpUser):
    """
    Each virtual user:
      1. Reads the current highest bid from GET /auctions/:id
      2. Places a bid for current_highest + random(1, 100) cents
    This creates genuine contention: 500 users race on the same stale price.
    """

    wait_time = between(0.05, 0.2)  # slight jitter so the ramp doesn't spike instantly

    def on_start(self):
        self.headers = {"Authorization": f"Bearer {BUYER_TOKEN}"}
        self.auction_url = f"/auctions/{AUCTION_ID}"
        self.bid_url = f"/auctions/{AUCTION_ID}/bid"

    @task
    def place_bid(self):
        # Step 1: read current price
        with self.client.get(
            self.auction_url,
            headers=self.headers,
            name="/auctions/:id (read)",
            catch_response=True,
        ) as resp:
            if resp.status_code != 200:
                resp.failure(f"GET auction failed: {resp.status_code}")
                return
            try:
                data = resp.json()
                current = data.get("current_highest_bid", 0)
            except Exception as exc:
                resp.failure(f"JSON parse error: {exc}")
                return

        # Step 2: bid slightly above current (random jitter creates contention)
        amount = current + random.randint(1, 100)
        with self.client.post(
            self.bid_url,
            json={"amount": amount},
            headers=self.headers,
            name="/auctions/:id/bid",
            catch_response=True,
        ) as resp:
            if resp.status_code == 201:
                resp.success()
            elif resp.status_code in (400, 409):
                # Rejected bid (too low / auction closed) — expected under contention
                resp.success()  # don't count as Locust failure; auction service tracks it
            else:
                resp.failure(f"Unexpected status: {resp.status_code} — {resp.text[:200]}")
