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
		if d.IsDir() || !strings.HasSuffix(p, ".yaml") || strings.HasPrefix(p, "catalog/icons/") || path.Dir(p) == "catalog" {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk catalog: %w", err)
	}
	sort.Strings(files)

	c := &Catalog{Providers: map[string]Provider{}}
	c.loadServiceIcons(fsys)
	for _, f := range files {
		raw, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		if path.Base(f) == "_provider.yaml" {
			var p Provider
			if err := yaml.Unmarshal(raw, &p); err != nil {
				return nil, fmt.Errorf("%s: %w", f, err)
			}
			if p.Name != path.Base(path.Dir(f)) || p.Source == "" {
				return nil, fmt.Errorf("%s: name must match directory and source is required", f)
			}
			c.Providers[p.Name] = p
			continue
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
	for _, e := range c.Entries {
		if _, ok := c.Providers[e.Provider]; !ok {
			return nil, fmt.Errorf("%s: no catalog/%s/_provider.yaml", e.ID, e.Provider)
		}
	}
	return c, nil
}

// LoadLayered loads the built-in catalog and then overlays extra directories
// (same layout). Entries with the same id replace built-in ones, so a team
// can ship its own catalog without forking iagram.
func LoadLayered(builtin fs.FS, extra ...fs.FS) (*Catalog, error) {
	base, err := Load(builtin)
	if err != nil {
		return nil, err
	}
	if len(extra) == 0 {
		return base, nil
	}
	merged := &Catalog{Providers: base.Providers, serviceIcons: base.serviceIcons}
	byID := map[string]int{}
	for _, e := range base.Entries {
		byID[e.ID] = len(merged.Entries)
		merged.Entries = append(merged.Entries, rawEntry(e))
	}
	for _, x := range extra {
		layer, err := loadRaw(x)
		if err != nil {
			return nil, err
		}
		for name, p := range layer.Providers {
			merged.Providers[name] = p
		}
		for _, e := range layer.Entries {
			if i, ok := byID[e.ID]; ok {
				merged.Entries[i] = e
			} else {
				byID[e.ID] = len(merged.Entries)
				merged.Entries = append(merged.Entries, e)
			}
		}
	}
	if err := merged.compile(); err != nil {
		return nil, err
	}
	return merged, nil
}

// rawEntry strips compile-time derived state so an entry can be recompiled.
func rawEntry(e Entry) Entry {
	e.Provider = ""
	return e
}

// loadRaw parses entries and providers without compiling (no cross-checks).
func loadRaw(fsys fs.FS) (*Catalog, error) {
	c := &Catalog{Providers: map[string]Provider{}}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".yaml") || strings.HasPrefix(p, "icons/") {
			return nil
		}
		raw, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		if path.Base(p) == "_provider.yaml" {
			var pr Provider
			if err := yaml.Unmarshal(raw, &pr); err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			c.Providers[pr.Name] = pr
			return nil
		}
		var e Entry
		if err := yaml.Unmarshal(raw, &e); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		want := path.Base(path.Dir(p)) + "." + strings.TrimSuffix(path.Base(p), ".yaml")
		if e.ID != want {
			return fmt.Errorf("%s: id %q must match path (%s)", p, e.ID, want)
		}
		c.Entries = append(c.Entries, e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

// loadServiceIcons reads catalog/icons/<provider>/services.yaml files.
func (c *Catalog) loadServiceIcons(fsys fs.FS) {
	c.serviceIcons = map[string]map[string]string{}
	dirs, err := fs.ReadDir(fsys, "catalog/icons")
	if err != nil {
		return
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		raw, err := fs.ReadFile(fsys, "catalog/icons/"+d.Name()+"/services.yaml")
		if err != nil {
			continue
		}
		var doc struct {
			Prefixes map[string]string `yaml:"prefixes"`
		}
		if yaml.Unmarshal(raw, &doc) == nil {
			c.serviceIcons[d.Name()] = doc.Prefixes
		}
	}
}
