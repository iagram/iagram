# Challenge: iagram vs. official AWS architecture diagrams

Date: 2026-09-12. Baseline: iagram `main` at a0d774c (v0.1.0-rc10).

Method: 19 diagram sets from the AWS Architecture Center, Prescriptive
Guidance, Well-Architected lenses and the Solutions Library (27 rendered
images inspected), plus 32 diagram-bearing posts from the official AWS blogs
(Architecture, Networking, Security, Containers, Compute, Big Data, DevOps,
Database, ML, Cloud Operations), plus the group and label conventions of the
AWS Architecture Icons deck. Every visual construct those diagrams use was
listed and checked against what iagram can draw, persist in `.iad`, and turn
into Terraform.

Counts below are "number of diagram sets using the construct" out of 19
(Architecture Center) and 32 (blogs).

## 1. What iagram already covers

| Construct | Frequency | iagram |
|---|---|---|
| AWS account boundary | 7 / 10 | `aws.account` box (provider block) |
| Region box, several side by side | 11 / 13 | `aws.region` box with `region` prop |
| VPC | 11 / 11 | `aws.vpc` box |
| Public / private subnet | 7 / 4 | `aws.subnet` box, icon switches on `public` |
| Cluster boxes (EKS, ECS, Aurora, EMR) | 5 | container entries with components |
| Service icons for every service seen (S3, Lambda, EC2, DynamoDB, Route 53, API Gateway, EventBridge, SNS, SQS, Step Functions, Kinesis, MSK, Glue, Athena, Redshift, SageMaker, Bedrock, OpenSearch, Cognito, TGW, DX, VPN, PrivateLink, WAF, Shield, Network Firewall, Cloud WAN, KMS, CloudTrail, Config, GuardDuty, Macie, Security Hub, Control Tower, Organizations, RAM, Backup, DMS, DataSync, Transfer, AppFlow, Code*, CloudFormation, Systems Manager...) | all | 151 palette families, 1721 Terraform types |
| Directional flow arrows between services | universal | catalog connection kinds + `references` edges |
| Attachments configured in the owner's panel | implicit | `Attachment` / `Component` entries |

Every AWS *service* that appears in the surveyed diagrams has a palette
family. Coverage of services is not the gap. The gap is the diagram
vocabulary around the services.

## 2. Gaps, ordered by how often official diagrams need them

### G1. Free-text generic group (13 / 10) — MISSING

The single most common container in AWS diagrams is a dashed grey box with a
label and no Terraform meaning: "data ingestion", "Parsing", "Console",
"presentation tier", "tenant A", "cell 1", "Deployment pipeline". The AWS
deck sanctions it as **Generic group** (grey #7D8998, dashed or solid, no
icon, centred label).

iagram has no node without a Terraform role. Every container is either a
provider root, a network scope, or a cluster.

Needed: a provider-neutral `common.group` container. Kind container, no
`terraform` block, allowed anywhere, allows any child, transparent to the
generator, validator and importer (children keep the nearest *Terraform*
ancestor as their logical parent). Round trips through `.iad` untouched.

### G2. Numbered step callouts (17 / 10) — MISSING

Almost every official diagram walks the reader through numbered steps: blue
squares (Architecture Center), black circles (Solutions Library), white
circles (Prescriptive Guidance), placed on arrows or next to icons, sometimes
`9a`/`9b`, sometimes the same number on two arrows, always paired with an
ordered list of step text.

iagram edges have no user label, no order, no note.

Needed: `Edge.label`, `Edge.step` (string, so `9a` works) and a document
level `steps: [{n, text}]` list rendered in a side "Legend / Steps" panel and
as badges on edges or nodes. Purely informational, ignored by the generator.

### G3. Non-AWS actors (10 / 18 users; 6 / 8 on-prem; SaaS, devices, IdP, internet) — MISSING

Users, clients, mobile, browser, internet cloud, corporate data center,
branch, SD-WAN appliance, on-prem DNS server, on-prem database, SaaS
application, IoT device, identity provider, third-party product logos, human
approver. The AWS icon package ships these as **General icons** (User, Users,
Client, Mobile client, Internet, Internet alt, Office building, Traditional
server, Disk, Database, Documents, SaaS, Camera, Programming language...).

iagram cannot draw anything that is not a Terraform resource.

Needed: a `common` provider (no Terraform) with actor leaves: `common.user`,
`common.users`, `common.client`, `common.mobile`, `common.internet`,
`common.saas`, `common.device`, `common.server`, `common.database`,
`common.idp`, `common.note`. Plus the group container `common.datacenter`
(deck: **Corporate data center**, grey, building icon) which is the natural
owner of `aws_customer_gateway` and the on-prem end of DX / VPN links.
Actors connect to anything with an informational edge kind (`flow`) that
sets no Terraform input.

### G4. Availability Zone box (6 / 7) — MISSING

Drawn as a dashed teal box (no icon, centred label) inside a Region or VPC,
holding subnets and instances. Reference three-tier diagrams are laid out as
two AZ columns.

iagram carries the AZ only as a text prop on the subnet. A user cannot draw
the AZ column that every reference diagram uses.

Needed: `aws.availability_zone` container, `allowed_parents: [aws.region,
aws.vpc]`, prop `zone` (a, b, c or full name), no Terraform block of its own,
but children pick it up through `inputs_from_parent` (subnet `az`, EC2 and
RDS `availability_zone`, EBS). Layout only for everything else. GCP zone and
Azure zone equivalents exist for the mirror table.

### G5. Spanning groups: Auto Scaling group (5) and cross-AZ tier bands — MISSING

The deck's **Auto Scaling group** (orange dashed) is designed to *span* other
groups: a band crossing two subnets in two AZs with instance icons inside.
Cross-AZ "Database cluster" / "Distributed cache cluster" / "Shared file
storage" bands work the same way (Moodle, DR whitepaper).

iagram containment is a strict tree. A node has one parent, so a box cannot
straddle two subnets. `aws_autoscaling_group` is a leaf today.

Needed: a spanning-container mode. Proposal: a `span: true` container whose
geometry may overlap sibling containers of the same parent; the set of
containers it overlaps becomes a collected input (`vpc_zone_identifier` for
ASG, `subnet_ids` for RDS subnet group, EFS mount targets, ALB subnets).
ASG should become a box with ghost instance tiles (its `desired_capacity`),
not Terraform nodes.

### G6. Link resources drawn as lines (attachments 6 / hybrid links 4) — PARTIAL

TGW "VPC attachment", "Transit gateway association", "Direct Connect
connection", "Transit VIF", "VPC association" (Route 53 zone), "service
association" (Lattice), VPC peering, TGW peering, RAM share: AWS draws these
as plain lines without arrowheads, sometimes colour coded (green VPC
attachment, purple DX, red dashed BGP, blue GRE), between two boxes.

In Terraform they are real resources with two ends:
`aws_ec2_transit_gateway_vpc_attachment`, `aws_vpc_peering_connection`,
`aws_ec2_transit_gateway_peering_attachment`, `aws_vpn_connection`,
`aws_dx_gateway_association`, `aws_route53_zone_association`,
`aws_ram_resource_share`, `aws_networkmanager_*_attachment`. iagram treats
them as attachments configured in a panel, or as boxes with two reference
edges. Neither looks like the AWS diagram.

Needed: an `edge` role in the catalog: a Terraform resource rendered as a
line, whose two endpoint attributes come from source and target
(`terraform: {role: edge, from: transit_gateway_id, to: vpc_id}`), with a
line style (`solid`, `dashed`, `dashdot`, colour) and no arrowhead. The
attachment classifier already knows which types bind to two owners; those
with exactly two required id attributes on different services are edge
candidates. Editing the line opens its props, same as today's attach flow.

### G7. Resource icons for network sub-resources (NAT GW 2, IGW 2, endpoints 6, ENI 2, resolver endpoints) — MISSING

AWS diagrams never draw a NAT gateway or an internet gateway with the VPC
service square; they use the line-art **resource icons** (`Res_Amazon-VPC_
NAT-Gateway`, `Internet-Gateway`, `Endpoints`, `Peering-Connection`,
`Router`, `Elastic-Network-Interface`, `Customer-Gateway`, `VPN-Gateway`,
`Flow-Logs`, `Network-Access-Control-List`; also `Res_Amazon-Route-53_
Resolver`, `Hosted-Zone`, `Res_Amazon-EC2_Instance`, `AMI`, `Res_Amazon-
Simple-Storage-Service_Bucket`, `Res_AWS-Lambda_Lambda-Function`, ...).

iagram extracted only the `Arch_` service icons. `aws_nat_gateway`,
`aws_internet_gateway`, `aws_vpc_endpoint`, `aws_network_interface`,
`aws_eip`, `aws_route_table`, `aws_vpc_peering_connection` all share the
"Virtual Private Cloud" icon and therefore the same palette tile.

Needed: extract the `Resource-Icons_*/Res_*` set (roughly 500 files, same
licence, same NOTICE) into `catalog/icons/aws/resources/`, map Terraform
types to them in `services.yaml` with the resource icon taking precedence,
and make them their own families. This also fixes the palette: NAT gateway,
internet gateway, endpoint and peering become distinct tiles.

### G8. Group icons and colour grammar of the deck — PARTIAL

The deck fixes a colour and line style per group: AWS Cloud dark solid,
account pink `#E7157B`, Region teal `#00A4A6` dashed, AZ teal dashed no icon,
VPC purple `#8C4FFF`, public subnet green `#7AA116`, private subnet teal
(blue in older decks), security group red `#DD344C` centred no icon, ASG
orange `#ED7100` dashed, corporate data center / server contents / generic
grey `#7D8998`, EC2 contents / Spot fleet / Beanstalk orange, Step Functions
workflow pink, Greengrass green. Service icon squares use the category
colours (Compute orange, Storage green, Database purple `#C925D1`,
Networking/Analytics `#8C4FFF`, Security red, Management/Integration pink,
ML `#01A88D`).

iagram containers have one generic style with the service icon in the
corner; the catalog has no `style` key.

Needed: `style: {border, fill, dash, label_position}` on container entries,
defaults per provider in `_provider.yaml`, and the AWS values above in the
curated entries. Cheap and visually decisive: users recognise an iagram
diagram as "an AWS diagram" or they do not.

### G9. Container captions: CIDR, Region code, role (11 VPC labels, 4 Region labels) — MISSING

AWS labels boxes "VPC – 10.0.0.0/16", "us-east-1 (primary Region)",
"Endpoint subnet 10.0.2.0/24", "Consumer account", "(in user VPC)". Icons
carry italic sub-captions: "vehicle lookup", "geolocation routing",
"customer account".

iagram shows name and type only.

Needed: `caption_prop` in the catalog (vpc → `cidr`, subnet → `cidr`, region
→ `region`, ec2 → `instance_type`, rds → `instance_class`) rendered as the
second line of the label, plus a free `caption` field on every node for the
italic role text. `.iad` field `caption`, informational.

### G10. Edge semantics beyond "flow" — PARTIAL

Bidirectional arrows (6 / 6), paired request/response arrows, dotted =
read-only or optional, red dash-dot with ✕ = denied traffic, filled origin
dot = replication source, greyed = inactive/standby capacity, lock badge =
encrypted, protocol chips (mTLS, MQTT, HTTP, "Private VIF"), colour-coded
link types with a legend.

iagram has one edge look (smoothstep, kind label on hover).

Needed: `Edge.style` {`direction`: one|both|none, `dash`, `color`,
`inactive`} and `Edge.label` (G2). Catalog connection rules get a default
style per kind (`replicates_to` both + dashed, `protects` none, `references`
dotted). Nothing changes in the generator.

### G11. Notes and legend (note boxes 3, legends 2, figure metadata) — MISSING

Grey folded-corner note with key/value config ("Outbound rule: domain,
type, target IP"), dashed note box linked by a dashed line, bracket with
rotated text, legend box, "Reviewed for technical accuracy" footer.

Needed: `common.note` leaf (markdown text, folded corner, optional dashed
link to a node) and a document `legend` toggle that lists the edge styles and
container colours in use. Metadata (`reviewed`, `author`) can go in `.iad`
`meta`.

### G12. Multi-account hierarchy: Organizations OU as a box (2 / 1) — MISSING

Landing-zone diagrams are org charts: Root → OUs (Security, Infrastructure,
Workloads, Prod/Test) → accounts, drawn as nested pink/grey/dashed boxes or
as an elbow tree. `aws_organizations_organizational_unit` and
`aws_organizations_account` are Terraform resources.

iagram classifies the OU as a leaf and the account is a provider root, so an
account cannot sit inside an OU.

Needed: `aws.organization` and `aws.organizational_unit` containers (role
`resource`, but *not* provider-bearing), allowed above `aws.account`; the
account keeps its provider role and gains `parent_id` from the OU through
`inputs_from_parent`. Tree layout (elbow connectors) for org charts is a
separate, optional renderer.

### G13. Global services live outside the Region (edge locations, 3 / 3)

CloudFront, WAF, Shield, Route 53, Global Accelerator, IAM are drawn in the
account but outside any Region box, or in an "AWS edge location" group.

iagram already allows CloudFront and Route 53 directly in `aws.account`.
Generated global types (`aws_wafv2_web_acl` with `scope = CLOUDFRONT`,
`aws_globalaccelerator_accelerator`, `aws_iam_*`) inherit "all containers of
the provider" and so can also be placed in the account. OK as is; add an
optional `common.group` labelled "Edge" (G1) for fidelity.

### G14. Kubernetes objects, Step Functions states, CloudFormation stacks (4 / 4) — OUT OF SCOPE FOR NOW

Pods, deployments, namespaces, ingress, services (EKS posts), state-machine
states as boxes, "Administrator stack / Member stack". These are a second
provider (`kubernetes`, `helm`) or non-Terraform detail. Until then `common.
group` + `common.note` cover the drawing need. A `kubernetes` provider layer
is the natural follow-up given `--catalog` already supports external layers.

### G15. Data mesh / landing-zone accounts side by side (multi-account 7 / 10)

Several `aws.account` boxes in one diagram, each with its own credentials or
assume-role. Supported by the model (one provider alias per account node)
but never exercised: the CLI has no way to give each account a profile or
role ARN, and the mirror/convert table maps a single root chain.

Needed: `profile` / `assume_role_arn` props on `aws.account` feeding
`provider_args`, an e2e test with two accounts, and equivalence-table support
for N roots (GCP projects, Azure subscriptions).

## 3. Recommended order

1. **G7 resource icons + G8 group styles** — pure catalog/icon work, no model
   change, and the diagrams start looking like AWS diagrams. One day.
2. **G1 generic group + G3 actors + G11 note** — introduce the `common`
   provider (no Terraform). Requires the generator/validator/importer to skip
   `common.*` nodes and look through `common.group` for the logical parent.
   Two days.
3. **G2 steps + G9 captions + G10 edge styles** — additive `.iad` fields
   (`label`, `step`, `style`, `caption`, `steps[]`), inspector widgets, a
   Steps panel. Document format stays v1 (additive). One to two days.
4. **G4 Availability Zone** — new container with `inputs_from_parent`
   propagation; equivalents `gcp.zone`, `azure.zone` in the mirror table. One
   day.
5. **G6 edge-role resources** — catalog `role: edge` + classifier rule +
   renderer. The biggest modelling change but it makes TGW/peering/VPN/DX
   diagrams natural. Two to three days.
6. **G5 spanning groups (ASG)** — overlap-based collection. Needs care in
   drag/re-parent logic. Two days.
7. **G12 Organizations, G15 multi-account** — follow-ups once a real
   multi-account plan is wanted.
8. **G14 Kubernetes** — separate catalog layer, later.

## 4. Sources

Architecture Center and docs: Modern Data Analytics, Supply Chain Data Lake,
GraphRAG with Neo4j, VPC Lattice use cases (6), Hybrid DNS with Route 53
Resolver (PDF, 3), Direct Connect + TGW over transit VIF, TGW hub and spoke
(multi-VPC whitepaper), TGW third-party integration, DR Orchestrator
(Prescriptive Guidance), Disaster Recovery of Workloads (4 figures), Moodle,
Microservices on EKS with VMware Cloud, Smart Farm, Connected Vehicle (5),
Control Tower account structure + pharma OU structure, Automated Security
Response, Centralized Logging with OpenSearch, Clickstream Analytics (2),
Serverless Lens RESTful microservices.

Blogs (32): event-driven invoice processing; Amazon Key Suite EDA; Macie PII
with Step Functions; hybrid cloud orchestration; Let's Architect serverless;
EKS cross-account ALB; ACK + EKS Blueprints; EventBridge + ECS; multi-Region
application parts 1 and 2; DR part IV active/active; event-driven
multi-Region DR; insurance 3-tier DR; network transformation part 2; TGW to
Cloud WAN migration; LexisNexis Cloud WAN; Client VPN native TGW attachment;
Route 53 PHZ cross-account; multi-Region PrivateLink failover; hybrid
multi-tenant stateful; DMS isolated VPCs; Customizations for Control Tower;
data mesh with Lake Formation; serverless data analytics pipeline; streaming
data mesh with Kinesis; Flink stream processing part 1; multi-tenant RAG with
Bedrock KB; binary embeddings RAG; protect APIs with API Gateway; network
perimeter for GenAI; OpenTelemetry/PromQL in CloudWatch; multi-Region
Terraform with CodePipeline; DevSecOps pipeline with open-source tools.

Icon deck: AWS Architecture Icons (release 07312026), group slide of the 2024
light/dark decks, awslabs/aws-icons-for-plantuml group colour encodings.
