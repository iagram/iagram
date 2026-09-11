variable "name" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
variable "fifo" {
  type    = bool
  default = false
}
variable "visibility_timeout_s" {
  type    = number
  default = 30
}
variable "retention_s" {
  type    = number
  default = 345600
}

resource "aws_sqs_queue" "this" {
  name                        = var.fifo ? "${var.name}.fifo" : var.name
  fifo_queue                  = var.fifo
  content_based_deduplication = var.fifo
  visibility_timeout_seconds  = var.visibility_timeout_s
  message_retention_seconds   = var.retention_s
  sqs_managed_sse_enabled     = true
  tags                        = merge(var.tags, { Name = var.name })
}

output "queue_arn" { value = aws_sqs_queue.this.arn }
output "queue_url" { value = aws_sqs_queue.this.url }
