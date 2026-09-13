# Catalog specification (v1)

The catalog is the single source of truth for everything iagram can draw. One
YAML file per element, validated against [`catalog/schema.json`](../../catalog/schema.json)
(JSON Schema 2020-12). The same file drives the palette, where the element can
be dropped, what it can be connected to, its property panel, validation, and
the Terraform it renders to. **There is no editor code per element or per
cloud.** This document is the contract; anything that passes the schema and the
cross-reference checks below loads in iagram, whether it ships with iagram or
not.

## Layout

```
catalog/
  schema.json                 # this spec, machine-readable
  <provider>/
    _provider.yaml            # Terraform provider configuration (one per provider)
    <name>.yaml               # one element; id must be "<provider>.<name>"
  icons/<provider>/*.svg
```

Load order: built-in catalog, then any directories passed with `--catalog DIR`
(same layout, rooted at `DIR`, i.e. `DIR/<provider>/<name>.yaml`). A later entry
with the same `id` replaces the earlier one. Cross-references are checked after
all layers are merged, so an external catalog may reference built-in types.

## `_provider.yaml`

```yaml
name: gcp                    # must equal the directory name
local_name: google           # Terraform provider name when it differs (gcp -> google, azure -> azurerm)
source: hashicorp/google     # required_providers source
version: "~> 6.0"
required_version: ">= 1.6.0" # optional terraform.required_version
extra_providers:             # optional, e.g. helpers modules need
  archive: {source: hashicorp/archive, version: "~> 2.4"}
```

## Element entry

```yaml
id: aws.ec2_instance          # <provider>.<name>, matches the file path
label: EC2 Instance
description: optional one-liner shown in the property panel
category: compute             # account|network|compute|database|storage|security|serverless|integration
icon: aws/ec2.svg             # under catalog/icons/
icon_variants:                # optional: icon when a boolean prop is true
  public: aws/subnet_public.svg
kind: leaf                    # leaf | container
size: {w: 120, h: 90}         # default canvas size

allowed_parents: [aws.subnet] # WHERE it may be dropped; "root" = the canvas itself
allowed_children: [...]       # containers only, optional whitelist

connections:                  # WHAT it may be connected to (see below)
  out:
    - {to: aws.rds_instance, kind: connects_to, label: connects to,
       terraform: {set: to, input: ingress_security_group_ids, value: from.security_group_id}}
  in:
    - {from: aws.alb, kind: routes_to, ...}

props:                        # JSON Schema (object) for the property panel + validation
  type: object
  required: [instance_type]
  properties:
    instance_type: {type: string, default: t3.micro, enum: [...]}
    root_volume_gb: {type: integer, default: 20, minimum: 8}
    public_ip: {type: boolean, default: false}
    cidr: {type: string, format: cidr}

terraform:
  role: module                # module (default) | account | region
  module: aws/ec2_instance    # path under modules/
  inputs_from_parent:         # input <- ancestor output
    subnet_id: parent.subnet_id
    vpc_id: parent.parent.vpc_id
  collect:                    # input <- outputs of all nodes of a type under an ancestor
    subnet_ids: {type: aws.subnet, under: parent.parent, output: subnet_id, min: 2, distinct: az}
outputs: [instance_id, private_ip]   # module outputs written back after apply
```

Property names may not be `source`, `version`, `providers`, `count`,
`for_each`, `depends_on`, `name` or `tags`: they become module inputs and
would shadow meta-arguments or the inputs the generator sets.

### Semantics

- **Placement is containment.** A node may be created only inside a node whose
  type is in its `allowed_parents` (or on the canvas when `root` is listed). If
  the parent lists `allowed_children`, the child must be in it. The editor makes
  invalid placement impossible; the validator re-checks headless.
- **Arrows are flow.** A connection is allowed only if a rule exists for the
  pair `(from type, to type)`. Exactly one rule per ordered pair. `kind` names
  what the arrow means; `label` is what the canvas shows. Never encode the same
  fact as both containment and an arrow. `requires_from` / `requires_to` list
  property values one end must have for the arrow to work (a DNS record needs
  the VM to have `public_ip: true`); the validator reports violations on the
  arrow.
- **Properties** support JSON Schema types `string` (`enum`, `pattern`,
  `format: cidr`), `integer`/`number` (`minimum`, `maximum`) and `boolean`.
  `default` values are applied when a node is created. `required` is enforced.
  Two iagram annotations shape the settings panel: `group: <section title>`
  puts the field under a collapsible section (default "Settings"), and
  `advanced: true` moves it under a collapsed "Advanced" section. `title` and
  `description` are shown as the field label and help text.
- **CIDR rules** are generic: any node with a `cidr` property must lie within
  the nearest ancestor that has one, and siblings must not overlap.

### Terraform rendering

- `role: module` emits `module "<sanitized node id>" { source = "./modules/<module>", name, tags, <props...>, <inputs> }`.
  Every property becomes an input with the same name.
- `role: account` / `role: region` emit **provider** blocks instead. `provider_args`
  is a template; `"${prop}"` strings are replaced by the node's property, and
  keys or list items whose property is empty are dropped:
  ```yaml
  terraform:
    role: region
    provider_args: {region: ${region}}
  ```
  Each region node becomes an aliased provider (`alias = <node id>`), inheriting
  the account's args. Every module under it gets `providers = { <provider> = <provider>.<alias> }`.
- `inputs_from_parent`: `parent`, `parent.parent`, ... followed by an output
  name. If that ancestor is not a module node (an account or region), the input
  is omitted, so one entry can accept several parent types (a Lambda in a
  region or in a subnet). Wrap the reference in brackets (`"[parent.subnet_id]"`)
  to pass a one-element list instead (empty when unresolved): modules must
  branch on `length()` of such lists, which OpenTofu knows at plan time, never
  on whether a referenced value is empty, which it does not.
- `collect`: gathers `${module.X.<output>}` for every node of `type` placed
  anywhere under the named ancestor, as a list. `min` makes the validator
  require that many gathered nodes; `distinct` names a property that must
  differ across at least `min` of them (two subnets in different AZs for a
  DB subnet group or a load balancer).
- Connection `terraform`: `{set: from|to, input, value: from.<output>|to.<output>}`
  appends the referenced output to the list `input` of the module on the `set`
  side. Lists are sorted for deterministic output.
- Outputs: the root emits `output "<module>" { value = {<outputs...>} }`;
  `iagram apply` reads them and writes them onto `nodes[].outputs` in the
  diagram file.

### Import mapping (`terraform.import`)

`iagram import` walks a Terraform state and uses these mappings to rebuild
nodes and containment:

```yaml
terraform:
  import:
    resource: aws_instance                  # primary resource type of the module
    parent: subnet_id                       # attribute holding the parent's primary id
    id: id                                  # attribute other resources reference (default id)
    name: tags.Name                         # attribute path for the node name (default: state name)
    props: {instance_type: instance_type, az: "availability_zone|last", scheme: "internal|bool:internal:internet-facing"}
```

Attribute paths walk maps and lists (`vpc_config.0.subnet_ids.0`); the
transforms are `|last` (last character) and `|bool:<if-true>:<if-false>`. For
`account` / `region` roles, `from` names where the grouping value comes from:
an attribute (`project`, `location`), `arn.account` / `arn.region` (parsed from
an `arn` attribute) or `arm.subscription` (parsed from an Azure resource id);
`prop` receives it. Resources with no resolvable parent fall into the sole
known container of an allowed type, and the report says so. Arrows are not
recovered.

### Module contract

See [`modules/README.md`](../../modules/README.md). In short: variables `name`
and `tags` exist; every prop, every input named above and every list input
(default `[]`) is a variable; every `outputs:` entry is an output; modules never
configure providers.

## Adding a provider

Nothing in iagram's code names a cloud. To add one:

1. `catalog/<provider>/_provider.yaml` with the Terraform provider source.
2. An `account`-role entry (`allowed_parents: [root]`) whose `provider_args`
   map its props (project, subscription, profile...) to the provider block, and
   usually a `region`-role entry under it.
3. Container entries for the network hierarchy (`gcp.vpc` → `gcp.subnetwork`,
   `azure.vnet` → `azure.subnet`), then leaf entries.
4. Icons under `catalog/icons/<provider>/`.
5. Modules under `modules/<provider>/...` following the module contract.
6. Run `iagram catalog check <dir>` (schema + cross-references) and `make test`.

A provider can live in its own repository and be loaded with `--catalog`; the
built-in ones are simply the layers iagram ships with.

## Stability

This is spec version 1. Additive changes (new optional fields) do not bump the
version. Removing or changing the meaning of a field bumps it, and iagram keeps
loading the previous version for at least two minor releases.

## Generated elements

Every resource type of a provider is available as a generated element with id
`<provider>.res.<terraform type>`, synthesised from the provider schema
snapshot (`schemas/<local name>.json.gz`, produced by `iagram schemas update`
from `tofu providers schema -json`). Its settings are the configurable
attributes of the schema (required/optional; computed-only attributes are
outputs); complex types and nested blocks are edited as JSON. It renders as a
plain `resource "<type>" "<node name>"` block bound to the region's provider
alias. It may be placed in any container of its provider and may reference any
element of its provider with a `references` arrow that sets one of its
attributes to `${<target address>.<output>}` (lists append). Curated elements
never reference generated ones automatically; their inputs are fixed by their
rules.

### Palette families

The palette lists one entry per official icon of a provider. Where a curated
element uses that icon, the curated element is the entry; otherwise a
*family* tile is shown whose drop creates the family's default resource type
(`defaults:` in `services.yaml`, else the type most other types bind to). The
family's other first-level types are offered as the element's **Resource
type** in the settings panel. Searching the palette also surfaces individual
first-level types.

### First-level elements and attachments

A generated element is *first-level* (drawn, in the palette) when its
resource type maps to an official provider icon in
`catalog/icons/<provider>/services.yaml` (longest type-prefix match) and no
other type owns it. Ownership is derived from the provider schemas:

- **name extension**: `aws_s3_bucket_versioning` extends `aws_s3_bucket`,
  `google_project_iam_member` extends `google_project`, unless the shorter
  type is the one referencing the longer (`google_sql_database` references
  `google_sql_database_instance`, so the instance is first-level) or the
  shorter type is a container (`aws_vpc_endpoint` lives in a VPC);
- **binding attribute**, within the same service: an attribute named after
  the owner's full type name (`storage_account_name`), or, within the same
  icon family, after its last word(s) either bare (`cluster`, `instance`,
  `topic`) or with a required `_id`/`_arn`/`_name` suffix (`function_name`,
  `target_key_id`). Container attributes (`vpc_id`, `subnet_id`,
  `resource_group_name`, `project`, `network`…) and generic words (`policy`,
  `name`, `key`, `role`…) never count; nested blocks never count.

Owned types split in two. *Components* are members deployed inside a
cluster-like owner (types ending in `_cluster` / `_replication_group` whose
owned types end in `node_group`, `node_pool`, `service`, `instance`,
`instance_group`, `broker`, `worker`, `pool`…): the owner becomes a
container box and the component is drawn inside it, placeable only there,
bound to it through its binding attribute. Every other owned type is an
*attachment*: it may be placed inside any element of
its provider (container or not), is not drawn, and is configured from its
parent's settings panel, bound to the parent through a `references` edge on
its binding attribute (`${<parent address>.<id|arn|name>}`). The `default_*`
adoption resources are attachments as well. Cross-service references between
first-level elements are arrows.

## Equivalences (`catalog/equivalences.yaml`)

`iagram convert --to <provider>` uses this table: `elements` rows list the
counterpart id per provider with property renames (`props`, `props_azure`)
and size classes (`class` → `classes`), `regions` maps region names, and
`roots` gives each provider's container chain and which property receives the
region. Elements, connections and properties without a counterpart are
reported and dropped.

## Drawing style (`style`, `style_variants`)

Containers may declare how they are drawn, following the provider's official
diagram grammar (for AWS: the group conventions of the Architecture Icons
deck):

```yaml
style: {border: "#8C4FFF", fill: "rgba(140,79,255,.04)", dash: solid, label: left}
style_variants:
  public: {border: "#7AA116"}   # applied when props.public is true
```

`border` and `fill` are CSS colours (`fill` defaults to a 5 % tint of the
border), `dash` is `solid | dashed | dotted`, `label` is `left | center`.
Variants are merged over `style` in declaration order for every boolean
property that is true.

## Icon sets (`catalog/icons/<provider>/services.yaml`)

```yaml
prefixes:   # Terraform type prefix (after the provider prefix) -> service icon
  s3: aws/services/amazon_simple_storage_service.svg
resources:  # prefix -> resource (line-art) icon; drawing only
  nat_gateway: aws/resources/amazon_vpc_nat_gateway.svg
defaults:   # prefix -> Terraform type created when the family tile is dropped
  s3: aws_s3_bucket
labels:     # icon file -> palette label, when the file name reads badly
  aws/services/amazon_simple_storage_service.svg: S3
```

The longest matching prefix wins. `prefixes` decide which types form one
*service* (used to classify attachments and components); `resources` only
change the icon that is drawn, and every distinct drawn icon becomes its own
palette tile.

## Provider-neutral vocabulary (`catalog/common`)

The `common` provider declares `no_terraform: true` and needs no `source`.
Its elements are the diagram vocabulary of the official reference
architectures that has no Terraform counterpart:

| Element | Kind | Purpose |
|---|---|---|
| `common.group` | container, `transparent` | labelled box: tier, stage, tenant, cell |
| `common.datacenter` | container, `transparent` | corporate data center, branch, partner network |
| `common.note` | leaf | free text (`text`, multiline) |
| `common.user`, `users`, `client`, `mobile`, `internet`, `saas`, `device`, `server`, `database`, `idp`, `documents`, `email` | leaf | actors drawn as bare icons with an optional `caption` |

Rules that make them cloud-neutral:

- `allowed_parents: ["*"]` means any container or the canvas.
- A **transparent** container has no Terraform meaning. Elements placed
  inside it are validated, wired (`inputs_from_parent`) and generated as if
  they were placed in the container's own parent: a subnet in a group in a
  VPC is a subnet of that VPC.
- An edge with an actor or note at either end is a `flow` edge: drawn as an
  arrow, sets no Terraform input, always valid.
- The generator ignores `common.*` nodes silently; `iagram convert` and the
  mirror tabs copy them unchanged.

## Zone boxes and provided properties (`provides`)

A container may give properties to everything drawn inside it:

```yaml
id: aws.availability_zone
kind: container
transparent: true
allowed_parents: [aws.region, aws.vpc]
provides:
  az: "${zone}"                      # curated subnet property
  availability_zone: "${region}${zone}"   # generated resources (EC2, EBS...)
props: {type: object, properties: {zone: {type: string, enum: [a, b, c]}}}
```

`provides` maps a child property to a template; `${x}` is the providing
node's property `x`, or the nearest ancestor's when the box lacks it (the
Region's `region` above). Only properties the child's schema declares are
given. The canvas fills them in when an element is dropped or moved into the
box and shows them read-only; the generator lets the box win over any drawn
value; the validator warns when a file disagrees with its box. Zone boxes are
transparent, so a subnet in an Availability Zone in a VPC is still a subnet
of that VPC. `iagram import` draws zone boxes around siblings whose zone
values differ. Shipped: `aws.availability_zone`, `gcp.zone`, `azure.zone`
(mapped to each other by the equivalence table).

## Link resources (`catalog/<provider>/_links.yaml`)

Some Terraform resources mean "connect A to B": VPC peering, Transit
Gateway attachments, Site-to-Site VPN connections, Direct Connect gateway
associations, Route 53 zone associations, RAM shares, target group
attachments. Official diagrams draw them as lines. `_links.yaml` lists them:

```yaml
links:
  aws_vpn_connection:
    label: Site-to-Site VPN
    from: customer_gateway_id
    to: transit_gateway_id|vpn_gateway_id      # alternatives, picked by the target's type
    style: {direction: none, dash: dashed, color: "#1d8bd6"}
  aws_networkmanager_vpc_attachment:
    from: core_network_id
    to: {attr: vpc_arn, output: arn}           # referenced output (default id)
  aws_lb_target_group_attachment:
    from: {attr: target_group_arn, output: arn}
    to: {attr: target_id, types: [aws_instance, aws_lambda_function]}   # explicit accepted types
```

Each end names the attribute receiving the reference; the accepted element
types are derived from the attribute name (`transit_gateway_id` accepts
`aws_ec2_transit_gateway` and the curated elements importing it) unless
`types` lists them. Consequences:

- Connecting two elements a link can join creates an edge of kind `link`
  with `type` (the link element), `name` and `props`; the arrow's inspector
  shows the resource's attributes (both ends excluded) and lets you switch
  between candidate links (VPC to VPC: peering; VPC to Transit Gateway:
  attachment).
- The generator renders one resource block per link edge, the two end
  attributes referencing the source and target addresses; plan results are
  mapped back onto the line.
- Link types never appear in the palette or as attachments. `iagram import`
  turns link resources whose ends resolve into link edges.
- Lines default to no arrow head (`direction: none`) because they are
  associations, not traffic.

## Spanning groups (`catalog/<provider>/_spans.yaml`)

An Auto Scaling group is drawn as a band across the subnets it uses. A span
declares that band:

```yaml
spans:
  aws_autoscaling_group:
    label: Auto Scaling group
    attr: vpc_zone_identifier      # list attribute filled with the covered containers
    over: [aws_subnet]             # what the band may cover
    ghost: {count: desired_capacity, icon: aws/resources/amazon_ec2_instance.svg}
    icon: aws/groups/auto_scaling_group.svg
    style: {border: "#ED7100", dash: dashed, label: center}
```

The element becomes a container drawn above its siblings and below leaves,
never a drop target. Whenever the band or the containers under it move or
resize, the canvas recomputes which containers it covers (40 % of their area
or more) and mirrors that as hidden `references` edges from the band to each
covered container on `attr`; the generator turns them into the list value.
The `.iad` file therefore stays plain: a node plus reference edges. Ghost
tiles show the capacity (`ghost.count` property) without creating resources.
