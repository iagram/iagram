# iagram

**Infrastructure as diagram.** Draw your cloud architecture on a canvas that only lets you draw things that can exist, then let Terraform build it. The diagram is the source of truth; Terraform is the engine.

```
$ iagram init
$ iagram up            # opens http://localhost:7777
```

iagram is a single local binary, like `terraform`:

- **Local only.** There is no server, account, or hosted component. The UI, the validator and (soon) the Terraform runner are one process on your machine.
- **Your credentials never leave your machine.** iagram does not store, prompt for or transmit cloud credentials. When it runs OpenTofu it uses the same local credential chain as the cloud CLIs do (`AWS_PROFILE`, SSO, `gcloud auth application-default`, `az login`).
- **Git-friendly.** The diagram is `iagram.json`, written deterministically so diffs are readable. Commit it next to your code. The generated Terraform is written alongside it; you can eject to plain Terraform at any time.
- **Nothing invalid is drawable.** A curated catalog defines every element, where it may be placed, what it may connect to and which properties it takes. An EC2 instance can only be dropped into a subnet; an ALB can only be wired to targets it can actually route to; two subnets in a VPC cannot overlap.

## Status

Early. Phase 1 of the roadmap below is what you get today: a working editor that enforces the catalog. No Terraform is generated yet.

| Phase | Scope | State |
|---|---|---|
| 1 | Canvas, palette, containment and connection rules, property panel, validation, save/load | done |
| 2 | Terraform generation (`.tf.json` module calls), `iagram plan`, plan overlay on the canvas | next |
| 3 | `iagram apply`, streamed progress, outputs written back onto the diagram, drift check | |
| 4 | GCP and Azure catalogs, state → diagram import | |

## Build from source

Requires Go 1.26+ and Node 22+.

```
make build       # builds web/ and embeds it into ./iagram
make test
```

For UI work: run `go run ./cmd/iagram up --no-open` in a directory with an `iagram.json`, then `cd web && npm run dev` and open http://localhost:5173 (Vite proxies `/api` to the Go process).

## The catalog

Every drawable element is one YAML file under `catalog/<provider>/<name>.yaml`, validated against `catalog/schema.json`. That one file drives the palette entry, the drop rules, the connection rules, the property panel, the validator and, in phase 2, the Terraform module call. Adding a resource type is a YAML file plus a Terraform module; no editor code. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

The core of iagram (everything outside `ee/`) is licensed under the [Apache License 2.0](LICENSE). Code under `ee/` is source-available under the [Elastic License 2.0](ee/LICENSE); today that directory is empty. Contributions require signing the [CLA](CLA.md). "iagram" is a trademark; see [TRADEMARK.md](TRADEMARK.md).
