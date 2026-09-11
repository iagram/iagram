# import example

A hand-written `terraform.tfstate` (format 4) with a VPC, two subnets, an EC2
instance in a module, an ALB, an S3 bucket, an RDS instance and an IAM role.

```
iagram import --state terraform.tfstate -o iagram.iad
iagram up
```

The IAM role has no curated element, so it imports as a generated element
(`aws.res.aws_iam_role`) with its attributes from state; the RDS instance has
no subnet attribute in state and is placed by fallback.
