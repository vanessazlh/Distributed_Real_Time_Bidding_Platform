# Infrastructure

Terraform for deploying SurpriseAuction on AWS ECS Fargate (Learner Lab).

## Architecture

```
Internet → ALB (path-based routing)
              ├── /auctions*           → auction service  (8081, autoscaled)
              ├── /auctions/*/bids*    → bid service      (8084)
              ├── /auctions/*/payment* → payment service  (8085)
              ├── /users*              → user service     (8082)
              ├── /users/*/payments*   → payment service  (8085)
              ├── /shops*, /items*     → shop service     (8083)
              ├── /ws*, /notifications*→ notification svc (8080)
              └── /*                  → frontend          (3000)

All services → ElastiCache Redis (streams + auction state)
user/shop/payment/auction → DynamoDB (persistent store)
shop → S3 (image uploads)
```

## Prerequisites

- AWS Learner Lab session active (`LabRole` must exist)
- Terraform >= 1.5
- Docker + AWS CLI (for image push)

## Deploy

**1. Init and apply**
```bash
cd infra
terraform init
terraform apply -var="jwt_secret=<your-secret>"
```

**2. Push images** (repeat for each service)
```bash
aws ecr get-login-password | docker login --username AWS --password-stdin \
  $(terraform output -raw ecr_repos | python3 -c "import sys,json; r=json.load(sys.stdin); print(list(r.values())[0].split('/')[0])")

for svc in auction user shop bid payment notification frontend; do
  docker build -t rtb/$svc -f services/$svc/Dockerfile .
  docker tag rtb/$svc $(terraform output -json ecr_repos | python3 -c "import sys,json; print(json.load(sys.stdin)['$svc'])")
  docker push $(terraform output -json ecr_repos | python3 -c "import sys,json; print(json.load(sys.stdin)['$svc'])")
done
```

**3. Check**
```bash
curl $(terraform output -raw alb_url)/auctions
```

## Files

| File | What it creates |
|------|----------------|
| `main.tf` | AWS provider, `LabRole` data source |
| `variables.tf` | All input variables |
| `vpc.tf` | VPC, 2 public subnets, security groups |
| `ecr.tf` | ECR repository per service |
| `dynamodb.tf` | Users, Shops, Items, payments, Auctions tables |
| `elasticache.tf` | Redis `cache.t3.micro` |
| `s3.tf` | Public uploads bucket |
| `alb.tf` | ALB + path-based routing rules |
| `ecs.tf` | ECS cluster, task definitions, services |
| `autoscaling.tf` | Auction service autoscaling (Experiment 2) |
| `outputs.tf` | ALB URL, ECR repos, Redis address |

## Autoscaling (Experiment 2)

Only the auction service autoscales. Metric: `ALBRequestCountPerTarget`.

```
min tasks: 2  (var.auction_min_tasks)
max tasks: 10 (var.auction_max_tasks)
scale-out threshold: 3000 req/target/min (var.autoscale_rps_target)
scale-out cooldown: 60s
scale-in cooldown:  300s
```

**Calibrating the threshold before the experiment:**
1. Run Locust without autoscaling (set `auction_min_tasks = auction_max_tasks`)
2. Note the peak `RequestCountPerTarget` in CloudWatch
3. Set `autoscale_rps_target` to ~65% of that value and re-apply

## Notes

- **Learner Lab**: IAM resources are not created. All tasks use the pre-existing `LabRole`.
- **Local vs production**: Services detect environment via `DYNAMODB_ENDPOINT` and `S3_ENDPOINT`. Set → local DynamoDB/MinIO. Unset → real AWS services via task role credentials.
- **Auction service**: runs Redis-only in docker-compose; uses DynamoDB backing store in ECS (both are supported).
- **WebSocket**: ALB supports WebSocket natively — no extra config needed for the notification service.
