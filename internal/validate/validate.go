// Package validate checks a document against the catalog. Most structural
// rules are already unenforceable in the editor; this is the authoritative
// pass that also runs headless (CLI, CI) and covers what the UI cannot.
package validate

import (
	"fmt"
	"net"
	"sort"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
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
			p, ok := v.nodes[n.Parent]
			if !ok {
				v.add(Problem{Level: Error, Node: n.ID, Message: fmt.Sprintf("parent %q does not exist", n.Parent)})
				continue
			}
			parentType = p.Type
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
	}
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
	for i := range v.d.Nodes {
		n := &v.d.Nodes[i]
		e, ok := v.c.Get(n.Type)
		if !ok {
			continue
		}
		for _, req := range e.Required() {
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
		if e.Kind != rule.Kind {
			v.add(Problem{Level: Error, Edge: e.ID, Message: fmt.Sprintf("connection %s -> %s must be of kind %q", src.Name, dst.Name, rule.Kind)})
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
		byParent[n.Parent] = append(byParent[n.Parent], ranged{n, ipn})

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
