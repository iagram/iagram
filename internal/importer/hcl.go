package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
)

// HCLResult is a parsed configuration: resources with literal attribute
// values, references between them, and the blocks that were not imported.
type HCLResult struct {
	Resources []HCLResource
	Providers []HCLProvider
	Skipped   []string // "variable x", "module y", ...
}

// HCLResource is one resource block.
type HCLResource struct {
	Type  string
	Name  string
	Attrs map[string]any    // literal values (blocks nested as objects/arrays)
	Refs  map[string]string // attr -> "type.name.attr" reference expression
	Exprs map[string]string // attr -> raw expression text for anything else (kept verbatim as ${...})
}

// HCLProvider is a provider block's literal attributes (region, project...).
type HCLProvider struct {
	Name  string
	Alias string
	Attrs map[string]any
}

// ParseHCL parses every .tf and .tf.json file in dir.
func ParseHCL(dir string) (*HCLResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	parser := hclparse.NewParser()
	var files []*hcl.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !(strings.HasSuffix(name, ".tf") || strings.HasSuffix(name, ".tf.json")) {
			continue
		}
		var f *hcl.File
		var diags hcl.Diagnostics
		if strings.HasSuffix(name, ".json") {
			f, diags = parser.ParseJSONFile(filepath.Join(dir, name))
		} else {
			f, diags = parser.ParseHCLFile(filepath.Join(dir, name))
		}
		if diags.HasErrors() {
			return nil, fmt.Errorf("%s: %s", name, diags.Error())
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .tf or .tf.json files in %s", dir)
	}
	res := &HCLResult{}
	schema := &hcl.BodySchema{Blocks: []hcl.BlockHeaderSchema{
		{Type: "resource", LabelNames: []string{"type", "name"}},
		{Type: "provider", LabelNames: []string{"name"}},
		{Type: "variable", LabelNames: []string{"name"}},
		{Type: "output", LabelNames: []string{"name"}},
		{Type: "module", LabelNames: []string{"name"}},
		{Type: "data", LabelNames: []string{"type", "name"}},
		{Type: "locals"}, {Type: "terraform"}, {Type: "moved"}, {Type: "import"}, {Type: "check"}, {Type: "removed"},
	}}
	for _, f := range files {
		content, _, diags := f.Body.PartialContent(schema)
		if diags.HasErrors() {
			return nil, errors.New(diags.Error())
		}
		for _, blk := range content.Blocks {
			switch blk.Type {
			case "resource":
				r := HCLResource{Type: blk.Labels[0], Name: blk.Labels[1], Attrs: map[string]any{}, Refs: map[string]string{}, Exprs: map[string]string{}}
				convertBody(blk.Body, f.Bytes, &r, "")
				normalizeJSONStrings(&r)
				stripIagramTags(&r)
				res.Resources = append(res.Resources, r)
			case "provider":
				p := HCLProvider{Name: blk.Labels[0], Attrs: map[string]any{}}
				tmp := HCLResource{Attrs: p.Attrs, Refs: map[string]string{}, Exprs: map[string]string{}}
				convertBody(blk.Body, f.Bytes, &tmp, "")
				if a, ok := p.Attrs["alias"].(string); ok {
					p.Alias = a
					delete(p.Attrs, "alias")
				}
				res.Providers = append(res.Providers, p)
			case "terraform", "moved", "import", "check", "removed", "output":
				// configuration plumbing, not infrastructure
			default:
				label := blk.Type
				if len(blk.Labels) > 0 {
					label += " " + strings.Join(blk.Labels, ".")
				}
				res.Skipped = append(res.Skipped, label)
			}
		}
	}
	sort.Slice(res.Resources, func(i, j int) bool {
		if res.Resources[i].Type != res.Resources[j].Type {
			return res.Resources[i].Type < res.Resources[j].Type
		}
		return res.Resources[i].Name < res.Resources[j].Name
	})
	return res, nil
}

// convertBody fills r.Attrs from attributes (literals), r.Refs from plain
// resource traversals and r.Exprs from anything else; nested blocks become
// objects (single) or arrays of objects under their block type name.
func convertBody(body hcl.Body, src []byte, r *HCLResource, prefix string) {
	attrs, blocks := splitBody(body)
	for name, attr := range attrs {
		key := prefix + name
		if ref, ok := traversalRef(attr.Expr); ok {
			r.Refs[key] = ref
			continue
		}
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() || !val.IsWhollyKnown() {
			r.Exprs[key] = string(attr.Expr.Range().SliceBytes(src))
			continue
		}
		r.Attrs[key] = ctyToGo(val)
	}
	byType := map[string][]any{}
	var order []string
	for _, b := range blocks {
		sub := HCLResource{Attrs: map[string]any{}, Refs: map[string]string{}, Exprs: map[string]string{}}
		convertBody(b.Body, src, &sub, "")
		obj := map[string]any{}
		for k, v := range sub.Attrs {
			obj[k] = v
		}
		for k, v := range sub.Refs {
			obj[k] = "${" + v + "}"
		}
		for k, v := range sub.Exprs {
			obj[k] = "${" + v + "}"
		}
		if _, seen := byType[b.Type]; !seen {
			order = append(order, b.Type)
		}
		byType[b.Type] = append(byType[b.Type], obj)
	}
	for _, t := range order {
		r.Attrs[prefix+t] = byType[t]
	}
}

var refPattern = regexp.MustCompile(`^\$\{([a-z][a-z0-9_]*\.[A-Za-z0-9_-]+\.[A-Za-z0-9_.]+)\}$`)

// normalizeJSONStrings handles Terraform JSON syntax, where expressions arrive
// as "${...}" strings: plain resource references become Refs, anything else
// with interpolation becomes an Expr, so JSON and native files import alike.
func normalizeJSONStrings(r *HCLResource) {
	for k, v := range r.Attrs {
		str, ok := v.(string)
		if !ok || !strings.Contains(str, "${") {
			continue
		}
		if m := refPattern.FindStringSubmatch(str); m != nil {
			root := strings.SplitN(m[1], ".", 2)[0]
			if root != "var" && root != "local" && root != "module" && root != "data" && root != "each" && root != "count" && root != "path" && root != "terraform" {
				r.Refs[k] = m[1]
				delete(r.Attrs, k)
				continue
			}
		}
		if strings.HasPrefix(str, "${") && strings.HasSuffix(str, "}") && strings.Count(str, "${") == 1 {
			r.Exprs[k] = str[2 : len(str)-1]
			delete(r.Attrs, k)
		}
	}
	// provider = aws.alias arrives as a two-part traversal (native) or a string (JSON).
	if v, ok := r.Attrs["provider"].(string); ok {
		r.Exprs["provider"] = v
		delete(r.Attrs, "provider")
	}
}

// stripIagramTags removes the bookkeeping tags iagram adds when generating, so
// a generated configuration imports back to the same diagram.
func stripIagramTags(r *HCLResource) {
	tags, ok := r.Attrs["tags"].(map[string]any)
	if !ok {
		return
	}
	for k, v := range tags {
		if strings.HasPrefix(k, "iagram_") || (k == "managed_by" && v == "iagram") {
			delete(tags, k)
		}
	}
	if len(tags) == 0 {
		delete(r.Attrs, "tags")
	}
}

func splitBody(body hcl.Body) (map[string]*hcl.Attribute, []*hcl.Block) {
	if sb, ok := body.(*hclsyntax.Body); ok {
		attrs := map[string]*hcl.Attribute{}
		for n, a := range sb.Attributes {
			attrs[n] = a.AsHCLAttribute()
		}
		var blocks []*hcl.Block
		for _, b := range sb.Blocks {
			blocks = append(blocks, b.AsHCLBlock())
		}
		return attrs, blocks
	}
	// JSON bodies: everything is an attribute; nested blocks arrive as values.
	attrs, _ := body.JustAttributes()
	return attrs, nil
}

// traversalRef recognises `<type>.<name>.<attr...>` expressions.
func traversalRef(e hcl.Expression) (string, bool) {
	vars := e.Variables()
	if len(vars) != 1 {
		return "", false
	}
	if se, ok := e.(*hclsyntax.ScopeTraversalExpr); !ok || len(se.Traversal) < 3 {
		return "", false
	}
	tr := vars[0]
	parts := make([]string, 0, len(tr))
	for _, step := range tr {
		switch st := step.(type) {
		case hcl.TraverseRoot:
			parts = append(parts, st.Name)
		case hcl.TraverseAttr:
			parts = append(parts, st.Name)
		default:
			return "", false
		}
	}
	root := parts[0]
	if root == "var" || root == "local" || root == "module" || root == "data" || root == "each" || root == "count" || root == "path" || root == "terraform" {
		return "", false
	}
	return strings.Join(parts, "."), true
}

func ctyToGo(v cty.Value) any {
	raw, err := ctyjson.Marshal(v, v.Type())
	if err != nil {
		return nil
	}
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

// BuildHCL turns a parsed configuration into a document of generated
// elements, with references as edges. Provider blocks become account/region
// containers when the catalog has matching roles.
func BuildHCL(c *catalog.Catalog, name string, parsed *HCLResult) (*document.Document, Report) {
	b := &builder{c: c, doc: document.New(name), byPrimaryID: map[string]string{}, groups: map[string]string{}}
	b.indexEntries()
	byAddr := map[string]string{} // type.name -> node id
	type placed struct {
		r   HCLResource
		idx int
	}
	var nodes []placed
	for _, r := range parsed.Resources {
		gid, ok := c.GeneratedID(r.Type)
		if !ok {
			b.report.Skipped = append(b.report.Skipped, r.Type)
			continue
		}
		e, ok := c.Get(gid)
		if !ok {
			b.report.Skipped = append(b.report.Skipped, r.Type)
			continue
		}
		n := document.Node{ID: nodeID(e, r.Type+"."+r.Name), Type: e.ID, Name: r.Name, Props: map[string]any{}}
		for k, v := range r.Attrs {
			n.Props[k] = v
		}
		for k, v := range r.Exprs {
			n.Props[k] = "${" + v + "}"
		}
		b.doc.Nodes = append(b.doc.Nodes, n)
		byAddr[r.Type+"."+r.Name] = n.ID
		nodes = append(nodes, placed{r, len(b.doc.Nodes) - 1})
	}
	// Provider blocks -> account/region containers per catalog provider.
	regionByLocal := map[string]string{}
	for _, p := range parsed.Providers {
		for provName, pc := range c.Providers {
			if pc.LocalName() != p.Name {
				continue
			}
			acct := b.accountsBy[provName]
			reg := b.regionsBy[provName]
			acctID := ""
			if acct != nil {
				acctID = b.ensureGroup("account:"+provName+":", acct, "", "")
				b.applyProviderArgs(acctID, acct, p.Attrs)
			}
			if reg != nil {
				val := ""
				if reg.Terraform.Import != nil && reg.Terraform.Import.Prop != "" {
					if v, ok := p.Attrs[reg.Terraform.Import.Prop]; ok {
						val = fmt.Sprint(v)
					}
				}
				regionByLocal[p.Name+"|"+p.Alias] = b.ensureGroup("region:"+provName+":"+acctID+":"+val, reg, acctID, val)
			}
		}
	}
	for _, pl := range nodes {
		n := &b.doc.Nodes[pl.idx]
		e, _ := c.Get(n.Type)
		local := c.Providers[e.Provider].LocalName()
		alias := ""
		if pv, ok := pl.r.Exprs["provider"]; ok {
			_, alias, _ = strings.Cut(pv, ".")
			delete(n.Props, "provider")
		}
		if parent, ok := regionByLocal[local+"|"+alias]; ok {
			n.Parent = parent
		} else if parent, ok := regionByLocal[local+"|"]; ok {
			n.Parent = parent
		} else {
			// No provider block: synthesise an empty account/region so the node has a home.
			n.Parent = b.groupParent(pending{res: Resource{Type: pl.r.Type, Attrs: map[string]any{}}, e: e, idx: pl.idx})
		}
		for attr, ref := range pl.r.Refs {
			parts := strings.SplitN(ref, ".", 3) // type, name, attr
			target, ok := byAddr[parts[0]+"."+parts[1]]
			if !ok {
				n.Props[attr] = "${" + ref + "}"
				continue
			}
			b.doc.Edges = append(b.doc.Edges, document.Edge{ID: edgeID(n.ID, attr, target), Kind: catalog.ReferencesKind, Source: n.ID, Target: target, Attr: attr, Output: parts[2]})
		}
	}
	b.report.Imported = len(nodes)
	b.report.Skipped = append(b.report.Skipped, parsed.Skipped...)
	attachToParents(c, b.doc)
	sort.Slice(b.doc.Nodes, func(i, j int) bool { return b.doc.Nodes[i].ID < b.doc.Nodes[j].ID })
	sort.Slice(b.doc.Edges, func(i, j int) bool { return b.doc.Edges[i].ID < b.doc.Edges[j].ID })
	return b.doc, b.report
}

// applyProviderArgs copies provider-block attributes into the account node's
// properties where the account entry's provider_args template names them.
func (b *builder) applyProviderArgs(acctID string, acct *catalog.Entry, attrs map[string]any) {
	if acct.Terraform == nil {
		return
	}
	for i := range b.doc.Nodes {
		if b.doc.Nodes[i].ID != acctID {
			continue
		}
		for arg, tpl := range acct.Terraform.ProviderArgs {
			s, ok := tpl.(string)
			if !ok || !strings.HasPrefix(s, "${") {
				continue
			}
			prop := strings.TrimSuffix(strings.TrimPrefix(s, "${"), "}")
			if v, ok := attrs[arg]; ok {
				b.doc.Nodes[i].Props[prop] = v
			}
		}
	}
}
