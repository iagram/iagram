package catalog_test

import (
	"testing"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
)

func load(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return c
}

func TestShippedCatalogLoads(t *testing.T) {
	c := load(t)
	if len(c.Entries) < 10 {
		t.Fatalf("expected >= 10 entries, got %d", len(c.Entries))
	}
	for _, e := range c.Entries {
		if e.Provider != "aws" {
			t.Errorf("%s: provider %q", e.ID, e.Provider)
		}
		if e.Icon == "" {
			t.Errorf("%s: no icon", e.ID)
		}
	}
}

func TestContainment(t *testing.T) {
	c := load(t)
	cases := []struct {
		parent, child string
		want          bool
	}{
		{catalog.Root, "aws.account", true},
		{catalog.Root, "aws.vpc", false},
		{"aws.account", "aws.region", true},
		{"aws.region", "aws.vpc", true},
		{"aws.vpc", "aws.subnet", true},
		{"aws.subnet", "aws.ec2_instance", true},
		{"aws.vpc", "aws.ec2_instance", false},
		{"aws.region", "aws.s3_bucket", true},
		{"aws.subnet", "aws.s3_bucket", false},
		{"aws.region", "aws.lambda_function", true},
		{"aws.subnet", "aws.lambda_function", true},
		{"aws.ec2_instance", "aws.subnet", false}, // leaf cannot contain
		{"aws.vpc", "nope", false},
	}
	for _, tc := range cases {
		if got := c.CanContain(tc.parent, tc.child); got != tc.want {
			t.Errorf("CanContain(%s, %s) = %v, want %v", tc.parent, tc.child, got, tc.want)
		}
	}
}

func TestConnections(t *testing.T) {
	c := load(t)
	if r, ok := c.Connection("aws.alb", "aws.ec2_instance"); !ok || r.Kind != "routes_to" {
		t.Errorf("alb->ec2 = %+v, %v", r, ok)
	}
	if _, ok := c.Connection("aws.ec2_instance", "aws.alb"); ok {
		t.Error("ec2->alb should not be allowed")
	}
	if r, ok := c.Connection("aws.security_group", "aws.rds_instance"); !ok || r.Kind != "protects" {
		t.Errorf("sg->rds = %+v, %v", r, ok)
	}
}

func TestEntryHelpers(t *testing.T) {
	c := load(t)
	e, _ := c.Get("aws.subnet")
	req := e.Required()
	if len(req) != 2 || req[0] != "cidr" || req[1] != "az" {
		t.Errorf("required = %v", req)
	}
	p, ok := e.Property("cidr")
	if !ok || p["format"] != "cidr" {
		t.Errorf("cidr property = %v, %v", p, ok)
	}
}
