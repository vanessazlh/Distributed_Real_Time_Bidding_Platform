"""
Experiment 2 — Horizontal Scaling During Auction Spikes
=========================================================
NOTE: This experiment requires AWS ECS Fargate + ALB. It cannot be run locally.

Setup checklist (see tests/TESTING.md for full details):
  - ECS Task Definitions created for all services
  - ALB routing rules configured (path-based)
  - Auto-scaling policy on Auction Service: CPU > 60%, scale up by 1, cooldown 60s
  - CloudWatch dashboard: CPU %, task count, ALB latency

Goal:
  50 simultaneous auctions, 100 bidders each (5000 total users).
  Start with 2 ECS tasks; ramp 0 → 5000 users over 60s.
  Measure: scale-up trigger time, latency during transition, bid loss rate.

Required env vars:
  ALB_HOST      — ALB DNS name (e.g. http://rtb-alb-123.us-east-1.elb.amazonaws.com)
  BUYER_TOKEN   — valid buyer JWT
  AUCTION_IDS   — comma-separated list of 50 auction IDs

Usage:
  export ALB_HOST="http://<alb-dns>" BUYER_TOKEN="<jwt>" AUCTION_IDS="id1,id2,..."
  locust -f loadtest/scenarios/exp2_scaling_spike.py \\
         --headless -u 5000 -r 83 -t 300s \\
         --host $ALB_HOST \\
         --csv loadtest/results/exp2_run1
"""

import os
import random

from locust import HttpUser, between, task

ALB_HOST = os.environ.get("ALB_HOST", "http://localhost:8081")
BUYER_TOKEN = os.environ.get("BUYER_TOKEN", "")
AUCTION_IDS = [a.strip() for a in os.environ.get("AUCTION_IDS", "").split(",") if a.strip()]

if not BUYER_TOKEN:
    raise SystemExit("ERROR: BUYER_TOKEN env var is required")
if not AUCTION_IDS:
    raise SystemExit("ERROR: AUCTION_IDS env var is required (comma-separated list of auction IDs)")


class ScalingBidUser(HttpUser):
    """
    Each user picks a random auction from the pool and continuously bids on it.
    With 5000 users across 50 auctions that's 100 bidders per auction.
    """

    wait_time = between(0.5, 1.5)

    def on_start(self):
        self._headers = {"Authorization": f"Bearer {BUYER_TOKEN}"}
        self._auction_id = random.choice(AUCTION_IDS)
        self._bid_url = f"/auctions/{self._auction_id}/bid"
        self._get_url = f"/auctions/{self._auction_id}"

    @task
    def place_bid(self):
        # Read current price
        with self.client.get(
            self._get_url,
            headers=self._headers,
            name="/auctions/:id (read)",
            catch_response=True,
        ) as resp:
            if resp.status_code != 200:
                resp.failure(f"GET auction failed: {resp.status_code}")
                return
            try:
                current = resp.json().get("current_highest_bid", 0)
            except Exception as exc:
                resp.failure(f"JSON parse error: {exc}")
                return

        amount = current + random.randint(1, 100)
        with self.client.post(
            self._bid_url,
            json={"amount": amount},
            headers=self._headers,
            name="/auctions/:id/bid",
            catch_response=True,
        ) as resp:
            if resp.status_code in (201, 400, 409):
                resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")
