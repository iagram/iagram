// Package document is the iagram.json file format: the diagram is the source
// of truth for the infrastructure, layout is metadata that rides along.
package document

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
)

// Version of the file format written by this build.
const Version = 1

// Document is one diagram.
type Document struct {
	Version int    `json:"version"`
	Name    string `json:"name,omitempty"`
	Nodes   []Node `json:"nodes"`
	Edges   []Edge `json:"edges"`
	// Steps is the numbered walkthrough of the diagram: nodes and edges carry
	// a Step number, this list holds the text for each number. Informational.
	Steps []Step `json:"steps,omitempty"`
}

// Step is one entry of the numbered walkthrough ("1", "2", "9a").
type Step struct {
	N    string `json:"n"`
	Text string `json:"text"`
}

// EdgeStyle is how an arrow is drawn. Direction is one|both|none (none draws
// a plain association line), Dash is solid|dashed|dotted, Color a CSS colour,
// Inactive greys the arrow out (standby capacity, disabled path).
type EdgeStyle struct {
	Direction string `json:"direction,omitempty"`
	Dash      string `json:"dash,omitempty"`
	Color     string `json:"color,omitempty"`
	Inactive  bool   `json:"inactive,omitempty"`
}

// Node is a placed catalog element.
type Node struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Parent string         `json:"parent,omitempty"`
	Props  map[string]any `json:"props"`
	Layout Layout         `json:"layout"`
	// Outputs are written back by `iagram apply` from the module outputs the
	// catalog declares (endpoints, ids, IPs). They are informational: the
	// generator never reads them and they never affect validation.
	Outputs map[string]any `json:"outputs,omitempty"`
	// Caption is a free second line under the name ("primary Region",
	// "vehicle lookup"); Step a callout number. Both informational.
	Caption string `json:"caption,omitempty"`
	Step    string `json:"step,omitempty"`
}

// Layout is position (relative to the parent) and, for containers, size.
type Layout struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w,omitempty"`
	H float64 `json:"h,omitempty"`
	// Z orders siblings front to back (higher is in front); 0 when unset.
	Z int `json:"z,omitempty"`
}

// Edge is a typed relationship between two nodes. For "references" edges from
// a generated element, Attr names the source attribute that receives the
// reference and Output the target attribute/output referenced (default id).
type Edge struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Target string `json:"target"`
	Attr   string `json:"attr,omitempty"`
	Output string `json:"output,omitempty"`
	// Label overrides the connection kind's label ("mTLS", "Private VIF");
	// Step is a callout number; Style the drawing. All informational.
	Label string     `json:"label,omitempty"`
	Step  string     `json:"step,omitempty"`
	Style *EdgeStyle `json:"style,omitempty"`
}

// New returns an empty document.
func New(name string) *Document {
	return &Document{Version: Version, Name: name, Nodes: []Node{}, Edges: []Edge{}}
}

// Load reads and decodes a document file.
func Load(path string) (*Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Decode(raw)
}

// Decode parses document JSON, upgrading older format versions first.
// Unknown fields are rejected so typos and drift surface immediately.
func Decode(raw []byte) (*Document, error) {
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("decode document: %w", err)
	}
	if generic == nil {
		return nil, errors.New("decode document: empty")
	}
	if _, err := Upgrade(generic); err != nil {
		return nil, err
	}
	upgraded, err := json.Marshal(generic)
	if err != nil {
		return nil, err
	}
	var d Document
	dec := json.NewDecoder(bytes.NewReader(upgraded))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("decode document: %w", err)
	}
	d.normalize()
	return &d, nil
}

// Save writes the document deterministically (sorted nodes/edges, sorted keys,
// two-space indent, trailing newline) so that diffs are readable.
func (d *Document) Save(path string) error {
	raw, err := d.Encode()
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Encode returns the canonical JSON form.
func (d *Document) Encode() ([]byte, error) {
	d.normalize()
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// Index returns nodes by id.
func (d *Document) Index() map[string]*Node {
	m := make(map[string]*Node, len(d.Nodes))
	for i := range d.Nodes {
		m[d.Nodes[i].ID] = &d.Nodes[i]
	}
	return m
}

func (d *Document) normalize() {
	d.Version = Version
	if d.Nodes == nil {
		d.Nodes = []Node{}
	}
	if d.Edges == nil {
		d.Edges = []Edge{}
	}
	for i := range d.Nodes {
		if d.Nodes[i].Props == nil {
			d.Nodes[i].Props = map[string]any{}
		}
	}
	sort.Slice(d.Nodes, func(i, j int) bool { return d.Nodes[i].ID < d.Nodes[j].ID })
	sort.Slice(d.Edges, func(i, j int) bool { return d.Edges[i].ID < d.Edges[j].ID })
}

// LogicalParent returns the nearest ancestor of n that is not transparent
// (as judged by the callback on the ancestor's type), or nil for the canvas.
// Transparent containers are drawing groups with no Terraform meaning.
func (d *Document) LogicalParent(byID map[string]*Node, n *Node, transparent func(typ string) bool) *Node {
	seen := map[string]bool{n.ID: true}
	for cur := n; cur.Parent != "" && !seen[cur.Parent]; {
		seen[cur.Parent] = true
		p, ok := byID[cur.Parent]
		if !ok {
			return nil
		}
		if !transparent(p.Type) {
			return p
		}
		cur = p
	}
	return nil
}
