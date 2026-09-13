package catalog

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// CheckSchema validates every entry YAML under root in fsys against the
// catalog JSON schema. It returns one error per offending file. Used by the
// test-suite for the shipped catalog and by `iagram catalog check` for
// external ones.
func CheckSchema(schemaJSON []byte, fsys fs.FS, root string) ([]error, error) {
	var schemaDoc any
	if err := json.Unmarshal(schemaJSON, &schemaDoc); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", schemaDoc); err != nil {
		return nil, err
	}
	schema, err := c.Compile("schema.json")
	if err != nil {
		return nil, err
	}
	var problems []error
	err = fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Entries live at <root>/<provider>/<name>.yaml; top-level files such as
		// equivalences.yaml and provider files follow other schemas.
		if d.IsDir() || !strings.HasSuffix(p, ".yaml") || strings.HasPrefix(path.Base(p), "_") || strings.Contains(p, "/icons/") || strings.HasPrefix(p, "icons/") || path.Dir(p) == root {
			return nil
		}
		src, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		var doc any
		if err := yaml.Unmarshal(src, &doc); err != nil {
			problems = append(problems, fmt.Errorf("%s: %v", p, err))
			return nil
		}
		if err := schema.Validate(jsonify(doc)); err != nil {
			problems = append(problems, fmt.Errorf("%s: %v", p, err))
		}
		return nil
	})
	return problems, err
}

// jsonify converts yaml.v3 values (ints, nested maps) into the JSON-shaped
// values a JSON-schema validator expects.
func jsonify(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = jsonify(val)
		}
		return out
	case []any:
		for i := range t {
			t[i] = jsonify(t[i])
		}
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	}
	return v
}
