variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "hash_key" {
  type    = string
  default = "id"
}
variable "billing_mode" {
  type    = string
  default = "PAY_PER_REQUEST"
}
variable "point_in_time_recovery" {
  type    = bool
  default = true
}

resource "aws_dynamodb_table" "this" {
  name           = var.name
  billing_mode   = var.billing_mode
  hash_key       = var.hash_key
  read_capacity  = var.billing_mode == "PROVISIONED" ? 5 : null
  write_capacity = var.billing_mode == "PROVISIONED" ? 5 : null
  tags           = merge(var.tags, { Name = var.name })

  attribute {
    name = var.hash_key
    type = "S"
  }

  point_in_time_recovery {
    enabled = var.point_in_time_recovery
  }

  server_side_encryption {
    enabled = true
  }
}

output "table_arn" { value = aws_dynamodb_table.this.arn }
output "table_name" { value = aws_dynamodb_table.this.name }
