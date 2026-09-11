variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "retention_days" {
  type    = number
  default = 7
}

resource "google_pubsub_topic" "this" {
  name                       = var.name
  message_retention_duration = "${var.retention_days * 86400}s"
  labels                     = { for k, v in var.tags : replace(lower(k), "/[^a-z0-9_-]/", "_") => replace(lower(v), "/[^a-z0-9_-]/", "_") }
}

output "topic_id" { value = google_pubsub_topic.this.id }
output "topic_name" { value = google_pubsub_topic.this.name }
