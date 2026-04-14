variable "aws_region" {
  description = "AWS region"
  default     = "us-east-1"
}

variable "app" {
  description = "Name prefix for all resources"
  default     = "rtb"
}

variable "image_tag" {
  description = "Docker image tag to deploy (set to git SHA in CI)"
  default     = "latest"
}

variable "jwt_secret" {
  description = "JWT signing secret"
  sensitive   = true
}

# Auction service autoscaling
variable "auction_min_tasks" {
  description = "Minimum number of auction service tasks (Experiment 2 starts here)"
  default     = 2
}

variable "auction_max_tasks" {
  description = "Maximum number of auction service tasks"
  default     = 10
}

variable "autoscale_rps_target" {
  description = "ALB RequestCountPerTarget per minute before scaling out auction service"
  default     = 3000
}
