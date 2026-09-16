# Fidelity to the vendor diagrams: reconciliation

How near are the templates to the reference drawings they come from? This
report compares the vendor image bundled with a template against iagram's
rendering of that template, on three axes scored 0 to 5:

- **content**: elements the reference shows that the template lacks, and
  elements we draw that the reference does not (mandatory scaffolding such as
  account, region, project or subscription boxes is ignored);
- **arrangement**: rows, columns and grouping as the vendor drew them;
- **clarity**: overlaps, detours, clutter.

The pilot covers 12 templates (4 per cloud) and was run on 2026-09-16 with
the grid-cell mechanism (`at: [row, col]` on template nodes, see
`docs/spec/templates.md`): the "after" arrangement score is the rendering once
the cells read off the reference were added to the template.

## Findings

1. **Arrangement is a data problem, not a heuristic one.** With cells read off
   the reference the arrangement score goes from 2.1 to 4.2 on average; the
   flow heuristic alone cannot know that a vendor put the hub in the middle
   with the spokes below, or the load-balancer chain in one row. The
   remaining arrangement gaps are nesting the model cannot express (listings
   inside exchange boxes, endpoint hops drawn as nodes) and edge routing.
2. **Content is the next gap (2.8 average).** Templates miss elements the
   reference draws (second consumer account, app tier, node pools, on-premises
   gateway, bronze/silver/gold layers) and draw infrastructure the reference
   leaves implicit (VPC scaffolding, NAT, health checks, private endpoints).
   Both come from the research catalogues behind `scripts/refarch/convert.py`.
3. **Some bundled images are not the diagram.** Three AWS templates carried a
   wrong picture: a Builder Center placeholder (three-tier-web), the TGW
   Connect figure from the same whitepaper page (tgw-hub-and-spoke), and the
   generative-AI variant of a tutorial (serverless-web-app-s3-apigw-cognito).
   The first and third images were removed from the bundle; the image scraper
   needs a check that the fetched picture belongs to the source page's own
   figure.
4. **Edges are the visible residue.** Even with a faithful arrangement, arrows
   still detour around boxes or cross a container's interior; an
   obstacle-aware orthogonal router is the next engine change.

## Scores

12 templates compared (vendor drawing vs our rendering), scores 0-5.

| Template | Content | Arrangement before | Arrangement after | Clarity before | Main gap |
|---|---|---|---|---|---|
| aws/eventbridge-central-bus-multi-account | 3 | 3 | 5 | 3 | Only one consumer account where the reference has two (invoice + forecasting); 'archive all events' label sits close to the central-bus caption. |
| aws/serverless-web-app-s3-apigw-cognito | 2 | 1 | 4 | 1 | Image/source metadata disagree; against the classic tutorial the content is a good match, against the fetched image it is a different stack. Cognito JWT return edge still overlaps the users->app-api edge near the users actor. |
| aws/tgw-hub-and-spoke | 2 | 2 | 4 | 2 | Template models centralized egress while the ref image shows hybrid connectivity (VPN, DX, peering, Connect); the picture reconciliation should re-fetch hub-and-spoke-design.png. TGW attachment edges still bend around the spoke-rt icon. |
| aws/three-tier-web | 2 | 3 | 4 | 3 | Image metadata points at a placeholder OG image instead of the whitepaper diagram. Content still single-tier compute (no app tier, no ASG, no standby RDS) so it is really a two-tier drawing. |
| azure/acr-geo-replication-image-flow | 2 | 2 | 4 | 3 | The reference's endpoint-hop decomposition (global endpoint, replica, data endpoint) is not modelled; our template shows infrastructure rather than the request flow, and the replicate edge is a self-loop. |
| azure/adf-medallion-landing-zone-baseline | 3 | 2 | 4 | 2 | No bronze/silver/gold layers or Power BI node; DevOps, Entra ID and Cost Management are absent; Site Recovery replication and Private Link edges still cross the VNet interior. |
| azure/aks-baseline-hub-spoke | 3 | 2 | 4 | 2 | No API-server integration subnet, no on-premises network, no node pools/pods; peering edge and egress edge still route through the middle of the spoke box. |
| azure/hub-spoke-firewall | 4 | 2 | 4 | 2 | Reference image is cropped on the right so the production spoke position is inferred; Virtual Network Manager frame and forced-tunnel/diagnostics arrow styling are not modelled. |
| gcp/analytics-hub-data-sharing | 3 | 2 | 4 | 3 | Exchange/listing nesting and the public/private split cannot be expressed with cells alone; users sits on the right instead of the [1,1] convention to preserve the reference's left-to-right publish direction. |
| gcp/enterprise-hub-spoke-nva-inspection | 3 | 2 | 4 | 2 | Reference image is cropped so the spoke side could not be verified; the onprem-router living in an auto-generated 'main' VPC in the hub project is a modelling artifact that no cell placement can hide; region grouping is absent. |
| gcp/global-alb-mig-cloud-sql-three-tier | 3 | 2 | 5 | 2 | Reference ref image is cropped so the Zone/instance-group half is not visible; 'instance template' and 'health checks' edge labels overlap each other near the MIG; no 'Load balancer' grouping box. |
| gcp/hub-and-spoke-ncc | 3 | 2 | 4 | 2 | Reference ref image is cropped at the bottom (spoke NATs partly visible); the ncc-hub -> spoke-a spoke edge takes a detour through the spoke-a project; the transitive workload-a -> shared-svc edge crosses the hub spoke edges; spokes lack their own NAT and GKE cluster. |

Averages: content 2.8, arrangement before 2.1 -> after 4.2, clarity before 2.2.

## Missing in iagram (per template)

- **aws/eventbridge-central-bus-multi-account**: second consumer account (Account D: Forecasting) with its event bus, Forecasting rule and Forecast target; 'New order created' event annotations on the cross-account arrows
- **aws/serverless-web-app-s3-apigw-cognito**: AWS Amplify (hosting / frontend entry point); AWS AppSync GraphQL API; Amazon Bedrock
- **aws/tgw-hub-and-spoke**: customer gateway + Site-to-Site VPN attachment; Direct Connect gateway attachment; Transit Gateway peering to a second Transit Gateway; third-party virtual appliance EC2 with TGW Connect / GRE tunnel and BGP peering; corporate SD-WAN and branch sites
- **aws/three-tier-web**: app tier (app servers in private subnets between web tier and DB); Auto Scaling group band over the web servers (tags promise it, no node); RDS standby replica in AZ b (private-b is empty); ElastiCache layer; S3 bucket for static assets / CloudFront origin; Internet gateway and NAT gateways
- **azure/acr-geo-replication-image-flow**: Global endpoint (myregistry.azurecr.io, auth and data plane APIs) as a distinct hop; Geo-replica with best network performance profile as a distinct node; Geo-replica data endpoint (blob storage or dedicated data endpoint) as a distinct node; Azure-managed routing and 307 redirect flow labels
- **azure/adf-medallion-landing-zone-baseline**: Bronze / Silver / Gold Data Lake Storage layers (Delta Lake) as separate containers; Power BI serve node (only a users actor); Azure DevOps; Microsoft Entra ID; Microsoft Cost Management; Ingest / Process / Serve / Store / Monitor-and-govern group labels
- **azure/aks-baseline-hub-spoke**: API server VNet integration delegated subnet with its internal load balancer; On-premises network connected to the gateway subnet (GatewaySubnet is drawn empty, no VPN/ExpressRoute gateway); System node pool and user node pool boxes with pods (CoreDNS, Envoy, metrics-server, TLS sync, workload); Bastion tunnel / kubectl operator flow label and Private IP address annotation
- **azure/hub-spoke-firewall**: Azure Virtual Network Manager (outer frame in the reference); virtual machines and VPN gateway device inside the cross-premises network box; multiple VMs per spoke resource subnet (we draw one VM per spoke); Azure Monitor is represented only by a Log Analytics workspace; forced-tunnel arrows from spokes to the firewall are not drawn as such
- **gcp/analytics-hub-data-sharing**: Public Exchanges vs Private Exchanges as two separate exchange containers (we have a single flat market-data-exchange); listings drawn inside their exchange box (ours are siblings of the exchange); second shared-resources box grouping (reference groups datasets into two 'Shared Resources' boxes)
- **gcp/enterprise-hub-spoke-nva-inspection**: on-premises Gateway element inside the on-premises box; four Dedicated Interconnect elements as a column (we have two interconnect attachments); per-region grouping (Region 1 / Region 2 with a Cloud Router and subnet each); restricted.googleapis.com advertised-route / Private Google Access endpoint; second region subnet in the hub Shared VPC
- **gcp/global-alb-mig-cloud-sql-three-tier**: 'Load balancer' grouping box around forwarding rule / target proxy / URL map / backend service; Zone box with the instance group's individual VMs (we show a single region MIG resource inside a subnet instead)
- **gcp/hub-and-spoke-ncc**: GKE cluster in spoke/workload 2 subnet; Cloud NAT gateway in each spoke VPC (reference: one NAT per network; we only have hub-nat); single Project containing all three VPCs (we split hub and spokes into three projects)

## Drawn by iagram but absent from the reference

- **aws/eventbridge-central-bus-multi-account**: producer-side local event bus (orders-local-bus) and forwarding rule (forward-to-central); the reference Lambda publishes straight to the central bus; central-archive (event archive) in the central account
- **aws/serverless-web-app-s3-apigw-cognito**: Route 53 zone; ACM certificate; CloudFront distribution + S3 frontend bucket (ref uses Amplify hosting); API Gateway REST API (ref uses AppSync); DynamoDB table (ref calls Bedrock)
- **aws/tgw-hub-and-spoke**: egress VPC internals (NAT gateways, internet gateway, public and tgw subnets); TGW route tables spoke-rt and egress-rt as nodes; EC2 instances app-a / app-b inside the spokes; internet actor
- **aws/three-tier-web**: security group drawn as a standalone node (reference uses SGs as boundary annotations)
- **azure/acr-geo-replication-image-flow**: Key Vault customer-managed key; Two regional AKS clusters with VNets/subnets and private endpoints (reference is a pure client-to-registry flow with no consumer infrastructure); Self-loop replicate edge on the registry
- **azure/adf-medallion-landing-zone-baseline**: Self-hosted integration runtime VM and snet-shir subnet; Private endpoint and snet-private-endpoints subnet; Databricks public and private subnets; VNet box (reference does not draw networking)
- **azure/aks-baseline-hub-spoke**: internet actor for firewall egress (reference has no explicit internet node); users actor (reference only shows an inbound arrow into Application Gateway)
- **azure/hub-spoke-firewall**: internet actor (SNAT egress target); user actor for Bastion access; subscription and resource group scaffolding per spoke
- **gcp/analytics-hub-data-sharing**: data-subscriber project with linked-market-dataset, linked-ticks-subscription, subscriber-analytics-dataset (reference shows only the publisher workflow); users actor; market-prices-table, market-ticks-topic (reference shows only generic datasets)
- **gcp/enterprise-hub-spoke-nva-inspection**: main VPC/subnetwork scaffolding holding onprem-router inside the hub project; nva-1, nva-2, nva-ilb-next-hop, nva-backend (NVA inspection is described in the source text but not in the cropped reference image); dns-hub-zone, hierarchical-fw-policy; hub-nat
- **gcp/global-alb-mig-cloud-sql-three-tier**: VPC network and two subnetworks; health check (web-hc) and instance template (web-template); Memorystore Redis (session-cache), Cloud SQL (app-db); Cloud Router + Cloud NAT (web-router, web-nat) and the internet node for egress; users actor (reference uses 'Internet' as the ingress source)
- **gcp/hub-and-spoke-ncc**: on-premises datacenter, HA VPN gateway (hub-vpn-gw) and Cloud Router (hub-router) with BGP; Cloud DNS private zone (corp-private-zone); internet node for the NAT egress

## Arrangement notes

- **aws/eventbridge-central-bus-multi-account**: Accounts were already left-to-right but the region/leaf order inside them was heuristic. Cells: accounts [1,1]/[1,2]/[1,3]; producer lambda -> local bus -> rule in one row; central bus at [1,1,2,1] with the routing rule top-right and archive below it (reference: bus left, rules right); consumer bus -> rule -> lambda in one row. Account-level resources are adopted into the sole region, so their cells are relative to the region grid.
- **aws/serverless-web-app-s3-apigw-cognito**: The image field points at the gen-AI variant tutorial (Amplify/AppSync/Lambda/Bedrock) while the source field is the classic S3/API Gateway/DynamoDB tutorial; content compared against the fetched image. Before: users sat on the right so every flow ran right-to-left with long detours, Route 53 at the bottom. Now root: users [1,1], account [1,2]. Region row 1: app-zone [1,1], global edge group [1,2], frontend-bucket [1,3]; row 2: app-cert [2,1], backend group [2,2,1,2]. Backend: Cognito [1,1] above API Gateway [2,1] (as Cognito sits above the entry point in the ref), then Lambda [2,2] and DynamoDB [2,3] left-to-right.
- **aws/tgw-hub-and-spoke**: The fetched -ref.jpg is the TGW Connect / SD-WAN figure from the same whitepaper page, not the hub-and-spoke-design.png the template's image field names; both draw the TGW as a central hub. Region grid 3x3: spoke-a [1,1] and spoke-b [3,1] on the left, spoke-rt [1,2] / hub-tgw [2,2] / egress-rt [3,2] in the middle column, egress VPC [1,3,3,1] on the right; inside egress VPC AZ a [1,1], AZ b [1,2], IGW [1,3]; AZ a stacks public [1,1] over tgw subnet [2,1]. Root: account [1,1], internet [1,2] so egress exits rightwards.
- **aws/three-tier-web**: The -ref.jpg is only the AWS Builder Center OG placeholder, not a diagram; compared against the whitepaper figure the source URL points to. Root: users [1,1] left of account. Region: Route 53 [1,1] and CloudFront [2,1] as a left edge column, VPC [1,2] spanning both rows. VPC: ALB [1,1,1,2] centered over the two AZ columns [2,1]/[2,2], web-sg [1,3] beside it; each AZ stacks public [1,1] over private [2,1].
- **azure/acr-geo-replication-image-flow**: Reference is a top-to-bottom flow: client, global endpoint, replica, data endpoint. Before: eastus stacked above westeurope on the left, registry RG floating to the right. After: user on top, rg-acr-global (registry + KV) spanning the top row, westeurope and eastus regional RGs side by side below with the private endpoint above the VNet so the pull path reads bottom-up toward the registry.
- **azure/adf-medallion-landing-zone-baseline**: Before: a tall VNet stacked four subnets vertically in the centre with ADF, storage, SQL, Key Vault and Log Analytics scattered on the right and Purview/Site Recovery on the left. After: Data Factory (ingest) left, VNet in the middle (SHIR subnet spanning left, Databricks subnets top, private-endpoints subnet below), storage account and SQL Server (store/serve) to the right, and a monitor-and-govern row underneath in reference order: Key Vault, Log Analytics (Monitor), Purview, Site Recovery. On-premises datacenter left, users right.
- **azure/aks-baseline-hub-spoke**: Before: workload subscription left, connectivity right, hub subnets stacked vertically, spoke subnets in one row with cluster nodes in the middle, KV/ACR/LAW in a column on the right. After: hub on top (Bastion | Firewall | Gateway left to right), spoke below with Private Link | Ingress | AppGW row over a full-width cluster-nodes subnet, Key Vault and Container Registry to the left of the spoke, Log Analytics (Monitor) to the right, operator user left of the hub, internet right of the hub, users right of the spoke.
- **azure/hub-spoke-firewall**: Before: prod spoke on the left, hub in the middle, nonprod spoke on the far right, on-premises and user floating far left, internet above the hub, peering lines wrapping around. After: user [1,1] and on-premises [2,1] on the left, connectivity subscription (hub) [1,2] spanning rows 1-2 in the centre with Bastion / Firewall / Gateway subnets stacked top to bottom as in the reference and the Log Analytics workspace to the right of the firewall (Azure Monitor position), internet [1,3] top right, prod spoke [2,3] right of the hub, nonprod spoke [3,2] below the hub as the reference draws it.
- **gcp/analytics-hub-data-sharing**: Before: subscriber on the left, publisher on the right, users far left, resources in a single row. After: publisher project on the left (reference left column) with shared resources in column 1 (table/dataset/topic) and exchange+listings in column 2 aligned per row so publish arrows are horizontal; subscriber project to the right as the consumer; users at far right since the reference reads publisher -> exchange left to right.
- **gcp/enterprise-hub-spoke-nva-inspection**: Before: prod and nonprod spoke projects were on the left, hub project on the right, on-premises top-left, peering lines crossing the hub. After: on-premises [1,1] on the left, hub project [1,2] spanning two rows in the middle, prod [1,3] and nonprod [2,3] stacked on the right; inside the hub the interconnect-facing 'main' VPC (onprem-router) is the left column with DNS zone and firewall policy under it, hub-shared-vpc spans the three rows to the right with VLAN attachments in column 1, routers/NAT in column 2 and the NVA backend above the NVA subnet in column 3.
- **gcp/global-alb-mig-cloud-sql-three-tier**: Before: LB objects were scattered in a two-column blob with the VPC to the right and criss-crossing edges. Cells: users [1,1], project [1,2], internet [1,3]; inside the project the LB chain forwarding rule -> HTTPS proxy -> URL map -> backend service is row 1 ([1,1]..[1,4]) with the VPC beside it at [1,5,2,1]; row 2 holds the supporting objects under their owners (global IP under the forwarding rule as the reference's 'IPv4 or IPv6 address' annotation, cert under the proxy, instance template and health check under URL map / backend). Inside the VPC: web-sub tall at [1,1,2,1], data-sub [1,2], Cloud SQL [2,2], router/NAT bottom row.
- **gcp/hub-and-spoke-ncc**: Before: the three projects sat in one row (spoke-a, hub, spoke-b) with the NCC hub tucked at the bottom of the hub project and spoke arrows crossing back. Cells: on-prem [1,1], hub project [1,2,1,2] on top, internet [1,4], spokes [2,2]/[2,3] centred below; inside the hub project the hub VPC spans [1,1,1,2] with the NCC hub resource beneath it at [2,1] (as the reference's 'Hub resource' between hub and spokes) and the DNS zone beside it; inside the hub VPC the VPN gateway + router form the left column facing on-prem, the subnet is the tall centre cell and the NAT sits lower right as in the reference.

## Full run (all templates with a bundled picture)

321 templates reviewed against their bundled picture; 241 annotated with grid cells; 80 pictures judged not to be that architecture's diagram (console screenshots, marketing strips, placeholders, another variant of the solution) and removed from the bundle.

| Cloud | Reviewed | Annotated | Picture removed |
|---|---|---|---|
| aws | 198 | 153 | 45 |
| azure | 48 | 36 | 12 |
| gcp | 75 | 52 | 23 |

### Most frequent gaps

Elements the references draw that templates lack (mentions across templates):

- route tables (4)
- quicksight (3)
- cloudwatch (3)
- microsoft entra id (3)
- iam (2)
- ci/cd pipeline (2)
- global accelerator (2)
- route 53 latency routing (2)
- memorydb (2)
- transit gateway (2)
- iam identity center (2)
- cloudwatch logs (2)
- aws backup (2)
- security hub (2)
- amazon cloudwatch (2)
- lake formation (2)
- spoke vpc (2)
- route 53 (2)
- amazon transcribe (2)
- amazon comprehend (2)
- amazon textract (2)
- aws iot core (2)
- secrets manager (2)
- bedrock agent (2)
- query/response messages (2)

Elements templates draw that the references leave implicit:

- cloudwatch log group (12)
- users actor (9)
- log analytics workspace (8)
- nat gateway (7)
- api gateway (7)
- internet gateway (7)
- alb (7)
- account wrapper (6)
- acm certificate (6)
- account and region wrappers (6)
- cognito user pool (6)
- internet (6)
- client actor (5)
- kms key (5)
- athena workgroup (5)
- vpc (5)
- account/region containers (5)
- route 53 zone (5)
- subscription/resource-group wrappers (5)
- azs and subnets (4)

The raw per-template findings are in `scripts/refarch/reconciliation.jsonl` (missing and extra elements, note, whether the picture was kept).

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
