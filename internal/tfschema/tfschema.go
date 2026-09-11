// Package tfschema turns `tofu providers schema -json` (the machine-readable
// Terraform reference for every resource type) into a compact registry that
// iagram ships inside the binary and uses to generate an element for each
// resource a provider has, with a settings panel derived from the schema.
package tfschema

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Attr is one resource attribute (compact form).
type Attr struct {
	Type        json.RawMessage `json:"t"`           // cty type: "string", ["list","string"], ...
	Required    bool            `json:"r,omitempty"` // must be set
	Optional    bool            `json:"o,omitempty"`
	Computed    bool            `json:"c,omitempty"` // set by the provider (an output when not also optional)
	Description string          `json:"d,omitempty"`
}

// Block is a resource body: attributes plus nested blocks.
type Block struct {
	Attrs  map[string]Attr         `json:"a"`
	Blocks map[string]*NestedBlock `json:"b,omitempty"`
}

// NestedBlock is a block type inside a resource (e.g. root_block_device).
type NestedBlock struct {
	Nesting  string `json:"m"` // single, list, set, map
	MaxItems int    `json:"x,omitempty"`
	MinItems int    `json:"n,omitempty"`
	Block
}

// Provider is the compact schema of one provider.
type Provider struct {
	Name      string            `json:"provider"` // local name: aws, google, azurerm
	Version   string            `json:"version,omitempty"`
	Resources map[string]*Block `json:"resources"`
}

// Registry holds compact schemas for several providers, loaded lazily.
type Registry struct {
	mu       sync.Mutex
	embedded fs.FS  // schemas/<local>.json.gz
	userDir  string // ~/.iagram/schemas overrides
	loaded   map[string]*Provider
}

// NewRegistry returns a registry reading embedded snapshots, overridden by
// files in userDir when present.
func NewRegistry(embedded fs.FS, userDir string) *Registry {
	return &Registry{embedded: embedded, userDir: userDir, loaded: map[string]*Provider{}}
}

// Provider loads the schema for a Terraform local provider name (aws, google, azurerm).
func (r *Registry) Provider(local string) (*Provider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.loaded[local]; ok {
		return p, nil
	}
	name := local + ".json.gz"
	var raw []byte
	var err error
	if r.userDir != "" {
		raw, err = os.ReadFile(filepath.Join(r.userDir, name))
	}
	if raw == nil {
		raw, err = fs.ReadFile(r.embedded, "schemas/"+name)
		if err != nil {
			return nil, fmt.Errorf("no schema snapshot for provider %q (run `iagram schemas update`)", local)
		}
	}
	p, err := decode(raw)
	if err != nil {
		return nil, fmt.Errorf("schema %s: %w", name, err)
	}
	r.loaded[local] = p
	return p, nil
}

// Has reports whether a snapshot exists for the provider (without decoding).
func (r *Registry) Has(local string) bool {
	if r.userDir != "" {
		if _, err := os.Stat(filepath.Join(r.userDir, local+".json.gz")); err == nil {
			return true
		}
	}
	_, err := fs.Stat(r.embedded, "schemas/"+local+".json.gz")
	return err == nil
}

// Resource returns the block of a resource type, resolving the provider from
// the type prefix (aws_instance -> aws, google_x -> google, azurerm_x -> azurerm).
func (r *Registry) Resource(tfType string) (*Block, *Provider, bool) {
	local, _, ok := strings.Cut(tfType, "_")
	if !ok {
		return nil, nil, false
	}
	p, err := r.Provider(local)
	if err != nil {
		return nil, nil, false
	}
	b, ok := p.Resources[tfType]
	return b, p, ok
}

func decode(raw []byte) (*Provider, error) {
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var p Provider
	if err := json.NewDecoder(zr).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Encode writes a provider snapshot as gzipped compact JSON.
func Encode(p *Provider, w io.Writer) error {
	zw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(zw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(p); err != nil {
		return err
	}
	return zw.Close()
}

// Extract runs `tofu providers schema -json` in a scratch directory that
// requires the given providers and returns compact schemas keyed by local
// name. Network access is needed the first time (provider download).
func Extract(ctx context.Context, tofuBin string, providers map[string]string, log io.Writer) (map[string]*Provider, error) {
	dir, err := os.MkdirTemp("", "iagram-schema-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	req := map[string]any{}
	for local, source := range providers {
		src, ver, _ := strings.Cut(source, "@")
		entry := map[string]any{"source": src}
		if ver != "" {
			entry["version"] = ver
		}
		req[local] = entry
	}
	cfg, _ := json.Marshal(map[string]any{"terraform": map[string]any{"required_providers": req}})
	if err := os.WriteFile(filepath.Join(dir, "main.tf.json"), cfg, 0o644); err != nil {
		return nil, err
	}
	run := func(args ...string) ([]byte, error) {
		c := exec.CommandContext(ctx, tofuBin, args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "TF_IN_AUTOMATION=1", "TF_INPUT=0", "CHECKPOINT_DISABLE=1")
		var out bytes.Buffer
		c.Stdout = &out
		c.Stderr = log
		if err := c.Run(); err != nil {
			return nil, fmt.Errorf("tofu %s: %w", args[0], err)
		}
		return out.Bytes(), nil
	}
	fmt.Fprintln(log, "$ tofu init")
	if _, err := run("init", "-backend=false", "-input=false", "-no-color"); err != nil {
		return nil, err
	}
	fmt.Fprintln(log, "$ tofu providers schema -json")
	raw, err := run("providers", "schema", "-json")
	if err != nil {
		return nil, err
	}
	versionsRaw, _ := run("version", "-json")
	var versions struct {
		Providers map[string]string `json:"provider_selections"`
	}
	_ = json.Unmarshal(versionsRaw, &versions)
	return Compact(raw, versions.Providers)
}

// Compact converts full `providers schema -json` output into compact form.
func Compact(raw []byte, versions map[string]string) (map[string]*Provider, error) {
	var full struct {
		ProviderSchemas map[string]struct {
			ResourceSchemas map[string]struct {
				Block fullBlock `json:"block"`
			} `json:"resource_schemas"`
		} `json:"provider_schemas"`
	}
	if err := json.Unmarshal(raw, &full); err != nil {
		return nil, fmt.Errorf("decode provider schema: %w", err)
	}
	out := map[string]*Provider{}
	for addr, ps := range full.ProviderSchemas {
		local := addr[strings.LastIndex(addr, "/")+1:]
		p := &Provider{Name: local, Resources: map[string]*Block{}}
		for v, ver := range versions {
			if strings.HasSuffix(v, "/"+local) {
				p.Version = ver
			}
		}
		for t, rs := range ps.ResourceSchemas {
			p.Resources[t] = compactBlock(rs.Block)
		}
		out[local] = p
	}
	return out, nil
}

type fullBlock struct {
	Attributes map[string]struct {
		Type        json.RawMessage `json:"type"`
		Required    bool            `json:"required"`
		Optional    bool            `json:"optional"`
		Computed    bool            `json:"computed"`
		Description string          `json:"description"`
		Deprecated  bool            `json:"deprecated"`
	} `json:"attributes"`
	BlockTypes map[string]struct {
		NestingMode string    `json:"nesting_mode"`
		MaxItems    int       `json:"max_items"`
		MinItems    int       `json:"min_items"`
		Block       fullBlock `json:"block"`
	} `json:"block_types"`
}

func compactBlock(b fullBlock) *Block {
	out := &Block{Attrs: map[string]Attr{}}
	for n, a := range b.Attributes {
		if a.Deprecated {
			continue
		}
		d := a.Description
		if len(d) > 160 {
			d = d[:157] + "..."
		}
		out.Attrs[n] = Attr{Type: a.Type, Required: a.Required, Optional: a.Optional, Computed: a.Computed, Description: d}
	}
	if len(b.BlockTypes) > 0 {
		out.Blocks = map[string]*NestedBlock{}
		for n, bt := range b.BlockTypes {
			nb := &NestedBlock{Nesting: bt.NestingMode, MaxItems: bt.MaxItems, MinItems: bt.MinItems}
			nb.Block = *compactBlock(bt.Block)
			out.Blocks[n] = nb
		}
	}
	return out
}

// Service guesses the service a resource type belongs to from its name
// (aws_ec2_instance_state -> ec2, google_compute_instance -> compute,
// azurerm_linux_virtual_machine -> linux_virtual_machine's first word).
func Service(tfType string) string {
	_, rest, ok := strings.Cut(tfType, "_")
	if !ok {
		return tfType
	}
	svc, _, _ := strings.Cut(rest, "_")
	return svc
}

// Types returns the sorted resource types of a provider.
func (p *Provider) Types() []string {
	out := make([]string, 0, len(p.Resources))
	for t := range p.Resources {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// JSONSchema derives the settings-panel schema for a resource block:
// configurable attributes (required or optional) become properties;
// computed-only attributes are outputs. Complex types and nested blocks
// become JSON-edited values (`x-json: true`).
func (b *Block) JSONSchema() (props map[string]any, required []string, outputs []string) {
	props = map[string]any{}
	names := make([]string, 0, len(b.Attrs))
	for n := range b.Attrs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		a := b.Attrs[n]
		if n == "id" || (a.Computed && !a.Optional && !a.Required) {
			if n != "id" || true {
				outputs = append(outputs, n)
			}
			continue
		}
		p := ctyToSchema(a.Type)
		if a.Description != "" {
			p["description"] = a.Description
		}
		if a.Computed {
			p["description"] = strings.TrimSpace(fmt.Sprint(p["description"], " (computed when unset)"))
		}
		props[n] = p
		if a.Required {
			required = append(required, n)
		}
	}
	blockNames := make([]string, 0, len(b.Blocks))
	for n := range b.Blocks {
		blockNames = append(blockNames, n)
	}
	sort.Strings(blockNames)
	for _, n := range blockNames {
		nb := b.Blocks[n]
		p := map[string]any{"x-json": true, "group": "Blocks", "title": n, "description": fmt.Sprintf("%s block (%s); edit as JSON", n, nb.Nesting)}
		if nb.Nesting == "single" || nb.MaxItems == 1 {
			p["type"] = "object"
		} else {
			p["type"] = "array"
		}
		props[n] = p
		if nb.MinItems > 0 {
			required = append(required, n)
		}
	}
	sort.Strings(required)
	sort.Strings(outputs)
	return props, required, outputs
}

// ctyToSchema maps a cty type to a JSON-schema fragment.
func ctyToSchema(t json.RawMessage) map[string]any {
	var s string
	if json.Unmarshal(t, &s) == nil {
		switch s {
		case "string":
			return map[string]any{"type": "string"}
		case "number":
			return map[string]any{"type": "number"}
		case "bool":
			return map[string]any{"type": "boolean"}
		}
		return map[string]any{"type": "string", "x-json": true}
	}
	var arr []json.RawMessage
	if json.Unmarshal(t, &arr) == nil && len(arr) == 2 {
		var kind string
		_ = json.Unmarshal(arr[0], &kind)
		var elem string
		if json.Unmarshal(arr[1], &elem) == nil && (elem == "string" || elem == "number" || elem == "bool") {
			switch kind {
			case "list", "set":
				return map[string]any{"type": "array", "items": map[string]any{"type": map[string]string{"string": "string", "number": "number", "bool": "boolean"}[elem]}}
			case "map":
				return map[string]any{"type": "object", "additionalProperties": map[string]any{"type": map[string]string{"string": "string", "number": "number", "bool": "boolean"}[elem]}, "x-json": true}
			}
		}
		if kind == "map" || kind == "object" {
			return map[string]any{"type": "object", "x-json": true}
		}
		return map[string]any{"type": "array", "x-json": true}
	}
	return map[string]any{"type": "string", "x-json": true}
}
