## Content pass (research catalogues regenerated)

249 templates with a trustworthy picture had their research-catalogue entry rewritten to match the vendor drawing and were regenerated with `convert.py --only` (cells kept). 1186 elements or actors added, 1097 removed.

| Cloud | Templates | Added | Removed |
|---|---|---|---|
| aws | 155 | 730 | 687 |
| azure | 39 | 225 | 163 |
| gcp | 55 | 231 | 247 |

Most added element types:

- aws_cloudwatch_log_group (7)
- aws_quicksight_dashboard (7)
- aws_s3_bucket (7)
- azurerm_powerbi_embedded (7)
- microsoft entra id (7)
- aws_wafv2_web_acl (5)
- aws_iam_role (5)
- aws_lambda_function (5)
- aws_cloudformation_stack (4)
- aws_codepipeline (4)
- aws_iot_topic_rule (4)
- aws_msk_cluster (4)
- aws_secretsmanager_secret (4)
- aws_codebuild_project (4)
- aws_redshift_cluster (4)
- internet actor (4)
- azurerm_network_ddos_protection_plan (4)
- internet (4)
- aws_cloudwatch_event_rule (3)
- aws_securityhub_account (3)

Per-template notes (what changed, what the model cannot express) are in `scripts/refarch/content-pass.jsonl`. Recurring limits: no cloud resource may sit inside the on-premises box (on-prem devices are named actors); Kubernetes workloads, Textract, Polly, Personalize, MediaPackage and other services without a Terraform resource live in flow labels; account, subscription and project cells come from `account_at`, Availability Zone and resource-group cells are kept from the shipped template.
