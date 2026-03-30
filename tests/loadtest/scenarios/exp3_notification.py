"""
Experiment 3 – Notification Fan-Out: WebSocket Push vs Polling Pull
====================================================================
Goal: Compare server-push (WebSocket) vs client-pull (polling) as the number
of connected clients scales to 1000.

Usage:

  # Push run (WebSocket):
  MODE=push RUN=1 AUCTION_ID=... BUYER_TOKEN=... \
    locust -f tests/loadtest/scenarios/exp3_notification.py \
           --headless -u 1000 -r 50 -t 180s \
           --host http://localhost:8081 \
           --csv tests/loadtest/results/exp3_push_run1

  # Pull run (Polling):
  MODE=pull RUN=1 AUCTION_ID=... BUYER_TOKEN=... \
    locust -f tests/loadtest/scenarios/exp3_notification.py \
           --headless -u 1000 -r 50 -t 180s \
           --host http://localhost:8081 \
           --csv tests/loadtest/results/exp3_pull_run1

How it works:
  - Push: 999 users open WebSocket connections to notification:8080, hold them open,
    and receive bid_placed events as they arrive. 1 user places 1 bid/second.
  - Pull: 999 users poll GET /auctions/:id every second on auction:8081.
    1 user places 1 bid/second.
  - Metrics saved from GET /metrics on notification service (push latency)
    and Locust CSV stats (RPS / latency on auction service for pull).
"""

import os
import json
import time
import threading
import urllib.request

import websocket
from locust import HttpUser, task, constant, events

# ── Configuration ─────────────────────────────────────────────────────────────
AUCTION_ID    = os.getenv("AUCTION_ID", "")
BUYER_TOKEN   = os.getenv("BUYER_TOKEN", "")
MODE          = os.getenv("MODE", "push")   # push | pull
RUN           = os.getenv("RUN", "1")

AUCTION_HOST  = os.getenv("AUCTION_HOST",  "http://localhost:8081")
NOTIF_HOST    = os.getenv("NOTIF_HOST",    "http://localhost:8080")
WS_HOST       = os.getenv("WS_HOST",       "ws://localhost:8080")
# ──────────────────────────────────────────────────────────────────────────────

_baseline = {}  # metrics snapshot at test start


@events.test_start.add_listener
def snapshot_baseline(environment, **kwargs):
    """Record notification metrics before the test begins so we can compute deltas."""
    try:
        with urllib.request.urlopen(f"{NOTIF_HOST}/metrics", timeout=5) as r:
            _baseline.update(json.loads(r.read()))
        print(f"[INFO] Baseline metrics: {_baseline}")
    except Exception as e:
        print(f"[WARN] Could not read baseline metrics: {e}")


@events.test_stop.add_listener
def save_metrics(environment, **kwargs):
    """Save notification service metrics (delta from baseline) at end of test."""
    try:
        with urllib.request.urlopen(f"{NOTIF_HOST}/metrics", timeout=5) as r:
            end = json.loads(r.read())

        delta_broadcasts = end.get("total_broadcasts", 0) - _baseline.get("total_broadcasts", 0)

        result = {
            "mode":                   MODE,
            "run":                    RUN,
            "timestamp":              time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "active_connections":     end.get("active_connections", 0),
            "total_broadcasts":       end.get("total_broadcasts", 0),
            "delta_broadcasts":       delta_broadcasts,
            "avg_delivery_latency_ms": end.get("avg_delivery_latency_ms", 0),
            "p99_delivery_latency_ms": end.get("p99_delivery_latency_ms", 0),
        }
        out = f"tests/loadtest/results/exp3_{MODE}_run{RUN}_metrics.json"
        with open(out, "w") as f:
            json.dump(result, f, indent=2)
        print(f"[INFO] Saved to {out}:")
        print(json.dumps(result, indent=2))
    except Exception as e:
        print(f"[WARN] Could not save metrics: {e}")


# ── WebSocket subscriber (push mode) ─────────────────────────────────────────

class WSSubscriber(HttpUser):
    """
    Opens a WebSocket connection to notification service and holds it for the
    duration of the test. Counts incoming bid_placed messages.
    """
    abstract = (MODE != "push")   # only active when MODE=push
    weight = 999
    wait_time = constant(60)      # task runs once; connection is kept by recv thread
    host = AUCTION_HOST

    def on_start(self):
        self._ws = None
        self._thread = None
        self._msg_count = 0
        if not AUCTION_ID:
            return

        ws_url = f"{WS_HOST}/auctions/{AUCTION_ID}/subscribe"
        t0 = time.time()
        try:
            ws = websocket.WebSocket()
            ws.connect(ws_url, timeout=5)
            elapsed_ms = (time.time() - t0) * 1000
            self._ws = ws

            # background thread receives messages without blocking gevent
            self._thread = threading.Thread(target=self._recv_loop, daemon=True)
            self._thread.start()

            events.request.fire(
                request_type="WS",
                name="connect /auctions/:id/subscribe",
                response_time=elapsed_ms,
                response_length=0,
                exception=None,
                context={},
            )
        except Exception as exc:
            events.request.fire(
                request_type="WS",
                name="connect /auctions/:id/subscribe",
                response_time=(time.time() - t0) * 1000,
                response_length=0,
                exception=exc,
                context={},
            )

    def _recv_loop(self):
        """Receive loop — runs in a daemon thread."""
        while self._ws and self._ws.connected:
            try:
                msg = self._ws.recv()
                if msg:
                    self._msg_count += 1
            except Exception:
                break

    def on_stop(self):
        if self._ws:
            try:
                self._ws.close()
            except Exception:
                pass

    @task
    def hold_connection(self):
        """Subscribers do nothing — the recv thread keeps the connection alive."""
        time.sleep(60)


# ── Polling subscriber (pull mode) ───────────────────────────────────────────

class PollingSubscriber(HttpUser):
    """
    Polls GET /auctions/:id every second — simulates clients who have no push
    channel and must ask the server for updates themselves.
    """
    abstract = (MODE != "pull")   # only active when MODE=pull
    weight = 999
    wait_time = constant(1)
    host = AUCTION_HOST

    def on_start(self):
        self.auction_id = AUCTION_ID
        self.headers = {"Authorization": f"Bearer {BUYER_TOKEN}"}

    @task
    def poll(self):
        if not self.auction_id:
            return
        with self.client.get(
            f"/auctions/{self.auction_id}",
            headers=self.headers,
            name="/auctions/:id [poll]",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                r.success()
            else:
                r.failure(f"poll {r.status_code}")


# ── Bidder (active in both modes) ────────────────────────────────────────────

class Bidder(HttpUser):
    """
    Places 1 bid every ~2 seconds — the event source that triggers notifications.
    Only 1 Bidder spawned (weight=1 vs 999 for subscribers).
    """
    abstract = False
    weight = 1
    wait_time = constant(2)
    host = AUCTION_HOST

    def on_start(self):
        self.auction_id = AUCTION_ID
        self.headers = {"Authorization": f"Bearer {BUYER_TOKEN}"}
        self.current_highest = self._get_price()

    def _get_price(self) -> int:
        try:
            with self.client.get(
                f"/auctions/{self.auction_id}",
                headers=self.headers,
                name="/auctions/:id [price]",
                catch_response=True,
            ) as r:
                if r.status_code == 200:
                    return int(r.json().get("current_highest_bid", 300))
                r.failure(f"price {r.status_code}")
        except Exception:
            pass
        return 300

    @task
    def place_bid(self):
        if not self.auction_id:
            return
        self.current_highest = self._get_price()
        amount = self.current_highest + 10
        with self.client.post(
            f"/auctions/{self.auction_id}/bid",
            json={"amount": amount},
            headers=self.headers,
            name="/auctions/:id/bid [bidder]",
            catch_response=True,
        ) as r:
            if r.status_code in (200, 201, 400, 409):
                r.success()
            else:
                r.failure(f"bid {r.status_code}: {r.text[:100]}")
