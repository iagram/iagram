package catalog

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads every catalog/<provider>/*.yaml from fsys and compiles the catalog.
func Load(fsys fs.FS) (*Catalog, error) {
	var files []string
	err := fs.WalkDir(fsys, "catalog", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".yaml") || strings.HasPrefix(p, "catalog/icons/") {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk catalog: %w", err)
	}
	sort.Strings(files)

	c := &Catalog{}
	for _, f := range files {
		raw, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		var e Entry
		if err := yaml.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		want := path.Base(path.Dir(f)) + "." + strings.TrimSuffix(path.Base(f), ".yaml")
		if e.ID != want {
			return nil, fmt.Errorf("%s: id %q must match path (%s)", f, e.ID, want)
		}
		c.Entries = append(c.Entries, e)
	}
	if err := c.compile(); err != nil {
		return nil, err
	}
	return c, nil
}
