"""
Experiment 3 — Notification Fan-Out (WS vs SSE vs pull baseline)
================================================================
This local harness supports both refined Experiment 3 variants:

  - 3A: WebSocket vs SSE for one-way auction updates
  - 3B: Push vs polling for architectural trade-offs

Usage:

  # 3A: WebSocket push
  MODE=ws RUN=1 AUCTION_ID=... BUYER_TOKEN=... \
    locust -f tests/loadtest_local/scenarios/exp3_notification.py \
           --headless -u 1000 -r 50 -t 180s \
           --host http://localhost:8081 \
           --csv tests/loadtest_local/results/exp3_ws_run1

  # 3A: SSE push
  MODE=sse RUN=1 AUCTION_ID=... BUYER_TOKEN=... \
    locust -f tests/loadtest_local/scenarios/exp3_notification.py \
           --headless -u 1000 -r 50 -t 180s \
           --host http://localhost:8081 \
           --csv tests/loadtest_local/results/exp3_sse_run1

  # 3B: Pull baseline
  MODE=pull RUN=1 AUCTION_ID=... BUYER_TOKEN=... \
    locust -f tests/loadtest_local/scenarios/exp3_notification.py \
           --headless -u 1000 -r 50 -t 180s \
           --host http://localhost:8081 \
           --csv tests/loadtest_local/results/exp3_pull_run1
"""

import datetime
import json
import os
import threading
import time
import urllib.request

import requests
import websocket
from locust import HttpUser, constant, events, task

# ── Configuration ─────────────────────────────────────────────────────────────
AUCTION_ID = os.getenv("AUCTION_ID", "")
BUYER_TOKEN = os.getenv("BUYER_TOKEN", "")
MODE = os.getenv("MODE", "ws").lower()
RUN = os.getenv("RUN", "1")

AUCTION_HOST = os.getenv("AUCTION_HOST", "http://localhost:8081")
NOTIF_HOST = os.getenv("NOTIF_HOST", "http://localhost:8080")
WS_HOST = os.getenv("WS_HOST", "ws://localhost:8080")

if MODE == "push":
    MODE = "ws"
if MODE == "poll":
    MODE = "pull"
if MODE not in ("ws", "sse", "pull"):
    raise SystemExit("ERROR: MODE must be one of ws, sse, or pull")

# ──────────────────────────────────────────────────────────────────────────────

_baseline = {}
_push_latency_samples = []
_push_latency_lock = threading.Lock()


def record_push_latency_from_payload(raw_payload):
    try:
        data = json.loads(raw_payload)
    except Exception:
        return
    bid_accepted_at = data.get("bid_accepted_at", "")
    if not bid_accepted_at:
        return
    try:
        accepted = datetime.datetime.fromisoformat(bid_accepted_at.replace("Z", "+00:00"))
        now = datetime.datetime.now(datetime.timezone.utc)
        latency_ms = (now - accepted).total_seconds() * 1000
    except Exception:
        return
    with _push_latency_lock:
        _push_latency_samples.append(latency_ms)


@events.test_start.add_listener
def snapshot_baseline(environment, **kwargs):
    """Record notification metrics before the test begins so we can compute deltas."""
    try:
        with urllib.request.urlopen(f"{NOTIF_HOST}/metrics", timeout=5) as response:
            _baseline.update(json.loads(response.read()))
        print(f"[INFO] Baseline metrics: {_baseline}")
    except Exception as exc:
        print(f"[WARN] Could not read baseline metrics: {exc}")


@events.test_stop.add_listener
def save_metrics(environment, **kwargs):
    """Save notification service and client-observed metrics at end of the test."""
    try:
        with urllib.request.urlopen(f"{NOTIF_HOST}/metrics", timeout=5) as response:
            end = json.loads(response.read())

        delta_broadcasts = end.get("total_broadcasts", 0) - _baseline.get("total_broadcasts", 0)
        avg_delivery = end.get("avg_delivery_latency_ms", 0)
        p99_delivery = end.get("p99_delivery_latency_ms", 0)

        with _push_latency_lock:
            samples = list(_push_latency_samples)

        client_avg = round(sum(samples) / len(samples), 1) if samples else 0
        sorted_samples = sorted(samples)
        if sorted_samples:
            p99_idx = max(0, int(len(sorted_samples) * 0.99) - 1)
            client_p99 = round(sorted_samples[p99_idx], 1)
        else:
            client_p99 = 0

        result = {
            "mode": MODE,
            "run": RUN,
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "active_connections": end.get("active_connections", 0),
            "total_broadcasts": end.get("total_broadcasts", 0),
            "delta_broadcasts": delta_broadcasts,
            "delivery_latency_comparable": MODE in ("ws", "sse"),
            "avg_delivery_latency_ms": avg_delivery if MODE in ("ws", "sse") else None,
            "p99_delivery_latency_ms": p99_delivery if MODE in ("ws", "sse") else None,
            "avg_internal_processing_ms": avg_delivery if MODE == "pull" else None,
            "p99_internal_processing_ms": p99_delivery if MODE == "pull" else None,
            "client_side_push_latency": {
                "samples": len(samples) if MODE in ("ws", "sse") else 0,
                "avg_ms": client_avg if MODE in ("ws", "sse") else None,
                "p99_ms": client_p99 if MODE in ("ws", "sse") else None,
            },
        }
        out = f"tests/loadtest_local/results/exp3_{MODE}_run{RUN}_metrics.json"
        with open(out, "w", encoding="utf-8") as output:
            json.dump(result, output, indent=2)
        print(f"[INFO] Saved to {out}:")
        print(json.dumps(result, indent=2))
    except Exception as exc:
        print(f"[WARN] Could not save metrics: {exc}")


# ── WebSocket subscriber (3A: push) ──────────────────────────────────────────

class WSSubscriber(HttpUser):
    """
    Opens a WebSocket connection to notification service and holds it for the
    duration of the test. Records client-observed latency from bid_accepted_at.
    """

    abstract = MODE != "ws"
    weight = 999
    wait_time = constant(60)
    host = AUCTION_HOST

    def on_start(self):
        self._ws = None
        self._thread = None
        if not AUCTION_ID:
            return

        ws_url = f"{WS_HOST}/auctions/{AUCTION_ID}/subscribe"
        start = time.time()
        try:
            ws = websocket.WebSocket()
            ws.connect(ws_url, timeout=5)
            self._ws = ws
            self._thread = threading.Thread(target=self._recv_loop, daemon=True)
            self._thread.start()
            events.request.fire(
                request_type="WS",
                name="connect /auctions/:id/subscribe",
                response_time=(time.time() - start) * 1000,
                response_length=0,
                exception=None,
                context={},
            )
        except Exception as exc:
            events.request.fire(
                request_type="WS",
                name="connect /auctions/:id/subscribe",
                response_time=(time.time() - start) * 1000,
                response_length=0,
                exception=exc,
                context={},
            )

    def _recv_loop(self):
        while self._ws and self._ws.connected:
            try:
                msg = self._ws.recv()
            except Exception:
                break
            if msg:
                record_push_latency_from_payload(msg)

    def on_stop(self):
        if self._ws:
            try:
                self._ws.close()
            except Exception:
                pass

    @task
    def hold_connection(self):
        time.sleep(60)


# ── SSE subscriber (3A: push) ────────────────────────────────────────────────

class SSESubscriber(HttpUser):
    """
    Opens an SSE stream to notification service and records client-observed
    latency from bid_accepted_at when bid_placed messages arrive.
    """

    abstract = MODE != "sse"
    weight = 999
    wait_time = constant(60)
    host = AUCTION_HOST

    def on_start(self):
        self._stop = threading.Event()
        self._session = requests.Session()
        self._response = None
        self._thread = None
        if not AUCTION_ID:
            return

        sse_url = f"{NOTIF_HOST}/auctions/{AUCTION_ID}/subscribe/sse"
        start = time.time()
        try:
            response = self._session.get(sse_url, stream=True, timeout=10)
            response.raise_for_status()
            self._response = response
            self._thread = threading.Thread(target=self._recv_loop, daemon=True)
            self._thread.start()
            events.request.fire(
                request_type="SSE",
                name="connect /auctions/:id/subscribe/sse",
                response_time=(time.time() - start) * 1000,
                response_length=0,
                exception=None,
                context={},
            )
        except Exception as exc:
            events.request.fire(
                request_type="SSE",
                name="connect /auctions/:id/subscribe/sse",
                response_time=(time.time() - start) * 1000,
                response_length=0,
                exception=exc,
                context={},
            )

    def _recv_loop(self):
        if not self._response:
            return
        try:
            for line in self._response.iter_lines(decode_unicode=True):
                if self._stop.is_set():
                    break
                if not line or not line.startswith("data: "):
                    continue
                record_push_latency_from_payload(line[6:])
        except Exception:
            pass

    def on_stop(self):
        self._stop.set()
        if self._response is not None:
            try:
                self._response.close()
            except Exception:
                pass
        try:
            self._session.close()
        except Exception:
            pass

    @task
    def hold_connection(self):
        time.sleep(60)


# ── Polling subscriber (3B: pull baseline) ───────────────────────────────────

class PollingSubscriber(HttpUser):
    """
    Polls GET /auctions/:id every second — simulates clients who have no push
    channel and must ask the server for updates themselves.
    """

    abstract = MODE != "pull"
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
        ) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(f"poll {response.status_code}")


# ── Bidder (active in all modes) ─────────────────────────────────────────────

class Bidder(HttpUser):
    """
    Places 1 bid every ~2 seconds — the event source that triggers notifications.
    Only 1 Bidder is effectively active (weight=1 vs 999 for subscribers).
    """

    abstract = False
    weight = 1
    wait_time = constant(2)
    host = AUCTION_HOST

    def on_start(self):
        self.auction_id = AUCTION_ID
        self.headers = {"Authorization": f"Bearer {BUYER_TOKEN}"}
        self.current_highest = self._get_price()

    def _get_price(self):
        try:
            with self.client.get(
                f"/auctions/{self.auction_id}",
                headers=self.headers,
                name="/auctions/:id [price]",
                catch_response=True,
            ) as response:
                if response.status_code == 200:
                    return int(response.json().get("current_highest_bid", 300))
                response.failure(f"price {response.status_code}")
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
        ) as response:
            if response.status_code in (200, 201, 400, 409):
                response.success()
            else:
                response.failure(f"bid {response.status_code}: {response.text[:100]}")
