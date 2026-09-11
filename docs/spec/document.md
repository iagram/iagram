# Document format (`iagram.json`, version 1)

The diagram file is the source of truth for the infrastructure. It is plain
JSON, written deterministically (sorted nodes and edges, sorted keys, two-space
indent, trailing newline) so that diffs and code review work. Commit it.

```json
{
  "version": 1,
  "name": "demo",
  "nodes": [
    {
      "id": "vpc-7f3a",
      "type": "aws.vpc",
      "name": "main",
      "parent": "reg-1c2d",
      "props": {"cidr": "10.0.0.0/16"},
      "layout": {"x": 40, "y": 60, "w": 800, "h": 480}
    }
  ],
  "edges": [
    {"id": "e-9a1b", "kind": "routes_to", "source": "lb-01", "target": "web-02"}
  ]
}
```

| Field | Meaning |
|---|---|
| `version` | Integer format version. Required. |
| `name` | Optional diagram name; becomes the `iagram_diagram` tag. |
| `nodes[].id` | Stable identifier. Becomes the Terraform module name (sanitised) and the `iagram_node` tag, so **renaming an id re-creates the resources**. Names (`name`) can change freely. |
| `nodes[].type` | Catalog id. |
| `nodes[].parent` | Id of the containing node; absent for top-level nodes. |
| `nodes[].props` | Properties per the catalog `props` schema. Always present (`{}` when empty). |
| `nodes[].layout` | `x`,`y` relative to the parent; `w`,`h` for containers only. Layout never affects the generated infrastructure. |
| `edges[]` | Typed connections; `kind` must match the catalog rule for the pair of node types. |

A machine-readable schema is in [`document.schema.json`](document.schema.json).
Unknown fields are rejected on load, so typos surface immediately.

## Versioning and migration

- Every file carries `version`. iagram refuses files newer than it understands
  with a clear "upgrade iagram" message.
- Older versions are upgraded on load by an ordered chain of migrations
  (`internal/document/migrate.go`), one per version step, operating on the raw
  JSON before strict decoding. Saving writes the current version.
- A version bump happens only for a breaking change to this table. Additive
  optional fields do not bump the version.
- Guarantee: any file written by a released iagram loads in every later
  release.
