# modules/

Terraform modules referenced by `terraform.module` in catalog entries. They are
embedded in the iagram binary and materialised into `.iagram/tf/modules/` at
generate time, so a diagram directory is self-contained and `tofu` never
reaches a module registry.

Conventions every module follows (the generator relies on them):

- `variable "name"` and `variable "tags"` (map) exist; `tags` gets
  `iagram_node`, `iagram_diagram`, `managed_by` and each resource merges it.
- Every catalog `props.properties.<key>` is a variable of the same name.
- Inputs listed in `inputs_from_parent` / `collect` / connection `terraform.input`
  are variables; list inputs default to `[]` so an unconnected node still plans.
- Every `outputs:` entry in the catalog is an `output`.
- Modules do not configure providers; the root passes one explicitly.
