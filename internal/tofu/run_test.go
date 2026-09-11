package tofu_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/iagram/iagram/internal/tofu"
)

// End-to-end through a real tofu binary using only the built-in
// terraform_data resource (no provider download, no cloud). Downloads
// OpenTofu on first run, so it is opt-in: IAGRAM_E2E=1.
func TestRunnerPlanRoundTrip(t *testing.T) {
	if os.Getenv("IAGRAM_E2E") == "" {
		t.Skip("set IAGRAM_E2E=1 to run against a real OpenTofu binary")
	}
	ctx := context.Background()
	var log bytes.Buffer
	bin, err := tofu.Ensure(ctx, &log)
	if err != nil {
		t.Fatalf("ensure: %v\n%s", err, log.String())
	}
	dir := t.TempDir()
	cfg := `{
  "module": {
    "web_1": {"source": "./modules/thing", "name": "web"},
    "db_1":  {"source": "./modules/thing", "name": "db"}
  }
}`
	mod := `variable "name" { type = string }
resource "terraform_data" "this" { input = var.name }
resource "terraform_data" "other" { input = "${var.name}-2" }
output "id" { value = terraform_data.this.id }`
	root := `{
  "module": {
    "web_1": {"source": "./modules/thing", "name": "web"},
    "db_1":  {"source": "./modules/thing", "name": "db"}
  },
  "output": {"web": {"value": {"id": "${module.web_1.id}"}}}
}`
	_ = cfg
	cfg = root
	if err := os.MkdirAll(filepath.Join(dir, "modules", "thing"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.tf.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "modules", "thing", "main.tf"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &tofu.Runner{Bin: bin, Dir: dir}
	if err := r.Init(ctx, &log); err != nil {
		t.Fatalf("%v\n%s", err, log.String())
	}
	changes, err := r.Plan(ctx, &log)
	if err != nil {
		t.Fatalf("%v\n%s", err, log.String())
	}
	if !changes {
		t.Error("expected changes")
	}
	plan, err := r.ShowPlan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sum := tofu.Summarize(plan, map[string]string{"web_1": "web-node", "db_1": "db-node"})
	if sum.Add != 4 || sum.Change != 0 || sum.Destroy != 0 {
		t.Errorf("summary = %+v", sum)
	}
	if np := sum.Nodes["web-node"]; np.Action != tofu.ActionCreate || len(np.Resources) != 2 {
		t.Errorf("web-node = %+v", np)
	}
	if len(sum.Orphans) != 0 {
		t.Errorf("orphans = %+v", sum.Orphans)
	}

	// Apply, then a second plan must be a no-op and outputs must exist.
	if err := r.Apply(ctx, &log); err != nil {
		t.Fatalf("%v\n%s", err, log.String())
	}
	changes, err = r.Plan(ctx, &log)
	if err != nil {
		t.Fatal(err)
	}
	if changes {
		t.Error("expected no changes after apply")
	}
	// Outputs are readable after apply, and a refresh-only plan is clean.
	outs, err := r.Outputs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := outs["web"]; !ok {
		t.Errorf("outputs = %v", outs)
	}
	drift, err := r.RefreshOnlyPlan(ctx, &log)
	if err != nil {
		t.Fatal(err)
	}
	if drift {
		t.Error("unexpected drift on terraform_data")
	}
	if err := r.Init(ctx, &log); err != nil {
		t.Fatal(err)
	}

	// Remove db_1 from config: its resources become orphans that the summary reports.
	if err := os.WriteFile(filepath.Join(dir, "main.tf.json"), []byte(`{"module":{"web_1":{"source":"./modules/thing","name":"web"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := r.Init(ctx, &log); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Plan(ctx, &log); err != nil {
		t.Fatal(err)
	}
	plan, err = r.ShowPlan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sum = tofu.Summarize(plan, map[string]string{"web_1": "web-node"})
	if sum.Destroy != 2 || len(sum.Orphans) != 2 {
		t.Errorf("after removal: %+v", sum)
	}
}
