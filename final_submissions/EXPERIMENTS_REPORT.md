# Experiments Report

This report summarizes the three main experiments used to evaluate the system.

## Scope

We studied three system questions:

1. How to keep bidding correct under heavy contention
2. How to scale under short burst traffic on AWS
3. How to deliver real-time updates efficiently to many watchers

The experiment code is in:

- `tests/loadtest_local/scenarios/exp1_bid_contention.py`
- `loadtest_aws/scenarios/exp2_scaling_spike.py`
- `tests/loadtest_local/scenarios/exp3_notification.py`

The main result files are in:

- `tests/loadtest_local/results/`
- `loadtest_aws/`
- `loadtest_aws/results/`

## Experiment 1: Bid Contention

### Purpose

This experiment studied the hot path of bidding.

We compared three concurrency strategies:

- `optimistic`
- `pessimistic`
- `queue`

The tradeoff was between throughput, latency, and correctness under heavy contention.

### Setup

- `500` concurrent users
- one auction
- `60` seconds per run
- `3` runs per strategy
- local Docker Compose environment

Metrics:

- successful bid rate
- average latency
- p99 latency
- correctness of bid ordering

### Results

| Strategy | Success rate | Avg latency | p99 latency |
| --- | --- | --- | --- |
| `optimistic` | `19.5%` | `1.15 ms` | `12.58 ms` |
| `pessimistic` | `29.2%` | `0.54 ms` | `1.16 ms` |
| `queue` | `22.3%` | `0.68 ms` | `2.64 ms` |

### Analysis

The strongest result was that `pessimistic` led on both key metrics.

It had the highest success rate and the lowest tail latency.
`optimistic` stayed correct, but its retry logic created a clear tail-latency penalty under conflict.
`queue` was stable, but full serialization limited throughput.

The main conclusion is that for hot auctions, `pessimistic` gave the best overall tradeoff.
It protected the critical update path without the retry storm of `optimistic` or the throughput bottleneck of `queue`.

### Limitations

This experiment was run locally, not on ECS.
It focused on one very hot auction, not a mix of low and high contention auctions.
It measured application behavior under load, but not failure recovery across multiple auction replicas.

## Experiment 2: Horizontal Scaling During Auction Spikes

### Purpose

This experiment tested whether the AWS deployment could absorb short spike traffic through horizontal scaling.

The tradeoff was between cost efficiency and responsiveness.
A low baseline saves resources, but may react too slowly when many auctions spike at once.

### Setup

- AWS ECS Fargate with ALB
- `50` auctions started together
- `100` bidders per auction
- about `5000` users in distributed Locust
- auction service scaled between `2` and `10` tasks across runs

We compared several operating points and focused on the final layered strategy:

- baseline capacity of `4` tasks
- scheduled pre-warm to `8` tasks before the spike
- reactive fallback with `ALBRequestCountPerTarget`

Metrics:

- autoscaling response time
- throughput
- median and tail latency
- request failures

### Results

Best final run:

- `1,862,754` total requests
- `0` failures
- `7765 req/s`
- `120 ms` median latency
- `500 ms` p99 latency

Key cross-run finding:

- reactive target tracking alone was too slow for a short spike
- a stronger baseline improved performance
- scheduled pre-warm improved it again and removed dependence on late scale-out during the main test window

### Analysis

The most important insight was about timing.
The problem was not that ECS could not scale.
The problem was that purely reactive scaling started too late for a short predictable surge.

The final layered strategy worked better because the service entered the spike with enough capacity already running.
Reactive target tracking still had value, but as a fallback layer rather than the primary protection.

The main conclusion is that short auction spikes need layered scaling:

- baseline capacity for normal load
- pre-warm for predictable spike windows
- reactive scaling for unexpected bursts

### Limitations

This experiment focused only on the `Auction Service`, because it was the main bottleneck on the bidding hot path.
Other services were not scaled in the same way.
The measured workload was a short spike profile, so the result should not be generalized to long steady-state traffic without more testing.
One operational nuance is that scheduled pre-warm controlled the service floor, not an instant drop in desired task count after the run.

## Experiment 3: Notification Fan-Out

### Purpose

This experiment studied how to deliver real-time updates to many watchers efficiently.

The original question was split into two parts:

- `WebSocket` vs `SSE` as push transports
- push vs polling as an architectural tradeoff

The tradeoff was between delivery efficiency, connection behavior, and read amplification.

### Setup

- about `1000` clients
- `999` subscribers and `1` bidder
- one hot auction
- `180` seconds per run
- `3` runs per mode
- local Docker Compose environment

Modes:

- `ws`
- `sse`
- `pull`

Metrics:

- connection setup success and latency
- server-side fan-out latency
- client-observed push latency
- extra HTTP load under polling

### Results

#### Push transport comparison

| Mode | Connection result | Server-side fan-out | Client-observed latency |
| --- | --- | --- | --- |
| `WebSocket` | `999/999` connected in every run | avg `13.5 ms`, p99 `51.0 ms` | avg `47.2 ms`, p99 `107.9 ms` |
| `SSE` | `999/999` connected in every run | avg `14.8 ms`, p99 `62.5 ms` | avg `27.4 ms`, p99 `92.5 ms` |

#### Push vs polling

| Mode | Main observation |
| --- | --- |
| `push` | long-lived connections, low ongoing read cost |
| `pull` | about `163,800` extra poll requests per run, avg poll latency `27.7 ms`, p99 `96 ms` |

### Analysis

Two findings stood out.

First, both `WebSocket` and `SSE` were viable push transports.
`WebSocket` connected faster and had lower server-side fan-out latency.
`SSE` showed lower average client-observed latency, but one run had a weaker connection tail.

Second, polling was clearly less efficient than push.
Its main cost was not delivery quality alone, but the large amount of extra read traffic it created.

The main conclusion is that the right architectural choice for this workload is push delivery.
Between push transports, both options were workable, with `WebSocket` remaining the default choice.

### Limitations

This experiment was run locally, not on AWS.
The comparison focused on one-way auction updates.
Polling was evaluated as a baseline architecture, not as a true peer for push delivery latency.
In `pull` mode, notification-side timing reflected internal processing overhead only.

## Overall Conclusions

Across all three experiments, the evidence supports three broad conclusions.

1. Correctness under heavy contention depends on the concurrency strategy.
   `pessimistic` was the best choice for hot auctions.

2. Short burst traffic cannot rely on reactive autoscaling alone.
   The strongest result came from layered scaling with baseline capacity, scheduled pre-warm, and reactive fallback.

3. Real-time auctions benefit from push delivery.
   Both `WebSocket` and `SSE` worked at fan-out scale, while polling created large unnecessary read traffic.

Overall, the system behaved correctly under contention, scaled successfully on AWS, and supported large real-time fan-out. The main lesson is that correctness, scalability, and user experience must be designed together.
