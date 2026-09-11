# iagram

**Infrastructure as diagram.** Draw your cloud architecture on a canvas that only lets you draw things that can exist, then let Terraform build it. The diagram is the source of truth; OpenTofu is the engine.

```
$ brew install --cask iagram/tap/iagram   # or: curl -fsSL https://raw.githubusercontent.com/iagram/iagram/main/install.sh | sh
$ iagram init
$ iagram up            # opens http://localhost:7777, draw, hit Plan
$ iagram plan          # same thing headless
$ iagram apply
```

Releases are built by GoReleaser for macOS, Linux (amd64/arm64) and Windows, with a `SHA256SUMS` file; `install.sh` verifies it.

iagram is a single local binary, like `terraform`:

- **Local only.** No server, account or hosted component. The UI, the validator, the generator and the OpenTofu runner are one process on your machine. OpenTofu is downloaded once, version-pinned and checksum-verified, into `~/.iagram/bin`.
- **Your credentials never leave your machine.** iagram does not store, prompt for or transmit cloud credentials. `tofu` inherits your shell's credential chain (`AWS_PROFILE`, SSO, `gcloud auth application-default`, `az login`) exactly as `terraform` would. See [SECURITY.md](SECURITY.md) for the complete list of network calls the binary makes.
- **Git-friendly.** The diagram is `iagram.json`, written deterministically so diffs are readable; commit it next to your code. Generated Terraform lands in `.iagram/tf/main.tf.json` with the modules alongside; you can eject to plain Terraform at any time.
- **Editor you can live in.** Drag an element into another container to move it there (refused with a message if the catalog forbids it), shift-drag to multi-select, ⌘C/⌘V/⌘D to copy, paste and duplicate subtrees, undo/redo throughout.
- **Nothing invalid is drawable.** A catalog defines every element, where it may be placed, what it may connect to and which properties it takes. An EC2 instance can only be dropped into a subnet; an ALB can only be wired to targets it can route to; sibling subnets cannot overlap. The plan is painted back onto the diagram: green create, amber update, red destroy.
- **Cloud-agnostic core.** No code in iagram names a cloud. AWS, Google Cloud and Azure ship as catalog directories plus Terraform modules; adding a fourth is [a directory of YAML and modules](docs/spec/catalog.md#adding-a-provider). External catalogs load with `--catalog DIR`.
- **Bring existing infrastructure.** `iagram import --state terraform.tfstate` rebuilds nodes and containment from any Terraform state using the catalog's import mappings; you draw the arrows and plan to see the gap.

## Status

| Phase | Scope | State |
|---|---|---|
| 1 | Canvas, palette, containment and connection rules, property panel, validation, undo/redo, save/load | done |
| 2 | Terraform generation (`.tf.json` module calls), OpenTofu install + `plan`, plan overlay on the canvas, `iagram plan` CLI | done |
| 3 | Apply from the UI with confirmation, outputs written back onto nodes, drift check (refresh-only plan) painted on the canvas | done |
| 4 | Google Cloud and Azure catalogs with official icons, provider switcher, `iagram import` from an existing Terraform state | done |
| 5 | Re-parenting by drag, multi-select, copy/paste/duplicate; GoReleaser releases, Homebrew tap, verified install script; Kubernetes, messaging and data elements for all three providers | done |
| 6 | Dark theme, connection inspector and label toggle, `iagram destroy` (CLI and UI, typed confirmation), CloudFront + Route 53, Cloud DNS, Azure DNS | done |

Shipped catalogs:

| Provider | Elements |
|---|---|
| AWS | account, region, VPC, subnet, security group, EC2, RDS, S3, ALB, Lambda, SQS, DynamoDB, EKS, CloudFront, Route 53 |
| Google Cloud | project, VPC network (with private services access), subnetwork, firewall rule, Compute Engine, Cloud SQL, Cloud Storage, Cloud Run, Pub/Sub, BigQuery, GKE, Cloud DNS |
| Azure | subscription, resource group, virtual network, subnet, network security group, Linux VM, storage account, PostgreSQL Flexible Server, Key Vault, Container Registry, AKS, DNS zone |

Every module defaults to the secure option (encrypted storage, IMDSv2, private databases, no public buckets, least-privilege IAM/roles derived from the arrows you draw). One diagram can hold several providers; each gets its own provider block.

## How it works

```
iagram.json ──validate──▶ generate ──▶ .iagram/tf/main.tf.json + modules/ ──▶ tofu init/plan ──▶ plan JSON ──▶ painted on the canvas
```

- One `module` block per node, named after the node id, `source = ./modules/<provider>/<type>`.
- Containment becomes wiring: a node inside a subnet inside a VPC gets `subnet_id` and `vpc_id` from those modules' outputs.
- Arrows become inputs: `ALB → EC2` appends the instance to the ALB's targets; `SG → RDS` attaches the group; `EC2 → RDS` opens the database port to the instance's identity group; `Lambda → S3` grants an IAM policy.
- Account and region nodes become provider blocks (aliased per region), so one diagram can span regions.
- Apply runs the exact plan you reviewed (it is refused if the diagram changed since), then reads the module outputs the catalog declares (endpoint, ids, IPs) and writes them onto the nodes in `iagram.json`; deployed nodes show a green dot and their outputs in the panel.
- Drift check runs `tofu plan -refresh-only`: nodes whose real infrastructure no longer matches state get an amber dotted ring and the list of changed attributes.

The whole mapping lives in YAML, specified in [docs/spec/catalog.md](docs/spec/catalog.md) and enforced by [`catalog/schema.json`](catalog/schema.json). The file format is specified in [docs/spec/document.md](docs/spec/document.md), versioned, with a migration path.

## Commands

```
iagram init [name]            create iagram.json and .iagram/
iagram up [-p PORT] [--no-open]
iagram validate               catalog rules, headless (exit 1 on errors)
iagram generate               write .iagram/tf/main.tf.json
iagram plan                   generate + tofu init + plan, summarised per node
iagram apply                  apply the last plan, write outputs back onto the diagram
iagram drift                  refresh-only plan; exit 1 when infrastructure drifted
iagram destroy [--yes]        plan the teardown, ask for the diagram name, apply; clears outputs
iagram import --state FILE    build a diagram from a terraform.tfstate or `tofu show -json` (nodes and containment; draw the arrows)
iagram catalog check DIR      validate an external catalog
iagram telemetry on|off|status
```

All diagram commands accept `-f FILE` and repeatable `--catalog DIR`. `IAGRAM_TOFU_PATH` points at your own tofu/terraform binary instead of the managed download; `IAGRAM_HOME` relocates `~/.iagram`.

## Telemetry

Off by default. iagram sends nothing unless you run `iagram telemetry on`; if you do, it sends a few anonymous usage events (command name, version, OS, bucketed counts), never names, properties, ids, account ids, paths or errors. The exact payload is in [docs/telemetry.md](docs/telemetry.md); the code is one file with tests that assert the default and the whitelist.

## Build from source

Requires Go 1.26+ and Node 22+.

```
make build       # builds web/ and embeds it into ./iagram
make test        # go vet + tests + frontend typecheck
IAGRAM_E2E=1 go test ./internal/tofu/   # exercises a real OpenTofu binary (downloads it once)
```

For UI work: `go run ./cmd/iagram up --no-open` in a directory with an `iagram.json`, then `cd web && npm run dev` (Vite proxies `/api` and `/icons` to the Go process).

## Contributing

Adding a resource type is a YAML file plus a Terraform module; adding a provider is a directory of those. See [CONTRIBUTING.md](CONTRIBUTING.md) and the [catalog spec](docs/spec/catalog.md). Contributions require the [CLA](CLA.md).

## License

Everything outside `ee/` is [Apache-2.0](LICENSE). Code under `ee/` is source-available under the [Elastic License 2.0](ee/LICENSE); today that directory is empty. AWS icons are used under the [AWS Architecture Icons](https://aws.amazon.com/architecture/icons/) terms, see [catalog/icons/aws/NOTICE.md](catalog/icons/aws/NOTICE.md). "iagram" is a trademark; see [TRADEMARK.md](TRADEMARK.md).
