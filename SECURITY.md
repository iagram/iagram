# Security

## Posture

iagram is a local tool. There is no server, account, or hosted component.

**Credentials.** iagram never asks for, stores, logs or transmits cloud
credentials. When it runs OpenTofu, the `tofu` process inherits the environment
of the shell you started iagram from, exactly like running `terraform` yourself;
providers pick up credentials through their normal chains (`AWS_PROFILE`, SSO
cache, instance roles, `gcloud auth application-default`, `az login`). The only
code that launches tofu is [`internal/tofu/run.go`](internal/tofu/run.go).

**Network.** The binary makes outbound connections for exactly these purposes:

| Purpose | Destination | Code |
|---|---|---|
| Download OpenTofu once, pinned version, SHA256 verified | `github.com/opentofu/opentofu/releases` | `internal/tofu/install.go` |
| Provider plugins and cloud API calls made by tofu itself | Terraform registry, your cloud's APIs | (tofu) |
| Anonymous usage events, [documented here](docs/telemetry.md), off with `IAGRAM_TELEMETRY=0` | `telemetry.iagram.dev` | `internal/telemetry/telemetry.go` |

Nothing else. Set `IAGRAM_TOFU_PATH` to skip the download and use your own binary.

**Local server.** `iagram up` listens on `127.0.0.1` only, on a port you choose,
and rejects requests carrying a non-localhost `Origin` header. It exposes the
diagram file, the catalog and the plan/apply operations of the directory you
started it in, to processes on your machine. Do not port-forward it.

**Files.** `iagram.json` contains what you drew: resource names, properties,
optionally an AWS account id if you typed one. It contains no secrets and is
meant to be committed. `.iagram/` holds generated Terraform, plans and local
state; state can contain secrets (as any Terraform state does) and is gitignored
by `iagram init`.

**Generated infrastructure defaults.** Modules default to encrypted storage,
IMDSv2, no public S3 access, no publicly accessible databases, and least-privilege
IAM derived from the arrows you drew. Review the plan; the canvas shows exactly
what will be created.

## Reporting a vulnerability

Please email security@iagram.dev, or open a GitHub security advisory on this
repository (Security → Report a vulnerability). Do not open a public issue for
anything that could expose users. We aim to acknowledge within 72 hours.
