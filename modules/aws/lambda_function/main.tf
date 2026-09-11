variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "runtime" {
  type    = string
  default = "python3.12"
}
variable "handler" {
  type    = string
  default = "app.handler"
}
variable "memory_mb" {
  type    = number
  default = 256
}
variable "timeout_s" {
  type    = number
  default = 30
}
variable "s3_bucket_arns" {
  type    = list(string)
  default = []
}
variable "security_group_ids" {
  type    = list(string)
  default = []
}
variable "subnet_id" {
  type        = string
  description = "Set when the function is placed in a subnet; empty means no VPC attachment."
  default     = ""
}
variable "vpc_id" {
  type    = string
  default = ""
}

locals {
  in_vpc        = var.subnet_id != ""
  function_name = replace(var.name, "/[^A-Za-z0-9-_]/", "-")
}

# Placeholder code so the function exists; replace via your own deploy step.
data "archive_file" "placeholder" {
  type        = "zip"
  output_path = "${path.module}/.placeholder-${local.function_name}.zip"
  source {
    filename = "app.py"
    content  = "def handler(event, context):\n    return {\"statusCode\": 200, \"body\": \"placeholder from iagram\"}\n"
  }
}

data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "this" {
  name               = "${local.function_name}-role"
  assume_role_policy = data.aws_iam_policy_document.assume.json
  tags               = var.tags
}

resource "aws_iam_role_policy_attachment" "basic" {
  role       = aws_iam_role.this.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy_attachment" "vpc" {
  count      = local.in_vpc ? 1 : 0
  role       = aws_iam_role.this.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"
}

data "aws_iam_policy_document" "s3" {
  count = length(var.s3_bucket_arns) > 0 ? 1 : 0
  statement {
    actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"]
    resources = concat(var.s3_bucket_arns, [for a in var.s3_bucket_arns : "${a}/*"])
  }
}

resource "aws_iam_role_policy" "s3" {
  count  = length(var.s3_bucket_arns) > 0 ? 1 : 0
  name   = "s3-access"
  role   = aws_iam_role.this.id
  policy = data.aws_iam_policy_document.s3[0].json
}

resource "aws_security_group" "fn" {
  count       = local.in_vpc ? 1 : 0
  name        = "${local.function_name}-fn"
  description = "Identity group for ${var.name} (managed by iagram)"
  vpc_id      = var.vpc_id
  tags        = merge(var.tags, { Name = "${var.name}-fn" })
}

resource "aws_vpc_security_group_egress_rule" "all" {
  count             = local.in_vpc ? 1 : 0
  security_group_id = aws_security_group.fn[0].id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  tags              = var.tags
}

resource "aws_lambda_function" "this" {
  function_name    = local.function_name
  role             = aws_iam_role.this.arn
  runtime          = var.runtime
  handler          = var.handler
  memory_size      = var.memory_mb
  timeout          = var.timeout_s
  filename         = data.archive_file.placeholder.output_path
  source_code_hash = data.archive_file.placeholder.output_base64sha256

  dynamic "vpc_config" {
    for_each = local.in_vpc ? [1] : []
    content {
      subnet_ids         = [var.subnet_id]
      security_group_ids = concat([aws_security_group.fn[0].id], var.security_group_ids)
    }
  }

  tags = merge(var.tags, { Name = var.name })

  lifecycle {
    ignore_changes = [filename, source_code_hash] # code is deployed outside iagram
  }
}

output "function_arn" { value = aws_lambda_function.this.arn }
output "function_name" { value = aws_lambda_function.this.function_name }
output "security_group_id" { value = local.in_vpc ? aws_security_group.fn[0].id : "" }
