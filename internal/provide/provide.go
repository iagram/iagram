// Package provide resolves the properties a node inherits from the containers
// it is drawn in: an Availability Zone box provides `az` to the subnets inside
// it, a GCP zone box provides `zone`. Catalog entries declare `provides` as
// child property -> template; "${x}" in a template is the providing node's
// property x, or the nearest ancestor's when the provider lacks it (a Region
// prop for "${region}${zone}").
package provide

import (
	"regexp"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
)

var placeholder = regexp.MustCompile(`\$\{([A-Za-z0-9_]+)\}`)

// Values returns the provided properties of n (nearest provider wins),
// limited to the properties n's catalog entry declares.
func Values(c *catalog.Catalog, byID map[string]*document.Node, n *document.Node) map[string]string {
	e, ok := c.Get(n.Type)
	if !ok {
		return nil
	}
	var out map[string]string
	chain := ancestors(byID, n)
	for i, anc := range chain {
		ae, ok := c.Get(anc.Type)
		if !ok || len(ae.Provides) == 0 {
			continue
		}
		for prop, tpl := range ae.Provides {
			if _, has := e.Property(prop); !has {
				continue
			}
			if _, done := out[prop]; done {
				continue
			}
			v, ok := render(tpl, chain[i:])
			if !ok {
				continue
			}
			if out == nil {
				out = map[string]string{}
			}
			out[prop] = v
		}
	}
	return out
}

// render substitutes each ${x} with the first value of x found walking the
// providing node then its ancestors. False when a placeholder is unresolved.
func render(tpl string, from []*document.Node) (string, bool) {
	ok := true
	s := placeholder.ReplaceAllStringFunc(tpl, func(m string) string {
		key := m[2 : len(m)-1]
		for _, a := range from {
			if v, has := a.Props[key]; has && v != nil && v != "" {
				return toString(v)
			}
		}
		ok = false
		return ""
	})
	return s, ok
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return itoa(int64(t))
		}
	case int:
		return itoa(int64(t))
	case int64:
		return itoa(t)
	}
	return ""
}

func itoa(i int64) string {
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
		if i == 0 {
			break
		}
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}

// ancestors lists parent, grandparent, ... including transparent containers.
func ancestors(byID map[string]*document.Node, n *document.Node) []*document.Node {
	var out []*document.Node
	seen := map[string]bool{n.ID: true}
	for cur := n; cur.Parent != "" && !seen[cur.Parent]; {
		seen[cur.Parent] = true
		p, ok := byID[cur.Parent]
		if !ok {
			break
		}
		out = append(out, p)
		cur = p
	}
	return out
}
