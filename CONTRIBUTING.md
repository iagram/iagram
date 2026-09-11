# Contributing to iagram

Thanks for helping. Two things to know up front:

1. Contributions require the [CLA](CLA.md). A bot will ask you to sign it on your first pull request.
2. Everything outside `ee/` is Apache-2.0. Please do not submit changes to `ee/` unless a maintainer asked you to.

## Adding a resource type

The whole point of iagram is that the catalog is the product. Adding a type is:

1. Create `catalog/<provider>/<name>.yaml`. `id` must be `<provider>.<name>`. The file must validate against `catalog/schema.json`.
2. Decide **where it can live** (`allowed_parents`) and, if it is a container, what it accepts (`allowed_children`, optional).
3. Decide **what it can connect to** (`connections.out` / `connections.in`), with a `kind` that says what the arrow means. Placement expresses containment; arrows express traffic, data or permission flow. Never encode the same fact both ways.
4. Describe its **properties** as a JSON Schema object in `props`. Supported today: `string` (with `enum`, `format: cidr`, `pattern`), `integer`/`number` (with `minimum`/`maximum`), `boolean`. Give sensible defaults; the goal is that a freshly dropped element already produces a valid plan.
5. Add an icon under `catalog/icons/<provider>/` and reference it in `icon`.
6. Add the Terraform mapping under `terraform:` (phase 2; the module can come in the same PR or a later one).
7. Run `make test`. `internal/catalog` tests load the shipped catalog and will fail on broken references.

Keep the catalog tight. A type that always produces a working plan is worth more than ten that fail half the time.

## Code

- Go: `gofmt`, `go vet`, tests next to the code. Standard library where it is enough.
- Frontend: TypeScript strict, React, React Flow, zustand. No component library.
- Commit messages: imperative subject line, a short body when the why is not obvious.

## Reporting problems

Open an issue with the `iagram.json` that reproduces it (strip anything sensitive; the file never contains credentials, but it may contain account ids).
