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

variable "auction_prewarm_enabled" {
  description = "Enable scheduled service-level prewarm actions for the auction ECS service"
  type        = bool
  default     = false
}

variable "auction_prewarm_min_tasks" {
  description = "Temporary auction service min capacity during the prewarm window"
  type        = number
  default     = 8
}

variable "auction_prewarm_scale_out_schedule" {
  description = "UTC/EventBridge schedule expression for prewarm scale-out (for example cron(55 21 * * ? *)); empty disables the action"
  type        = string
  default     = ""
}

variable "auction_prewarm_scale_in_schedule" {
  description = "UTC/EventBridge schedule expression for post-spike scale-in (for example cron(10 23 * * ? *)); empty disables the action"
  type        = string
  default     = ""
}

variable "auction_prewarm_timezone" {
  description = "Timezone for the scheduled prewarm actions (IANA name, for example UTC or America/New_York)"
  type        = string
  default     = "UTC"
}
