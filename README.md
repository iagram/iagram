<p align="center">
  <strong>iagram</strong><br>
  <em>Infrastructure as diagram.</em> Draw your cloud architecture on a canvas that only lets you draw things that can exist; OpenTofu builds it.
</p>

<p align="center">
  <a href="https://github.com/iagram/iagram/actions/workflows/ci.yml"><img alt="ci" src="https://github.com/iagram/iagram/actions/workflows/ci.yml/badge.svg"></a>
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/badge/license-Apache--2.0-blue"></a>
  <a href="https://github.com/iagram/iagram/releases"><img alt="release" src="https://img.shields.io/github/v/release/iagram/iagram?include_prereleases"></a>
</p>

---

**The diagram is the source of truth. Terraform is the engine.** iagram is a single local binary, like `terraform`: no server, no account, no hosted component. You draw; it generates Terraform, runs OpenTofu with the credentials already on your machine, and paints the plan back onto the drawing.

```
$ iagram init
$ iagram up            # opens http://localhost:7777
   draw → Plan → review the colours → Apply
$ git add iagram.iad   # the diagram is a file (.iad = infrastructure as diagram); commit it like code
```

## Contents

- [Why](#why)
- [Install](#install)
- [Quick start](#quick-start)
- [How it works](#how-it-works)
- [The editor](#the-editor)
- [Commands](#commands)
- [Providers and elements](#providers-and-elements)
- [Bringing existing infrastructure](#bringing-existing-infrastructure)
- [Running in Docker](#running-in-docker)
- [Security posture](#security-posture)
- [Extending iagram](#extending-iagram)
- [Project layout](#project-layout)
- [Development](#development)
- [Roadmap and status](#roadmap-and-status)
- [License](#license)

## Why

Infrastructure diagrams and infrastructure code drift apart the day after they are drawn. iagram removes the gap by making the drawing *be* the definition:

- **Nothing invalid is drawable.** A catalog defines every element, where it may be placed, what it may connect to and which properties it takes. An EC2 instance can only be dropped into a subnet; an ALB can only be wired to targets it can route to; sibling subnets cannot overlap; RDS needs two subnets in two AZs, and the canvas tells you before Terraform would.
- **Placement is containment, arrows are meaning.** A node inside a subnet inside a VPC gets `subnet_id` and `vpc_id`. An arrow from a load balancer to an instance registers the target; from a workload to a bucket grants an IAM policy; from a security group to a database attaches it. The connection inspector shows exactly what each arrow does in Terraform.
- **The plan is painted on the diagram.** Green create, amber update, red destroy, per element, with the resource list in the panel. Apply runs the exact plan you reviewed and writes outputs (IPs, endpoints, ARNs) back onto the nodes. Drift checks paint what changed outside iagram.
- **Local, boring, auditable.** One binary. Credentials come from the same local chain the cloud CLIs use and never pass through iagram. Every outbound call the binary makes is listed in [SECURITY.md](SECURITY.md). Telemetry is off unless you turn it on.
- **Every Terraform resource, both ways.** Besides the curated elements, iagram derives an element for **every resource type of the AWS, Google and Azure providers** (3,958 today) from the providers' own schemas, with a settings panel generated from the schema and a plain `resource` block as output. References between them are arrows or attributes, kept as Terraform expressions. `iagram import --hcl` turns existing `.tf` into a diagram and `generate` turns it back: import → generate → import is a fixed point.
- **Cloud-agnostic core.** No code in iagram names a cloud. AWS, Google Cloud and Azure ship as catalog directories plus Terraform modules; a fourth is a directory of YAML and modules. `iagram convert --to gcp` rewrites a diagram for another provider from an equivalence table.

## Install

| Method | Command |
|---|---|
| Homebrew (macOS, Linux) | `brew install --cask iagram/tap/iagram` |
| Script (macOS, Linux; verifies `SHA256SUMS`) | `curl -fsSL https://raw.githubusercontent.com/iagram/iagram/main/install.sh \| sh` |
| pip (any platform; downloads the matching release on first run, verified) | `pip install iagram` |
| Docker | `docker run --rm -it -p 127.0.0.1:7777:7777 -v "$PWD:/work" ghcr.io/iagram/iagram` (see [Running in Docker](#running-in-docker)) |
| Binary | [Releases](https://github.com/iagram/iagram/releases): macOS and Linux (amd64, arm64), Windows (amd64) |
| From source | `make build` (Go 1.26+, Node 22+) |

OpenTofu is downloaded once on first `plan`, version-pinned and checksum-verified, into `~/.iagram/bin`. Set `IAGRAM_TOFU_PATH` to use your own `tofu` or `terraform` binary instead.

## Quick start

```
mkdir my-infra && cd my-infra
iagram init                # creates iagram.json and .iagram/ (state, plans; gitignored)
iagram up                  # opens the canvas
```

In the canvas:

1. Pick a provider tab (AWS, Azure, Google Cloud; the other tabs will mirror what you draw) and drag an **Account** (or Project / Subscription) onto the canvas, then a **Region**, a **VPC**, **Subnets**, and the resources inside them. Containers turn green or red while you drag to show where a drop is allowed.
2. Click an element to configure it in the panel on the right: typed fields, dropdowns, required markers, inline validation, grouped into sections.
3. Connect elements by dragging from the right handle of one to the left handle of another: valid targets light up green, forbidden ones show a ✕. The connection's configuration opens as soon as it attaches. You can also attach by reference from the settings panel (🔗 on an attribute).
4. **Plan**. iagram saves, generates Terraform under `.iagram/tf/`, runs `tofu init` and `tofu plan`, streams the log, and colours the nodes.
5. **Apply**. Confirm the summary (destroys are called out in red). Outputs appear on the nodes when it finishes.
6. Later: **Check drift** to see what changed outside iagram; edit the diagram and Plan again to reconcile; **⋯ → Destroy infrastructure…** to tear everything down (asks you to type the diagram name).

Or headless, for CI and scripts:

```
iagram validate && iagram plan && iagram apply
```

Try the examples: `cd examples/aws-three-tier && iagram up` (also `gcp-web`, `azure-web`).

## How it works

```
iagram.json ──validate──▶ generate ──▶ .iagram/tf/main.tf.json + modules/ ──▶ tofu init / plan ──▶ plan JSON ──▶ canvas
                                                                                     └─ apply ──▶ outputs ──▶ nodes[].outputs
```

- **`iagram.iad`** is the diagram (JSON inside): nodes (type, name, parent, properties, layout), edges (kind, source, target, and for references the attribute/output). Deterministic, readable diffs, versioned with a migration path. Legacy `iagram.json` still loads. Spec: [docs/spec/document.md](docs/spec/document.md).
- **The catalog** (`catalog/<provider>/*.yaml`) is the single source of truth: one YAML file per element drives the palette entry, the drop rules, the connection rules, the settings panel, validation, the Terraform module call and the import mapping. Spec: [docs/spec/catalog.md](docs/spec/catalog.md), enforced by [`catalog/schema.json`](catalog/schema.json).
- **Two tiers of elements.** *Curated* elements (39) are opinionated modules with semantic arrows (`routes_to`, `protects`, …). *Generated* elements exist for every other resource type, with settings derived from `tofu providers schema -json` (shipped compressed in the binary, refresh with `iagram schemas update`). A generated element renders as a plain `resource` block named after the element, so imported configurations regenerate with the same addresses; its arrows are `references` that set an attribute to `${type.name.attr}`.
- **Icon-first palette.** The palette holds **one element per official provider icon** (`catalog/icons/<provider>/services.yaml`): the curated element when one exists, else a *family* tile (Lambda, IAM, Kinesis…). An element's Terraform resource type is chosen in its settings among the first-level types behind that icon (IAM → role, user, group, policy…; drop creates the family default). Types that are owned by another resource are never in the palette: Ownership is derived from the schemas: a type whose name extends another's (`aws_s3_bucket_versioning`), or that must reference an owner of the same family (`aws_lambda_alias.function_name`, `aws_ecs_service.cluster`, `google_container_node_pool.cluster`, `azurerm_storage_container.storage_account_name`) is an *attachment*: added and configured inside the settings panel of the element it belongs to (a ⚙ badge shows how many), generated alongside it, never drawn as a box. Placement references (`vpc_id`, `subnet_id`, `resource_group_name`, `project`) are containment, not ownership, so instances, gateways and endpoints stay first-level. Cross-service references are arrows.
- **Boxes and components.** An element that holds deployed members is drawn as a **box**: clusters (EKS/GKE/AKS, ECS, Aurora, EMR…) contain their node groups, node pools, services and cluster instances, which are *components*: owned, but drawn inside the box and configured by clicking them. A box sits at the level that spans its zones (the VPC), and the subnets it uses are collected through the schema. Multi-zone elements without member resources (a load balancer, a Multi-AZ RDS instance, an MSK cluster) stay single nodes.
- **Generation** emits one `module` block per curated node and one `resource` block per generated node (named after the node id, `source = ./modules/<provider>/<type>`), provider blocks from account/region nodes (aliased per region, so one diagram can span regions and clouds), inputs wired from the containment chain and the arrows, and root outputs per node. Modules are embedded in the binary and materialised next to the config, so `.iagram/tf/` is plain Terraform you can inspect or eject to at any time.
- **Execution** is OpenTofu, run as a subprocess that inherits your shell environment. Plan and apply stream over a local job API; the plan JSON is mapped back to nodes by module name. State is local under `.iagram/tf/` by default, like a fresh Terraform project (configure a remote backend there if you want one).

## Reference architectures

The front page of the editor is a gallery of official reference architectures from AWS, Google Cloud and Azure (hub and spoke, three-tier web, fan-out, data lake, RAG, multi-region DR, landing zones, CI/CD...), redrawn as `.iad` diagrams that validate and generate Terraform. Filter by cloud, category or text, read the source page, and open one in the editor with everything pre-configured; then adapt the names, ids and sizes and plan. The gallery is the entry page: it opens on every launch (`?gallery=0` skips it) and from File > Reference architectures; "Open my diagram" takes you to the current file.

Each template is a compact spec under `templates/<provider>/<slug>/template.yaml` (see `docs/spec/templates.md`). `iagram templates list` prints them, `iagram templates build DIR` writes their `.iad` files, and `iagram init --template aws/tgw-hub-and-spoke` starts a project from one. Placeholders such as `changeme`, the all-zero subscription id or `my-project-123456` mark the values you must set.

## The editor

| Area | What it does |
|---|---|
| **Canvas tabs** | One tab per provider. In **mirror** mode (default) the tab you draw on is the source and the other tabs show its live equivalent through the equivalence table, dropped elements listed in a banner; editing a mirrored tab makes that provider the source. Turn mirroring off for independent canvases in one file. Plan/Apply run the source. |
| **Palette** (left) | Search; curated elements grouped by category; **More services**: one tile per official icon (its resource type is picked in the settings). Searching also lists individual matching resource types. Items that fit the selected container are highlighted, the rest dimmed. Drag onto the canvas. |
| **Canvas** | Nested containers drawn with the official colour grammar (account → region → Availability Zone → VPC → subnet, organization → OU), generic groups, corporate data centers, actors (users, clients, internet, devices, SaaS), notes, Auto Scaling bands spanning subnets, typed arrows with labels, step callouts and styles (both ways, dashed, inactive), link lines for peering / attachments / VPN, plan and drift overlays, legend. Shift-drag to select several, ⌘-click to add. Drag an element into another container to move it there (refused with a message if the catalog forbids it); right-click for cut / copy / paste / z-order. |
| **Settings panel** (right) | Name, **Annotation** (caption, step number), properties in sections (`Compute`, `Networking`, `Advanced`…), **Attachments** (add and configure the non-graphical resources that apply to this element), **Components** for cluster boxes, validation messages, planned resource changes, drift details, live outputs after apply (click to copy), delete. Click an arrow for the connection inspector: label, step, direction, line style, colour, and for link lines the Terraform resource and its attributes. With nothing selected: the walkthrough steps editor and the legend toggle. |
| **Menubar and toolbar** | File (new, open, import from Terraform state, save, download, export Terraform / PNG / SVG), Edit (undo, redo, clipboard, select all), View (zoom, labels, minimap, snap, theme), Infrastructure (plan, apply, drift, destroy, log), Help (shortcuts, docs, about); quick buttons for undo/redo, zoom, labels, theme, Save, Check drift, Plan, Apply. |
| **Log drawer** | Streams `tofu` output for plan/apply/drift; lists orphaned state that a plan would destroy. |

Shortcuts: ⌘S save · ⌘Z / ⌘⇧Z undo, redo · ⌘X / ⌘C / ⌘V / ⌘D cut, copy, paste, duplicate · ⌘] / ⌘[ (⇧ for front/back) z-order · ⌘A select all · Esc deselect · ⌫ delete · `?` shortcut list · double-click an element to focus its settings. Deep links: `?theme=dark`, `?provider=gcp`, `?legend=1`, `#select=<node id>`.

## Commands

```
iagram init [name] [--template P/SLUG]
                              create iagram.iad (optionally from a reference architecture) and .iagram/
iagram up [-p PORT] [--host ADDR] [--no-open]
                              open the canvas (127.0.0.1 by default; 0.0.0.0 only inside Docker)
iagram validate               check the diagram against the catalog; exit 1 on errors
iagram generate               write .iagram/tf/main.tf.json and modules/
iagram plan                   generate + tofu init + plan, summarised per element
iagram apply                  apply the last plan; write outputs back onto the diagram
iagram drift                  refresh-only plan; exit 1 if infrastructure drifted
iagram destroy [--yes]        plan the teardown, ask for the diagram name, apply; clear outputs
iagram import --state FILE    build a diagram from a terraform.tfstate or `tofu show -json`
iagram catalog check DIR      validate an external catalog directory
iagram templates list         list the shipped reference architectures
iagram templates build DIR    write them as DIR/<provider>/<slug>/iagram.iad
iagram telemetry on|off|status
iagram version
```

Diagram commands accept `-f FILE` and repeatable `--catalog DIR`. Environment: `IAGRAM_TOFU_PATH` (your own tofu/terraform), `IAGRAM_HOME` (default `~/.iagram`), `IAGRAM_TELEMETRY=0`.

## Providers and elements

| Provider | Containers | Elements |
|---|---|---|
| **AWS** | organization, organizational unit, account, region, Availability Zone, VPC, subnet, EKS cluster, Auto Scaling group (band) | security group, EC2, RDS, S3, ALB, Lambda, SQS, DynamoDB, CloudFront, Route 53, plus one palette tile per official service or resource icon (about 190) covering every resource type of the provider |
| **Google Cloud** | project, zone, VPC network, subnetwork, GKE cluster | firewall rule, Compute Engine, Cloud SQL, Cloud Storage, Cloud Run, Pub/Sub, BigQuery, GKE, Cloud DNS |
| **Azure** | subscription, resource group, availability zone, virtual network, subnet, AKS cluster | network security group, Linux VM, storage account, PostgreSQL Flexible Server, Key Vault, Container Registry, AKS, DNS zone |

Cloud-neutral vocabulary (every tab): generic group, corporate data center, note, and actors (user, users, client, mobile, internet, SaaS, device, server, database, identity provider, documents, email); they never generate Terraform and survive conversion between clouds.

Link lines (Terraform resources drawn between two boxes): VPC peering, Transit Gateway VPC and peering attachments, Site-to-Site VPN, Direct Connect gateway association, Route 53 zone association, Cloud WAN attachment, target group attachment, RAM share; GCP network peering and NCC spokes; Azure VNet peering, hub connection, private DNS link.

Arrows and what they generate (examples): `routes_to` (ALB → EC2/Lambda: target groups), `protects` (security group / NSG / firewall → workload), `connects_to` (workload → database: ingress from the workload's identity group, or Cloud SQL client role), `reads_writes` / `publishes_to` / `sends_to` (workload → bucket / table / topic / queue: least-privilege IAM), `triggers` (SQS → Lambda: event source mapping), `pulls_from` (AKS → ACR), `reads_secrets` (VM → Key Vault), `serves_from` (CloudFront → S3/ALB), `resolves_to` (DNS zone → load balancer / distribution / VM).

Modules default to the secure option: encrypted storage, IMDSv2, private databases, no public buckets, generated SSH keys and system identities, and roles derived only from the arrows you drew. Each example under `examples/` is validated against the real providers in CI.

## Bringing existing infrastructure

```
iagram import --hcl ./infra -o iagram.iad          # from Terraform configuration
iagram import --state terraform.tfstate -o iagram.iad   # from state (or: tofu show -json | iagram import --state -)
iagram up
```

From **configuration**, every `resource` block becomes a generated element with its literal attributes; references (`aws_vpc.main.id`) become arrows, other expressions (`var.x`, functions) are kept verbatim, provider blocks become account/region containers, and `variable`/`module`/`data` blocks are reported as skipped. `iagram generate` then produces an equivalent configuration, and importing that again yields the same diagram.

From **state**, curated mappings recover nodes and containment; every other resource type falls back to its generated element with the configurable attributes from state, and attribute values that equal another resource's id become reference arrows. Example: `examples/import/`.

Convert a diagram between providers with `iagram convert --to azure` (or File → Convert diagram to…): elements, size classes and regions map through [`catalog/equivalences.yaml`](catalog/equivalences.yaml); whatever has no counterpart is listed.

## Running in Docker

`compose.yaml` runs the canvas from a container against the diagram in the current directory:

```
docker compose up                    # http://localhost:7777
docker compose run --rm iagram plan
docker compose run --rm iagram apply
```

It publishes the port on `127.0.0.1` only, mounts `~/.aws`, `~/.config/gcloud` and `~/.azure` read-only (the same files the binary would read natively), passes the usual credential environment variables through, runs as your host uid/gid (`IAGRAM_UID`/`IAGRAM_GID`, so files in the mounted directory stay yours on Linux), and keeps OpenTofu and the provider cache in a named volume. Build locally with `docker compose build`, or pull `ghcr.io/iagram/iagram:<version>` (multi-arch images are published by the release pipeline).

## Security posture

- **Credentials never pass through iagram.** `tofu` runs as a child process with your shell's environment; providers authenticate the way they always do. The only code that launches it is [`internal/tofu/run.go`](internal/tofu/run.go).
- **Network.** The binary contacts GitHub once to download the pinned, checksum-verified OpenTofu; everything else is `tofu` talking to registries and your cloud. Telemetry is **off by default** and, when opted in, sends only the whitelisted fields in [docs/telemetry.md](docs/telemetry.md).
- **Local server.** `iagram up` listens on loopback, rejects foreign `Origin` headers, and exposes only the current directory's diagram and operations. Do not port-forward it.
- **Files.** `iagram.json` has no secrets and is meant to be committed. `.iagram/` (generated Terraform, plans, state) is gitignored; state may contain secrets, as any Terraform state does.

Full statement and reporting instructions: [SECURITY.md](SECURITY.md).

## Extending iagram

- **Add an element**: one YAML file under `catalog/<provider>/` plus a Terraform module under `modules/<provider>/<name>/`. The YAML declares placement, connections, properties (JSON Schema with `group`/`advanced` panel hints), the module mapping and the import mapping. Guide: [CONTRIBUTING.md](CONTRIBUTING.md); contract: [docs/spec/catalog.md](docs/spec/catalog.md).
- **Add a provider**: a directory with `_provider.yaml`, an account-role entry, usually a region-role entry, containers, leaves, icons and modules. [Recipe](docs/spec/catalog.md#adding-a-provider). Providers can live in their own repository and load with `--catalog DIR`; `iagram catalog check DIR` validates them.
- **Document format**: [docs/spec/document.md](docs/spec/document.md) and [`document.schema.json`](docs/spec/document.schema.json). Additive changes never bump the version; breaking ones come with a migration.

## Project layout

```
cmd/iagram/            entry point
internal/cli           commands
internal/catalog       catalog model, loader, rules, schema check
internal/document      iagram.json model, canonical save, migrations
internal/validate      the authoritative validator (also runs headless)
internal/generate      document -> main.tf.json
internal/tofu          OpenTofu install (pinned, verified), runner, plan summary
internal/workspace     .iagram/tf orchestration: generate, plan, apply, drift, destroy
internal/importer      Terraform state -> diagram
internal/layout        grid auto-layout for imports
internal/jobs          one-at-a-time background jobs with streaming logs
internal/server        local HTTP API + embedded UI
internal/telemetry     opt-in usage events (whitelisted)
catalog/               the specification of what can be drawn (YAML + icons + equivalences)
schemas/               compact provider schema snapshots (every resource type)
internal/tfschema      provider schema extraction and registry
internal/convert       cross-provider conversion
modules/               Terraform modules embedded in the binary
web/                   React + React Flow editor (built into internal/web/dist)
examples/              diagrams validated in CI against real providers
packaging/pypi         the pip launcher
docs/spec              catalog and document specifications
```

## Development

Requires Go 1.26+ and Node 22+.

```
make build                 # web build embedded into ./iagram
make test                  # go vet + tests + frontend checks
IAGRAM_E2E=1 go test ./internal/tofu/     # real OpenTofu (downloaded once), no cloud
```

UI work: `go run ./cmd/iagram up --no-open` in a directory with an `iagram.json`, then `cd web && npm run dev` (Vite on :5173 proxies `/api` and `/icons` to the Go process).

Releases: tag `vX.Y.Z` and push; GoReleaser builds binaries, checksums, GHCR images and the Homebrew cask, and the PyPI launcher is published from the same tag.

## Roadmap and status

| Phase | Scope | State |
|---|---|---|
| 1 | Canvas, palette, containment and connection rules, settings panel, validation, undo/redo | done |
| 2 | Terraform generation, OpenTofu install + plan, plan overlay | done |
| 3 | Apply with confirmation, outputs written back, drift detection | done |
| 4 | Google Cloud and Azure catalogs, provider switcher, `iagram import` | done |
| 5 | Drag re-parenting, multi-select, clipboard; GoReleaser, Homebrew, install script; Kubernetes, messaging and data elements | done |
| 6 | Dark theme, connection inspector, `iagram destroy`, DNS/CDN elements, grouped settings | done |
| 7 | Docker Compose, pip launcher, comprehensive docs | done |
| 8 | `.iad` files, generated elements for every provider resource type, HCL import and lossless round trip, per-provider canvases, connection UX, cross-provider convert, menubar | done |
| 9 | Official resource and group icons, deck colour grammar, generic groups / data centers / actors / notes, walkthrough steps, captions, edge styles and legend, Availability Zone boxes, link lines (peering, attachments, VPN), Auto Scaling bands, Organizations tree and assume-role accounts (from the AWS reference-diagram gap analysis, `docs/aws-diagram-gap-analysis.md`) | done |
| next | Visual identity; validation on real GCP/Azure accounts; Kubernetes catalog layer | |

Early software: the shipped catalogs are validated against the real providers, but the first production apply should be reviewed plan by plan, as with any Terraform.

## License

Everything outside `ee/` is [Apache-2.0](LICENSE). `ee/` is reserved for source-available code under the [Elastic License 2.0](ee/LICENSE) and is empty today. Contributions require the [CLA](CLA.md). "iagram" is a trademark; see [TRADEMARK.md](TRADEMARK.md). Cloud provider icons are used under their owners' terms; see the `NOTICE.md` in each `catalog/icons/<provider>/` directory.
