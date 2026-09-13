// Package convert rewrites a diagram for another cloud provider using the
// equivalence table in catalog/equivalences.yaml: element counterparts,
// property renames, size classes, region names and root container chains.
// Anything without a counterpart is dropped and reported.
package convert

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
)

// Table is the parsed equivalence file.
type Table struct {
	Regions  []map[string]string            `yaml:"regions"`
	Classes  map[string][]map[string]string `yaml:"classes"`
	Roots    map[string]Root                `yaml:"roots"`
	Elements []Element                      `yaml:"elements"`
}

// Root describes a provider's container chain.
type Root struct {
	Chain      []string `yaml:"chain"`
	RegionProp string   `yaml:"region_prop"` // "<element id>.<prop>"
}

// Element is one equivalence row. Provider keys hold element ids (or empty).
type Element struct {
	IDs        map[string]string            `yaml:",inline"`
	Props      map[string]string            `yaml:"props"`       // source prop -> target prop (same name family)
	PropsAzure map[string]string            `yaml:"props_azure"` // overrides when the target is azure
	Class      map[string]map[string]string `yaml:"class"`       // class name -> provider -> prop holding the class value
	RegionProp map[string]string            `yaml:"region_prop"` // provider -> prop receiving the region
}

// Load parses catalog/equivalences.yaml from the catalog FS.
func Load(fsys fs.FS) (*Table, error) {
	raw, err := fs.ReadFile(fsys, "catalog/equivalences.yaml")
	if err != nil {
		return nil, err
	}
	var t Table
	if err := yaml.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("equivalences.yaml: %w", err)
	}
	// yaml inline of a map with other keys: strip the known keys.
	for i := range t.Elements {
		for k := range t.Elements[i].IDs {
			switch k {
			case "props", "props_azure", "class", "region_prop":
				delete(t.Elements[i].IDs, k)
			}
		}
	}
	return &t, nil
}

// Report lists what did not convert.
type Report struct {
	Converted int      `json:"converted"`
	Dropped   []string `json:"dropped,omitempty"` // "<name> (<type>): no counterpart in <provider>"
	Edges     []string `json:"edges,omitempty"`   // dropped connections
	Notes     []string `json:"notes,omitempty"`   // property values that had no mapping
}

// Run converts d to the target provider.
func Run(c *catalog.Catalog, t *Table, d *document.Document, target string) (*document.Document, Report, error) {
	root, ok := t.Roots[target]
	if !ok {
		return nil, Report{}, fmt.Errorf("no root chain for provider %q", target)
	}
	out := document.New(d.Name)
	out.Steps = d.Steps
	rep := Report{}
	byID := d.Index()

	// Which provider(s) does the source use, and which region?
	sourceRegion := map[string]string{} // source root node id -> region value
	for _, n := range d.Nodes {
		if e, ok := c.Get(n.Type); ok && e.Terraform != nil && e.Terraform.Role == catalog.RoleRegion {
			for _, v := range n.Props {
				sourceRegion[n.ID] = fmt.Sprint(v)
			}
		}
	}

	// Target root chain: one chain per source top-level root (account).
	// Region-role source nodes collapse into the chain's region prop.
	newParent := map[string]string{} // source node id -> target node id (for containers/roots)
	var chainBottomFor func(srcID string) string
	chainCache := map[string]string{}
	chainBottomFor = func(srcRootID string) string {
		if id, ok := chainCache[srcRootID]; ok {
			return id
		}
		region := ""
		// find region under this source root by walking source nodes
		for id, r := range sourceRegion {
			if n := byID[id]; n != nil && (n.ID == srcRootID || n.Parent == srcRootID) {
				region = r
			}
		}
		parent := ""
		for i, elemID := range root.Chain {
			e, _ := c.Get(elemID)
			n := document.Node{ID: fmt.Sprintf("%s-%d-%s", strings.SplitN(elemID, ".", 2)[1], i, short(srcRootID)), Type: elemID, Name: defaultName(e, srcRootID, byID), Parent: parent, Props: map[string]any{}}
			// defaults
			if props, ok := e.Props["properties"].(map[string]any); ok {
				for k, v := range props {
					if pm, ok := v.(map[string]any); ok && pm["default"] != nil {
						n.Props[k] = pm["default"]
					}
				}
			}
			if strings.HasPrefix(root.RegionProp, elemID+".") && region != "" {
				if tr := translateRegion(t, region, target); tr != "" {
					n.Props[strings.TrimPrefix(root.RegionProp, elemID+".")] = tr
				}
			}
			n.Layout = document.Layout{X: 0, Y: 0, W: 1200 - float64(i)*60, H: 800 - float64(i)*60}
			if i > 0 {
				n.Layout.X, n.Layout.Y = 30, 50
			}
			out.Nodes = append(out.Nodes, n)
			parent = n.ID
		}
		chainCache[srcRootID] = parent
		return parent
	}

	// Map every non-root node.
	idMap := map[string]string{} // source -> target node id
	for _, n := range d.Nodes {
		e, ok := c.Get(n.Type)
		if !ok {
			continue
		}
		if !c.HasTerraform(e.Provider) {
			// Groups, actors and notes are cloud-neutral: copied as they are.
			cp := n
			cp.Props = map[string]any{}
			for k, v := range n.Props {
				cp.Props[k] = v
			}
			out.Nodes = append(out.Nodes, cp)
			idMap[n.ID] = n.ID
			continue
		}
		if e.Terraform == nil {
			continue
		}
		if e.Terraform.Role == catalog.RoleAccount || e.Terraform.Role == catalog.RoleRegion {
			continue // collapsed into the target chain
		}
		if e.Terraform.Role == catalog.RoleResource {
			rep.Dropped = append(rep.Dropped, fmt.Sprintf("%s (%s): generated elements are provider-specific", n.Name, e.Terraform.Resource))
			continue
		}
		row := findRow(t, n.Type)
		targetType := ""
		if row != nil {
			targetType = row.IDs[target]
		}
		if targetType == "" {
			rep.Dropped = append(rep.Dropped, fmt.Sprintf("%s (%s): no counterpart in %s", n.Name, e.Label, target))
			continue
		}
		te, ok := c.Get(targetType)
		if !ok {
			rep.Dropped = append(rep.Dropped, fmt.Sprintf("%s: target %s missing from catalog", n.Name, targetType))
			continue
		}
		tn := document.Node{ID: n.ID, Type: targetType, Name: n.Name, Props: map[string]any{}, Layout: n.Layout, Caption: n.Caption, Step: n.Step}
		// defaults first, then mapped properties
		if props, ok := te.Props["properties"].(map[string]any); ok {
			for k, v := range props {
				if pm, ok := v.(map[string]any); ok && pm["default"] != nil {
					tn.Props[k] = pm["default"]
				}
			}
		}
		propMap := row.Props
		if target == "azure" && row.PropsAzure != nil {
			propMap = row.PropsAzure
		}
		for src, dst := range propMap {
			if v, ok := n.Props[src]; ok {
				if _, exists := te.Property(dst); exists {
					tn.Props[dst] = v
				}
			}
		}
		for className, per := range row.Class {
			srcProp, dstProp := per[e.Provider], per[target]
			if v, ok := n.Props[srcProp]; ok && dstProp != "" {
				if mapped := translateClass(t, className, e.Provider, fmt.Sprint(v), target); mapped != "" {
					tn.Props[dstProp] = mapped
				} else {
					rep.Notes = append(rep.Notes, fmt.Sprintf("%s: no %s class for %v in %s, kept the default", n.Name, className, v, target))
				}
			}
		}
		if rp, ok := row.RegionProp[target]; ok {
			if r := regionOf(&n, byID, sourceRegion); r != "" {
				if tr := translateRegion(t, r, target); tr != "" {
					tn.Props[rp] = tr
				}
			}
		}
		out.Nodes = append(out.Nodes, tn)
		idMap[n.ID] = tn.ID
		rep.Converted++
	}

	// Parents: nearest converted ancestor, else the target chain bottom.
	// out.Nodes grows while chains are created, so look nodes up by id each time.
	find := func(id string) *document.Node {
		for i := range out.Nodes {
			if out.Nodes[i].ID == id {
				return &out.Nodes[i]
			}
		}
		return nil
	}
	for _, n := range d.Nodes {
		tid, ok := idMap[n.ID]
		if !ok {
			continue
		}
		parent := ""
		for cur := byID[n.Parent]; cur != nil; cur = byID[cur.Parent] {
			if p, ok := idMap[cur.ID]; ok {
				parent = p
				break
			}
			if ce, ok := c.Get(cur.Type); ok && ce.Terraform != nil && (ce.Terraform.Role == catalog.RoleAccount || ce.Terraform.Role == catalog.RoleRegion) {
				parent = chainBottomFor(topRoot(&n, byID))
				break
			}
		}
		if parent == "" {
			parent = chainBottomFor(topRoot(&n, byID))
		}
		tn := find(tid)
		te, _ := c.Get(tn.Type)
		if pn := find(parent); pn != nil && !c.CanContain(pn.Type, tn.Type) {
			bottom := chainBottomFor(topRoot(&n, byID))
			if bn := find(bottom); bn != nil && !c.CanContain(bn.Type, tn.Type) {
				for i := range out.Nodes {
					if oe, _ := c.Get(out.Nodes[i].Type); !oe.Transparent && c.CanContain(out.Nodes[i].Type, tn.Type) {
						bottom = out.Nodes[i].ID
						break
					}
				}
			}
			rep.Notes = append(rep.Notes, fmt.Sprintf("%s (%s) moved to %s: %s cannot contain it", tn.Name, te.Label, find(bottom).Name, pn.Type))
			parent = bottom
			tn = find(tid)
		}
		tn.Parent = parent
	}
	outIdx := out.Index()
	_ = newParent

	// Edges: keep those whose kind exists for the converted pair.
	for _, e := range d.Edges {
		s, sok := idMap[e.Source]
		t2, tok := idMap[e.Target]
		if !sok || !tok {
			rep.Edges = append(rep.Edges, fmt.Sprintf("%s -> %s (%s): an endpoint was dropped", byID[e.Source].Name, byID[e.Target].Name, e.Kind))
			continue
		}
		rule, ok := c.Connection(outIdx[s].Type, outIdx[t2].Type)
		if !ok {
			rep.Edges = append(rep.Edges, fmt.Sprintf("%s -> %s (%s): no equivalent connection in %s", byID[e.Source].Name, byID[e.Target].Name, e.Kind, target))
			continue
		}
		out.Edges = append(out.Edges, document.Edge{ID: e.ID, Kind: rule.Kind, Source: s, Target: t2, Label: e.Label, Step: e.Step, Style: e.Style})
	}
	// Root containers usually need an identifier only the user knows.
	for _, elemID := range root.Chain {
		if e, ok := c.Get(elemID); ok {
			for _, req := range e.Required() {
				for _, n := range out.Nodes {
					if n.Type == elemID {
						if _, set := n.Props[req]; !set {
							rep.Notes = append(rep.Notes, fmt.Sprintf("%s: set %s before planning", n.Name, req))
						}
					}
				}
			}
		}
	}
	sort.Strings(rep.Dropped)
	sort.Strings(rep.Edges)
	sort.Strings(rep.Notes)
	return out, rep, nil
}

func findRow(t *Table, id string) *Element {
	for i := range t.Elements {
		for _, v := range t.Elements[i].IDs {
			if v == id {
				return &t.Elements[i]
			}
		}
	}
	return nil
}

func translateRegion(t *Table, value, target string) string {
	for _, row := range t.Regions {
		for _, v := range row {
			if v == value {
				return row[target]
			}
		}
	}
	return ""
}

func translateClass(t *Table, class, from, value, target string) string {
	for _, row := range t.Classes[class] {
		if row[from] == value {
			return row[target]
		}
	}
	return ""
}

// regionOf finds the region value governing a node (nearest region-role
// ancestor, or a region-ish property on the node itself).
func regionOf(n *document.Node, byID map[string]*document.Node, regions map[string]string) string {
	for _, k := range []string{"region", "location"} {
		if v, ok := n.Props[k]; ok && fmt.Sprint(v) != "" {
			return fmt.Sprint(v)
		}
	}
	for cur := byID[n.Parent]; cur != nil; cur = byID[cur.Parent] {
		if r, ok := regions[cur.ID]; ok {
			return r
		}
		for _, k := range []string{"region", "location"} {
			if v, ok := cur.Props[k]; ok && fmt.Sprint(v) != "" {
				return fmt.Sprint(v)
			}
		}
	}
	return ""
}

func topRoot(n *document.Node, byID map[string]*document.Node) string {
	cur := n
	for cur.Parent != "" {
		p, ok := byID[cur.Parent]
		if !ok {
			break
		}
		cur = p
	}
	return cur.ID
}

func defaultName(e *catalog.Entry, srcRootID string, byID map[string]*document.Node) string {
	if r, ok := byID[srcRootID]; ok && r.Name != "" {
		if e.Terraform != nil && e.Terraform.Role == catalog.RoleAccount {
			return r.Name
		}
	}
	return strings.ToLower(strings.ReplaceAll(e.Label, " ", "-"))
}

func short(s string) string {
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, strings.ToLower(s))
	if len(s) > 6 {
		s = s[:6]
	}
	return s
}
