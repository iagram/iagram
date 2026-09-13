package catalog

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// Span describes a spanning group (catalog/<provider>/_spans.yaml): an
// element drawn as a band across several containers; the covered containers
// become the value of Attr (an Auto Scaling group's vpc_zone_identifier).
type Span struct {
	ID       string `yaml:"-" json:"id"` // element id
	Resource string `yaml:"-" json:"resource"`
	Provider string `yaml:"-" json:"provider"`
	Label    string `yaml:"label" json:"label"`
	// Attr is the list attribute receiving a reference per covered container.
	Attr string `yaml:"attr" json:"attr"`
	// Over lists the Terraform types the band may cover.
	Over []string `yaml:"over" json:"over"`
	// Elements / Outputs are resolved at load: element ids the band may
	// cover and the output referenced for each.
	Elements []string          `yaml:"-" json:"elements"`
	Outputs  map[string]string `yaml:"-" json:"outputs"`
	Ghost    *Ghost            `yaml:"ghost,omitempty" json:"ghost,omitempty"`
	Icon     string            `yaml:"icon,omitempty" json:"icon,omitempty"`
	Style    *Style            `yaml:"style,omitempty" json:"style,omitempty"`
}

// Ghost draws faded tiles inside the band: Count names the property giving
// how many, Icon the tile icon.
type Ghost struct {
	Count string `yaml:"count" json:"count"`
	Icon  string `yaml:"icon" json:"icon"`
}

func (c *Catalog) loadSpans(provider string, raw []byte) error {
	var doc struct {
		Spans map[string]Span `yaml:"spans"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	names := make([]string, 0, len(doc.Spans))
	for t := range doc.Spans {
		names = append(names, t)
	}
	sort.Strings(names)
	for _, t := range names {
		sp := doc.Spans[t]
		if sp.Attr == "" || len(sp.Over) == 0 {
			return fmt.Errorf("span %s: attr and over are required", t)
		}
		sp.ID, sp.Resource, sp.Provider = provider+".res."+t, t, provider
		if sp.Label == "" {
			sp.Label = humanize(t)
		}
		c.Spans = append(c.Spans, sp)
	}
	return nil
}

func (c *Catalog) spanOf(provider, tfType string) *Span {
	for i := range c.Spans {
		if c.Spans[i].Provider == provider && c.Spans[i].Resource == tfType {
			return &c.Spans[i]
		}
	}
	return nil
}

// resolveSpanElements fills Elements and Outputs from the catalog: generated
// elements of the covered types plus curated elements importing them.
func (c *Catalog) resolveSpanElements() {
	for i := range c.Spans {
		sp := &c.Spans[i]
		sp.Outputs = map[string]string{}
		var ids []string
		for _, t := range sp.Over {
			for _, id := range c.ownerElementIDs(sp.Provider, t) {
				ids = append(ids, id)
				if e, ok := c.byID[id]; ok {
					sp.Outputs[id] = outputFor("x_id", e)
				} else {
					sp.Outputs[id] = "id"
				}
			}
		}
		sort.Strings(ids)
		sp.Elements = ids
	}
}
