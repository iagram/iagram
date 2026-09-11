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

Plan-time rule: `count` and `for_each` may only depend on values known before
apply. Booleans and numbers from properties are known; outputs of other modules
are not. To branch on the presence of a parent, take it as a one-element list
(`"[parent.x]"` in the catalog) and test `length()`. Never filter lists of
references (`if x != ""`) and count the result; require the condition through
the connection rule's `requires_from` / `requires_to` instead.
