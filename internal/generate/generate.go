// Package generate turns a document into Terraform JSON. One module block
// per resource node, provider blocks from account/region nodes, wiring from
// the containment tree and the typed edges. The generator knows no cloud:
// every name, input and reference comes from the catalog.
package generate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
)

// Result is the generated configuration plus the module→node map the plan
// summary needs to paint the canvas.
type Result struct {
	Config       map[string]any    // main.tf.json content
	ModuleToNode map[string]string // sanitized module name -> node id
	Modules      []string          // module source paths used (e.g. aws/ec2_instance)
	Warnings     []string
}

// JSON renders the config deterministically.
func (r *Result) JSON() ([]byte, error) {
	raw, err := json.MarshalIndent(r.Config, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

type gen struct {
	c     *catalog.Catalog
	d     *document.Document
	nodes map[string]*document.Node
	res   *Result

	modules   map[string]map[string]any            // module name -> block
	resources map[string]map[string]map[string]any // tf type -> name -> block
	providers map[string][]map[string]any
	aliases   map[string]string // node id (region/account) -> alias
}

// Address returns the Terraform address prefix of a node: module.<name> for
// curated elements, <type>.<name> for generated ones.
func (g *gen) address(n *document.Node) (string, bool) {
	e, ok := g.c.Get(n.Type)
	if !ok || e.Terraform == nil {
		return "", false
	}
	switch e.Terraform.Role {
	case catalog.RoleModule:
		return "module." + sanitize(n.ID), true
	case catalog.RoleResource:
		return e.Terraform.Resource + "." + sanitize(n.Name), true
	}
	return "", false
}

// Run generates Terraform for d.
func Run(c *catalog.Catalog, d *document.Document) (*Result, error) {
	g := &gen{
		c: c, d: d, nodes: d.Index(),
		res:       &Result{ModuleToNode: map[string]string{}},
		modules:   map[string]map[string]any{},
		resources: map[string]map[string]map[string]any{},
		providers: map[string][]map[string]any{},
		aliases:   map[string]string{},
	}
	if err := g.providerBlocks(); err != nil {
		return nil, err
	}
	if err := g.moduleBlocks(); err != nil {
		return nil, err
	}
	if err := g.resourceBlocks(); err != nil {
		return nil, err
	}
	g.edgeWiring()
	g.referenceWiring()
	g.assemble()
	return g.res, nil
}

// providerBlocks emits one provider block per region node (aliased by the
// node id) and a default one per provider, merging account args downwards.
func (g *gen) providerBlocks() error {
	used := map[string]bool{}
	for i := range g.d.Nodes {
		n := &g.d.Nodes[i]
		e, ok := g.c.Get(n.Type)
		if !ok || e.Terraform == nil || e.Terraform.Role != catalog.RoleRegion {
			continue
		}
		args := map[string]any{}
		for _, anc := range g.ancestors(n) {
			if ae, ok := g.c.Get(anc.Type); ok && ae.Terraform != nil && ae.Terraform.Role == catalog.RoleAccount {
				merge(args, render(ae.Terraform.ProviderArgs, anc.Props))
			}
		}
		merge(args, render(e.Terraform.ProviderArgs, n.Props))
		alias := sanitize(n.ID)
		args["alias"] = alias
		g.aliases[n.ID] = alias
		g.providers[g.localName(e.Provider)] = append(g.providers[g.localName(e.Provider)], args)
		used[e.Provider] = true
	}
	// Account-only diagrams (no region node) still get an account-level alias
	// so global resources have a provider to bind to.
	for i := range g.d.Nodes {
		n := &g.d.Nodes[i]
		e, ok := g.c.Get(n.Type)
		if !ok || e.Terraform == nil || e.Terraform.Role != catalog.RoleAccount {
			continue
		}
		if g.hasRegionChild(n) {
			continue
		}
		args := render(e.Terraform.ProviderArgs, n.Props)
		alias := sanitize(n.ID)
		args["alias"] = alias
		g.aliases[n.ID] = alias
		g.providers[g.localName(e.Provider)] = append(g.providers[g.localName(e.Provider)], args)
		used[e.Provider] = true
	}
	// A default (unaliased) provider per used provider keeps `tofu` happy for
	// anything that does not receive an explicit provider (copies the first).
	for p, list := range g.providers {
		if len(list) == 0 {
			continue
		}
		def := map[string]any{}
		for k, v := range list[0] {
			if k != "alias" {
				def[k] = v
			}
		}
		g.providers[p] = append([]map[string]any{def}, list...)
	}
	return nil
}

func (g *gen) hasRegionChild(acct *document.Node) bool {
	for i := range g.d.Nodes {
		n := &g.d.Nodes[i]
		lp := g.d.LogicalParent(g.nodes, n, g.transparent)
		if lp == nil || lp.ID != acct.ID {
			continue
		}
		if e, ok := g.c.Get(n.Type); ok && e.Terraform != nil && e.Terraform.Role == catalog.RoleRegion {
			return true
		}
	}
	return false
}

func (g *gen) moduleBlocks() error {
	for i := range g.d.Nodes {
		n := &g.d.Nodes[i]
		e, ok := g.c.Get(n.Type)
		if !ok {
			return fmt.Errorf("node %s: unknown type %s", n.ID, n.Type)
		}
		if e.Terraform == nil {
			if g.c.HasTerraform(e.Provider) {
				g.res.Warnings = append(g.res.Warnings, fmt.Sprintf("%s (%s) has no terraform mapping; skipped", n.Name, e.Label))
			}
			continue
		}
		if e.Terraform.Role != catalog.RoleModule {
			continue
		}
		name := sanitize(n.ID)
		if prev, dup := g.res.ModuleToNode[name]; dup {
			return fmt.Errorf("nodes %s and %s collide on module name %s", prev, n.ID, name)
		}
		g.res.ModuleToNode[name] = n.ID

		block := map[string]any{
			"source": "./modules/" + e.Terraform.Module,
			"name":   n.Name,
			"tags": map[string]any{
				"iagram_node":    n.ID,
				"iagram_diagram": g.d.Name,
				"managed_by":     "iagram",
			},
		}
		if alias := g.providerAlias(n, e.Provider); alias != "" {
			local := g.localName(e.Provider)
			block["providers"] = map[string]any{local: local + "." + alias}
		}
		for k, v := range n.Props {
			if v == nil || v == "" {
				continue
			}
			block[k] = v
		}
		for input, ref := range e.Terraform.InputsFromParent {
			// "[parent.x]" yields a one-element list (or [] when unresolved) so a
			// module can branch on length(), which is known at plan time, rather
			// than on the value, which is not.
			if list := strings.HasPrefix(ref, "[") && strings.HasSuffix(ref, "]"); list {
				if expr, ok := g.resolveAncestorRef(n, ref[1:len(ref)-1]); ok {
					block[input] = []any{expr}
				} else {
					block[input] = []any{}
				}
				continue
			}
			if expr, ok := g.resolveAncestorRef(n, ref); ok {
				block[input] = expr
			}
		}
		for input, col := range e.Terraform.Collect {
			anc := g.ancestorAt(n, col.Under)
			if anc == nil {
				continue
			}
			var refs []any
			for j := range g.d.Nodes {
				m := &g.d.Nodes[j]
				if m.Type == col.Type && g.isUnder(m, anc.ID) {
					refs = append(refs, fmt.Sprintf("${module.%s.%s}", sanitize(m.ID), col.Output))
				}
			}
			sort.Slice(refs, func(a, b int) bool { return refs[a].(string) < refs[b].(string) })
			block[input] = refs
		}
		g.modules[name] = block
		g.res.Modules = appendUnique(g.res.Modules, e.Terraform.Module)
	}
	return nil
}

// resourceBlocks emits one plain resource block per generated element, with
// every property as an attribute. The Terraform resource name is the node
// name, so an imported configuration regenerates with the same addresses.
func (g *gen) resourceBlocks() error {
	for i := range g.d.Nodes {
		n := &g.d.Nodes[i]
		e, ok := g.c.Get(n.Type)
		if !ok || e.Terraform == nil || e.Terraform.Role != catalog.RoleResource {
			continue
		}
		tfType := e.Terraform.Resource
		name := sanitize(n.Name)
		if g.resources[tfType] == nil {
			g.resources[tfType] = map[string]map[string]any{}
		}
		if _, dup := g.resources[tfType][name]; dup {
			return fmt.Errorf("two %s resources named %q", tfType, n.Name)
		}
		block := map[string]any{}
		for k, v := range n.Props {
			if v == nil || v == "" {
				continue
			}
			block[k] = v
		}
		if alias := g.providerAlias(n, e.Provider); alias != "" {
			local := g.localName(e.Provider)
			block["provider"] = local + "." + alias
		}
		// Tag the resource for plan mapping when the schema has tags and none were set.
		if props, _ := e.Props["properties"].(map[string]any); props != nil {
			if _, hasTags := props["tags"]; hasTags {
				if _, set := block["tags"]; !set {
					block["tags"] = map[string]any{"iagram_node": n.ID, "iagram_diagram": g.d.Name, "managed_by": "iagram"}
				}
			}
		}
		g.resources[tfType][name] = block
		g.res.ModuleToNode[tfType+"."+name] = n.ID
	}
	return nil
}

// referenceWiring resolves "references" edges: the source attribute gets
// "${<target address>.<output>}" (appended when the attribute is a list).
func (g *gen) referenceWiring() {
	for _, ed := range g.d.Edges {
		if ed.Kind != catalog.ReferencesKind || ed.Attr == "" {
			continue
		}
		src, sok := g.nodes[ed.Source]
		dst, dok := g.nodes[ed.Target]
		if !sok || !dok {
			continue
		}
		se, ok := g.c.Get(src.Type)
		if !ok || se.Terraform == nil || se.Terraform.Role != catalog.RoleResource {
			continue
		}
		addr, ok := g.address(dst)
		if !ok {
			continue
		}
		out := ed.Output
		if out == "" {
			out = "id"
		}
		ref := "${" + addr + "." + out + "}"
		block := g.resources[se.Terraform.Resource][sanitize(src.Name)]
		if block == nil {
			continue
		}
		schema, _ := se.Property(ed.Attr)
		if schema != nil && schema["type"] == "array" {
			list, _ := block[ed.Attr].([]any)
			list = append(list, ref)
			block[ed.Attr] = list
		} else {
			block[ed.Attr] = ref
		}
	}
}

// edgeWiring appends "${module.X.output}" to the list input named by the
// rule, on whichever side the rule says.
func (g *gen) edgeWiring() {
	for _, ed := range g.d.Edges {
		src, sok := g.nodes[ed.Source]
		dst, dok := g.nodes[ed.Target]
		if !sok || !dok {
			continue
		}
		rule, ok := g.c.Connection(src.Type, dst.Type)
		if !ok || rule.Terraform == nil {
			continue
		}
		t := rule.Terraform
		setOn, other := src, dst
		if t.Set == "to" {
			setOn, other = dst, src
		}
		valueSide, output, _ := strings.Cut(t.Value, ".")
		valueNode := src
		if valueSide == "to" {
			valueNode = dst
		}
		_ = other
		block, ok := g.modules[sanitize(setOn.ID)]
		if !ok {
			continue
		}
		ref := fmt.Sprintf("${module.%s.%s}", sanitize(valueNode.ID), output)
		list, _ := block[t.Input].([]any)
		list = append(list, ref)
		sort.Slice(list, func(a, b int) bool { return list[a].(string) < list[b].(string) })
		block[t.Input] = list
	}
}

func (g *gen) assemble() {
	required := map[string]any{}
	requiredVersion := ""
	for _, pc := range g.c.Providers {
		if _, used := g.providers[pc.LocalName()]; !used {
			continue
		}
		required[pc.LocalName()] = map[string]any{"source": pc.Source, "version": pc.Version}
		for name, src := range pc.Extra {
			required[name] = map[string]any{"source": src.Source, "version": src.Version}
		}
		if pc.RequiredVersion != "" && (requiredVersion == "" || pc.RequiredVersion > requiredVersion) {
			requiredVersion = pc.RequiredVersion
		}
	}
	tf := map[string]any{"required_providers": required}
	if requiredVersion != "" {
		tf["required_version"] = requiredVersion
	}

	outputs := map[string]any{}
	for name, id := range g.res.ModuleToNode {
		n := g.nodes[id]
		e, _ := g.c.Get(n.Type)
		if e.Terraform.Role == catalog.RoleResource {
			// Generated elements: id (and arn when present) are enough to write back.
			vals := map[string]any{"id": fmt.Sprintf("${%s.id}", name)}
			for _, o := range e.Outputs {
				if o == "arn" {
					vals["arn"] = fmt.Sprintf("${%s.arn}", name)
				}
			}
			outputs[sanitize(name)] = map[string]any{"value": vals, "sensitive": true}
			continue
		}
		if len(e.Outputs) == 0 {
			continue
		}
		vals := map[string]any{}
		for _, o := range e.Outputs {
			vals[o] = fmt.Sprintf("${module.%s.%s}", name, o)
		}
		outputs[name] = map[string]any{"value": vals, "sensitive": true}
	}

	cfg := map[string]any{"terraform": tf}
	if len(g.providers) > 0 {
		cfg["provider"] = g.providers
	}
	if len(g.modules) > 0 {
		cfg["module"] = g.modules
	}
	if len(g.resources) > 0 {
		cfg["resource"] = g.resources
	}
	if len(outputs) > 0 {
		cfg["output"] = outputs
	}
	g.res.Config = cfg
}

// transparent reports whether a node type is a drawing-only container.
func (g *gen) transparent(typ string) bool {
	e, ok := g.c.Get(typ)
	return ok && e.Transparent
}

// localName maps a catalog provider (gcp) to its Terraform name (google).
func (g *gen) localName(provider string) string {
	if pc, ok := g.c.Providers[provider]; ok {
		return pc.LocalName()
	}
	return provider
}

// providerAlias finds the nearest region (then account) ancestor of the
// same provider and returns its alias.
func (g *gen) providerAlias(n *document.Node, provider string) string {
	for _, anc := range g.ancestors(n) {
		e, ok := g.c.Get(anc.Type)
		if !ok || e.Provider != provider || e.Terraform == nil {
			continue
		}
		if e.Terraform.Role == catalog.RoleRegion || e.Terraform.Role == catalog.RoleAccount {
			if a, ok := g.aliases[anc.ID]; ok {
				return a
			}
		}
	}
	return ""
}

// ancestors returns parent, grandparent, ... (nearest first).
func (g *gen) ancestors(n *document.Node) []*document.Node {
	var out []*document.Node
	seen := map[string]bool{n.ID: true}
	for cur := n; cur.Parent != "" && !seen[cur.Parent]; {
		seen[cur.Parent] = true
		p, ok := g.nodes[cur.Parent]
		if !ok {
			break
		}
		out = append(out, p)
		cur = p
	}
	return out
}

// ancestorAt resolves "parent", "parent.parent", ...
func (g *gen) ancestorAt(n *document.Node, path string) *document.Node {
	depth := strings.Count(path, "parent")
	anc := g.ancestors(n)
	if depth == 0 || depth > len(anc) {
		return nil
	}
	return anc[depth-1]
}

// resolveAncestorRef turns "parent.parent.vpc_id" into "${module.<id>.vpc_id}"
// if that ancestor is a module node; otherwise reports false so the input is
// left out (e.g. a Lambda placed directly in a region has no subnet).
func (g *gen) resolveAncestorRef(n *document.Node, ref string) (string, bool) {
	i := strings.LastIndex(ref, ".")
	if i < 0 {
		return "", false
	}
	path, output := ref[:i], ref[i+1:]
	anc := g.ancestorAt(n, path)
	if anc == nil {
		return "", false
	}
	e, ok := g.c.Get(anc.Type)
	if !ok || e.Terraform == nil || e.Terraform.Role != catalog.RoleModule {
		return "", false
	}
	return fmt.Sprintf("${module.%s.%s}", sanitize(anc.ID), output), true
}

func (g *gen) isUnder(n *document.Node, ancestorID string) bool {
	for _, a := range g.ancestors(n) {
		if a.ID == ancestorID {
			return true
		}
	}
	return false
}

var unsafe = regexp.MustCompile(`[^A-Za-z0-9_]`)

// sanitize makes a node id a valid Terraform identifier.
func sanitize(id string) string {
	s := unsafe.ReplaceAllString(id, "_")
	if s == "" || (s[0] >= '0' && s[0] <= '9') {
		s = "n_" + s
	}
	return s
}

// render substitutes "${prop}" strings in a provider_args template with node
// property values, dropping empties.
func render(tpl map[string]any, props map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range tpl {
		if r, ok := renderValue(v, props); ok {
			out[k] = r
		}
	}
	return out
}

var placeholder = regexp.MustCompile(`^\$\{([A-Za-z0-9_]+)\}$`)

func renderValue(v any, props map[string]any) (any, bool) {
	switch t := v.(type) {
	case string:
		m := placeholder.FindStringSubmatch(t)
		if m == nil {
			return t, true
		}
		pv, ok := props[m[1]]
		if !ok || pv == nil || pv == "" {
			return nil, false
		}
		return pv, true
	case []any:
		var out []any
		for _, item := range t {
			if r, ok := renderValue(item, props); ok {
				out = append(out, r)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	case map[string]any:
		out := render(t, props)
		// A literal empty map (no placeholders at all) is kept: azurerm's
		// mandatory `features {}` block is exactly that.
		if len(out) == 0 && len(t) > 0 {
			return nil, false
		}
		return out, true
	}
	return v, true
}

func merge(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

func appendUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}
