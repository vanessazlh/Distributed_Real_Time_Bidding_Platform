output "alb_url" {
  description = "Application Load Balancer URL — set this as the base URL in Locust and the frontend"
  value       = "http://${aws_alb.main.dns_name}"
}

output "redis_addr" {
  description = "ElastiCache Redis address (used in REDIS_ADDR env var)"
  value       = local.redis_addr
}

output "ecr_repos" {
  description = "ECR repository URLs — use these in the image push script"
  value       = { for k, v in aws_ecr_repository.services : k => v.repository_url }
}

output "s3_bucket" {
  description = "S3 uploads bucket name"
  value       = aws_s3_bucket.uploads.bucket
}

output "ecs_cluster" {
  description = "ECS cluster name"
  value       = aws_ecs_cluster.main.name
}
