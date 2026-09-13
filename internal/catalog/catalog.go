// Package catalog defines the resource catalog: the single source of truth for
// what can be drawn, where it can be placed, what it can connect to, which
// properties it takes, and (later) how it is rendered to Terraform.
package catalog

import (
	"fmt"
	"sort"
	"strings"
	"sync"
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
	ID              string            `yaml:"id" json:"id"`
	Label           string            `yaml:"label" json:"label"`
	Description     string            `yaml:"description,omitempty" json:"description,omitempty"`
	Provider        string            `yaml:"-" json:"provider"`
	Category        string            `yaml:"category" json:"category"`
	Icon            string            `yaml:"icon,omitempty" json:"icon,omitempty"`
	IconVariants    map[string]string `yaml:"icon_variants,omitempty" json:"icon_variants,omitempty"`
	Kind            string            `yaml:"kind" json:"kind"`
	AllowedParents  []string          `yaml:"allowed_parents" json:"allowed_parents"`
	AllowedChildren []string          `yaml:"allowed_children,omitempty" json:"allowed_children,omitempty"`
	Connections     Connections       `yaml:"connections,omitempty" json:"-"`
	Props           map[string]any    `yaml:"props,omitempty" json:"props,omitempty"`
	Terraform       *Terraform        `yaml:"terraform,omitempty" json:"terraform,omitempty"`
	Outputs         []string          `yaml:"outputs,omitempty" json:"outputs,omitempty"`
	Size            *Size             `yaml:"size,omitempty" json:"size,omitempty"`
	// CaptionProp names the property shown as the element's second line when
	// the node has no caption of its own (cidr, region, instance_type).
	CaptionProp string `yaml:"caption_prop,omitempty" json:"caption_prop,omitempty"`
	// Style is the drawing convention of a container (border colour, dash,
	// label position), following the provider's official diagram grammar.
	Style *Style `yaml:"style,omitempty" json:"style,omitempty"`
	// StyleVariants override Style when the named boolean property is true
	// (public subnets are green, private ones teal).
	StyleVariants map[string]Style `yaml:"style_variants,omitempty" json:"style_variants,omitempty"`
	// Provides declares properties given to every element drawn inside this
	// container: child property -> template ("${zone}", "${region}${zone}").
	// See internal/provide.
	Provides map[string]string `yaml:"provides,omitempty" json:"provides,omitempty"`
	// Transparent containers have no Terraform meaning (generic groups, data
	// centers): their children behave as if placed in the container's parent.
	Transparent bool `yaml:"transparent,omitempty" json:"transparent,omitempty"`
	// Attachment marks a generated element with no graphical counterpart: it is
	// not drawn on the canvas but configured inside the element it references.
	Attachment bool `yaml:"-" json:"attachment,omitempty"`
	// Component marks an owned element that is nevertheless drawn, inside its
	// owner's box (node groups in a cluster, services in an ECS cluster).
	Component bool `yaml:"-" json:"component,omitempty"`
	// Link marks a link resource: drawn as a line between two elements.
	Link bool `yaml:"-" json:"link,omitempty"`
	// Span marks a spanning group: a band across containers (Auto Scaling group).
	Span *Span `yaml:"-" json:"span,omitempty"`
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
	From      string         `yaml:"from,omitempty"`
	To        string         `yaml:"to,omitempty"`
	Kind      string         `yaml:"kind"`
	Label     string         `yaml:"label,omitempty"`
	Style     *ConnStyle     `yaml:"style,omitempty"`
	Terraform *ConnTerraform `yaml:"terraform,omitempty"`
	// RequiresFrom / RequiresTo are property values the source / target must
	// have for the connection to work (a VM needs a public IP before a DNS
	// record can point at it). Checked by the validator.
	RequiresFrom map[string]any `yaml:"requires_from,omitempty"`
	RequiresTo   map[string]any `yaml:"requires_to,omitempty"`
}

// ConnTerraform says what an edge means in Terraform: append Value (an output
// reference on one side, "from.<output>" or "to.<output>") to the list Input
// of the module on the Set side ("from" or "to").
type ConnTerraform struct {
	Set   string `yaml:"set" json:"set"`
	Input string `yaml:"input" json:"input"`
	Value string `yaml:"value" json:"value"`
}

// Rule is a compiled, directional connection rule.
// ConnStyle is the default drawing of a connection kind: Direction one|both|
// none, Dash solid|dashed|dotted, Color a CSS colour.
type ConnStyle struct {
	Direction string `yaml:"direction,omitempty" json:"direction,omitempty"`
	Dash      string `yaml:"dash,omitempty" json:"dash,omitempty"`
	Color     string `yaml:"color,omitempty" json:"color,omitempty"`
}

type Rule struct {
	From  string     `json:"from"`
	To    string     `json:"to"`
	Kind  string     `json:"kind"`
	Label string     `json:"label,omitempty"`
	Style *ConnStyle `json:"style,omitempty"`
	// Type is the link element (<provider>.res.<type>) a "link" edge creates.
	Type         string         `json:"type,omitempty"`
	Terraform    *ConnTerraform `json:"terraform,omitempty"`
	RequiresFrom map[string]any `json:"requires_from,omitempty"`
	RequiresTo   map[string]any `json:"requires_to,omitempty"`
}

// Terraform describes how an entry renders.
//
// Role "module" (default) emits one module block per node. Roles "account"
// and "region" configure the provider instead of creating resources.
//
// InputsFromParent maps a module input to an ancestor output: "parent.vpc_id"
// or "parent.parent.vpc_id". When the ancestor is not a module node (an
// account or region) the input is omitted, so the same entry can live under
// several parent types.
//
// Collect gathers outputs from every node of a type placed under an ancestor,
// for inputs that need a set (a DB subnet group, an ALB's subnets).
type Terraform struct {
	Role   string `yaml:"role,omitempty" json:"role,omitempty"`
	Module string `yaml:"module,omitempty" json:"module,omitempty"`
	// Resource is the Terraform resource type of a generated element (role resource).
	Resource string `yaml:"resource,omitempty" json:"resource,omitempty"`
	// ProviderArgs (roles account/region) is a template for the provider
	// block: any string "${prop}" is replaced by the node's property value;
	// keys (or list items) whose property is empty are dropped.
	ProviderArgs     map[string]any     `yaml:"provider_args,omitempty" json:"provider_args,omitempty"`
	InputsFromParent map[string]string  `yaml:"inputs_from_parent,omitempty" json:"inputs_from_parent,omitempty"`
	Collect          map[string]Collect `yaml:"collect,omitempty" json:"collect,omitempty"`
	// Import says how to recognise this element in an existing Terraform
	// state (`iagram import`).
	Import *Import `yaml:"import,omitempty" json:"import,omitempty"`
}

// Import maps a state resource back to an element.
//
// For role module: Resource is the primary resource type (aws_instance),
// Props maps element properties to attribute paths ("tags.Name",
// "vpc_config.0.subnet_ids.0", with optional "|last" or
// "|bool:<if-true>:<if-false>" transforms), Parent is the attribute holding
// the parent's primary id, ID the attribute others reference (default id),
// Name the attribute path for the node name (default: the state name).
//
// For roles account/region: From says where to read the value that groups
// resources: an attribute name ("project", "location"), "arn.account",
// "arn.region" (parsed from an `arn` attribute) or "arm.subscription"
// (parsed from an Azure resource id). Prop names the element property that
// receives it.
type Import struct {
	Resource string            `yaml:"resource,omitempty" json:"resource,omitempty"`
	Props    map[string]string `yaml:"props,omitempty" json:"props,omitempty"`
	Parent   string            `yaml:"parent,omitempty" json:"parent,omitempty"`
	ID       string            `yaml:"id,omitempty" json:"id,omitempty"`
	Name     string            `yaml:"name,omitempty" json:"name,omitempty"`
	From     string            `yaml:"from,omitempty" json:"from,omitempty"`
	Prop     string            `yaml:"prop,omitempty" json:"prop,omitempty"`
}

// Collect describes one gathered list input.
type Collect struct {
	Type   string `yaml:"type" json:"type"`     // catalog id of nodes to gather
	Under  string `yaml:"under" json:"under"`   // "parent", "parent.parent", ...
	Output string `yaml:"output" json:"output"` // output of each gathered module
	// Min is the minimum number of gathered nodes for the element to be valid
	// (e.g. an RDS subnet group needs two subnets). Distinct names a property
	// of the gathered nodes that must differ across at least Min of them
	// (e.g. two subnets in different availability zones).
	Min      int    `yaml:"min,omitempty" json:"min,omitempty"`
	Distinct string `yaml:"distinct,omitempty" json:"distinct,omitempty"`
}

// Provider is the per-cloud Terraform provider configuration, loaded from
// catalog/<provider>/_provider.yaml.
// Style is how a container box is drawn: colours in CSS notation, Dash one of
// solid|dashed|dotted, Label one of left|center.
type Style struct {
	Border string `yaml:"border,omitempty" json:"border,omitempty"`
	Fill   string `yaml:"fill,omitempty" json:"fill,omitempty"`
	Dash   string `yaml:"dash,omitempty" json:"dash,omitempty"`
	Label  string `yaml:"label,omitempty" json:"label,omitempty"`
}

type Provider struct {
	Name string `yaml:"name" json:"name"`
	// Local is the Terraform local provider name when it differs from the
	// catalog provider (gcp -> google, azure -> azurerm). Defaults to Name.
	Local           string            `yaml:"local_name,omitempty" json:"local_name,omitempty"`
	Source          string            `yaml:"source" json:"source"`
	Version         string            `yaml:"version" json:"version"`
	RequiredVersion string            `yaml:"required_version,omitempty" json:"required_version,omitempty"`
	Extra           map[string]Source `yaml:"extra_providers,omitempty" json:"extra_providers,omitempty"`
	// NoTerraform marks a provider-neutral vocabulary (groups, actors, notes)
	// that the generator ignores.
	NoTerraform bool `yaml:"no_terraform,omitempty" json:"no_terraform,omitempty"`
}

// AnyParent in allowed_parents means any container, or the canvas.
const AnyParent = "*"

// FlowKind is the informational edge between an actor or note and anything
// else: it draws an arrow and sets no Terraform input.
const FlowKind = "flow"

// HasTerraform reports whether elements of the provider render to Terraform.
func (c *Catalog) HasTerraform(provider string) bool {
	p, ok := c.Providers[provider]
	return ok && !p.NoTerraform
}

// LocalName returns the Terraform provider name used in configuration.
func (p Provider) LocalName() string {
	if p.Local != "" {
		return p.Local
	}
	return p.Name
}

// Source is a required_providers entry.
type Source struct {
	Source  string `yaml:"source" json:"source"`
	Version string `yaml:"version" json:"version"`
}

// Catalog is the loaded, validated set of entries and compiled rules.
type Catalog struct {
	// Links are the link resources of every provider (catalog/<p>/_links.yaml).
	Links []Link `json:"links"`
	// Spans are the spanning groups of every provider (catalog/<p>/_spans.yaml).
	Spans     []Span              `json:"spans"`
	Entries   []Entry             `json:"entries"`
	Rules     []Rule              `json:"connections"`
	Providers map[string]Provider `json:"providers"`

	byID map[string]*Entry

	// Generated elements: one per Terraform resource type known to the
	// registry, synthesised on first use and cached. See generated.go.
	registry  ResourceRegistry
	genMu     sync.Mutex
	generated map[string]*Entry
	// serviceIcons: provider -> tf type prefix (without provider prefix) -> icon path.
	serviceIcons    map[string]map[string]string
	serviceDefaults map[string]map[string]string // provider -> prefix -> default tf type of the family
	serviceLabels   map[string]map[string]string // provider -> icon file -> palette label
	resourceIcons   map[string]map[string]string // provider -> prefix -> resource (line-art) icon, drawing only
	attachCache     map[string][]AttachmentOption
	attachKinds     map[string]map[string]bool // provider -> tf type -> is attachment
}

// Role values for Terraform.Role.
const (
	RoleModule   = "module"
	RoleAccount  = "account"
	RoleRegion   = "region"
	RoleResource = "resource" // a generated element: one plain resource block
)

// ReferencesKind is the connection kind between a generated element and
// anything it references through an attribute.
const ReferencesKind = "references"

// Get returns the entry with the given id, synthesising generated elements
// ("<provider>.res.<tf_type>") from the resource registry on demand.
func (c *Catalog) Get(id string) (*Entry, bool) {
	if e, ok := c.byID[id]; ok {
		return e, true
	}
	return c.generatedEntry(id)
}

// CanContain reports whether a node of childType may be placed inside a node
// of parentType (Root for the canvas itself).
func (c *Catalog) CanContain(parentType, childType string) bool {
	child, ok := c.Get(childType)
	if !ok {
		return false
	}
	// Attachments live inside the element they configure, container or not.
	if child.Attachment && parentType != Root {
		parent, ok := c.Get(parentType)
		return ok && parent.Provider == child.Provider
	}
	// Components live only inside their owner (curated or generated).
	if child.Component {
		return contains(child.AllowedParents, parentType)
	}
	if parentType == Root {
		return contains(child.AllowedParents, Root) || contains(child.AllowedParents, AnyParent)
	}
	parent, ok := c.Get(parentType)
	if !ok || parent.Kind != KindContainer {
		return false
	}
	// A transparent container accepts anything its own parent could; the
	// validator checks the child against that logical parent.
	if parent.Transparent {
		return true
	}
	if !contains(child.AllowedParents, parentType) && !contains(child.AllowedParents, AnyParent) {
		return false
	}
	if len(parent.AllowedChildren) > 0 && !contains(parent.AllowedChildren, childType) {
		return false
	}
	return true
}

// Connection returns the rule allowing an edge from fromType to toType. A
// generated element may reference any element of the same provider.
func (c *Catalog) Connection(fromType, toType string) (Rule, bool) {
	for _, r := range c.Rules {
		if r.From == fromType && r.To == toType {
			return r, true
		}
	}
	from, fok := c.Get(fromType)
	to, tok := c.Get(toType)
	if fok && tok && (!c.HasTerraform(from.Provider) || !c.HasTerraform(to.Provider)) {
		return Rule{From: fromType, To: toType, Kind: FlowKind}, true
	}
	if cands := c.LinkCandidates(fromType, toType); len(cands) > 0 {
		l := cands[0].Link
		return Rule{From: fromType, To: toType, Kind: LinkKind, Type: l.ID, Label: l.Label, Style: l.Style}, true
	}
	if from, ok := c.Get(fromType); ok && from.Terraform != nil && from.Terraform.Role == RoleResource {
		if to, ok := c.Get(toType); ok && to.Provider == from.Provider {
			return Rule{From: fromType, To: toType, Kind: ReferencesKind, Label: "references", Style: &ConnStyle{Dash: "dotted"}}, true
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
		// Properties become module inputs, so they cannot shadow module
		// meta-arguments or the inputs the generator sets itself.
		if props, ok := e.Props["properties"].(map[string]any); ok {
			for name := range props {
				if reservedInputs[name] {
					return fmt.Errorf("%s: property %q is reserved (module meta-argument or generator input); rename it", e.ID, name)
				}
			}
		}
		c.byID[e.ID] = e
	}

	seen := map[string]bool{}
	for _, e := range c.Entries {
		for _, p := range e.AllowedParents {
			if p != Root && p != AnyParent {
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
		if e.Terraform != nil {
			if e.Terraform.Role == "" {
				e.Terraform.Role = "module"
			}
			if e.Terraform.Role == RoleModule && e.Terraform.Module == "" {
				return fmt.Errorf("%s: terraform.module is required for role module", e.ID)
			}
			for input, col := range e.Terraform.Collect {
				if _, ok := c.byID[col.Type]; !ok {
					return fmt.Errorf("%s: collect %s: unknown type %q", e.ID, input, col.Type)
				}
			}
		}
		for _, r := range e.Connections.Out {
			if err := c.addRule(seen, Rule{From: e.ID, To: r.To, Kind: r.Kind, Label: r.Label, Style: r.Style, Terraform: r.Terraform, RequiresFrom: r.RequiresFrom, RequiresTo: r.RequiresTo}); err != nil {
				return fmt.Errorf("%s: %w", e.ID, err)
			}
		}
		for _, r := range e.Connections.In {
			if err := c.addRule(seen, Rule{From: r.From, To: e.ID, Kind: r.Kind, Label: r.Label, Style: r.Style, Terraform: r.Terraform, RequiresFrom: r.RequiresFrom, RequiresTo: r.RequiresTo}); err != nil {
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
	if t := r.Terraform; t != nil {
		if t.Set != "from" && t.Set != "to" {
			return fmt.Errorf("connection %s: terraform.set must be from or to", key)
		}
		if t.Input == "" || (!strings.HasPrefix(t.Value, "from.") && !strings.HasPrefix(t.Value, "to.")) {
			return fmt.Errorf("connection %s: terraform needs input and value (from.<output> or to.<output>)", key)
		}
	}
	seen[key] = true
	c.Rules = append(c.Rules, r)
	return nil
}

// reservedInputs are names a property may not use: Terraform module
// meta-arguments plus the inputs every generated module block already sets.
var reservedInputs = map[string]bool{
	"source": true, "version": true, "providers": true, "count": true, "for_each": true, "depends_on": true,
	"name": true, "tags": true,
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
