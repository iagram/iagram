// Package validate checks a document against the catalog. Most structural
// rules are already unenforceable in the editor; this is the authoritative
// pass that also runs headless (CLI, CI) and covers what the UI cannot.
package validate

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/provide"
)

// Level of a problem.
const (
	Error   = "error"
	Warning = "warning"
)

// Problem is one finding, anchored to a node, an edge, or the document.
type Problem struct {
	Level   string `json:"level"`
	Node    string `json:"node,omitempty"`
	Edge    string `json:"edge,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// Result of a validation run.
type Result struct {
	Problems []Problem `json:"problems"`
}

// OK reports whether there are no errors (warnings allowed).
func (r Result) OK() bool {
	for _, p := range r.Problems {
		if p.Level == Error {
			return false
		}
	}
	return true
}

// Run validates d against c.
func Run(c *catalog.Catalog, d *document.Document) Result {
	v := &validator{c: c, d: d, nodes: d.Index()}
	v.nodes = d.Index()
	v.structure()
	v.props()
	v.edges()
	v.cidrs()
	v.collects()
	sort.SliceStable(v.out, func(i, j int) bool { return v.out[i].Level < v.out[j].Level })
	if v.out == nil {
		v.out = []Problem{}
	}
	return Result{Problems: v.out}
}

type validator struct {
	c     *catalog.Catalog
	d     *document.Document
	nodes map[string]*document.Node
	out   []Problem
}

func (v *validator) add(p Problem) { v.out = append(v.out, p) }

func (v *validator) structure() {
	seenIDs := map[string]bool{}
	names := map[string]string{} // type+name -> id
	for i := range v.d.Nodes {
		n := &v.d.Nodes[i]
		if seenIDs[n.ID] {
			v.add(Problem{Level: Error, Node: n.ID, Message: fmt.Sprintf("duplicate node id %q", n.ID)})
		}
		seenIDs[n.ID] = true

		e, ok := v.c.Get(n.Type)
		if !ok {
			v.add(Problem{Level: Error, Node: n.ID, Message: fmt.Sprintf("unknown type %q", n.Type)})
			continue
		}
		if n.Name == "" {
			v.add(Problem{Level: Error, Node: n.ID, Field: "name", Message: e.Label + " needs a name"})
		} else if prev, dup := names[n.Type+"/"+n.Name]; dup {
			v.add(Problem{Level: Error, Node: n.ID, Field: "name", Message: fmt.Sprintf("name %q already used by another %s (%s)", n.Name, e.Label, prev)})
		} else {
			names[n.Type+"/"+n.Name] = n.ID
		}

		parentType := catalog.Root
		if n.Parent != "" {
			if _, ok := v.nodes[n.Parent]; !ok {
				v.add(Problem{Level: Error, Node: n.ID, Message: fmt.Sprintf("parent %q does not exist", n.Parent)})
				continue
			}
			// Groups and other transparent containers do not count: the
			// element must fit where the group itself sits.
			if lp := v.logicalParent(n); lp != nil {
				parentType = lp.Type
			}
		}
		if !v.c.CanContain(parentType, n.Type) {
			where := "on the canvas"
			if parentType != catalog.Root {
				if pe, ok := v.c.Get(parentType); ok {
					where = "inside a " + pe.Label
				}
			}
			v.add(Problem{Level: Error, Node: n.ID, Message: fmt.Sprintf("%s cannot be placed %s", e.Label, where)})
		}
		if v.hasCycle(n) {
			v.add(Problem{Level: Error, Node: n.ID, Message: "containment cycle"})
		}
		for prop, want := range provide.Values(v.c, v.nodes, n) {
			if got, set := n.Props[prop]; set && got != nil && got != "" && fmt.Sprint(got) != want {
				v.add(Problem{Level: Warning, Node: n.ID, Field: prop, Message: fmt.Sprintf("%s is %v but the box it is drawn in gives %s; the box wins when generating", prop, got, want)})
			}
		}
	}
}

// logicalParent skips transparent containers (groups) on the way up.
func (v *validator) logicalParent(n *document.Node) *document.Node {
	return v.d.LogicalParent(v.nodes, n, func(typ string) bool {
		e, ok := v.c.Get(typ)
		return ok && e.Transparent
	})
}

func (v *validator) hasCycle(n *document.Node) bool {
	seen := map[string]bool{n.ID: true}
	for cur := n; cur.Parent != ""; {
		if seen[cur.Parent] {
			return true
		}
		seen[cur.Parent] = true
		next, ok := v.nodes[cur.Parent]
		if !ok {
			return false
		}
		cur = next
	}
	return false
}

func (v *validator) props() {
	// Attributes bound through a references edge count as set.
	bound := map[string]map[string]bool{}
	for _, e := range v.d.Edges {
		if e.Kind == catalog.ReferencesKind && e.Attr != "" {
			if bound[e.Source] == nil {
				bound[e.Source] = map[string]bool{}
			}
			bound[e.Source][e.Attr] = true
		}
	}
	for i := range v.d.Nodes {
		n := &v.d.Nodes[i]
		e, ok := v.c.Get(n.Type)
		if !ok {
			continue
		}
		for _, req := range e.Required() {
			if bound[n.ID][req] {
				continue
			}
			if val, ok := n.Props[req]; !ok || val == "" || val == nil {
				v.add(Problem{Level: Error, Node: n.ID, Field: req, Message: fmt.Sprintf("%s is required", req)})
			}
		}
		for name, val := range n.Props {
			schema, ok := e.Property(name)
			if !ok {
				v.add(Problem{Level: Warning, Node: n.ID, Field: name, Message: fmt.Sprintf("unknown property %q", name)})
				continue
			}
			v.checkValue(n, name, schema, val)
		}
	}
}

func (v *validator) checkValue(n *document.Node, name string, schema map[string]any, val any) {
	if val == nil {
		return
	}
	switch schema["type"] {
	case "string":
		s, ok := val.(string)
		if !ok {
			v.add(Problem{Level: Error, Node: n.ID, Field: name, Message: name + " must be a string"})
			return
		}
		if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
			found := false
			for _, e := range enum {
				if e == s {
					found = true
					break
				}
			}
			if !found {
				v.add(Problem{Level: Error, Node: n.ID, Field: name, Message: fmt.Sprintf("%s: %q is not one of the allowed values", name, s)})
			}
		}
		if schema["format"] == "cidr" && s != "" {
			if _, _, err := net.ParseCIDR(s); err != nil {
				v.add(Problem{Level: Error, Node: n.ID, Field: name, Message: fmt.Sprintf("%s: %q is not a valid CIDR", name, s)})
			}
		}
	case "integer", "number":
		f, ok := toFloat(val)
		if !ok {
			v.add(Problem{Level: Error, Node: n.ID, Field: name, Message: name + " must be a number"})
			return
		}
		if min, ok := toFloat(schema["minimum"]); ok && f < min {
			v.add(Problem{Level: Error, Node: n.ID, Field: name, Message: fmt.Sprintf("%s must be >= %v", name, min)})
		}
		if max, ok := toFloat(schema["maximum"]); ok && f > max {
			v.add(Problem{Level: Error, Node: n.ID, Field: name, Message: fmt.Sprintf("%s must be <= %v", name, max)})
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			v.add(Problem{Level: Error, Node: n.ID, Field: name, Message: name + " must be true or false"})
		}
	}
}

func (v *validator) edges() {
	seen := map[string]bool{}
	for _, e := range v.d.Edges {
		src, sok := v.nodes[e.Source]
		dst, dok := v.nodes[e.Target]
		if !sok || !dok {
			v.add(Problem{Level: Error, Edge: e.ID, Message: "connection references a missing node"})
			continue
		}
		if seen[e.Source+"->"+e.Target] {
			v.add(Problem{Level: Warning, Edge: e.ID, Message: fmt.Sprintf("duplicate connection %s -> %s", src.Name, dst.Name)})
		}
		seen[e.Source+"->"+e.Target] = true
		rule, ok := v.c.Connection(src.Type, dst.Type)
		if !ok {
			v.add(Problem{Level: Error, Edge: e.ID, Message: fmt.Sprintf("%s cannot connect to %s", src.Name, dst.Name)})
			continue
		}
		if rule.Kind == catalog.ReferencesKind {
			if e.Attr == "" {
				v.add(Problem{Level: Warning, Edge: e.ID, Message: fmt.Sprintf("%s -> %s: choose which attribute of %s receives the reference", src.Name, dst.Name, src.Name)})
			} else if se, ok := v.c.Get(src.Type); ok {
				if _, has := se.Property(e.Attr); !has {
					v.add(Problem{Level: Error, Edge: e.ID, Message: fmt.Sprintf("%s has no attribute %q", src.Name, e.Attr)})
				}
			}
		}
		if e.Kind != rule.Kind {
			v.add(Problem{Level: Error, Edge: e.ID, Message: fmt.Sprintf("connection %s -> %s must be of kind %q", src.Name, dst.Name, rule.Kind)})
		}
		v.requires(e, src, rule.RequiresFrom, src, dst)
		v.requires(e, dst, rule.RequiresTo, src, dst)
	}
}

// requires checks the property values a connection rule demands on one end.
func (v *validator) requires(e document.Edge, on *document.Node, want map[string]any, src, dst *document.Node) {
	for prop, expected := range want {
		got, ok := on.Props[prop]
		if !ok {
			if entry, ok := v.c.Get(on.Type); ok {
				if schema, ok := entry.Property(prop); ok {
					got = schema["default"]
				}
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(expected) {
			v.add(Problem{Level: Error, Edge: e.ID, Node: on.ID, Field: prop, Message: fmt.Sprintf("%s -> %s needs %s.%s = %v", src.Name, dst.Name, on.Name, prop, expected)})
		}
	}
}

// cidrs enforces, for any node with a `cidr` property: the child range lies
// within the nearest ancestor that also has a cidr, and sibling ranges do not
// overlap. Generic on purpose: VPC/subnet, VNet/subnet, VPC/subnetwork.
func (v *validator) cidrs() {
	type ranged struct {
		n   *document.Node
		net *net.IPNet
	}
	byParent := map[string][]ranged{}
	for i := range v.d.Nodes {
		n := &v.d.Nodes[i]
		s, _ := n.Props["cidr"].(string)
		if s == "" {
			continue
		}
		_, ipn, err := net.ParseCIDR(s)
		if err != nil {
			continue // reported by props()
		}
		group := ""
		if lp := v.logicalParent(n); lp != nil {
			group = lp.ID
		}
		byParent[group] = append(byParent[group], ranged{n, ipn})

		if anc := v.nearestCIDRAncestor(n); anc != nil {
			if !anc.Contains(ipn.IP) || !anc.Contains(lastIP(ipn)) {
				v.add(Problem{Level: Error, Node: n.ID, Field: "cidr", Message: fmt.Sprintf("%s is not within the enclosing range %s", s, anc)})
			}
		}
	}
	for _, group := range byParent {
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				a, b := group[i], group[j]
				if a.net.Contains(b.net.IP) || b.net.Contains(a.net.IP) {
					v.add(Problem{Level: Error, Node: b.n.ID, Field: "cidr", Message: fmt.Sprintf("%s overlaps with %s (%s)", b.net, a.n.Name, a.net)})
				}
			}
		}
	}
}

func (v *validator) nearestCIDRAncestor(n *document.Node) *net.IPNet {
	for cur := n; cur.Parent != ""; {
		p, ok := v.nodes[cur.Parent]
		if !ok {
			return nil
		}
		if s, _ := p.Props["cidr"].(string); s != "" {
			if _, ipn, err := net.ParseCIDR(s); err == nil {
				return ipn
			}
		}
		cur = p
	}
	return nil
}

func lastIP(n *net.IPNet) net.IP {
	ip := make(net.IP, len(n.IP))
	for i := range n.IP {
		ip[i] = n.IP[i] | ^n.Mask[i]
	}
	return ip
}

// toFloat accepts the numeric types produced by encoding/json (float64) and
// yaml.v3 (int, int64, float64).
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	}
	return 0, false
}

// collects enforces catalog terraform.collect minimums: an element that
// gathers nodes of a type under an ancestor needs at least Min of them,
// with distinct values of the Distinct property when set.
func (v *validator) collects() {
	for i := range v.d.Nodes {
		n := &v.d.Nodes[i]
		e, ok := v.c.Get(n.Type)
		if !ok || e.Terraform == nil {
			continue
		}
		for _, col := range e.Terraform.Collect {
			if col.Min == 0 {
				continue
			}
			anc := v.ancestorAt(n, col.Under)
			if anc == nil {
				continue
			}
			ce, _ := v.c.Get(col.Type)
			ae, _ := v.c.Get(anc.Type)
			what, where := col.Type, anc.Type
			if ce != nil {
				what = ce.Label + "s"
			}
			if ae != nil {
				where = ae.Label
			}
			count := 0
			distinct := map[string]bool{}
			for j := range v.d.Nodes {
				m := &v.d.Nodes[j]
				if m.Type != col.Type || !v.isUnder(m, anc.ID) {
					continue
				}
				count++
				if col.Distinct != "" {
					distinct[fmt.Sprint(m.Props[col.Distinct])] = true
				}
			}
			switch {
			case count < col.Min:
				v.add(Problem{Level: Error, Node: n.ID, Message: fmt.Sprintf("%s needs at least %d %s in its %s (found %d)", e.Label, col.Min, what, where, count)})
			case col.Distinct != "" && len(distinct) < col.Min:
				v.add(Problem{Level: Error, Node: n.ID, Message: fmt.Sprintf("%s needs %s in at least %d different %s values in its %s (found %d)", e.Label, what, col.Min, col.Distinct, where, len(distinct))})
			}
		}
	}
}

func (v *validator) ancestorAt(n *document.Node, path string) *document.Node {
	depth := strings.Count(path, "parent")
	cur := n
	// Transparent containers (groups, zone boxes) do not count as a level.
	for i := 0; i < depth; i++ {
		for {
			if cur.Parent == "" {
				return nil
			}
			p, ok := v.nodes[cur.Parent]
			if !ok {
				return nil
			}
			cur = p
			if pe, ok := v.c.Get(p.Type); !ok || !pe.Transparent {
				break
			}
		}
	}
	if cur == n {
		return nil
	}
	return cur
}

func (v *validator) isUnder(n *document.Node, ancestorID string) bool {
	seen := map[string]bool{}
	for cur := n; cur.Parent != "" && !seen[cur.ID]; {
		seen[cur.ID] = true
		if cur.Parent == ancestorID {
			return true
		}
		p, ok := v.nodes[cur.Parent]
		if !ok {
			return false
		}
		cur = p
	}
	return false
}
