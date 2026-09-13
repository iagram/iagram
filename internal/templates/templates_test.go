package templates_test

import (
	"testing"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/generate"
	"github.com/iagram/iagram/internal/templates"
	"github.com/iagram/iagram/internal/tfschema"
	"github.com/iagram/iagram/internal/validate"
)

// Every shipped reference architecture must render, validate without errors
// and generate Terraform; the gallery promises diagrams you can plan.
func TestShippedTemplatesAreValid(t *testing.T) {
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	c.SetRegistry(tfschema.Catalog{Registry: tfschema.NewRegistry(iagram.SchemasFS, "")})
	list, err := templates.Load(iagram.TemplatesFS, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 500 {
		t.Errorf("expected at least 500 templates, got %d", len(list))
	}
	seen := map[string]bool{}
	for _, tpl := range list {
		if seen[tpl.ID] {
			t.Errorf("duplicate template id %s", tpl.ID)
		}
		seen[tpl.ID] = true
		if tpl.Title == "" || tpl.Category == "" || tpl.Source == "" || tpl.Description == "" {
			t.Errorf("%s: incomplete metadata", tpl.ID)
		}
		r := validate.Run(c, tpl.Document)
		for _, p := range r.Problems {
			if p.Level == validate.Error {
				t.Errorf("%s: %s", tpl.ID, p.Message)
			}
		}
		if _, err := generate.Run(c, tpl.Document); err != nil {
			t.Errorf("%s: generate: %v", tpl.ID, err)
		}
		if tpl.Elements < 3 {
			t.Errorf("%s: only %d elements", tpl.ID, tpl.Elements)
		}
	}
}
