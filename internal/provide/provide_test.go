package provide_test

import (
	"testing"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/provide"
	"github.com/iagram/iagram/internal/tfschema"
)

func TestZoneBoxProvidesValues(t *testing.T) {
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	d := document.New("z")
	d.Nodes = []document.Node{
		{ID: "acct", Type: "aws.account", Props: map[string]any{}},
		{ID: "reg", Type: "aws.region", Parent: "acct", Props: map[string]any{"region": "eu-west-1"}},
		{ID: "vpc", Type: "aws.vpc", Parent: "reg", Props: map[string]any{"cidr": "10.0.0.0/16"}},
		{ID: "grp", Type: "common.group", Parent: "vpc", Props: map[string]any{}},
		{ID: "az", Type: "aws.availability_zone", Parent: "grp", Props: map[string]any{"zone": "b"}},
		{ID: "sub", Type: "aws.subnet", Parent: "az", Props: map[string]any{"cidr": "10.0.1.0/24", "az": "a"}},
		{ID: "vol", Type: "aws.res.aws_ebs_volume", Parent: "az", Props: map[string]any{}},
		{ID: "bucket", Type: "aws.s3_bucket", Parent: "reg", Props: map[string]any{}},
	}
	c.SetRegistry(tfschema.Catalog{Registry: tfschema.NewRegistry(iagram.SchemasFS, "")})
	if _, ok := c.Get("aws.res.aws_ebs_volume"); !ok {
		t.Fatal("generated ebs volume entry missing")
	}
	byID := d.Index()
	got := provide.Values(c, byID, byID["sub"])
	if got["az"] != "b" {
		t.Errorf("subnet az = %v", got)
	}
	got = provide.Values(c, byID, byID["vol"])
	if got["availability_zone"] != "eu-west-1b" {
		t.Errorf("volume availability_zone = %v", got)
	}
	if got := provide.Values(c, byID, byID["bucket"]); len(got) != 0 {
		t.Errorf("bucket should inherit nothing: %v", got)
	}
}
