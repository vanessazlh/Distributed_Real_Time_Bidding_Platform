# Experiment 2 AWS Load Test Report

## Scope

This report summarizes the AWS deployment and the completed load-test runs for **Experiment 2 — Horizontal Scaling During Auction Spikes**.

It covers:

- infrastructure deployment to AWS ECS Fargate
- seed data creation for the scaling scenario
- a short sanity run to validate the setup
- the original full load-test run using the intended Experiment 2 parameters
- a distributed Locust rerun to remove the client-side bottleneck
- a tuned autoscaling rerun using a higher baseline and lower target
- key findings observed across all runs

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

The full run used the intended Experiment 2 settings from `loadtest_aws/scenarios/exp2_scaling_spike.py`:

- users: `5000`
- spawn rate: `83/s`
- duration: `300s`
- host: ALB URL above
- auction pool: `50` auctions

This matches the intended workload of roughly `100` bidders per auction during a spike.

### Final Locust results

From `loadtest_aws/results/exp2_run1_stats.csv`:

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

From `loadtest_aws/results/exp2_run1_failures.csv`:

- the only recorded failure was a single `GET /auctions/:id` request with `GET auction failed: 0`

### Locust exit code

The full Locust run exited with code `1`, but this was **not caused by server-side instability**.

Locust reported:

- local CPU usage was too high during the run

So the non-zero exit code reflects a **client-side load-generator limitation on the MacBook**, not an application crash or widespread request failure.

## Autoscaling Behavior

Autoscaling did trigger successfully.

Observed ECS timeline from `loadtest_aws/results/exp2_run1_ecs.jsonl`:

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

During the full run, the values captured in `loadtest_aws/results/exp2_run1_metrics.jsonl` jumped between much smaller and much larger totals.

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

## Run 2: Distributed Locust Baseline

After Run 1, the next question was whether the poor latency was partly caused by the single-machine Locust process rather than the auction service itself.

To isolate that factor, the same workload was rerun with Locust in multi-process mode:

- command shape: `locust --processes 4`
- autoscaling config unchanged from Run 1
- auction service baseline unchanged at min `2`, max `10`, target `3000`

### Final Locust results

From `loadtest_aws/exp2_run2_dist_stats.csv`:

- total requests: `1,160,611`
- total failures: `0`
- aggregated median latency: `450 ms`
- aggregated average latency: `530 ms`
- aggregated p95 latency: `1200 ms`
- aggregated p99 latency: `1600 ms`
- worst observed latency: `13,565 ms`
- achieved throughput: `4834 req/s`

Per endpoint:

- `GET /auctions/:id`
  - requests: `580,244`
  - failures: `0`
  - average latency: `400 ms`
  - p99 latency: `1100 ms`
- `POST /auctions/:id/bid`
  - requests: `580,367`
  - failures: `0`
  - average latency: `660 ms`
  - p99 latency: `1800 ms`

### Autoscaling behavior

Observed ECS timeline from `loadtest_aws/exp2_run2_dist_ecs.jsonl` and AWS scaling activities:

- full Locust process started at `22:52:08Z`
- all `5000` users were spawned and stats reset at `22:53:09Z`
- auction service stayed at `desired=2`, `running=2` for the measured run
- `22:57:02Z`: target tracking triggered scale-out to `desired=10`
- `22:57:09Z`: the test finished before new capacity materially helped the measured window

### Interpretation

This rerun showed that Run 1 was **partly distorted by the load generator**:

- throughput increased from about `1848 req/s` to about `4834 req/s`
- median latency dropped from about `1700 ms` to `450 ms`
- Locust finished with exit code `0`

However, it also confirmed the core infrastructure finding from Run 1:

- the autoscaling policy still reacted **too late**
- the scale-out trigger happened only in the final seconds of the run

## Run 3: Tuned Autoscaling Rerun

Because Run 2 removed the client-side bottleneck but still showed late scale-out, the next step was a limited tuning pass on the live autoscaling policy.

The following overrides were applied with Terraform before rerunning the same distributed Locust workload:

- auction min tasks: `2 -> 4`
- `ALBRequestCountPerTarget` target value: `3000 -> 2000`
- max tasks remained `10`
- cooldowns remained `scale_out=60s`, `scale_in=300s`

### Final Locust results

From `loadtest_aws/exp2_run3_tuned_dist_stats.csv`:

- total requests: `1,785,381`
- total failures: `0`
- aggregated median latency: `140 ms`
- aggregated average latency: `167 ms`
- aggregated p95 latency: `300 ms`
- aggregated p99 latency: `720 ms`
- worst observed latency: `25,225 ms`
- achieved throughput: `7441 req/s`

Per endpoint:

- `GET /auctions/:id`
  - requests: `892,531`
  - failures: `0`
  - average latency: `163 ms`
  - p99 latency: `640 ms`
- `POST /auctions/:id/bid`
  - requests: `892,850`
  - failures: `0`
  - average latency: `170 ms`
  - p99 latency: `790 ms`

### Autoscaling behavior

Observed ECS timeline from `loadtest_aws/exp2_run3_tuned_dist_ecs.jsonl` and AWS scaling activities:

- Locust process started at `23:03:24Z`
- all `5000` users were spawned and stats reset at `23:04:25Z`
- auction service stayed at `desired=4`, `running=4` throughout the measured run
- `23:08:25Z`: target tracking triggered scale-out to `desired=10`
- `23:08:32Z`: ECS started launching `6` additional tasks

### Interpretation

The tuning worked **as performance tuning**, but not yet as **responsiveness tuning**:

- throughput improved again, from about `4834 req/s` to about `7441 req/s`
- median latency dropped again, from `450 ms` to `140 ms`
- p99 latency dropped from `1600 ms` to `720 ms`
- request success remained effectively perfect

But the scale-out still arrived at the **very end** of the run, which means:

- raising the baseline from `2` to `4` helped absorb the spike
- lowering the target from `3000` to `2000` did not make target tracking react early enough for a short five-minute surge

## Run 4: Scheduled Prewarm + Reactive Fallback

Because Run 3 showed that `min=4` and `target=2000` improved performance but still left
target tracking too slow for a short spike, the next step was to test the full
three-layer strategy:

- baseline scaling: keep auction min tasks at `4`
- scheduled prewarm: temporarily raise the auction service floor to `8`
- reactive scaling: keep `ALBRequestCountPerTarget` target tracking enabled

This run used one-off AWS Application Auto Scaling scheduled actions:

- `23:59:29Z`: set auction service `min_capacity` to `8`
- `00:12:29Z`: restore auction service `min_capacity` to `4`

Before the measured load window began, ECS reached:

- `desired=8`
- `running=8`

### Final Locust results

From `loadtest_aws/exp2_run4_prewarm_dist_stats.csv`:

- total requests: `1,862,754`
- total failures: `0`
- aggregated median latency: `120 ms`
- aggregated average latency: `140 ms`
- aggregated p95 latency: `230 ms`
- aggregated p99 latency: `500 ms`
- worst observed latency: `37,129 ms`
- achieved throughput: `7765 req/s`

Per endpoint:

- `GET /auctions/:id`
  - requests: `930,991`
  - failures: `0`
  - average latency: `137 ms`
  - p99 latency: `480 ms`
- `POST /auctions/:id/bid`
  - requests: `931,763`
  - failures: `0`
  - average latency: `142 ms`
  - p99 latency: `530 ms`

### Autoscaling behavior

Observed ECS timeline from `loadtest_aws/exp2_run4_prewarm_dist_ecs.jsonl` and AWS scaling activities:

- `23:59:30Z`: scheduled prewarm scale-out action triggered
- `00:00:36Z`: ECS completed the move to `desired=8`, `running=8`
- `00:01:57Z`: all `5000` Locust users were spawned and stats were reset
- throughout the measured five-minute run, auction service stayed at `desired=8`, `running=8`
- `00:06:25Z`: target tracking still triggered a reactive scale-out to `desired=10`
- `00:13:11Z`: scheduled prewarm scale-in action triggered and restored `min_capacity` to `4`

### Interpretation

This is the first run where the full layered strategy was exercised end-to-end.

Compared with Run 3:

- throughput improved from about `7441 req/s` to about `7765 req/s`
- median latency improved from `140 ms` to `120 ms`
- p99 latency improved from `720 ms` to `500 ms`
- request success remained perfect (`0` failures)

Most importantly, the measured spike no longer depended on late target tracking to
save the run:

- the service entered the spike already prewarmed at `8` tasks
- the full run completed while ECS remained stable at `8/8`
- the reactive signal still fired later, but only as a fallback after the main run

One operational nuance is worth noting:

- the scheduled scale-in action successfully restored the scalable target floor from `8` back to `4`
- however, that action restored `min_capacity`, not the already-elevated `desiredCount`
- because a reactive high alarm had already pushed `desiredCount` to `10` after the run, ECS did not immediately drop back to `4`

That behavior is acceptable for this experiment, but it means scheduled prewarm is
best understood as controlling the **floor** of the service rather than forcing an
instant post-run shrink.

## Cross-Run Summary

Across the four runs:

- `Run 1` proved the AWS deployment and policy wiring worked, but it mixed server behavior with a Locust CPU bottleneck
- `Run 2` removed the client bottleneck and confirmed the main remaining problem was **late autoscaling**
- `Run 3` showed that a higher baseline plus a lower target can dramatically improve latency and throughput even before horizontal scale-out happens
- `Run 4` showed that scheduled prewarm solves the short-spike timing problem much more effectively than reactive target tracking alone

The strongest current configuration for this workload is therefore:

- keep Terraform defaults conservative for initial bring-up, but use the tuned `4`-task baseline for spike experiments
- use scheduled prewarm for known spike windows
- keep `ALBRequestCountPerTarget` target tracking as the fallback layer, not the primary protection for a five-minute surge

## Overall Conclusion

The deployment and all follow-up Experiment 2 runs were successfully executed on AWS.

The combined evidence now shows that:

- the ECS/Fargate deployment works end-to-end
- the auction service autoscaling policy is correctly wired and does trigger
- system correctness and availability remain strong under heavy spike load
- the best results came from combining a stronger baseline with scheduled prewarm and reactive fallback

Repository note:

- Terraform variable defaults are still the conservative `2` / `3000`
- the validated Experiment 2 recommendation is `4` / `2000` plus optional prewarm to `8`

In short:

- distributed Locust removed the client-side distortion from Run 1
- tuning the baseline to `4` and the target to `2000` materially improved observed performance
- scheduled prewarm to `8` before the spike improved performance again and avoided dependence on late reactive scale-out
- for this short, predictable spike profile, the most effective strategy is:
  baseline + scheduled prewarm + reactive target tracking fallback

## Recommended Next Steps

1. Keep the stronger baseline (`4` tasks) for future spike runs, since it materially improves latency even before scale-out.
2. Keep scheduled prewarm in the experiment design for known load windows, since it performed better than target tracking alone on a five-minute surge.
3. Capture CloudWatch charts for `RequestCountPerTarget`, `TargetResponseTime`, scalable target min capacity, and task count so the final report can show the interaction between prewarm and reactive scaling.
4. If post-spike cost recovery becomes important, consider an additional mechanism that more aggressively reduces `desiredCount` after the run instead of only restoring `min_capacity`.
5. Fix the shop service validation path so missing `lat/lng` returns a `4xx` instead of `500`.
6. Replace instance-local auction metrics with a centralized or scrapeable aggregate if cross-task experiment metrics are needed.
