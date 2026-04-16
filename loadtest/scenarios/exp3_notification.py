"""
Experiment 3 — Notification Fan-Out (WebSocket push vs. HTTP polling)
=======================================================================
Compares delivery latency and server load between:
  - Push run : 1000 WebSocket clients subscribe to one auction; 1 bidder
               places 1 bid per second for 5 minutes.
  - Pull run : 1000 polling clients call GET /auctions/:id every second;
               same bid source.

The Notification Service (:8080) exposes GET /metrics with:
  active_connections, total_broadcasts,
  avg_delivery_latency_ms, p99_delivery_latency_ms

Required env vars:
  AUCTION_ID    — ID of a long-lived OPEN auction
  BUYER_TOKEN   — valid buyer JWT
  MODE          — ws (default) | poll

Required host mappings (pass two --host flags or set via env):
  AUCTION_HOST        — Auction Service   (default http://localhost:8081)
  NOTIFICATION_HOST   — Notification Svc  (default http://localhost:8080)

Usage:

  # Push run (WebSocket)
  export AUCTION_ID="<id>" BUYER_TOKEN="<jwt>" MODE=ws
  locust -f loadtest/scenarios/exp3_notification.py \\
         --headless -u 1001 -r 50 -t 300s \\
         --host http://localhost:8080 \\
         --csv loadtest/results/exp3_ws

  # Poll run
  export MODE=poll
  locust -f loadtest/scenarios/exp3_notification.py \\
         --headless -u 1001 -r 50 -t 300s \\
         --host http://localhost:8081 \\
         --csv loadtest/results/exp3_poll

Results (latency + metrics) are written to loadtest/results/exp3_ws_vs_poll.json.

Notes:
  - User #0 is the single bid source (BidSourceUser); the rest are observers.
  - WebSocket delivery latency is measured end-to-end: from bid_accepted_at
    (stamped by Auction Service in the BidPlacedEvent, echoed in OutbidMessage)
    to the moment the WS frame is received by the client. This matches the
    metric already tracked by the Notification Service hub.
  - The /metrics endpoint on Notification Service is the authoritative latency
    source; Locust response times measure HTTP round-trip, not WS push latency.
"""

import json
import os
import random
import threading
import time

import requests
import websocket  # websocket-client package
from locust import HttpUser, between, events, task

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------

AUCTION_ID = os.environ.get("AUCTION_ID", "")
BUYER_TOKEN = os.environ.get("BUYER_TOKEN", "")
MODE = os.environ.get("MODE", "ws").lower()  # "ws" or "poll"
AUCTION_HOST = os.environ.get("AUCTION_HOST", "http://localhost:8081")
NOTIFICATION_HOST = os.environ.get("NOTIFICATION_HOST", "http://localhost:8080")

if not AUCTION_ID:
    raise SystemExit("ERROR: AUCTION_ID env var is required")
if not BUYER_TOKEN:
    raise SystemExit("ERROR: BUYER_TOKEN env var is required")
if MODE not in ("ws", "poll"):
    raise SystemExit("ERROR: MODE must be 'ws' or 'poll'")

# Shared counter so only one Locust user acts as the bid source.
_bid_source_claimed = threading.Event()

# Accumulate client-side WS latency samples (ms).
_ws_latency_samples: list[float] = []
_ws_latency_lock = threading.Lock()


# ---------------------------------------------------------------------------
# Lifecycle hooks
# ---------------------------------------------------------------------------

@events.test_start.add_listener
def on_test_start(environment, **kwargs):
    print(f"[exp3] Starting — mode={MODE}, auction={AUCTION_ID}")


@events.test_stop.add_listener
def on_test_stop(environment, **kwargs):
    # Fetch authoritative metrics from Notification Service
    try:
        resp = requests.get(f"{NOTIFICATION_HOST}/metrics", timeout=5)
        notif_metrics = resp.json() if resp.status_code == 200 else {}
    except Exception as exc:
        notif_metrics = {"error": str(exc)}

    # Compute client-side WS latency stats (if any)
    with _ws_latency_lock:
        samples = list(_ws_latency_samples)

    client_avg = round(sum(samples) / len(samples), 1) if samples else 0
    sorted_s = sorted(samples)
    if sorted_s:
        p99_idx = max(0, int(len(sorted_s) * 0.99) - 1)
        client_p99 = round(sorted_s[p99_idx], 1)
    else:
        client_p99 = 0

    result = {
        "mode": MODE,
        "auction_id": AUCTION_ID,
        "notification_service_metrics": notif_metrics,
        "client_side_ws_latency": {
            "samples": len(samples),
            "avg_ms": client_avg,
            "p99_ms": client_p99,
        },
    }

    # Merge with any existing results file so both push and pull runs land together.
    out_path = "loadtest/results/exp3_ws_vs_poll.json"
    os.makedirs("loadtest/results", exist_ok=True)
    existing = {}
    try:
        with open(out_path) as f:
            existing = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        pass

    existing[MODE] = result
    with open(out_path, "w") as f:
        json.dump(existing, f, indent=2)

    print(f"[exp3] Results saved to {out_path}")
    print(json.dumps(result, indent=2))


# ---------------------------------------------------------------------------
# Bid source user (exactly one per run)
# ---------------------------------------------------------------------------

class BidSourceUser(HttpUser):
    """
    Places 1 bid per second on the target auction.
    Exactly one instance is active (guarded by _bid_source_claimed).
    All other instances of this class immediately stop and do nothing.
    """

    wait_time = between(1, 1)
    host = AUCTION_HOST
    weight = 1  # low weight — Locust spawns ~1 per 1000 users

    def on_start(self):
        if _bid_source_claimed.is_set():
            self._active = False
            return
        _bid_source_claimed.set()
        self._active = True
        self._headers = {"Authorization": f"Bearer {BUYER_TOKEN}"}
        self._bid_url = f"{AUCTION_HOST}/auctions/{AUCTION_ID}/bid"
        self._get_url = f"{AUCTION_HOST}/auctions/{AUCTION_ID}"
        print("[exp3] BidSourceUser active — placing 1 bid/sec")

    @task
    def bid(self):
        if not getattr(self, "_active", False):
            time.sleep(1)
            return

        # Read current price first
        try:
            r = requests.get(self._get_url, headers=self._headers, timeout=5)
            current = r.json().get("current_highest_bid", 0) if r.status_code == 200 else 0
        except Exception:
            current = 0

        amount = current + random.randint(1, 50)
        try:
            requests.post(
                self._bid_url,
                json={"amount": amount},
                headers=self._headers,
                timeout=5,
            )
        except Exception as exc:
            print(f"[exp3] bid error: {exc}")


# ---------------------------------------------------------------------------
# WebSocket observer user
# ---------------------------------------------------------------------------

class WSObserverUser(HttpUser):
    """
    Connects to GET /auctions/:id/subscribe (WebSocket) on the Notification Service.
    Receives bid_placed events and records client-side delivery latency.
    """

    wait_time = between(300, 300)  # stay connected for the full test duration
    host = NOTIFICATION_HOST
    weight = 999

    def on_start(self):
        if MODE != "ws":
            return
        ws_host = NOTIFICATION_HOST.replace("http://", "ws://").replace("https://", "wss://")
        ws_url = f"{ws_host}/auctions/{AUCTION_ID}/subscribe"
        self._ws = None

        def on_message(ws, message):
            try:
                data = json.loads(message)
            except Exception:
                return
            bid_accepted_at = data.get("bid_accepted_at", "")
            if not bid_accepted_at:
                return
            try:
                import datetime
                accepted = datetime.datetime.fromisoformat(
                    bid_accepted_at.replace("Z", "+00:00")
                )
                now = datetime.datetime.now(datetime.timezone.utc)
                latency_ms = (now - accepted).total_seconds() * 1000
                with _ws_latency_lock:
                    _ws_latency_samples.append(latency_ms)
            except Exception:
                pass

        def on_error(ws, error):
            pass  # connection resets are expected at end of test

        def on_close(ws, close_status_code, close_msg):
            pass

        self._ws = websocket.WebSocketApp(
            ws_url,
            on_message=on_message,
            on_error=on_error,
            on_close=on_close,
        )
        self._ws_thread = threading.Thread(
            target=self._ws.run_forever, daemon=True
        )
        self._ws_thread.start()

    def on_stop(self):
        if getattr(self, "_ws", None):
            try:
                self._ws.close()
            except Exception:
                pass

    @task
    def stay_connected(self):
        # Keep the Locust user alive; actual work is done in the WS thread.
        if MODE != "ws":
            self.wait()
        time.sleep(1)


# ---------------------------------------------------------------------------
# HTTP polling observer user
# ---------------------------------------------------------------------------

class PollObserverUser(HttpUser):
    """
    Polls GET /auctions/:id every ~1 second on the Auction Service.
    Simulates a client that refreshes to check for updates instead of using WebSocket.
    """

    wait_time = between(0.9, 1.1)
    host = AUCTION_HOST
    weight = 999

    def on_start(self):
        self._skip = MODE != "poll"

    @task
    def poll_auction(self):
        if self._skip:
            time.sleep(1)
            return
        with self.client.get(
            f"/auctions/{AUCTION_ID}",
            name="/auctions/:id (poll)",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"poll failed: {resp.status_code}")
