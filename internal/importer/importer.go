// Package importer builds a diagram from an existing Terraform state so
// infrastructure that was never drawn can be brought under iagram. It is
// driven by the catalog's terraform.import mappings; no resource type is
// named here.
//
// The result is a starting point: nodes and containment are recovered,
// connections (arrows) are not, and properties the state does not carry are
// left at their defaults. A plan after import shows the gap.
package importer

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
)

// Resource is a state resource in a shape-independent form.
type Resource struct {
	Address string
	Module  string // "module.a.module.b" or ""
	Type    string
	Name    string
	Attrs   map[string]any
}

// Report lists what could not be mapped.
type Report struct {
	Imported  int      `json:"imported"`
	Skipped   []string `json:"skipped,omitempty"`  // resource types with no mapping
	Unplaced  []string `json:"unplaced,omitempty"` // nodes whose parent could not be resolved
	Providers []string `json:"providers,omitempty"`
}

// ParseState accepts a raw terraform.tfstate (format version 4) or the JSON
// printed by `tofu show -json`, and returns managed resources.
func ParseState(raw []byte) ([]Resource, error) {
	var probe struct {
		Version       int    `json:"version"`
		FormatVersion string `json:"format_version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	switch {
	case probe.FormatVersion != "":
		return parseShowJSON(raw)
	case probe.Version == 4:
		return parseTFState(raw)
	}
	return nil, fmt.Errorf("state: unsupported format (want terraform.tfstate v4 or `tofu show -json` output)")
}

func parseTFState(raw []byte) ([]Resource, error) {
	var st struct {
		Resources []struct {
			Module    string `json:"module"`
			Mode      string `json:"mode"`
			Type      string `json:"type"`
			Name      string `json:"name"`
			Instances []struct {
				IndexKey   any            `json:"index_key"`
				Attributes map[string]any `json:"attributes"`
			} `json:"instances"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	var out []Resource
	for _, r := range st.Resources {
		if r.Mode != "managed" {
			continue
		}
		for _, inst := range r.Instances {
			addr := r.Type + "." + r.Name
			if inst.IndexKey != nil {
				addr += fmt.Sprintf("[%v]", inst.IndexKey)
			}
			if r.Module != "" {
				addr = r.Module + "." + addr
			}
			out = append(out, Resource{Address: addr, Module: r.Module, Type: r.Type, Name: r.Name, Attrs: inst.Attributes})
		}
	}
	return out, nil
}

func parseShowJSON(raw []byte) ([]Resource, error) {
	type module struct {
		Address   string `json:"address"`
		Resources []struct {
			Address string         `json:"address"`
			Mode    string         `json:"mode"`
			Type    string         `json:"type"`
			Name    string         `json:"name"`
			Values  map[string]any `json:"values"`
		} `json:"resources"`
		ChildModules []json.RawMessage `json:"child_modules"`
	}
	var st struct {
		Values struct {
			RootModule json.RawMessage `json:"root_module"`
		} `json:"values"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	var out []Resource
	var walk func(raw json.RawMessage) error
	walk = func(raw json.RawMessage) error {
		if len(raw) == 0 {
			return nil
		}
		var m module
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		for _, r := range m.Resources {
			if r.Mode != "managed" {
				continue
			}
			out = append(out, Resource{Address: r.Address, Module: m.Address, Type: r.Type, Name: r.Name, Attrs: r.Values})
		}
		for _, c := range m.ChildModules {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(st.Values.RootModule); err != nil {
		return nil, err
	}
	return out, nil
}

// Build turns resources into a document using the catalog import mappings.
func Build(c *catalog.Catalog, name string, resources []Resource) (*document.Document, Report) {
	b := &builder{c: c, doc: document.New(name), byPrimaryID: map[string]string{}, groups: map[string]string{}}
	b.indexEntries()
	b.run(resources)
	return b.doc, b.report
}

type builder struct {
	c      *catalog.Catalog
	doc    *document.Document
	report Report

	byResource  map[string]*catalog.Entry // resource type -> entry
	accountsBy  map[string]*catalog.Entry // provider -> account entry
	regionsBy   map[string]*catalog.Entry // provider -> region entry
	byPrimaryID map[string]string         // primary id -> node id
	groups      map[string]string         // "account:aws:123" / "region:aws:123:eu-west-1" -> node id
	skipped     map[string]bool
}

func (b *builder) indexEntries() {
	b.byResource = map[string]*catalog.Entry{}
	b.accountsBy = map[string]*catalog.Entry{}
	b.regionsBy = map[string]*catalog.Entry{}
	b.skipped = map[string]bool{}
	for i := range b.c.Entries {
		e := &b.c.Entries[i]
		if e.Terraform == nil || e.Terraform.Import == nil {
			continue
		}
		switch e.Terraform.Role {
		case "account":
			b.accountsBy[e.Provider] = e
		case "region":
			b.regionsBy[e.Provider] = e
		default:
			if e.Terraform.Import.Resource != "" {
				b.byResource[e.Terraform.Import.Resource] = e
			}
		}
	}
}

type pending struct {
	res Resource
	e   *catalog.Entry
	idx int // index into doc.Nodes; the slice grows, so no pointers
}

func (b *builder) run(resources []Resource) {
	// Pass 1: create nodes for mapped resources, index their primary ids.
	var pend []pending
	var raw []pending // generated elements, wired by reference recovery below
	for _, r := range resources {
		e, ok := b.byResource[r.Type]
		if !ok {
			// No curated mapping: use the generated element for the type when the
			// provider schema knows it, so any resource round-trips.
			if gid, ok := b.c.GeneratedID(r.Type); ok {
				if ge, ok := b.c.Get(gid); ok {
					n := b.rawNode(ge, r)
					b.doc.Nodes = append(b.doc.Nodes, n)
					raw = append(raw, pending{res: r, e: ge, idx: len(b.doc.Nodes) - 1})
					if id, ok := lookup(r.Attrs, "id"); ok && fmt.Sprint(id) != "" {
						b.byPrimaryID[fmt.Sprint(id)] = n.ID
					}
					continue
				}
			}
			if !b.skipped[r.Type] {
				b.skipped[r.Type] = true
				b.report.Skipped = append(b.report.Skipped, r.Type)
			}
			continue
		}
		imp := e.Terraform.Import
		idAttr := imp.ID
		if idAttr == "" {
			idAttr = "id"
		}
		n := document.Node{ID: nodeID(e, r.Address), Type: e.ID, Name: b.nameFor(r, imp), Props: map[string]any{}}
		for prop, path := range imp.Props {
			if v, ok := b.value(r.Attrs, path); ok {
				n.Props[prop] = v
			}
		}
		if pid, ok := lookup(r.Attrs, idAttr); ok {
			if s := fmt.Sprint(pid); s != "" {
				b.byPrimaryID[s] = n.ID
			}
		}
		b.doc.Nodes = append(b.doc.Nodes, n)
		pend = append(pend, pending{res: r, e: e, idx: len(b.doc.Nodes) - 1})
	}
	// Pass 2: parents. Explicit attribute first, else the account/region
	// grouping, else the first existing container of an allowed type.
	seenProv := map[string]bool{}
	for _, p := range pend {
		seenProv[p.e.Provider] = true
		parent := ""
		if attr := p.e.Terraform.Import.Parent; attr != "" {
			if v, ok := lookup(p.res.Attrs, attr); ok {
				parent = b.byPrimaryID[fmt.Sprint(v)]
			}
		}
		if parent == "" {
			parent = b.groupParent(p)
		}
		if parent == "" {
			if parent = b.fallbackParent(p.e); parent != "" {
				b.report.Unplaced = append(b.report.Unplaced, fmt.Sprintf("%s placed in %s by fallback", p.res.Address, parent))
			} else {
				b.report.Unplaced = append(b.report.Unplaced, p.res.Address)
			}
		}
		b.doc.Nodes[p.idx].Parent = parent
	}
	// Generated elements: parent from the account/region grouping; attribute
	// values equal to another node's primary id become reference edges.
	for _, p := range raw {
		seenProv[p.e.Provider] = true
		parent := b.groupParent(p)
		if parent == "" {
			parent = b.fallbackParent(p.e)
		}
		b.doc.Nodes[p.idx].Parent = parent
		b.recoverReferences(p.idx)
	}
	for prov := range seenProv {
		b.report.Providers = append(b.report.Providers, prov)
	}
	sort.Strings(b.report.Providers)
	b.report.Imported = len(pend) + len(raw)
	// Containers created after their children must still sort deterministically.
	sort.Slice(b.doc.Nodes, func(i, j int) bool { return b.doc.Nodes[i].ID < b.doc.Nodes[j].ID })
}

// rawNode builds a generated-element node from a state resource: every
// configurable attribute (per the provider schema) that has a value.
func (b *builder) rawNode(e *catalog.Entry, r Resource) document.Node {
	n := document.Node{ID: nodeID(e, r.Address), Type: e.ID, Name: r.Name, Props: map[string]any{}}
	if r.Module != "" && (r.Name == "this" || r.Name == "main" || r.Name == "default") {
		segs := strings.Split(r.Module, ".")
		n.Name = segs[len(segs)-1]
	}
	props, _ := e.Props["properties"].(map[string]any)
	for attr := range props {
		v, ok := r.Attrs[attr]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if t == "" {
				continue
			}
		case []any:
			if len(t) == 0 {
				continue
			}
		case map[string]any:
			if len(t) == 0 {
				continue
			}
		}
		n.Props[attr] = v
	}
	return n
}

// recoverReferences turns attribute values that equal another node's primary
// id into references edges, so containment and wiring survive a round trip.
func (b *builder) recoverReferences(idx int) {
	n := &b.doc.Nodes[idx]
	names := make([]string, 0, len(n.Props))
	for k := range n.Props {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, attr := range names {
		switch v := n.Props[attr].(type) {
		case string:
			if target, ok := b.byPrimaryID[v]; ok && target != n.ID {
				b.doc.Edges = append(b.doc.Edges, document.Edge{ID: edgeID(n.ID, attr, target), Kind: catalog.ReferencesKind, Source: n.ID, Target: target, Attr: attr, Output: "id"})
				delete(n.Props, attr) // the edge carries it; the generator writes the reference back
			}
		case []any:
			var keep []any
			for _, item := range v {
				s, _ := item.(string)
				if target, ok := b.byPrimaryID[s]; ok && s != "" && target != n.ID {
					b.doc.Edges = append(b.doc.Edges, document.Edge{ID: edgeID(n.ID, attr, target), Kind: catalog.ReferencesKind, Source: n.ID, Target: target, Attr: attr, Output: "id"})
					continue
				}
				keep = append(keep, item)
			}
			if len(keep) == 0 {
				delete(n.Props, attr)
			} else {
				n.Props[attr] = keep
			}
		}
	}
}

func edgeID(src, attr, dst string) string {
	h := sha1.Sum([]byte(src + "|" + attr + "|" + dst))
	return "e-" + hex.EncodeToString(h[:])[:8]
}

// groupParent resolves the account/region containers for a resource whose
// parent is not another resource (a VPC, a bucket), creating them on demand.
func (b *builder) groupParent(p pending) string {
	acct := b.accountsBy[p.e.Provider]
	reg := b.regionsBy[p.e.Provider]
	allowed := func(t string) bool {
		for _, a := range p.e.AllowedParents {
			if a == t {
				return true
			}
		}
		return false
	}
	var acctID string
	if acct != nil {
		val, ok := b.groupValue(p.res.Attrs, acct.Terraform.Import.From)
		if !ok {
			// No account on this resource (S3 ARNs, for one): reuse the only
			// account we know of rather than inventing an anonymous one.
			if id, single := b.soleGroup("account:" + p.e.Provider + ":"); single {
				acctID = id
			} else {
				acctID = b.ensureGroup("account:"+p.e.Provider+":", acct, "", "")
			}
		} else {
			acctID = b.ensureGroup("account:"+p.e.Provider+":"+val, acct, "", val)
		}
	}
	if reg != nil && allowed(reg.ID) {
		val, ok := b.groupValue(p.res.Attrs, reg.Terraform.Import.From)
		if ok && val != "" {
			return b.ensureGroup("region:"+p.e.Provider+":"+acctID+":"+val, reg, acctID, val)
		}
		if id, single := b.soleGroup("region:" + p.e.Provider + ":" + acctID + ":"); single {
			return id
		}
	}
	if acct != nil && allowed(acct.ID) {
		return acctID
	}
	return ""
}

// soleGroup returns the only group node with the given key prefix, if there
// is exactly one.
func (b *builder) soleGroup(prefix string) (string, bool) {
	var found string
	n := 0
	for k, id := range b.groups {
		if strings.HasPrefix(k, prefix) {
			found = id
			n++
		}
	}
	return found, n == 1
}

func (b *builder) ensureGroup(key string, e *catalog.Entry, parent, value string) string {
	if id, ok := b.groups[key]; ok {
		return id
	}
	name := value
	if name == "" {
		name = e.Label
	}
	n := document.Node{ID: nodeID(e, key), Type: e.ID, Name: strings.ToLower(name), Parent: parent, Props: map[string]any{}}
	if e.Terraform.Import.Prop != "" && value != "" {
		n.Props[e.Terraform.Import.Prop] = value
	}
	b.doc.Nodes = append(b.doc.Nodes, n)
	b.groups[key] = n.ID
	return n.ID
}

// groupValue reads the grouping value described by From.
func (b *builder) groupValue(attrs map[string]any, from string) (string, bool) {
	switch from {
	case "arn.account", "arn.region":
		arn, ok := lookup(attrs, "arn")
		if !ok {
			return "", false
		}
		parts := strings.Split(fmt.Sprint(arn), ":") // arn:partition:service:region:account:...
		if len(parts) < 6 {
			return "", false
		}
		if from == "arn.region" {
			return parts[3], parts[3] != ""
		}
		return parts[4], parts[4] != ""
	case "arm.subscription":
		id, ok := lookup(attrs, "id")
		if !ok {
			return "", false
		}
		segs := strings.Split(fmt.Sprint(id), "/")
		for i := 0; i+1 < len(segs); i++ {
			if strings.EqualFold(segs[i], "subscriptions") {
				return segs[i+1], true
			}
		}
		return "", false
	case "":
		return "", false
	}
	v, ok := lookup(attrs, from)
	if !ok || v == nil || fmt.Sprint(v) == "" {
		return "", false
	}
	return fmt.Sprint(v), true
}

func (b *builder) fallbackParent(e *catalog.Entry) string {
	for _, allowed := range e.AllowedParents {
		for i := range b.doc.Nodes {
			if b.doc.Nodes[i].Type == allowed {
				return b.doc.Nodes[i].ID
			}
		}
	}
	return ""
}

func (b *builder) nameFor(r Resource, imp *catalog.Import) string {
	if imp.Name != "" {
		if v, ok := lookup(r.Attrs, imp.Name); ok && v != nil && fmt.Sprint(v) != "" {
			return fmt.Sprint(v)
		}
	}
	if (r.Name == "this" || r.Name == "main" || r.Name == "default") && r.Module != "" {
		segs := strings.Split(r.Module, ".")
		return segs[len(segs)-1]
	}
	return r.Name
}

// value reads an attribute path with an optional transform suffix.
func (b *builder) value(attrs map[string]any, spec string) (any, bool) {
	path, transform, _ := strings.Cut(spec, "|")
	v, ok := lookup(attrs, path)
	if !ok || v == nil {
		return nil, false
	}
	switch {
	case transform == "last":
		s := fmt.Sprint(v)
		if s == "" {
			return nil, false
		}
		return s[len(s)-1:], true
	case strings.HasPrefix(transform, "bool:"):
		parts := strings.SplitN(strings.TrimPrefix(transform, "bool:"), ":", 2)
		if len(parts) != 2 {
			return v, true
		}
		if bv, _ := v.(bool); bv {
			return parts[0], true
		}
		return parts[1], true
	}
	if f, ok := v.(float64); ok {
		return f, true
	}
	return v, true
}

// lookup walks "a.b.0.c" through maps and lists.
func lookup(attrs map[string]any, path string) (any, bool) {
	var cur any = attrs
	for _, seg := range strings.Split(path, ".") {
		switch t := cur.(type) {
		case map[string]any:
			v, ok := t[seg]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(t) {
				return nil, false
			}
			cur = t[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

func nodeID(e *catalog.Entry, seed string) string {
	short := strings.SplitN(e.ID, ".", 2)[1]
	if i := strings.Index(short, "_"); i > 0 {
		short = short[:i]
	}
	h := sha1.Sum([]byte(seed))
	return short + "-" + hex.EncodeToString(h[:])[:6]
}
