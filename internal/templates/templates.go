// Package templates ships reference architectures: compact YAML specs under
// templates/<provider>/<slug>/template.yaml rendered into .iad documents that
// the gallery lists and the editor opens pre-configured.
package templates

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/layout"
	"github.com/iagram/iagram/internal/provide"
)

// Spec is the hand-written source of a template.
type Spec struct {
	Title    string   `yaml:"title" json:"title"`
	Category string   `yaml:"category" json:"category"`
	Tags     []string `yaml:"tags" json:"tags"`
	Source   string   `yaml:"source" json:"source"`
	// Image is the original diagram on the source page, loaded by the gallery
	// from the vendor site (never copied into the repository).
	Image       string          `yaml:"image" json:"image,omitempty"`
	Description string          `yaml:"description" json:"description"`
	Provider    string          `yaml:"provider" json:"provider"`
	Nodes       []SpecNode      `yaml:"nodes" json:"-"`
	Edges       []SpecEdge      `yaml:"edges" json:"-"`
	Steps       []document.Step `yaml:"steps" json:"-"`
}

// SpecNode: Type is a curated element (short "vpc" -> "<provider>.vpc", or a
// full id such as "common.users"); TF a Terraform type resolved to the curated
// element importing it, else the generated element.
type SpecNode struct {
	ID      string         `yaml:"id"`
	Type    string         `yaml:"type"`
	TF      string         `yaml:"tf"`
	Name    string         `yaml:"name"`
	Parent  string         `yaml:"parent"`
	Props   map[string]any `yaml:"props"`
	Caption string         `yaml:"caption"`
	Step    string         `yaml:"step"`
	Size    []float64      `yaml:"size"`
	// At is the grid cell inside the parent, read off the reference diagram:
	// [row, col] or [row, col, rowspan, colspan], 1-based.
	At []int `yaml:"at"`
}

type SpecEdge struct {
	From  string              `yaml:"from"`
	To    string              `yaml:"to"`
	Label string              `yaml:"label"`
	Step  string              `yaml:"step"`
	Link  string              `yaml:"link"` // Terraform type of a link resource
	Name  string              `yaml:"name"`
	Kind  string              `yaml:"kind"` // force a kind (flow)
	Style *document.EdgeStyle `yaml:"style"`
	Attr  string              `yaml:"attr"` // references edges
}

// Meta is what the gallery lists.
type Meta struct {
	ID       string   `json:"id"` // <provider>/<slug>
	Provider string   `json:"provider"`
	Slug     string   `json:"slug"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
	Source   string   `json:"source"`
	Image    string   `json:"image,omitempty"`
	// ImageSource is the page the bundled diagram was taken from (credit).
	ImageSource string `json:"image_source,omitempty"`
	// DiagramFile is the bundled copy under the template directory, if any.
	DiagramFile string `json:"-"`
	Description string `json:"description"`
	Elements    int    `json:"elements"`
}

// TypeInfo lets the gallery draw a preview without loading every element.
type TypeInfo struct {
	Icon      string `json:"icon,omitempty"`
	Container bool   `json:"container,omitempty"`
	Border    string `json:"border,omitempty"`
	Dash      string `json:"dash,omitempty"`
}

// Template is a listed reference architecture with its document.
type Template struct {
	Meta
	Document *document.Document  `json:"document"`
	Types    map[string]TypeInfo `json:"types"`
}

// Load reads every templates/<provider>/<slug>/template.yaml from fsys and
// renders it against the catalog.
func Load(fsys fs.FS, c *catalog.Catalog) ([]Template, error) {
	var out []Template
	err := fs.WalkDir(fsys, "templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Base(p) != "template.yaml" {
			return err
		}
		raw, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		var spec Spec
		if err := yaml.Unmarshal(raw, &spec); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		provider, slug := path.Base(path.Dir(path.Dir(p))), path.Base(path.Dir(p))
		if spec.Provider == "" {
			spec.Provider = provider
		}
		doc, err := Render(c, slug, &spec)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		types := map[string]TypeInfo{}
		for _, n := range doc.Nodes {
			if e, ok := c.Get(n.Type); ok {
				ti := TypeInfo{Icon: e.Icon, Container: e.Kind == catalog.KindContainer}
				if e.Style != nil {
					ti.Border, ti.Dash = e.Style.Border, e.Style.Dash
				}
				types[n.Type] = ti
			}
		}
		meta := Meta{ID: provider + "/" + slug, Provider: provider, Slug: slug, Title: spec.Title, Category: spec.Category, Tags: spec.Tags, Source: spec.Source, Description: spec.Description, Elements: countElements(c, doc)}
		// A bundled copy of the vendor diagram (credited on the card) is served
		// locally; without one the card shows the generated miniature.
		for _, ext := range []string{"jpg", "svg", "png", "webp"} {
			f := path.Join(path.Dir(p), "diagram."+ext)
			if _, err := fs.Stat(fsys, f); err == nil {
				meta.DiagramFile, meta.Image, meta.ImageSource = f, "/api/templates/"+provider+"/"+slug+"/diagram", spec.Image
				break
			}
		}
		out = append(out, Template{Meta: meta, Document: doc, Types: types})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Title < out[j].Title
	})
	return out, nil
}

func countElements(c *catalog.Catalog, d *document.Document) int {
	n := 0
	for _, node := range d.Nodes {
		if e, ok := c.Get(node.Type); ok && c.HasTerraform(e.Provider) {
			n++
		}
	}
	return n
}

// Render turns a spec into a laid-out document.
func Render(c *catalog.Catalog, slug string, spec *Spec) (*document.Document, error) {
	d := document.New(spec.Title)
	d.Steps = spec.Steps
	ids := map[string]bool{}
	for i, sn := range spec.Nodes {
		typ, err := resolveType(c, spec.Provider, sn)
		if err != nil {
			return nil, fmt.Errorf("node %d (%s): %w", i, sn.Name, err)
		}
		e, _ := c.Get(typ)
		id := sn.ID
		if id == "" {
			id = slugify(sn.Name)
		}
		if ids[id] {
			return nil, fmt.Errorf("duplicate node id %q", id)
		}
		ids[id] = true
		name := sn.Name
		if name == "" {
			name = id
		}
		props := map[string]any{}
		if p, ok := e.Props["properties"].(map[string]any); ok {
			for k, v := range p {
				if pm, ok := v.(map[string]any); ok && pm["default"] != nil {
					props[k] = pm["default"]
				}
			}
		}
		for k, v := range sn.Props {
			props[k] = v
		}
		if sn.Parent != "" && !ids[sn.Parent] {
			return nil, fmt.Errorf("node %s: parent %q must be declared before it", id, sn.Parent)
		}
		n := document.Node{ID: id, Type: typ, Name: name, Parent: sn.Parent, Props: props, Caption: sn.Caption, Step: sn.Step}
		if len(sn.At) >= 2 {
			n.Layout.Row, n.Layout.Col = sn.At[0], sn.At[1]
			if len(sn.At) >= 4 {
				n.Layout.RowSpan, n.Layout.ColSpan = sn.At[2], sn.At[3]
			}
		}
		if len(sn.Size) == 2 {
			n.Layout.W, n.Layout.H = sn.Size[0], sn.Size[1]
		}
		d.Nodes = append(d.Nodes, n)
	}
	byID := d.Index()
	linkNames := map[string]int{} // two links between the same pair (ECMP tunnels) need distinct names
	uniqueLink := func(name string) string {
		linkNames[name]++
		if n := linkNames[name]; n > 1 {
			return fmt.Sprintf("%s_%d", name, n)
		}
		return name
	}
	// Required properties nobody supplies get a placeholder, so every template
	// validates and can be planned after the obvious edits.
	for i := range d.Nodes {
		n := &d.Nodes[i]
		e, _ := c.Get(n.Type)
		given := provide.Values(c, byID, n)
		autoNamed := e.Terraform != nil && e.Terraform.Role == catalog.RoleResource
		for _, req := range e.Required() {
			if _, set := n.Props[req]; set {
				continue
			}
			if _, ok := given[req]; ok || (req == "name" && autoNamed) {
				continue
			}
			schema, _ := e.Property(req)
			n.Props[req] = placeholder(req, schema)
		}
	}
	for i, se := range spec.Edges {
		src, sok := byID[se.From]
		dst, dok := byID[se.To]
		if !sok || !dok {
			return nil, fmt.Errorf("edge %d: unknown endpoint %q or %q", i, se.From, se.To)
		}
		ed := document.Edge{ID: fmt.Sprintf("e%d-%s-%s", i+1, se.From, se.To), Source: se.From, Target: se.To, Label: se.Label, Step: se.Step, Style: se.Style}
		switch {
		case se.Link != "":
			linkID := spec.Provider + ".res." + se.Link
			b, ok := c.LinkFor(src.Type, dst.Type, linkID)
			if !ok || b.Link.ID != linkID {
				return nil, fmt.Errorf("edge %d: %s does not join %s and %s", i, se.Link, src.Type, dst.Type)
			}
			ed.Kind, ed.Type, ed.Name = catalog.LinkKind, linkID, se.Name
			if ed.Name == "" {
				ed.Name = slugify(src.Name + "_" + dst.Name)
			}
			ed.Name = uniqueLink(ed.Name)
			ed.Props = linkProps(c, b)
		case se.Kind != "":
			ed.Kind = se.Kind
		default:
			rule, ok := c.Connection(src.Type, dst.Type)
			if !ok {
				return nil, fmt.Errorf("edge %d: %s cannot connect to %s", i, src.Type, dst.Type)
			}
			ed.Kind = rule.Kind
			// The arrow's preconditions become facts of the drawing (a DNS record
			// pointing at a VM means the VM has a public address).
			for k, v := range rule.RequiresFrom {
				src.Props[k] = v
			}
			for k, v := range rule.RequiresTo {
				dst.Props[k] = v
			}
			if rule.Kind == catalog.ReferencesKind {
				ed.Attr, ed.Output = se.Attr, "id"
				if ed.Attr == "" {
					ed.Kind = catalog.FlowKind // a plain arrow unless an attribute is named
				}
			}
			if rule.Kind == catalog.LinkKind {
				b, _ := c.LinkFor(src.Type, dst.Type, rule.Type)
				ed.Type, ed.Name, ed.Props = rule.Type, uniqueLink(slugify(src.Name+"_"+dst.Name)), linkProps(c, b)
			}
		}
		d.Edges = append(d.Edges, ed)
	}
	// Reading order: actors and data centers on the left, clouds to the right.
	rank := func(n document.Node) int {
		if n.Parent != "" {
			return 2 // children keep their relative order
		}
		if !c.HasTerraform(strings.SplitN(n.Type, ".", 2)[0]) {
			return 0 // actors, data centers, groups first
		}
		return 1 // cloud roots
	}
	sort.SliceStable(d.Nodes, func(i, j int) bool { return rank(d.Nodes[i]) < rank(d.Nodes[j]) })
	spanReferences(c, d)
	adoptRegion(c, d)
	layout.Auto(c, d)
	return d, nil
}

// placeholder picks a value that passes validation for a required property.
func placeholder(name string, schema map[string]any) any {
	switch name {
	case "project_id":
		return "my-project-123456"
	case "subscription_id":
		return "00000000-0000-0000-0000-000000000000"
	case "domain":
		return "example.com"
	case "location":
		return "westeurope"
	}
	if schema == nil {
		return "changeme"
	}
	if list, ok := schema["enum"].([]any); ok && len(list) > 0 {
		return list[0]
	}
	switch schema["type"] {
	case "object":
		return map[string]any{}
	case "array":
		return []any{}
	case "number", "integer":
		return 1
	case "boolean":
		return true
	}
	return "changeme"
}

// linkProps starts a link edge with the link's defaults and a placeholder for
// every other required attribute besides the two ends.
func linkProps(c *catalog.Catalog, b catalog.LinkBinding) map[string]any {
	props := map[string]any{}
	for k, v := range b.Link.Defaults {
		props[k] = v
	}
	e, ok := c.Get(b.Link.ID)
	if !ok {
		return props
	}
	for _, req := range e.Required() {
		if req == b.SrcAttr || req == b.DstAttr {
			continue
		}
		if _, set := props[req]; set {
			continue
		}
		schema, _ := e.Property(req)
		props[req] = placeholder(req, schema)
	}
	return props
}

func resolveType(c *catalog.Catalog, provider string, sn SpecNode) (string, error) {
	if sn.Type != "" {
		if _, ok := c.Get(sn.Type); ok {
			return sn.Type, nil
		}
		full := provider + "." + sn.Type
		if _, ok := c.Get(full); ok {
			return full, nil
		}
		return "", fmt.Errorf("unknown element %q", sn.Type)
	}
	if sn.TF == "" {
		return "", fmt.Errorf("type or tf is required")
	}
	for _, e := range c.Entries {
		if e.Provider == provider && e.Terraform != nil && e.Terraform.Import != nil && e.Terraform.Import.Resource == sn.TF {
			return e.ID, nil
		}
	}
	id := provider + ".res." + sn.TF
	if _, ok := c.Get(id); !ok {
		return "", fmt.Errorf("unknown Terraform type %q", sn.TF)
	}
	return id, nil
}

func slugify(s string) string {
	var b strings.Builder
	prev := '-'
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prev = r
		default:
			if prev != '-' {
				b.WriteRune('-')
			}
			prev = '-'
		}
	}
	return strings.Trim(b.String(), "-")
}

// spanReferences turns a spanning band's arrows at leaf elements into the
// reference edges the generator and the layout work from (the subnets an Auto
// Scaling group covers), the way the client does when a band is dragged over
// containers. Undocumented "scales" arrows are dropped: the band drawn around
// the instances says it.
func spanReferences(c *catalog.Catalog, d *document.Document) {
	byID := d.Index()
	have := map[string]bool{}
	for _, e := range d.Edges {
		if e.Kind == "references" {
			have[e.Source+"\x00"+e.Target] = true
		}
	}
	var keep, add []document.Edge
	for _, e := range d.Edges {
		var sp *catalog.Span
		if src := byID[e.Source]; src != nil {
			if ent, ok := c.Get(src.Type); ok {
				sp = ent.Span
			}
		}
		if sp == nil || e.Kind == "references" {
			keep = append(keep, e)
			continue
		}
		allowed := map[string]bool{}
		for _, t := range sp.Elements {
			allowed[t] = true
		}
		var cont *document.Node
		for cur := byID[e.Target]; cur != nil; cur = byID[cur.Parent] {
			if allowed[cur.Type] {
				cont = cur
				break
			}
		}
		if cont == nil {
			keep = append(keep, e)
			continue
		}
		if key := e.Source + "\x00" + cont.ID; !have[key] {
			have[key] = true
			out := sp.Outputs[cont.Type]
			if out == "" {
				out = "id"
			}
			add = append(add, document.Edge{ID: "span-" + e.Source + "-" + cont.ID, Kind: "references", Source: e.Source, Target: cont.ID, Attr: sp.Attr, Output: out})
		}
		if e.Step != "" || (e.Label != "" && e.Label != "scales") {
			keep = append(keep, e)
		}
	}
	d.Edges = append(keep, add...)
}

// adoptRegion moves regional resources that a spec hung directly off an AWS
// account into the account's only region, so the diagram shows one region
// box holding its services instead of an empty region beside them.
func adoptRegion(c *catalog.Catalog, d *document.Document) {
	byID := d.Index()
	regions := map[string][]string{} // account id -> region ids
	for _, n := range d.Nodes {
		if n.Type == "aws.region" && n.Parent != "" {
			regions[n.Parent] = append(regions[n.Parent], n.ID)
		}
	}
	for i := range d.Nodes {
		n := &d.Nodes[i]
		p := byID[n.Parent]
		if p == nil || p.Type != "aws.account" || len(regions[p.ID]) != 1 {
			continue
		}
		e, ok := c.Get(n.Type)
		if !ok || e.Kind == catalog.KindContainer || !allowsParent(e, "aws.region") {
			continue
		}
		n.Parent = regions[p.ID][0]
	}
}

func allowsParent(e *catalog.Entry, parent string) bool {
	for _, a := range e.AllowedParents {
		if a == "*" || a == parent {
			return true
		}
	}
	return false
}
