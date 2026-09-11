variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "subnet_id" { type = string }
variable "vpc_id" { type = string }
variable "instance_type" {
  type    = string
  default = "t3.micro"
}
variable "ami" {
  type        = string
  description = "AMI id, or al2023-latest to resolve the newest Amazon Linux 2023 (x86_64) at plan time."
  default     = "al2023-latest"
}
variable "root_volume_gb" {
  type    = number
  default = 20
}
variable "public_ip" {
  type    = bool
  default = false
}
variable "security_group_ids" {
  type    = list(string)
  default = []
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

data "aws_ssm_parameter" "al2023" {
  count = var.ami == "al2023-latest" ? 1 : 0
  name  = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64"
}

locals {
  ami_id = var.ami == "al2023-latest" ? data.aws_ssm_parameter.al2023[0].value : var.ami
}

# The instance's own identity group: other resources allow traffic *from* it.
resource "aws_security_group" "instance" {
  name        = "${var.name}-instance"
  description = "Identity group for ${var.name} (managed by iagram)"
  vpc_id      = var.vpc_id
  tags        = merge(var.tags, { Name = "${var.name}-instance" })
}

resource "aws_vpc_security_group_egress_rule" "all" {
  security_group_id = aws_security_group.instance.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  tags              = var.tags
}

data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "this" {
  name               = "${var.name}-instance-role"
  assume_role_policy = data.aws_iam_policy_document.assume.json
  tags               = var.tags
}

resource "aws_iam_role_policy_attachment" "ssm" {
  role       = aws_iam_role.this.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
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

resource "aws_iam_instance_profile" "this" {
  name = "${var.name}-instance-profile"
  role = aws_iam_role.this.name
  tags = var.tags
}

resource "aws_instance" "this" {
  ami                         = local.ami_id
  instance_type               = var.instance_type
  subnet_id                   = var.subnet_id
  associate_public_ip_address = var.public_ip
  vpc_security_group_ids      = concat([aws_security_group.instance.id], var.security_group_ids)
  iam_instance_profile        = aws_iam_instance_profile.this.name

  root_block_device {
    volume_size = var.root_volume_gb
    volume_type = "gp3"
    encrypted   = true
  }

  metadata_options {
    http_tokens = "required"
  }

  tags = merge(var.tags, { Name = var.name })
}

output "instance_id" { value = aws_instance.this.id }
output "private_ip" { value = aws_instance.this.private_ip }
output "public_ip" { value = aws_instance.this.public_ip }
output "security_group_id" { value = aws_security_group.instance.id }
