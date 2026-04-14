# PAY_PER_REQUEST avoids capacity planning — fine for variable experiment load
resource "aws_dynamodb_table" "users" {
  name         = "Users"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "user_id"

  attribute { name = "user_id" type = "S" }
  attribute { name = "email"   type = "S" }

  global_secondary_index {
    name            = "email-index"
    hash_key        = "email"
    projection_type = "ALL"
  }
}

resource "aws_dynamodb_table" "shops" {
  name         = "Shops"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "shop_id"

  attribute { name = "shop_id"  type = "S" }
  attribute { name = "owner_id" type = "S" }

  global_secondary_index {
    name            = "owner_id-index"
    hash_key        = "owner_id"
    projection_type = "ALL"
  }
}

resource "aws_dynamodb_table" "items" {
  name         = "Items"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "item_id"

  attribute { name = "item_id" type = "S" }
  attribute { name = "shop_id" type = "S" }

  global_secondary_index {
    name            = "shop_id-index"
    hash_key        = "shop_id"
    projection_type = "ALL"
  }
}

resource "aws_dynamodb_table" "payments" {
  name         = "payments"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "payment_id"

  attribute { name = "payment_id" type = "S" }
  attribute { name = "auction_id" type = "S" }
  attribute { name = "user_id"    type = "S" }
  attribute { name = "created_at" type = "S" }

  global_secondary_index {
    name            = "auction-index"
    hash_key        = "auction_id"
    projection_type = "ALL"
  }

  global_secondary_index {
    name            = "user-index"
    hash_key        = "user_id"
    range_key       = "created_at"
    projection_type = "ALL"
  }
}

# Auctions table — not in init_tables.go, created by Terraform for production
resource "aws_dynamodb_table" "auctions" {
  name         = "Auctions"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "auction_id"

  attribute { name = "auction_id" type = "S" }
  attribute { name = "shop_id"    type = "S" }

  global_secondary_index {
    name            = "shop_id-index"
    hash_key        = "shop_id"
    projection_type = "ALL"
  }
}
