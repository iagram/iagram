variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "vpc_id" { type = string }
variable "internet_gateway_id" {
  type    = string
  default = ""
}
variable "cidr" { type = string }
variable "az" {
  type        = string
  description = "Availability-zone suffix (a, b, c...) appended to the provider region."
}
variable "public" {
  type    = bool
  default = false
}

data "aws_region" "current" {}

locals {
  availability_zone = "${data.aws_region.current.region}${var.az}"
}

resource "aws_subnet" "this" {
  vpc_id                  = var.vpc_id
  cidr_block              = var.cidr
  availability_zone       = local.availability_zone
  map_public_ip_on_launch = var.public
  tags                    = merge(var.tags, { Name = var.name, Tier = var.public ? "public" : "private" })
}

resource "aws_route_table" "this" {
  vpc_id = var.vpc_id
  tags   = merge(var.tags, { Name = "${var.name}-rt" })
}

resource "aws_route" "internet" {
  count                  = var.public && var.internet_gateway_id != "" ? 1 : 0
  route_table_id         = aws_route_table.this.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = var.internet_gateway_id
}

resource "aws_route_table_association" "this" {
  subnet_id      = aws_subnet.this.id
  route_table_id = aws_route_table.this.id
}

output "subnet_id" { value = aws_subnet.this.id }
output "availability_zone" { value = local.availability_zone }
output "route_table_id" { value = aws_route_table.this.id }
