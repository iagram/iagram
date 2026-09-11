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
	Root string // .iagram
	Dir  string // .iagram/tf
}

// ConfigFile is the generated root configuration.
const ConfigFile = "main.tf.json"

// For returns the workspace next to a diagram file.
func For(docPath string) *Workspace {
	root := filepath.Join(filepath.Dir(docPath), ".iagram")
	return &Workspace{Root: root, Dir: filepath.Join(root, "tf")}
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

// Apply applies the last plan file. Phase 3 wires this into the UI; the CLI
// exposes it now because it is a one-liner on top of Plan.
func (w *Workspace) Apply(ctx context.Context, log io.Writer) error {
	bin, err := tofu.Ensure(ctx, log)
	if err != nil {
		return err
	}
	r := &tofu.Runner{Bin: bin, Dir: w.Dir}
	if _, err := os.Stat(filepath.Join(w.Dir, tofu.PlanFile)); err != nil {
		return fmt.Errorf("no plan to apply; run `iagram plan` first")
	}
	fmt.Fprintln(log, "$ tofu apply "+tofu.PlanFile)
	return r.Apply(ctx, log)
}
