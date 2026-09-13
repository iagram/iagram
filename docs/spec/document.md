# Document format (`.iad`, version 1)

The diagram file (`iagram.iad`; `.iad` = infrastructure as diagram) is the
source of truth for the infrastructure. It is plain JSON, written deterministically (sorted nodes and edges, sorted keys, two-space
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
| `nodes[].outputs` | Optional. Written by `iagram apply` from the module outputs the catalog declares (endpoint, ids, IPs). Informational: never read by the generator, never validated. |
| `nodes[].caption` | Optional free second line under the name ("primary Region", "vehicle lookup"). When absent the canvas shows the catalog's `caption_prop` value (CIDR, region, instance type). Informational. |
| `nodes[].step` | Optional callout number of the walkthrough (`"1"`, `"9a"`). Informational. |
| `nodes[].layout` | `x`,`y` relative to the parent; `w`,`h` for containers only; optional `z` stacking order among siblings. Layout never affects the generated infrastructure. |
| `edges[]` | Typed connections; `kind` must match the catalog rule for the pair of node types. For `references` edges from generated elements, `attr` is the source attribute receiving the reference and `output` the referenced target attribute (default `id`). |
| `edges[].label`, `edges[].step` | Optional label overriding the kind's default ("mTLS", "Private VIF") and callout number. Informational. |
| `edges[].style` | Optional drawing: `direction` `one` (default) / `both` / `none` (plain association line), `dash` `solid` / `dashed` / `dotted`, `color` (CSS), `inactive` (greyed standby path). The catalog rule of the kind provides defaults. Informational. |
| `steps[]` | Optional walkthrough text: `{n, text}` per callout number, shown in the legend. Informational. |
| `nodes[].type` for generated elements | `<provider>.res.<terraform type>`, e.g. `aws.res.aws_kms_key`; properties are the resource's attributes as in Terraform, nested blocks as arrays/objects. |

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
