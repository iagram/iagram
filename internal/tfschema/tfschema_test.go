package tfschema_test

import (
	"encoding/json"
	"testing"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/tfschema"
	"github.com/iagram/iagram/internal/tofu"
)

func TestSnapshotsShipForAllProviders(t *testing.T) {
	reg := tfschema.NewRegistry(iagram.SchemasFS, "")
	want := map[string]int{"aws": 1500, "google": 900, "azurerm": 900}
	for local, min := range want {
		p, err := reg.Provider(local)
		if err != nil {
			t.Fatalf("%s: %v", local, err)
		}
		if len(p.Resources) < min {
			t.Errorf("%s: only %d resources", local, len(p.Resources))
		}
	}
	b, p, ok := reg.Resource("aws_instance")
	if !ok || p.Name != "aws" {
		t.Fatal("aws_instance missing")
	}
	props, required, outputs := b.JSONSchema()
	if props["instance_type"].(map[string]any)["type"] != "string" {
		t.Errorf("instance_type schema = %v", props["instance_type"])
	}
	if props["vpc_security_group_ids"].(map[string]any)["type"] != "array" {
		t.Errorf("vpc_security_group_ids = %v", props["vpc_security_group_ids"])
	}
	if _, isProp := props["arn"]; isProp {
		t.Error("computed-only arn must be an output, not a property")
	}
	found := false
	for _, o := range outputs {
		if o == "arn" {
			found = true
		}
	}
	if !found {
		t.Error("arn not in outputs")
	}
	if rb, ok := props["root_block_device"].(map[string]any); !ok || rb["x-json"] != true {
		t.Errorf("nested block should be x-json: %v", props["root_block_device"])
	}
	_ = required
	if tfschema.Service("aws_ec2_transit_gateway") != "ec2" || tfschema.Service("google_compute_instance") != "compute" {
		t.Error("service heuristic")
	}
	_ = tofu.Version
}

func TestCompactRoundTrip(t *testing.T) {
	raw := []byte(`{"provider_schemas":{"registry.opentofu.org/hashicorp/aws":{"resource_schemas":{"aws_thing":{"block":{"attributes":{"name":{"type":"string","required":true},"id":{"type":"string","computed":true},"tags":{"type":["map","string"],"optional":true}},"block_types":{"opts":{"nesting_mode":"list","max_items":1,"block":{"attributes":{"x":{"type":"number","optional":true}}}}}}}}}}}`)
	out, err := tfschema.Compact(raw, map[string]string{"registry.opentofu.org/hashicorp/aws": "6.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	p := out["aws"]
	if p.Version != "6.0.0" || p.Resources["aws_thing"].Attrs["name"].Required != true || p.Resources["aws_thing"].Blocks["opts"].MaxItems != 1 {
		b, _ := json.Marshal(p)
		t.Errorf("compact = %s", b)
	}
}
