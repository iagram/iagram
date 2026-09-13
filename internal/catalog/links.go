package catalog

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LinkKind is the edge kind of a link resource: a Terraform resource whose
// meaning is "connect A to B" (peering, gateway attachment, VPN connection),
// drawn as a line between the two elements and configured on the line.
const LinkKind = "link"

// LinkEnd is one end of a link resource: the attribute that receives the
// reference, the referenced output (default id, or the curated equivalent)
// and, optionally, the Terraform types accepted at that end. Attr may list
// alternatives separated by "|"; the one matching the target's type is used.
type LinkEnd struct {
	Attr   string   `yaml:"attr" json:"attr"`
	Output string   `yaml:"output,omitempty" json:"output,omitempty"`
	Types  []string `yaml:"types,omitempty" json:"types,omitempty"`
	// Elements lists the catalog element ids accepted at this end (resolved
	// at load: generated elements of the matching types plus curated ones).
	Elements []string `yaml:"-" json:"elements"`
}

// UnmarshalYAML accepts a bare attribute name or the object form.
func (le *LinkEnd) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		le.Attr = n.Value
		return nil
	}
	type plain LinkEnd
	var p plain
	if err := n.Decode(&p); err != nil {
		return err
	}
	*le = LinkEnd(p)
	return nil
}

// Link is one entry of catalog/<provider>/_links.yaml.
type Link struct {
	ID       string     `yaml:"-" json:"id"` // element id, <provider>.res.<type>
	Resource string     `yaml:"-" json:"resource"`
	Provider string     `yaml:"-" json:"provider"`
	Label    string     `yaml:"label" json:"label"`
	From     LinkEnd    `yaml:"from" json:"from"`
	To       LinkEnd    `yaml:"to" json:"to"`
	Style    *ConnStyle `yaml:"style,omitempty" json:"style,omitempty"`
}

// LinkBinding is a link resolved for a concrete pair of elements: which
// attribute and output each end of the drawn edge uses.
type LinkBinding struct {
	Link               Link
	SrcAttr, SrcOutput string
	DstAttr, DstOutput string
}

func (c *Catalog) loadLinks(provider string, raw []byte) error {
	var doc struct {
		Links map[string]Link `yaml:"links"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	names := make([]string, 0, len(doc.Links))
	for t := range doc.Links {
		names = append(names, t)
	}
	sort.Strings(names)
	for _, t := range names {
		l := doc.Links[t]
		if l.From.Attr == "" || l.To.Attr == "" {
			return fmt.Errorf("link %s: from and to are required", t)
		}
		l.ID, l.Resource, l.Provider = provider+".res."+t, t, provider
		if l.Label == "" {
			l.Label = humanize(t)
		}
		c.Links = append(c.Links, l)
	}
	return nil
}

// isLink reports whether a Terraform type is a link resource of the provider.
func (c *Catalog) isLink(provider, tfType string) bool {
	for _, l := range c.Links {
		if l.Provider == provider && l.Resource == tfType {
			return true
		}
	}
	return false
}

// tfTypeOf returns the Terraform resource type an element stands for.
func (c *Catalog) tfTypeOf(e *Entry) string {
	if e == nil || e.Terraform == nil {
		return ""
	}
	if e.Terraform.Resource != "" {
		return e.Terraform.Resource
	}
	if e.Terraform.Import != nil {
		return e.Terraform.Import.Resource
	}
	return ""
}

// endAttr returns the attribute of a link end that accepts tfType, or "".
func endAttr(end LinkEnd, tfType string) string {
	if tfType == "" {
		return ""
	}
	alts := strings.Split(end.Attr, "|")
	if len(end.Types) > 0 {
		for _, t := range end.Types {
			if t == tfType {
				return alts[0]
			}
		}
		return ""
	}
	_, rest, _ := strings.Cut(tfType, "_")
	for _, a := range alts {
		token := a
		for _, sfx := range []string{"_id", "_arn", "_name", "_self_link"} {
			token = strings.TrimSuffix(token, sfx)
		}
		for _, pfx := range []string{"peer_", "associated_", "remote_", "linked_"} {
			token = strings.TrimPrefix(token, pfx)
		}
		if token == "" {
			continue
		}
		if rest == token || strings.HasSuffix(rest, "_"+token) || strings.HasSuffix(rest, token) {
			return a
		}
	}
	return ""
}

func endOutput(end LinkEnd, attr string, target *Entry) string {
	if end.Output != "" && target != nil && target.Terraform != nil && target.Terraform.Role == RoleResource {
		return end.Output
	}
	if target != nil && target.Terraform != nil && target.Terraform.Role != RoleResource {
		// curated module: pick the output matching the attribute's shape
		probe := attr
		if end.Output != "" {
			probe = "x_" + end.Output
		}
		return outputFor(probe, target)
	}
	if end.Output != "" {
		return end.Output
	}
	return "id"
}

// LinkCandidates lists the link resources that can join fromType to toType
// (in either orientation), most specific first.
func (c *Catalog) LinkCandidates(fromType, toType string) []LinkBinding {
	fe, fok := c.Get(fromType)
	te, tok := c.Get(toType)
	if !fok || !tok || fe.Provider != te.Provider {
		return nil
	}
	ftf, ttf := c.tfTypeOf(fe), c.tfTypeOf(te)
	var out []LinkBinding
	for _, l := range c.Links {
		if l.Provider != fe.Provider {
			continue
		}
		if fa, ta := endAttr(l.From, ftf), endAttr(l.To, ttf); fa != "" && ta != "" {
			out = append(out, LinkBinding{Link: l, SrcAttr: fa, SrcOutput: endOutput(l.From, fa, fe), DstAttr: ta, DstOutput: endOutput(l.To, ta, te)})
			continue
		}
		if fa, ta := endAttr(l.From, ttf), endAttr(l.To, ftf); fa != "" && ta != "" {
			// drawn the other way round: the edge source is the link's "to" end
			out = append(out, LinkBinding{Link: l, SrcAttr: ta, SrcOutput: endOutput(l.To, ta, fe), DstAttr: fa, DstOutput: endOutput(l.From, fa, te)})
		}
	}
	return out
}

// LinkFor resolves the link an edge uses: the one named by linkID when it is
// a candidate, else the first candidate.
func (c *Catalog) LinkFor(fromType, toType, linkID string) (LinkBinding, bool) {
	cands := c.LinkCandidates(fromType, toType)
	if len(cands) == 0 {
		return LinkBinding{}, false
	}
	for _, b := range cands {
		if b.Link.ID == linkID {
			return b, true
		}
	}
	return cands[0], true
}

// resolveLinkElements fills LinkEnd.Elements from the registry (generated
// elements whose type matches, plus curated elements importing that type).
func (c *Catalog) resolveLinkElements() {
	if c.registry == nil {
		return
	}
	for i := range c.Links {
		l := &c.Links[i]
		pc, ok := c.Providers[l.Provider]
		if !ok {
			continue
		}
		for _, end := range []*LinkEnd{&l.From, &l.To} {
			var ids []string
			for _, t := range c.registry.Types(pc.LocalName()) {
				if endAttr(*end, t) == "" || t == l.Resource {
					continue
				}
				ids = append(ids, c.ownerElementIDs(l.Provider, t)...)
			}
			sort.Strings(ids)
			end.Elements = ids
		}
	}
}
