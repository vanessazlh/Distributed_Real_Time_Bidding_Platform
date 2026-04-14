resource "aws_alb" "main" {
  name               = "${var.app}-alb"
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id
}

locals {
  # service name → container port
  target_groups = {
    frontend     = 3000
    auction      = 8081
    user         = 8082
    shop         = 8083
    bid          = 8084
    payment      = 8085
    notification = 8080
  }
}

resource "aws_alb_target_group" "services" {
  for_each = local.target_groups

  name        = "${var.app}-${each.key}"
  port        = each.value
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"

  health_check {
    path                = "/"
    matcher             = "200-404" # services return 404 on / without auth — still alive
    healthy_threshold   = 2
    unhealthy_threshold = 3
    interval            = 30
    timeout             = 5
  }
}

resource "aws_alb_listener" "http" {
  load_balancer_arn = aws_alb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_alb_target_group.services["frontend"].arn
  }
}

# Path-based routing — more specific rules first (lower priority number = higher precedence)
#
# Tricky paths that need to come before their prefix siblings:
#   /auctions/*/payment  → payment service  (before /auctions*)
#   /auctions/*/bids     → bid service      (before /auctions*)
#   /users/*/payments    → payment service  (before /users*)
locals {
  listener_rules = [
    { priority = 10, path = "/auctions/*/payment*", target = "payment" },
    { priority = 11, path = "/users/*/payments*",   target = "payment" },
    { priority = 12, path = "/payments*",           target = "payment" },
    { priority = 20, path = "/auctions/*/bids*",    target = "bid" },
    { priority = 30, path = "/auctions*",           target = "auction" },
    { priority = 31, path = "/admin*",              target = "auction" },
    { priority = 40, path = "/users*",              target = "user" },
    { priority = 41, path = "/auth*",               target = "user" },
    { priority = 50, path = "/shops*",              target = "shop" },
    { priority = 51, path = "/sellers*",            target = "shop" },
    { priority = 52, path = "/items*",              target = "shop" },
    { priority = 53, path = "/uploads*",            target = "shop" },
    { priority = 60, path = "/ws*",                 target = "notification" },
    { priority = 61, path = "/notifications*",      target = "notification" },
  ]
}

resource "aws_alb_listener_rule" "routes" {
  count        = length(local.listener_rules)
  listener_arn = aws_alb_listener.http.arn
  priority     = local.listener_rules[count.index].priority

  action {
    type             = "forward"
    target_group_arn = aws_alb_target_group.services[local.listener_rules[count.index].target].arn
  }

  condition {
    path_pattern {
      values = [local.listener_rules[count.index].path]
    }
  }
}
