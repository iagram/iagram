variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "vpc_id" { type = string }
variable "subnet_ids" {
  type        = list(string)
  description = "All subnets of the enclosing VPC (RDS needs a subnet group spanning at least two AZs)."
}
variable "engine" {
  type    = string
  default = "postgres"
}
variable "engine_version" {
  type    = string
  default = ""
}
variable "instance_class" {
  type    = string
  default = "db.t3.micro"
}
variable "allocated_storage_gb" {
  type    = number
  default = 20
}
variable "multi_az" {
  type    = bool
  default = false
}
variable "security_group_ids" {
  type    = list(string)
  default = []
}
variable "ingress_security_group_ids" {
  type        = list(string)
  description = "Security groups allowed to reach the database port (from connects_to arrows)."
  default     = []
}

locals {
  port       = var.engine == "postgres" ? 5432 : 3306
  identifier = replace(lower(var.name), "/[^a-z0-9-]/", "-")
}

resource "aws_db_subnet_group" "this" {
  name       = local.identifier
  subnet_ids = var.subnet_ids
  tags       = merge(var.tags, { Name = var.name })
}

resource "aws_security_group" "db" {
  name        = "${var.name}-db"
  description = "Database group for ${var.name} (managed by iagram)"
  vpc_id      = var.vpc_id
  tags        = merge(var.tags, { Name = "${var.name}-db" })
}

resource "aws_vpc_security_group_ingress_rule" "from" {
  count                        = length(var.ingress_security_group_ids)
  security_group_id            = aws_security_group.db.id
  referenced_security_group_id = var.ingress_security_group_ids[count.index]
  ip_protocol                  = "tcp"
  from_port                    = local.port
  to_port                      = local.port
  tags                         = var.tags
}

resource "aws_db_instance" "this" {
  identifier                  = local.identifier
  engine                      = var.engine
  engine_version              = var.engine_version != "" ? var.engine_version : null
  instance_class              = var.instance_class
  allocated_storage           = var.allocated_storage_gb
  storage_type                = "gp3"
  storage_encrypted           = true
  multi_az                    = var.multi_az
  db_subnet_group_name        = aws_db_subnet_group.this.name
  vpc_security_group_ids      = concat([aws_security_group.db.id], var.security_group_ids)
  publicly_accessible         = false
  username                    = "iagram_admin"
  manage_master_user_password = true
  skip_final_snapshot         = true
  apply_immediately           = true
  tags                        = merge(var.tags, { Name = var.name })
}

output "endpoint" { value = aws_db_instance.this.endpoint }
output "address" { value = aws_db_instance.this.address }
output "port" { value = aws_db_instance.this.port }
output "security_group_id" { value = aws_security_group.db.id }
