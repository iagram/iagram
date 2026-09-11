package catalog_test

import (
	"testing"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
)

// Every shipped catalog entry must validate against catalog/schema.json. This
// is what makes the schema a real spec rather than documentation: a third
// party catalog that passes the schema loads in iagram.
func TestShippedEntriesMatchSchema(t *testing.T) {
	schema, err := iagram.CatalogFS.ReadFile("catalog/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	problems, err := catalog.CheckSchema(schema, iagram.CatalogFS, "catalog")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}
