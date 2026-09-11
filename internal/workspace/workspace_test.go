package workspace_test

import (
	"path/filepath"
	"testing"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/workspace"
)

func TestWriteOutputsRoundTripsThroughTheFile(t *testing.T) {
	d := document.New("t")
	d.Nodes = []document.Node{{ID: "web", Type: "aws.ec2_instance", Name: "web", Props: map[string]any{}}, {ID: "db", Type: "aws.rds_instance", Name: "db", Props: map[string]any{}}}
	n := workspace.WriteOutputs(d, map[string]map[string]any{"web": {"instance_id": "i-123", "private_ip": "10.0.1.5"}})
	if n != 1 || d.Nodes[1].Outputs != nil || d.Nodes[0].Outputs["instance_id"] != "i-123" {
		t.Fatalf("n=%d nodes=%+v", n, d.Nodes)
	}
	p := filepath.Join(t.TempDir(), "iagram.json")
	if err := d.Save(p); err != nil {
		t.Fatal(err)
	}
	back, err := document.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if back.Nodes[1].Outputs["private_ip"] != "10.0.1.5" { // sorted by id: db, web
		t.Errorf("outputs lost: %+v", back.Nodes)
	}
}

func TestGenerateRefusesInvalidDocument(t *testing.T) {
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	d := document.New("t")
	d.Nodes = []document.Node{{ID: "v", Type: "aws.vpc", Name: "v", Props: map[string]any{"cidr": "10.0.0.0/16"}}}
	w := workspace.For(filepath.Join(dir, "iagram.json"))
	if _, err := w.Generate(c, d); err == nil {
		t.Error("expected validation error (VPC on the canvas)")
	}
}
