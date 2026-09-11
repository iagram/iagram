// Package catalog defines the resource catalog: the single source of truth for
// what can be drawn, where it can be placed, what it can connect to, which
// properties it takes, and (later) how it is rendered to Terraform.
package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// Root is the pseudo parent type for nodes placed directly on the canvas.
const Root = "root"

// Kind of a catalog entry.
const (
	KindLeaf      = "leaf"
	KindContainer = "container"
)

// Entry is one drawable element.
type Entry struct {
	ID              string         `yaml:"id" json:"id"`
	Label           string         `yaml:"label" json:"label"`
	Description     string         `yaml:"description,omitempty" json:"description,omitempty"`
	Provider        string         `yaml:"-" json:"provider"`
	Category        string         `yaml:"category" json:"category"`
	Icon            string         `yaml:"icon,omitempty" json:"icon,omitempty"`
	Kind            string         `yaml:"kind" json:"kind"`
	AllowedParents  []string       `yaml:"allowed_parents" json:"allowed_parents"`
	AllowedChildren []string       `yaml:"allowed_children,omitempty" json:"allowed_children,omitempty"`
	Connections     Connections    `yaml:"connections,omitempty" json:"-"`
	Props           map[string]any `yaml:"props,omitempty" json:"props,omitempty"`
	Terraform       *Terraform     `yaml:"terraform,omitempty" json:"terraform,omitempty"`
	Outputs         []string       `yaml:"outputs,omitempty" json:"outputs,omitempty"`
	Size            *Size          `yaml:"size,omitempty" json:"size,omitempty"`
}

// Size is the default canvas size of a node.
type Size struct {
	W float64 `yaml:"w" json:"w"`
	H float64 `yaml:"h" json:"h"`
}

// Connections declared on an entry. "in" rules are authored from the target's
// point of view, "out" rules from the source's; both compile to Rule.
type Connections struct {
	In  []ConnRule `yaml:"in,omitempty"`
	Out []ConnRule `yaml:"out,omitempty"`
}

// ConnRule is one authored connection rule.
type ConnRule struct {
	From  string `yaml:"from,omitempty"`
	To    string `yaml:"to,omitempty"`
	Kind  string `yaml:"kind"`
	Label string `yaml:"label,omitempty"`
}

// Rule is a compiled, directional connection rule.
type Rule struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
}

// Terraform describes how an entry renders (phase 2; carried but unused now).
type Terraform struct {
	Module           string            `yaml:"module" json:"module"`
	InputsFromParent map[string]string `yaml:"inputs_from_parent,omitempty" json:"inputs_from_parent,omitempty"`
}

// Catalog is the loaded, validated set of entries and compiled rules.
type Catalog struct {
	Entries []Entry `json:"entries"`
	Rules   []Rule  `json:"connections"`

	byID map[string]*Entry
}

// Get returns the entry with the given id.
func (c *Catalog) Get(id string) (*Entry, bool) {
	e, ok := c.byID[id]
	return e, ok
}

// CanContain reports whether a node of childType may be placed inside a node
// of parentType (Root for the canvas itself).
func (c *Catalog) CanContain(parentType, childType string) bool {
	child, ok := c.byID[childType]
	if !ok {
		return false
	}
	if !contains(child.AllowedParents, parentType) {
		return false
	}
	if parentType == Root {
		return true
	}
	parent, ok := c.byID[parentType]
	if !ok || parent.Kind != KindContainer {
		return false
	}
	if len(parent.AllowedChildren) > 0 && !contains(parent.AllowedChildren, childType) {
		return false
	}
	return true
}

// Connection returns the rule allowing an edge from fromType to toType.
func (c *Catalog) Connection(fromType, toType string) (Rule, bool) {
	for _, r := range c.Rules {
		if r.From == fromType && r.To == toType {
			return r, true
		}
	}
	return Rule{}, false
}

// Required returns the required property names of an entry.
func (e *Entry) Required() []string {
	req, _ := e.Props["required"].([]any)
	out := make([]string, 0, len(req))
	for _, r := range req {
		if s, ok := r.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Property returns the JSON-schema fragment of one property.
func (e *Entry) Property(name string) (map[string]any, bool) {
	props, _ := e.Props["properties"].(map[string]any)
	p, ok := props[name].(map[string]any)
	return p, ok
}

// compile indexes entries, derives providers, flattens rules and checks
// cross-references. It is called once by the loader.
func (c *Catalog) compile() error {
	c.byID = make(map[string]*Entry, len(c.Entries))
	for i := range c.Entries {
		e := &c.Entries[i]
		if e.ID == "" {
			return fmt.Errorf("catalog entry without id")
		}
		if _, dup := c.byID[e.ID]; dup {
			return fmt.Errorf("duplicate catalog id %q", e.ID)
		}
		if e.Kind != KindLeaf && e.Kind != KindContainer {
			return fmt.Errorf("%s: kind must be %q or %q", e.ID, KindLeaf, KindContainer)
		}
		if len(e.AllowedParents) == 0 {
			return fmt.Errorf("%s: allowed_parents is empty; use [%s] for top-level elements", e.ID, Root)
		}
		e.Provider, _, _ = strings.Cut(e.ID, ".")
		if e.Props == nil {
			e.Props = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		c.byID[e.ID] = e
	}

	seen := map[string]bool{}
	for _, e := range c.Entries {
		for _, p := range e.AllowedParents {
			if p != Root {
				if pe, ok := c.byID[p]; !ok {
					return fmt.Errorf("%s: unknown parent %q", e.ID, p)
				} else if pe.Kind != KindContainer {
					return fmt.Errorf("%s: parent %q is not a container", e.ID, p)
				}
			}
		}
		for _, ch := range e.AllowedChildren {
			if _, ok := c.byID[ch]; !ok {
				return fmt.Errorf("%s: unknown child %q", e.ID, ch)
			}
		}
		for _, r := range e.Connections.Out {
			if err := c.addRule(seen, Rule{From: e.ID, To: r.To, Kind: r.Kind, Label: r.Label}); err != nil {
				return fmt.Errorf("%s: %w", e.ID, err)
			}
		}
		for _, r := range e.Connections.In {
			if err := c.addRule(seen, Rule{From: r.From, To: e.ID, Kind: r.Kind, Label: r.Label}); err != nil {
				return fmt.Errorf("%s: %w", e.ID, err)
			}
		}
	}
	sort.Slice(c.Rules, func(i, j int) bool {
		if c.Rules[i].From != c.Rules[j].From {
			return c.Rules[i].From < c.Rules[j].From
		}
		return c.Rules[i].To < c.Rules[j].To
	})
	return nil
}

func (c *Catalog) addRule(seen map[string]bool, r Rule) error {
	if r.From == "" || r.To == "" || r.Kind == "" {
		return fmt.Errorf("connection rule needs from, to and kind (got %+v)", r)
	}
	if _, ok := c.byID[r.From]; !ok {
		return fmt.Errorf("connection from unknown type %q", r.From)
	}
	if _, ok := c.byID[r.To]; !ok {
		return fmt.Errorf("connection to unknown type %q", r.To)
	}
	key := r.From + "->" + r.To
	if seen[key] {
		return fmt.Errorf("connection %s declared twice", key)
	}
	seen[key] = true
	c.Rules = append(c.Rules, r)
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
