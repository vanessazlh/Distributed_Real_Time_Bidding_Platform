locals {
  ecr_base = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.aws_region}.amazonaws.com/${var.app}"
  redis_addr = "${aws_elasticache_cluster.redis.cache_nodes[0].address}:6379"
}

resource "aws_ecs_cluster" "main" {
  name = "${var.app}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

# ── CloudWatch log groups (one per service) ───────────────────────────────────

resource "aws_cloudwatch_log_group" "services" {
  for_each          = local.services
  name              = "/ecs/${var.app}/${each.key}"
  retention_in_days = 7
}

# ── Task definitions ──────────────────────────────────────────────────────────

resource "aws_ecs_task_definition" "auction" {
  family                   = "${var.app}-auction"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 512
  memory                   = 1024
  execution_role_arn       = data.aws_iam_role.lab.arn
  task_role_arn            = data.aws_iam_role.lab.arn

  container_definitions = jsonencode([{
    name      = "auction"
    image     = "${local.ecr_base}/auction:${var.image_tag}"
    essential = true
    portMappings = [{ containerPort = 8081 }]
    environment = [
      { name = "SERVER_ADDR",           value = ":8081" },
      { name = "REDIS_ADDR",            value = local.redis_addr },
      { name = "JWT_SECRET",            value = var.jwt_secret },
      { name = "CONCURRENCY_STRATEGY",  value = "pessimistic" },
      { name = "AWS_REGION",            value = var.aws_region },
      # DYNAMODB_ENDPOINT intentionally unset → SDK uses ECS task role + real DynamoDB
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = "/ecs/${var.app}/auction"
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])
}

resource "aws_ecs_task_definition" "user" {
  family                   = "${var.app}-user"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = data.aws_iam_role.lab.arn
  task_role_arn            = data.aws_iam_role.lab.arn

  container_definitions = jsonencode([{
    name      = "user"
    image     = "${local.ecr_base}/user:${var.image_tag}"
    essential = true
    portMappings = [{ containerPort = 8082 }]
    environment = [
      { name = "SERVER_ADDR",      value = ":8082" },
      { name = "JWT_SECRET",       value = var.jwt_secret },
      { name = "AWS_REGION",       value = var.aws_region },
      # Routes /users/:id/bids through ALB → bid service (priority 20 in alb.tf)
      { name = "BID_SERVICE_URL",  value = "http://${aws_alb.main.dns_name}" },
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = "/ecs/${var.app}/user"
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])
}

resource "aws_ecs_task_definition" "shop" {
  family                   = "${var.app}-shop"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = data.aws_iam_role.lab.arn
  task_role_arn            = data.aws_iam_role.lab.arn

  container_definitions = jsonencode([{
    name      = "shop"
    image     = "${local.ecr_base}/shop:${var.image_tag}"
    essential = true
    portMappings = [{ containerPort = 8083 }]
    environment = [
      { name = "SERVER_ADDR",   value = ":8083" },
      { name = "JWT_SECRET",    value = var.jwt_secret },
      { name = "AWS_REGION",    value = var.aws_region },
      { name = "S3_BUCKET",     value = aws_s3_bucket.uploads.bucket },
      { name = "S3_PUBLIC_URL", value = "http://${aws_alb.main.dns_name}/uploads" },
      # S3_ENDPOINT intentionally unset → real S3 with task role credentials
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = "/ecs/${var.app}/shop"
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])
}

resource "aws_ecs_task_definition" "bid" {
  family                   = "${var.app}-bid"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = data.aws_iam_role.lab.arn
  task_role_arn            = data.aws_iam_role.lab.arn

  container_definitions = jsonencode([{
    name      = "bid"
    image     = "${local.ecr_base}/bid:${var.image_tag}"
    essential = true
    portMappings = [{ containerPort = 8084 }]
    environment = [
      { name = "SERVER_ADDR", value = ":8084" },
      { name = "REDIS_ADDR",  value = local.redis_addr },
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = "/ecs/${var.app}/bid"
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])
}

resource "aws_ecs_task_definition" "payment" {
  family                   = "${var.app}-payment"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = data.aws_iam_role.lab.arn
  task_role_arn            = data.aws_iam_role.lab.arn

  container_definitions = jsonencode([{
    name      = "payment"
    image     = "${local.ecr_base}/payment:${var.image_tag}"
    essential = true
    portMappings = [{ containerPort = 8085 }]
    environment = [
      { name = "SERVER_ADDR", value = ":8085" },
      { name = "REDIS_ADDR",  value = local.redis_addr },
      { name = "AWS_REGION",  value = var.aws_region },
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = "/ecs/${var.app}/payment"
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])
}

resource "aws_ecs_task_definition" "notification" {
  family                   = "${var.app}-notification"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = data.aws_iam_role.lab.arn
  task_role_arn            = data.aws_iam_role.lab.arn

  container_definitions = jsonencode([{
    name      = "notification"
    image     = "${local.ecr_base}/notification:${var.image_tag}"
    essential = true
    portMappings = [{ containerPort = 8080 }]
    environment = [
      { name = "REDIS_ADDR", value = local.redis_addr },
      { name = "PORT",       value = "8080" },
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = "/ecs/${var.app}/notification"
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])
}

resource "aws_ecs_task_definition" "frontend" {
  family                   = "${var.app}-frontend"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = data.aws_iam_role.lab.arn
  task_role_arn            = data.aws_iam_role.lab.arn

  container_definitions = jsonencode([{
    name      = "frontend"
    image     = "${local.ecr_base}/frontend:${var.image_tag}"
    essential = true
    portMappings = [{ containerPort = 3000 }]
    environment = [
      { name = "VITE_API_URL", value = "http://${aws_alb.main.dns_name}" },
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = "/ecs/${var.app}/frontend"
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])
}

# ── ECS Services ──────────────────────────────────────────────────────────────

locals {
  service_definitions = {
    auction      = { task = aws_ecs_task_definition.auction.arn,      tg = "auction",      desired = var.auction_min_tasks }
    user         = { task = aws_ecs_task_definition.user.arn,         tg = "user",         desired = 1 }
    shop         = { task = aws_ecs_task_definition.shop.arn,         tg = "shop",         desired = 1 }
    bid          = { task = aws_ecs_task_definition.bid.arn,          tg = "bid",          desired = 1 }
    payment      = { task = aws_ecs_task_definition.payment.arn,      tg = "payment",      desired = 1 }
    notification = { task = aws_ecs_task_definition.notification.arn, tg = "notification", desired = 1 }
    frontend     = { task = aws_ecs_task_definition.frontend.arn,     tg = "frontend",     desired = 1 }
  }
}

resource "aws_ecs_service" "services" {
  for_each        = local.service_definitions
  name            = "${var.app}-${each.key}"
  cluster         = aws_ecs_cluster.main.id
  task_definition = each.value.task
  desired_count   = each.value.desired
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.public[*].id
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_alb_target_group.services[each.value.tg].arn
    container_name   = each.key
    container_port   = local.target_groups[each.key]
  }

  depends_on = [aws_alb_listener.http]

  lifecycle {
    # Prevent Terraform from resetting task count that autoscaling has changed
    ignore_changes = [desired_count]
  }
}
