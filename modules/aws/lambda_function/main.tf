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
variable "sqs_queue_arns" {
  type    = list(string)
  default = []
}
variable "dynamodb_table_arns" {
  type    = list(string)
  default = []
}
variable "sqs_trigger_arns" {
  type        = list(string)
  description = "Queues that invoke this function (from triggers arrows)."
  default     = []
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

locals {
  has_data_access = length(var.s3_bucket_arns) + length(var.sqs_queue_arns) + length(var.dynamodb_table_arns) > 0
}

# One policy derived from the arrows: buckets, queues and tables this
# workload was connected to, nothing else.
data "aws_iam_policy_document" "data_access" {
  count = local.has_data_access ? 1 : 0
  dynamic "statement" {
    for_each = length(var.s3_bucket_arns) > 0 ? [1] : []
    content {
      actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"]
      resources = concat(var.s3_bucket_arns, [for a in var.s3_bucket_arns : "${a}/*"])
    }
  }
  dynamic "statement" {
    for_each = length(var.sqs_queue_arns) > 0 ? [1] : []
    content {
      actions   = ["sqs:SendMessage", "sqs:GetQueueUrl", "sqs:GetQueueAttributes"]
      resources = var.sqs_queue_arns
    }
  }
  dynamic "statement" {
    for_each = length(var.dynamodb_table_arns) > 0 ? [1] : []
    content {
      actions   = ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem", "dynamodb:DeleteItem", "dynamodb:Query", "dynamodb:Scan", "dynamodb:BatchGetItem", "dynamodb:BatchWriteItem"]
      resources = concat(var.dynamodb_table_arns, [for a in var.dynamodb_table_arns : "${a}/index/*"])
    }
  }
}

resource "aws_iam_role_policy" "data_access" {
  count  = local.has_data_access ? 1 : 0
  name   = "data-access"
  role   = aws_iam_role.this.id
  policy = data.aws_iam_policy_document.data_access[0].json
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

data "aws_iam_policy_document" "sqs_consume" {
  count = length(var.sqs_trigger_arns) > 0 ? 1 : 0
  statement {
    actions   = ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"]
    resources = var.sqs_trigger_arns
  }
}

resource "aws_iam_role_policy" "sqs_consume" {
  count  = length(var.sqs_trigger_arns) > 0 ? 1 : 0
  name   = "sqs-consume"
  role   = aws_iam_role.this.id
  policy = data.aws_iam_policy_document.sqs_consume[0].json
}

resource "aws_lambda_event_source_mapping" "sqs" {
  count            = length(var.sqs_trigger_arns)
  event_source_arn = var.sqs_trigger_arns[count.index]
  function_name    = aws_lambda_function.this.arn
  batch_size       = 10
  depends_on       = [aws_iam_role_policy.sqs_consume]
}

output "function_arn" { value = aws_lambda_function.this.arn }
output "function_name" { value = aws_lambda_function.this.function_name }
output "security_group_id" { value = local.in_vpc ? aws_security_group.fn[0].id : "" }
