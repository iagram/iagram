variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "price_class" {
  type    = string
  default = "PriceClass_100"
}
variable "default_root_object" {
  type    = string
  default = "index.html"
}
variable "enabled" {
  type    = bool
  default = true
}
variable "origin_bucket_names" {
  type    = list(string)
  default = []
}
variable "origin_alb_dns_names" {
  type    = list(string)
  default = []
}

data "aws_s3_bucket" "origin" {
  count  = length(var.origin_bucket_names)
  bucket = var.origin_bucket_names[count.index]
}

resource "aws_cloudfront_origin_access_control" "s3" {
  count                             = length(var.origin_bucket_names) > 0 ? 1 : 0
  name                              = "${var.name}-oac"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

locals {
  default_origin = length(var.origin_bucket_names) > 0 ? "s3-0" : length(var.origin_alb_dns_names) > 0 ? "alb-0" : "none"
}

resource "aws_cloudfront_distribution" "this" {
  comment             = var.name
  enabled             = var.enabled
  price_class         = var.price_class
  default_root_object = var.default_root_object
  tags                = merge(var.tags, { Name = var.name })

  dynamic "origin" {
    for_each = { for i, b in var.origin_bucket_names : "s3-${i}" => i }
    content {
      origin_id                = origin.key
      domain_name              = data.aws_s3_bucket.origin[origin.value].bucket_regional_domain_name
      origin_access_control_id = aws_cloudfront_origin_access_control.s3[0].id
    }
  }

  dynamic "origin" {
    for_each = { for i, d in var.origin_alb_dns_names : "alb-${i}" => d }
    content {
      origin_id   = origin.key
      domain_name = origin.value
      custom_origin_config {
        http_port              = 80
        https_port             = 443
        origin_protocol_policy = "http-only"
        origin_ssl_protocols   = ["TLSv1.2"]
      }
    }
  }

  # No origins connected yet: a placeholder so the distribution still plans.
  dynamic "origin" {
    for_each = local.default_origin == "none" ? [1] : []
    content {
      origin_id   = "none"
      domain_name = "example.com"
      custom_origin_config {
        http_port              = 80
        https_port             = 443
        origin_protocol_policy = "https-only"
        origin_ssl_protocols   = ["TLSv1.2"]
      }
    }
  }

  default_cache_behavior {
    target_origin_id       = local.default_origin
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    cache_policy_id        = "658327ea-f89d-4fab-a63d-7e88639e58f6" # Managed-CachingOptimized
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }
}

data "aws_iam_policy_document" "bucket" {
  count = length(var.origin_bucket_names)
  statement {
    actions   = ["s3:GetObject"]
    resources = ["${data.aws_s3_bucket.origin[count.index].arn}/*"]
    principals {
      type        = "Service"
      identifiers = ["cloudfront.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values   = [aws_cloudfront_distribution.this.arn]
    }
  }
}

resource "aws_s3_bucket_policy" "origin" {
  count  = length(var.origin_bucket_names)
  bucket = var.origin_bucket_names[count.index]
  policy = data.aws_iam_policy_document.bucket[count.index].json
}

output "domain_name" { value = aws_cloudfront_distribution.this.domain_name }
output "distribution_id" { value = aws_cloudfront_distribution.this.id }
# "<name>|<dns>|<hosted zone>" consumed by Route 53 alias records.
output "alias_target" { value = "${var.name}|${aws_cloudfront_distribution.this.domain_name}|${aws_cloudfront_distribution.this.hosted_zone_id}" }
