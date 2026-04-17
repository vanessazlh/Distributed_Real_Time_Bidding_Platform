# Experiment 2 Run 1 Report

## Scope

This report summarizes the first full AWS deployment and load-test run for **Experiment 2 — Horizontal Scaling During Auction Spikes**.

It covers:

- infrastructure deployment to AWS ECS Fargate
- seed data creation for the scaling scenario
- a short sanity run to validate the setup
- one full load-test run using the intended Experiment 2 parameters
- key findings observed during and after the run

## Environment

- AWS account: `573628268485`
- Region: `us-east-1`
- ALB: `http://rtb-alb-2035582644.us-east-1.elb.amazonaws.com`
- ECS cluster: `rtb-cluster`
- Auction autoscaling policy: `ALBRequestCountPerTarget` target tracking
- Auction service task range: min `2`, max `10`

## Deployment Summary

The stack was successfully deployed to AWS using Terraform and Docker/ECR:

1. `terraform apply` created the VPC, public subnets, ALB, target groups, DynamoDB tables, ECR repositories, CloudWatch log groups, ElastiCache Redis, ECS cluster, ECS services, and the auction autoscaling policy.
2. Docker images for `auction`, `user`, `shop`, `bid`, `payment`, `notification`, and `frontend` were built and pushed to ECR.
3. ECS services recovered automatically once images became available and reached steady state.

At the end of deployment:

- `auction` was healthy with `2` running tasks
- `user`, `shop`, `bid`, `payment`, and `notification` were healthy with `1` running task each
- the ALB endpoint responded successfully to `GET /auctions`

## Seed Data Preparation

To prepare the Experiment 2 scenario, the deployed stack was seeded through the public ALB:

- `1` buyer account
- `1` seller account
- `1` shop
- `50` items
- `50` auctions

The seeded shop was:

- `shop_id = f7b181f8-a0b3-432b-a94e-efde729f3ff2`

The seeded buyer used for the load test was:

- `exp2buyer_1776415823@test.com`

The `AUCTION_IDS` environment variable was populated with all `50` created auction IDs and reused directly by the Locust scenario.

## Sanity Run

Before the full run, a smaller validation run was executed to confirm that:

- the ALB routing was correct
- the Locust scenario could authenticate and hit the correct endpoints
- the seeded auction pool was usable
- the system stayed stable under light concurrent load

### Sanity parameters

- users: `200`
- spawn rate: `20/s`
- duration: `60s`

### Sanity result

- total requests: `18,980`
- failures: `0`
- aggregated average latency: `87 ms`
- aggregated median latency: `84 ms`
- aggregated p95 latency: `96 ms`
- aggregated p99 latency: `190 ms`

This confirmed that the deployed stack and the test script were working correctly before scaling up.

## Full Experiment 2 Run

### Target parameters

The full run used the intended Experiment 2 settings from `loadtest/scenarios/exp2_scaling_spike.py`:

- users: `5000`
- spawn rate: `83/s`
- duration: `300s`
- host: ALB URL above
- auction pool: `50` auctions

This matches the intended workload of roughly `100` bidders per auction during a spike.

### Final Locust results

From `loadtest/results/exp2_run1_stats.csv`:

- total requests: `598,786`
- total failures: `1`
- overall failure rate: `0.00017%`
- aggregated median latency: `1700 ms`
- aggregated average latency: `1710 ms`
- aggregated p95 latency: `2300 ms`
- aggregated p99 latency: `4000 ms`
- worst observed latency: `46,142 ms`
- achieved throughput: `1848 req/s`

Per endpoint:

- `GET /auctions/:id`
  - requests: `300,008`
  - failures: `1`
  - average latency: `1858 ms`
  - p99 latency: `4100 ms`
- `POST /auctions/:id/bid`
  - requests: `298,778`
  - failures: `0`
  - average latency: `1561 ms`
  - p99 latency: `3900 ms`

From `loadtest/results/exp2_run1_failures.csv`:

- the only recorded failure was a single `GET /auctions/:id` request with `GET auction failed: 0`

### Locust exit code

The full Locust run exited with code `1`, but this was **not caused by server-side instability**.

Locust reported:

- local CPU usage was too high during the run

So the non-zero exit code reflects a **client-side load-generator limitation on the MacBook**, not an application crash or widespread request failure.

## Autoscaling Behavior

Autoscaling did trigger successfully.

Observed ECS timeline from `loadtest/results/exp2_run1_ecs.jsonl`:

- `08:52:35Z` full run started
- until `08:56:00Z`, auction service stayed at `desired=2`, `running=2`
- `08:56:05Z`, desired count changed from `2` to `10`
- `08:56:11Z`, ECS reported `pending=8`
- `08:56:41Z`, `running=5`, `pending=5`
- `08:56:47Z`, auction service reached steady state with `running=10`

AWS Application Auto Scaling activity confirmed the trigger:

- scale-out activity start: `2026-04-17T01:56:02.862-07:00`
- action: set desired count to `10`
- status: `Successful`
- cause: target tracking alarm for policy `rtb-auction-rps-tracking`

### Interpretation

The policy wiring is correct and functional, but the trigger happened **late** relative to the start of the spike:

- scale-out trigger occurred about `3m 27s` after the full run started
- the service reached `10` running tasks about `4m 12s` after the run started

Because the test lasted `5` minutes total, the scale-out arrived only in the final portion of the run.

## Key Findings

### 1. Autoscaling works, but reacts too late for this workload

The system did scale from `2` to `10` auction tasks, proving that the `ALBRequestCountPerTarget` policy is wired correctly.

However, the trigger occurred late enough that most of the test ran under elevated latency before the additional capacity arrived.

### 2. Reliability stayed strong despite the latency increase

Even under the full `5000`-user run:

- only `1` request failed out of nearly `600k`
- bid writes had `0` Locust-recorded failures
- no consistency violations were observed

So the system remained functionally correct and highly available during the spike.

### 3. Latency degraded heavily before scale-out completed

Although failure rate stayed near zero, latency increased substantially:

- median latency rose into the `1.6s–1.9s` range
- p95 rose to about `2.3s`
- p99 rose to about `4.0s`
- tail latency reached `46s`

This suggests the current threshold is conservative enough to protect correctness, but not aggressive enough to protect responsiveness during a sudden spike.

### 4. `/admin/metrics` is not a reliable global metric source behind the ALB

During the full run, the values captured in `exp2_run1_metrics.jsonl` jumped between much smaller and much larger totals.

This is consistent with the current `/admin/metrics` endpoint being **instance-local in memory**, while requests are distributed across multiple auction tasks by the ALB.

As a result:

- `/admin/metrics` snapshots from the ALB are useful for spot checks
- but they do **not** represent a clean globally aggregated total once the service scales horizontally

### 5. A seed-time validation bug was exposed in the shop service

During seed creation, `POST /shops` initially returned:

- `HTTP 500 {"error":"internal error"}`

The root cause was not AWS infrastructure; it was application behavior:

- `CreateShop` currently requires `lat` and `lng`
- if they are omitted, the service returns a validation error internally
- the handler maps that error to `500` instead of a client-side validation response

After providing coordinates explicitly, seed creation succeeded and the full test proceeded normally.

## Overall Conclusion

The deployment and Experiment 2 load test were both successfully executed on AWS.

The run demonstrates that:

- the ECS/Fargate deployment works end-to-end
- the auction service autoscaling policy is active and functional
- the system maintains very high request success under heavy spike load
- the main weakness is **latency during the pre-scale window**, not correctness or availability

In short: the current autoscaling setup is **correct but slow to react** for this spike profile.

## Recommended Next Steps

1. Lower `autoscale_rps_target` and rerun the same scenario to see whether earlier scale-out reduces the latency spike.
2. Capture CloudWatch `RequestCountPerTarget`, `TargetResponseTime`, and task-count charts alongside this run for the final report.
3. Fix the shop service validation path so missing `lat/lng` returns a `4xx` instead of `500`.
4. Replace instance-local auction metrics with a centralized or scrapeable aggregate if cross-task experiment metrics are needed.
