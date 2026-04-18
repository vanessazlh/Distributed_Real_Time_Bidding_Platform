# Autoscaling applies only to the auction service — the one under experiment load.
# Other services stay at fixed desired_count = 1.

resource "aws_appautoscaling_target" "auction" {
  max_capacity       = var.auction_max_tasks
  min_capacity       = var.auction_min_tasks
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.services["auction"].name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "auction_rps" {
  name               = "${var.app}-auction-rps-tracking"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.auction.resource_id
  scalable_dimension = aws_appautoscaling_target.auction.scalable_dimension
  service_namespace  = aws_appautoscaling_target.auction.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ALBRequestCountPerTarget"
      # ALB + target group suffix required for ALBRequestCountPerTarget
      resource_label = "${aws_alb.main.arn_suffix}/${aws_alb_target_group.services["auction"].arn_suffix}"
    }

    # Tune this before Experiment 2:
    # 1. Run a dry-run Locust load without autoscaling
    # 2. Note the peak RequestCountPerTarget in CloudWatch
    # 3. Set target_value to ~60-70% of that peak
    target_value = var.autoscale_rps_target

    scale_out_cooldown = 60  # fast scale-out — this is what Experiment 2 measures
    scale_in_cooldown  = 300 # slow scale-in — avoid flapping during the experiment window
  }
}

locals {
  auction_prewarm_actions = var.auction_prewarm_enabled ? {
    scale_out = {
      schedule     = trimspace(var.auction_prewarm_scale_out_schedule)
      min_capacity = var.auction_prewarm_min_tasks
    }
    scale_in = {
      schedule     = trimspace(var.auction_prewarm_scale_in_schedule)
      min_capacity = var.auction_min_tasks
    }
  } : {}

  auction_prewarm_actions_enabled = {
    for name, action in local.auction_prewarm_actions : name => action
    if action.schedule != ""
  }
}

# Optional service-level prewarm for predictable high-traffic windows.
# This raises the auction service floor ahead of the spike, then returns it
# to the normal baseline afterward. Reactive target tracking remains enabled.
resource "aws_appautoscaling_scheduled_action" "auction_prewarm" {
  for_each = local.auction_prewarm_actions_enabled

  name               = "${var.app}-auction-prewarm-${each.key}"
  service_namespace  = aws_appautoscaling_target.auction.service_namespace
  resource_id        = aws_appautoscaling_target.auction.resource_id
  scalable_dimension = aws_appautoscaling_target.auction.scalable_dimension
  schedule           = each.value.schedule
  timezone           = var.auction_prewarm_timezone

  scalable_target_action {
    min_capacity = each.value.min_capacity
    max_capacity = var.auction_max_tasks
  }
}
