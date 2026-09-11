variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "domain" { type = string }
variable "alias_targets" {
  type        = list(string)
  description = "\"<name>|<dns name>|<hosted zone id>\" entries from resolves_to arrows."
  default     = []
}

locals {
  targets = [for t in var.alias_targets : { name = split("|", t)[0], dns = split("|", t)[1], zone = split("|", t)[2] }]
}

resource "aws_route53_zone" "this" {
  name    = var.domain
  comment = var.name
  tags    = merge(var.tags, { Name = var.name })
}

# The first target answers at the apex; every target gets <name>.<domain>.
resource "aws_route53_record" "apex" {
  count   = length(local.targets) > 0 ? 1 : 0
  zone_id = aws_route53_zone.this.zone_id
  name    = var.domain
  type    = "A"
  alias {
    name                   = local.targets[0].dns
    zone_id                = local.targets[0].zone
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "named" {
  count   = length(local.targets)
  zone_id = aws_route53_zone.this.zone_id
  name    = "${local.targets[count.index].name}.${var.domain}"
  type    = "A"
  alias {
    name                   = local.targets[count.index].dns
    zone_id                = local.targets[count.index].zone
    evaluate_target_health = false
  }
}

output "zone_id" { value = aws_route53_zone.this.zone_id }
output "name_servers" { value = join(",", aws_route53_zone.this.name_servers) }
