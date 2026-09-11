variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "vpc_id" { type = string }
variable "description" {
  type    = string
  default = "managed by iagram"
}
variable "ingress_ports" {
  type        = string
  description = "Comma-separated TCP ports opened from ingress_cidr."
  default     = "443"
}
variable "ingress_cidr" {
  type    = string
  default = "0.0.0.0/0"
}

locals {
  ports = toset([for p in split(",", var.ingress_ports) : trimspace(p) if trimspace(p) != ""])
}

resource "aws_security_group" "this" {
  name        = var.name
  description = var.description
  vpc_id      = var.vpc_id
  tags        = merge(var.tags, { Name = var.name })
}

resource "aws_vpc_security_group_ingress_rule" "tcp" {
  for_each          = local.ports
  security_group_id = aws_security_group.this.id
  cidr_ipv4         = var.ingress_cidr
  ip_protocol       = "tcp"
  from_port         = tonumber(each.value)
  to_port           = tonumber(each.value)
  tags              = var.tags
}

resource "aws_vpc_security_group_egress_rule" "all" {
  security_group_id = aws_security_group.this.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  tags              = var.tags
}

output "security_group_id" { value = aws_security_group.this.id }
