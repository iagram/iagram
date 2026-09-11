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
	providers := map[string]int{}
	for _, e := range c.Entries {
		providers[e.Provider]++
		if _, ok := c.Providers[e.Provider]; !ok {
			t.Errorf("%s: provider %q has no _provider.yaml", e.ID, e.Provider)
		}
		if e.Icon == "" {
			t.Errorf("%s: no icon", e.ID)
		}
		if _, err := iagram.CatalogFS.ReadFile("catalog/icons/" + e.Icon); err != nil {
			t.Errorf("%s: icon %s missing", e.ID, e.Icon)
		}
	}
	for _, want := range []string{"aws", "gcp", "azure"} {
		if providers[want] < 7 {
			t.Errorf("provider %s has %d entries", want, providers[want])
		}
	}
	if c.Providers["gcp"].LocalName() != "google" || c.Providers["azure"].LocalName() != "azurerm" || c.Providers["aws"].LocalName() != "aws" {
		t.Errorf("local names: %+v", c.Providers)
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

func TestReservedPropertyNamesRejected(t *testing.T) {
	c := load(t)
	// Every shipped module path must exist for every module-role entry.
	for _, e := range c.Entries {
		if e.Terraform == nil || e.Terraform.Role != "module" {
			continue
		}
		if _, err := iagram.ModulesFS.ReadFile("modules/" + e.Terraform.Module + "/main.tf"); err != nil {
			t.Errorf("%s: module %s missing", e.ID, e.Terraform.Module)
		}
	}
}
