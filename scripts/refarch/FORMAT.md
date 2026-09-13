# Research catalogue format

One YAML list per file, `scripts/refarch/<provider>-<domain>.yaml`. Each item:

```yaml
- slug: tgw-hub-and-spoke              # unique, kebab-case, no provider prefix
  title: Transit Gateway hub and spoke
  category: networking                 # networking | web | serverless | containers | data | ml | resilience | security | iot | devops | migration | media | analytics-bi | observability | identity | storage | database | compute | edge | industry
  tags: [transit gateway, hub and spoke]
  source: https://...                  # ONE official page (docs, Architecture Center, Solutions Library, Prescriptive Guidance, official blog)
  description: Two sentences, factual, no marketing.
  containers:
    - datacenter: on-premises          # optional on-prem site
    - region: eu-west-1                # AWS: region (account optional: `account: prod`); GCP: `project: name` + `region:`; Azure: `subscription: name` + `resource_group: {name, location}`
      groups: [{label: ingestion}]     # optional labelled boxes
      vpcs:                            # AWS/GCP `vpcs`, Azure `vnets`
        - name: main
          cidr: 10.0.0.0/16
          subnets:
            - {name: public-a, cidr: 10.0.0.0/24, az: a, public: true}   # `az` AWS only; GCP `region:`; Azure no az
  elements:                            # 6-14 per architecture, real Terraform resource TYPES of the provider
    - {tf: aws_lb, name: web-alb, in: vpc:main}          # in: region | region:<name> | vpc:<name> | subnet:<name> | group:<label> | datacenter | account:<name> | project:<name> | resource_group:<name>
  actors: [users, internet]            # user, users, client, mobile, internet, saas, device, server, database, idp, datacenter
  flows:
    - {from: users, to: web-alb, label: HTTPS, step: 1}
    - {from: hub-tgw, to: vpc:spoke, link: aws_ec2_transit_gateway_vpc_attachment}   # link resources: peering, TGW/NCC/vWAN attachments, VPN connections, DX gateway associations, zone associations, SNS subscriptions, Lambda event source mappings, EventBridge targets
```

Rules: quote any label that contains a colon; only reference declared
containers and element names; prefer top-level resources (load balancer,
cluster, function, bucket, database) over their configuration (listeners,
rules, policies, IAM bindings, route tables) which the converter drops.
