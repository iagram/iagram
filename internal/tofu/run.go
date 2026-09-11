package tofu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
)

// Runner executes tofu in a working directory. It inherits the process
// environment on purpose: that is how cloud credentials reach the providers
// (AWS_PROFILE, SSO cache, gcloud ADC, az login), exactly as with terraform.
type Runner struct {
	Bin string
	Dir string
}

// PlanFile is the binary plan written by Plan and read by ShowPlan.
const PlanFile = "plan.tfplan"

func (r *Runner) cmd(ctx context.Context, out io.Writer, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, r.Bin, args...)
	c.Dir = r.Dir
	c.Stdout = out
	c.Stderr = out
	env := append([]string{}, os.Environ()...)
	env = append(env, "TF_IN_AUTOMATION=1", "TF_INPUT=0", "CHECKPOINT_DISABLE=1")
	if home, err := Home(); err == nil {
		cache := filepath.Join(home, "plugin-cache")
		if os.MkdirAll(cache, 0o755) == nil {
			env = append(env, "TF_PLUGIN_CACHE_DIR="+cache)
		}
	}
	c.Env = env
	return c
}

// Init runs tofu init.
func (r *Runner) Init(ctx context.Context, out io.Writer) error {
	if err := r.cmd(ctx, out, "init", "-no-color", "-input=false").Run(); err != nil {
		return fmt.Errorf("tofu init: %w", err)
	}
	return nil
}

// Validate runs tofu validate (after init).
func (r *Runner) Validate(ctx context.Context, out io.Writer) error {
	if err := r.cmd(ctx, out, "validate", "-no-color").Run(); err != nil {
		return fmt.Errorf("tofu validate: %w", err)
	}
	return nil
}

// Plan writes PlanFile and reports whether it contains changes.
func (r *Runner) Plan(ctx context.Context, out io.Writer) (changes bool, err error) {
	c := r.cmd(ctx, out, "plan", "-no-color", "-input=false", "-detailed-exitcode", "-out="+PlanFile)
	err = c.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 2 {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("tofu plan: %w", err)
	}
	return false, nil
}

// ShowPlan decodes PlanFile as JSON.
func (r *Runner) ShowPlan(ctx context.Context) (*tfjson.Plan, error) {
	var buf bytes.Buffer
	if err := r.cmd(ctx, &buf, "show", "-json", "-no-color", PlanFile).Run(); err != nil {
		return nil, fmt.Errorf("tofu show: %w: %s", err, strings.TrimSpace(buf.String()))
	}
	var p tfjson.Plan
	if err := json.Unmarshal(buf.Bytes(), &p); err != nil {
		return nil, fmt.Errorf("decode plan: %w", err)
	}
	return &p, nil
}

// Apply applies PlanFile.
func (r *Runner) Apply(ctx context.Context, out io.Writer) error {
	if err := r.cmd(ctx, out, "apply", "-no-color", "-input=false", PlanFile).Run(); err != nil {
		return fmt.Errorf("tofu apply: %w", err)
	}
	return nil
}

// Outputs returns root outputs as JSON.
func (r *Runner) Outputs(ctx context.Context) (map[string]tfjson.StateOutput, error) {
	var buf bytes.Buffer
	if err := r.cmd(ctx, &buf, "output", "-json", "-no-color").Run(); err != nil {
		return nil, fmt.Errorf("tofu output: %w", err)
	}
	var out map[string]tfjson.StateOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// VersionString returns the first line of `tofu version`.
func (r *Runner) VersionString(ctx context.Context) string {
	var buf bytes.Buffer
	_ = r.cmd(ctx, &buf, "version").Run()
	line, _, _ := strings.Cut(buf.String(), "\n")
	return strings.TrimSpace(line)
}

// RefreshOnlyPlan writes PlanFile from `plan -refresh-only`: the changes it
// contains are differences between real infrastructure and state, i.e. drift.
func (r *Runner) RefreshOnlyPlan(ctx context.Context, out io.Writer) (drift bool, err error) {
	c := r.cmd(ctx, out, "plan", "-no-color", "-input=false", "-refresh-only", "-detailed-exitcode", "-out="+PlanFile)
	err = c.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 2 {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("tofu plan -refresh-only: %w", err)
	}
	return false, nil
}
