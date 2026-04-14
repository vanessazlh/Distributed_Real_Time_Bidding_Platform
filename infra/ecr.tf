locals {
  services = toset(["auction", "user", "shop", "bid", "payment", "notification", "frontend"])
}

resource "aws_ecr_repository" "services" {
  for_each             = local.services
  name                 = "${var.app}/${each.key}"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = false
  }
}
