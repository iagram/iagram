variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "cidr" { type = string }
variable "enable_dns_hostnames" {
  type    = bool
  default = true
}

resource "aws_vpc" "this" {
  cidr_block           = var.cidr
  enable_dns_support   = true
  enable_dns_hostnames = var.enable_dns_hostnames
  tags                 = merge(var.tags, { Name = var.name })
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
  tags   = merge(var.tags, { Name = "${var.name}-igw" })
}

output "vpc_id" { value = aws_vpc.this.id }
output "internet_gateway_id" { value = aws_internet_gateway.this.id }
output "cidr" { value = aws_vpc.this.cidr_block }
