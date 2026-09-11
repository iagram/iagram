variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "vpc_id" { type = string }
variable "subnet_ids" {
  type        = list(string)
  description = "Subnets of the enclosing VPC (an ALB needs at least two AZs)."
}
variable "scheme" {
  type    = string
  default = "internet-facing"
}
variable "listener_port" {
  type    = number
  default = 80
}
variable "security_group_ids" {
  type    = list(string)
  default = []
}
variable "target_instance_ids" {
  type    = list(string)
  default = []
}
variable "target_lambda_arns" {
  type    = list(string)
  default = []
}

locals {
  has_instances = length(var.target_instance_ids) > 0
  has_lambdas   = length(var.target_lambda_arns) > 0
  lb_name       = substr(replace(var.name, "/[^A-Za-z0-9-]/", "-"), 0, 32)
}

resource "aws_security_group" "lb" {
  name        = "${var.name}-lb"
  description = "Listener group for ${var.name} (managed by iagram)"
  vpc_id      = var.vpc_id
  tags        = merge(var.tags, { Name = "${var.name}-lb" })
}

resource "aws_vpc_security_group_ingress_rule" "listener" {
  security_group_id = aws_security_group.lb.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "tcp"
  from_port         = var.listener_port
  to_port           = var.listener_port
  tags              = var.tags
}

resource "aws_vpc_security_group_egress_rule" "all" {
  security_group_id = aws_security_group.lb.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  tags              = var.tags
}

resource "aws_lb" "this" {
  name               = local.lb_name
  load_balancer_type = "application"
  internal           = var.scheme == "internal"
  subnets            = var.subnet_ids
  security_groups    = concat([aws_security_group.lb.id], var.security_group_ids)
  tags               = merge(var.tags, { Name = var.name })
}

resource "aws_lb_target_group" "instances" {
  count       = local.has_instances ? 1 : 0
  name        = substr("${local.lb_name}-ec2", 0, 32)
  port        = 80
  protocol    = "HTTP"
  target_type = "instance"
  vpc_id      = var.vpc_id
  health_check {
    path = "/"
  }
  tags = var.tags
}

resource "aws_lb_target_group_attachment" "instances" {
  count            = length(var.target_instance_ids)
  target_group_arn = aws_lb_target_group.instances[0].arn
  target_id        = var.target_instance_ids[count.index]
  port             = 80
}

resource "aws_lb_target_group" "lambdas" {
  count       = local.has_lambdas ? 1 : 0
  name        = substr("${local.lb_name}-fn", 0, 32)
  target_type = "lambda"
  tags        = var.tags
}

resource "aws_lambda_permission" "alb" {
  count         = length(var.target_lambda_arns)
  statement_id  = "AllowALB${count.index}"
  action        = "lambda:InvokeFunction"
  function_name = var.target_lambda_arns[count.index]
  principal     = "elasticloadbalancing.amazonaws.com"
  source_arn    = aws_lb_target_group.lambdas[0].arn
}

resource "aws_lb_target_group_attachment" "lambdas" {
  count            = length(var.target_lambda_arns)
  target_group_arn = aws_lb_target_group.lambdas[0].arn
  target_id        = var.target_lambda_arns[count.index]
  depends_on       = [aws_lambda_permission.alb]
}

resource "aws_lb_listener" "this" {
  load_balancer_arn = aws_lb.this.arn
  port              = var.listener_port
  protocol          = "HTTP"

  dynamic "default_action" {
    for_each = local.has_instances || local.has_lambdas ? [1] : []
    content {
      type             = "forward"
      target_group_arn = local.has_instances ? aws_lb_target_group.instances[0].arn : aws_lb_target_group.lambdas[0].arn
    }
  }

  dynamic "default_action" {
    for_each = local.has_instances || local.has_lambdas ? [] : [1]
    content {
      type = "fixed-response"
      fixed_response {
        content_type = "text/plain"
        message_body = "no targets connected in iagram"
        status_code  = "503"
      }
    }
  }

  tags = var.tags
}

output "dns_name" { value = aws_lb.this.dns_name }
# "<name>|<dns>|<hosted zone>" consumed by Route 53 alias records.
output "alias_target" { value = "${var.name}|${aws_lb.this.dns_name}|${aws_lb.this.zone_id}" }
output "alb_arn" { value = aws_lb.this.arn }
output "security_group_id" { value = aws_security_group.lb.id }
