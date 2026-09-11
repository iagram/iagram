variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "versioning" {
  type    = bool
  default = false
}
variable "block_public_access" {
  type    = bool
  default = true
}

data "aws_caller_identity" "current" {}

locals {
  # Bucket names are global: suffix with the account id and keep it DNS-safe.
  bucket = substr("${replace(lower(var.name), "/[^a-z0-9-]/", "-")}-${data.aws_caller_identity.current.account_id}", 0, 63)
}

resource "aws_s3_bucket" "this" {
  bucket = local.bucket
  tags   = merge(var.tags, { Name = var.name })
}

resource "aws_s3_bucket_versioning" "this" {
  bucket = aws_s3_bucket.this.id
  versioning_configuration {
    status = var.versioning ? "Enabled" : "Suspended"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "this" {
  bucket = aws_s3_bucket.this.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "this" {
  count                   = var.block_public_access ? 1 : 0
  bucket                  = aws_s3_bucket.this.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

output "bucket_name" { value = aws_s3_bucket.this.bucket }
output "bucket_arn" { value = aws_s3_bucket.this.arn }
