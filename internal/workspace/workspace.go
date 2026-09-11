// Package workspace is the per-diagram Terraform working directory:
// <diagram dir>/.iagram/tf holds main.tf.json, the materialised modules, the
// plan file and (by default) the local state. Everything in it is derived or
// gitignored; the diagram is the source of truth.
package workspace

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/generate"
	"github.com/iagram/iagram/internal/tofu"
	"github.com/iagram/iagram/internal/validate"
)

// Workspace paths.
type Workspace struct {
	DocPath string // the diagram file
	Root    string // .iagram
	Dir     string // .iagram/tf
}

// ConfigFile is the generated root configuration.
const ConfigFile = "main.tf.json"

// For returns the workspace next to a diagram file.
func For(docPath string) *Workspace {
	root := filepath.Join(filepath.Dir(docPath), ".iagram")
	return &Workspace{DocPath: docPath, Root: root, Dir: filepath.Join(root, "tf")}
}

// Generate validates, renders main.tf.json and syncs the modules. It refuses
// to generate from a document with validation errors.
func (w *Workspace) Generate(c *catalog.Catalog, d *document.Document) (*generate.Result, error) {
	if v := validate.Run(c, d); !v.OK() {
		n := 0
		for _, p := range v.Problems {
			if p.Level == validate.Error {
				n++
			}
		}
		return nil, fmt.Errorf("diagram has %d validation error(s); fix them before generating", n)
	}
	res, err := generate.Run(c, d)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(w.Dir, 0o755); err != nil {
		return nil, err
	}
	if err := w.ensureGitignore(); err != nil {
		return nil, err
	}
	raw, err := res.JSON()
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(w.Dir, ConfigFile), raw, 0o644); err != nil {
		return nil, err
	}
	if err := w.syncModules(); err != nil {
		return nil, err
	}
	return res, nil
}

// syncModules replaces .iagram/tf/modules with the embedded set.
func (w *Workspace) syncModules() error {
	dst := filepath.Join(w.Dir, "modules")
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return fs.WalkDir(iagram.ModulesFS, "modules", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("modules", filepath.FromSlash(p))
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := iagram.ModulesFS.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
}

func (w *Workspace) ensureGitignore() error {
	p := filepath.Join(w.Root, ".gitignore")
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	return os.WriteFile(p, []byte("*\n!.gitignore\n"), 0o644)
}

// PlanResult is what `iagram plan` and the UI consume.
type PlanResult struct {
	Changes     bool         `json:"changes"`
	Summary     tofu.Summary `json:"summary"`
	ConfigPath  string       `json:"config_path"`
	TofuVersion string       `json:"tofu_version"`
	DurationS   float64      `json:"duration_s"`
	Warnings    []string     `json:"warnings,omitempty"`
}

// Plan generates, installs tofu if needed, runs init and plan, and maps the
// result back onto nodes. All tofu output goes to log.
func (w *Workspace) Plan(ctx context.Context, c *catalog.Catalog, d *document.Document, log io.Writer) (*PlanResult, error) {
	start := time.Now()
	fmt.Fprintln(log, "Generating Terraform...")
	res, err := w.Generate(c, d)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(log, "Wrote %s (%d module(s))\n", filepath.Join(w.Dir, ConfigFile), len(res.ModuleToNode))
	for _, wmsg := range res.Warnings {
		fmt.Fprintln(log, "warning:", wmsg)
	}

	bin, err := tofu.Ensure(ctx, log)
	if err != nil {
		return nil, err
	}
	r := &tofu.Runner{Bin: bin, Dir: w.Dir}
	fmt.Fprintf(log, "Using %s\n\n", r.VersionString(ctx))

	fmt.Fprintln(log, "$ tofu init")
	if err := r.Init(ctx, log); err != nil {
		return nil, err
	}
	fmt.Fprintln(log, "\n$ tofu plan")
	changes, err := r.Plan(ctx, log)
	if err != nil {
		return nil, err
	}
	plan, err := r.ShowPlan(ctx)
	if err != nil {
		return nil, err
	}
	sum := tofu.Summarize(plan, res.ModuleToNode)
	fmt.Fprintf(log, "\nPlan: %d to add, %d to change, %d to destroy.\n", sum.Add, sum.Change, sum.Destroy)
	return &PlanResult{
		Changes:     changes,
		Summary:     sum,
		ConfigPath:  filepath.Join(w.Dir, ConfigFile),
		TofuVersion: r.VersionString(ctx),
		DurationS:   time.Since(start).Seconds(),
		Warnings:    res.Warnings,
	}, nil
}

// ApplyResult is what `iagram apply` and the UI consume.
type ApplyResult struct {
	Outputs      map[string]map[string]any `json:"outputs"` // node id -> output name -> value
	NodesUpdated int                       `json:"nodes_updated"`
	DurationS    float64                   `json:"duration_s"`
	TofuVersion  string                    `json:"tofu_version"`
}

// Apply applies the plan file written by Plan, then reads the root outputs
// and writes them back onto the nodes of the diagram file. The document
// passed must be the one that was planned; the caller enforces that.
func (w *Workspace) Apply(ctx context.Context, c *catalog.Catalog, d *document.Document, log io.Writer) (*ApplyResult, error) {
	start := time.Now()
	if _, err := os.Stat(filepath.Join(w.Dir, tofu.PlanFile)); err != nil {
		return nil, fmt.Errorf("no plan to apply; run plan first")
	}
	bin, err := tofu.Ensure(ctx, log)
	if err != nil {
		return nil, err
	}
	r := &tofu.Runner{Bin: bin, Dir: w.Dir}
	fmt.Fprintln(log, "$ tofu apply "+tofu.PlanFile)
	if err := r.Apply(ctx, log); err != nil {
		return nil, err
	}
	// The plan file is single-use.
	_ = os.Remove(filepath.Join(w.Dir, tofu.PlanFile))

	fmt.Fprintln(log, "\nReading outputs...")
	outs, err := r.Outputs(ctx)
	if err != nil {
		return nil, err
	}
	res := &ApplyResult{Outputs: map[string]map[string]any{}, TofuVersion: r.VersionString(ctx)}
	gen, err := generate.Run(c, d)
	if err != nil {
		return nil, err
	}
	for module, nodeID := range gen.ModuleToNode {
		o, ok := outs[module]
		if !ok {
			continue
		}
		vals, ok := o.Value.(map[string]any)
		if !ok || len(vals) == 0 {
			continue
		}
		res.Outputs[nodeID] = vals
	}
	res.NodesUpdated = WriteOutputs(d, res.Outputs)
	if err := d.Save(w.DocPath); err != nil {
		return nil, fmt.Errorf("write outputs to %s: %w", w.DocPath, err)
	}
	fmt.Fprintf(log, "Wrote outputs for %d node(s) to %s\n", res.NodesUpdated, filepath.Base(w.DocPath))
	res.DurationS = time.Since(start).Seconds()
	return res, nil
}

// WriteOutputs sets node outputs from a node id -> outputs map and clears
// outputs of nodes no longer present in it. Returns the number of nodes set.
func WriteOutputs(d *document.Document, outputs map[string]map[string]any) int {
	n := 0
	for i := range d.Nodes {
		node := &d.Nodes[i]
		if vals, ok := outputs[node.ID]; ok {
			node.Outputs = vals
			n++
		}
	}
	return n
}

// DriftResult is a refresh-only plan mapped onto nodes.
type DriftResult struct {
	Drift       bool         `json:"drift"`
	Summary     tofu.Summary `json:"summary"`
	CheckedAt   time.Time    `json:"checked_at"`
	DurationS   float64      `json:"duration_s"`
	TofuVersion string       `json:"tofu_version"`
}

// Drift runs `tofu plan -refresh-only` against the generated configuration
// and reports which nodes have infrastructure that no longer matches state.
// It regenerates first so the working directory matches the diagram, but it
// does not write a plan the UI could apply (the plan file is removed).
func (w *Workspace) Drift(ctx context.Context, c *catalog.Catalog, d *document.Document, log io.Writer) (*DriftResult, error) {
	start := time.Now()
	res, err := w.Generate(c, d)
	if err != nil {
		return nil, err
	}
	bin, err := tofu.Ensure(ctx, log)
	if err != nil {
		return nil, err
	}
	r := &tofu.Runner{Bin: bin, Dir: w.Dir}
	fmt.Fprintln(log, "$ tofu init")
	if err := r.Init(ctx, log); err != nil {
		return nil, err
	}
	fmt.Fprintln(log, "\n$ tofu plan -refresh-only")
	drift, err := r.RefreshOnlyPlan(ctx, log)
	if err != nil {
		return nil, err
	}
	plan, err := r.ShowPlan(ctx)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(filepath.Join(w.Dir, tofu.PlanFile))
	sum := tofu.Summarize(plan, res.ModuleToNode)
	if drift {
		fmt.Fprintf(log, "\nDrift detected in %d resource(s).\n", sum.Change+sum.Destroy+sum.Add)
	} else {
		fmt.Fprintln(log, "\nNo drift. Infrastructure matches state.")
	}
	return &DriftResult{Drift: drift, Summary: sum, CheckedAt: time.Now().UTC(), DurationS: time.Since(start).Seconds(), TofuVersion: r.VersionString(ctx)}, nil
}
