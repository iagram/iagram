package document

import (
	"encoding/json"
	"fmt"
)

// A migration upgrades a raw document from version v to v+1 in place. Raw
// form (map) is used so a migration can rename or reshape fields freely
// before the strict typed decode runs.
type migration func(raw map[string]any) error

// migrations is keyed by the version a migration upgrades FROM. Every past
// format version must have an entry; Decode refuses documents older than the
// oldest registered migration. Version 1 is the first released format.
var migrations = map[int]migration{}

// Upgrade brings raw to the current Version, applying migrations in order.
// It returns the version the document had before upgrading.
func Upgrade(raw map[string]any) (from int, err error) {
	v, ok := versionOf(raw)
	if !ok {
		return 0, fmt.Errorf("document has no integer version")
	}
	from = v
	if v > Version {
		return from, fmt.Errorf("document version %d is newer than this iagram (%d); upgrade iagram", v, Version)
	}
	for v < Version {
		m, ok := migrations[v]
		if !ok {
			return from, fmt.Errorf("no migration from document version %d", v)
		}
		if err := m(raw); err != nil {
			return from, fmt.Errorf("migrate v%d -> v%d: %w", v, v+1, err)
		}
		v++
		raw["version"] = v
	}
	return from, nil
}

func versionOf(raw map[string]any) (int, bool) {
	switch v := raw["version"].(type) {
	case float64:
		if v == float64(int(v)) && v >= 0 {
			return int(v), true
		}
	case json.Number:
		if i, err := v.Int64(); err == nil && i >= 0 {
			return int(i), true
		}
	}
	return 0, false
}
