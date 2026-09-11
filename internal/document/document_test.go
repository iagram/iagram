package document

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestRoundTripIsCanonical(t *testing.T) {
	d := New("demo")
	d.Nodes = []Node{
		{ID: "b", Type: "aws.vpc", Name: "vpc", Parent: "a", Props: map[string]any{"z": 1.0, "a": "x"}},
		{ID: "a", Type: "aws.region", Name: "eu", Props: nil},
	}
	d.Edges = []Edge{{ID: "e2", Kind: "k", Source: "a", Target: "b"}, {ID: "e1", Kind: "k", Source: "b", Target: "a"}}

	first, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if d.Nodes[0].ID != "a" || d.Edges[0].ID != "e1" {
		t.Error("nodes/edges not sorted")
	}
	if d.Nodes[0].Props == nil {
		t.Error("nil props not normalised to {}")
	}
	if !bytes.HasSuffix(first, []byte("\n")) {
		t.Error("missing trailing newline")
	}
	back, err := Decode(first)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := back.Encode()
	if !bytes.Equal(first, second) {
		t.Errorf("not stable:\n%s\n---\n%s", first, second)
	}
	// props keys sorted by encoding/json for maps
	if idx := bytes.Index(first, []byte(`"a": "x"`)); idx < 0 || idx > bytes.Index(first, []byte(`"z": 1`)) {
		t.Error("props keys not sorted")
	}
}

func TestSaveLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "iagram.json")
	if err := New("x").Save(p); err != nil {
		t.Fatal(err)
	}
	d, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "x" || d.Version != Version {
		t.Errorf("got %+v", d)
	}
}

func TestDecodeRejectsUnknownAndNewer(t *testing.T) {
	if _, err := Decode([]byte(`{"version":1,"nodes":[],"edges":[],"bogus":1}`)); err == nil {
		t.Error("unknown field accepted")
	}
	if _, err := Decode([]byte(`{"version":99,"nodes":[],"edges":[]}`)); err == nil {
		t.Error("newer version accepted")
	}
	if _, err := Decode([]byte(`{"nodes":[],"edges":[]}`)); err == nil {
		t.Error("missing version accepted")
	}
}
