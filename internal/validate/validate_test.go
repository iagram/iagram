package validate_test

import (
	"strings"
	"testing"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/validate"
)

func fixture(t *testing.T) (*catalog.Catalog, *document.Document) {
	t.Helper()
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	d := document.New("t")
	d.Nodes = []document.Node{
		{ID: "acct", Type: "aws.account", Name: "prod", Props: map[string]any{}},
		{ID: "reg", Type: "aws.region", Name: "eu", Parent: "acct", Props: map[string]any{"region": "eu-west-1"}},
		{ID: "vpc", Type: "aws.vpc", Name: "main", Parent: "reg", Props: map[string]any{"cidr": "10.0.0.0/16"}},
		{ID: "sub1", Type: "aws.subnet", Name: "a", Parent: "vpc", Props: map[string]any{"cidr": "10.0.1.0/24", "az": "a"}},
		{ID: "sub2", Type: "aws.subnet", Name: "b", Parent: "vpc", Props: map[string]any{"cidr": "10.0.2.0/24", "az": "b"}},
		{ID: "web", Type: "aws.ec2_instance", Name: "web", Parent: "sub1", Props: map[string]any{"instance_type": "t3.micro"}},
		{ID: "db", Type: "aws.rds_instance", Name: "db", Parent: "sub2", Props: map[string]any{"engine": "postgres", "instance_class": "db.t3.micro"}},
		{ID: "lb", Type: "aws.alb", Name: "lb", Parent: "vpc", Props: map[string]any{"scheme": "internet-facing"}},
	}
	d.Edges = []document.Edge{
		{ID: "e1", Kind: "routes_to", Source: "lb", Target: "web"},
		{ID: "e2", Kind: "connects_to", Source: "web", Target: "db"},
	}
	return c, d
}

func messages(r validate.Result) string {
	var b strings.Builder
	for _, p := range r.Problems {
		b.WriteString(p.Level + " " + p.Node + p.Edge + " " + p.Message + "\n")
	}
	return b.String()
}

func TestValidDocument(t *testing.T) {
	c, d := fixture(t)
	r := validate.Run(c, d)
	if !r.OK() || len(r.Problems) != 0 {
		t.Fatalf("expected clean, got:\n%s", messages(r))
	}
}

func expectProblem(t *testing.T, r validate.Result, substr string) {
	t.Helper()
	if !strings.Contains(messages(r), substr) {
		t.Errorf("expected problem containing %q, got:\n%s", substr, messages(r))
	}
}

func TestStructuralProblems(t *testing.T) {
	c, d := fixture(t)
	d.Nodes = append(d.Nodes,
		document.Node{ID: "bad1", Type: "aws.ec2_instance", Name: "loose", Parent: "vpc", Props: map[string]any{"instance_type": "t3.micro"}},
		document.Node{ID: "bad2", Type: "aws.vpc", Name: "top", Props: map[string]any{"cidr": "10.1.0.0/16"}},
		document.Node{ID: "bad3", Type: "aws.nope", Name: "x", Parent: "reg"},
		document.Node{ID: "bad4", Type: "aws.subnet", Name: "a", Parent: "vpc", Props: map[string]any{"cidr": "10.0.9.0/24", "az": "a"}},
	)
	r := validate.Run(c, d)
	expectProblem(t, r, "EC2 Instance cannot be placed inside a VPC")
	expectProblem(t, r, "VPC cannot be placed on the canvas")
	expectProblem(t, r, `unknown type "aws.nope"`)
	expectProblem(t, r, `name "a" already used`)
}

func TestPropertyProblems(t *testing.T) {
	c, d := fixture(t)
	d.Nodes[5].Props = map[string]any{"instance_type": "t9.huge", "root_volume_gb": 2.0, "mystery": true}
	d.Nodes[3].Props = map[string]any{"cidr": "not-a-cidr"}
	r := validate.Run(c, d)
	expectProblem(t, r, "is not one of the allowed values")
	expectProblem(t, r, "root_volume_gb must be >= 8")
	expectProblem(t, r, `unknown property "mystery"`)
	expectProblem(t, r, "az is required")
	expectProblem(t, r, "is not a valid CIDR")
}

func TestCIDRProblems(t *testing.T) {
	c, d := fixture(t)
	d.Nodes[4].Props["cidr"] = "10.0.1.128/25" // overlaps sub1
	d.Nodes = append(d.Nodes, document.Node{ID: "sub3", Type: "aws.subnet", Name: "c", Parent: "vpc", Props: map[string]any{"cidr": "192.168.0.0/24", "az": "c"}})
	r := validate.Run(c, d)
	expectProblem(t, r, "overlaps with")
	expectProblem(t, r, "is not within the enclosing range 10.0.0.0/16")
}

func TestEdgeProblems(t *testing.T) {
	c, d := fixture(t)
	d.Edges = append(d.Edges,
		document.Edge{ID: "e3", Kind: "routes_to", Source: "web", Target: "lb"},
		document.Edge{ID: "e4", Kind: "wrong", Source: "lb", Target: "web"},
		document.Edge{ID: "e5", Kind: "routes_to", Source: "lb", Target: "ghost"},
	)
	r := validate.Run(c, d)
	expectProblem(t, r, `connection web -> lb must be of kind "flow"`)
	expectProblem(t, r, `must be of kind "routes_to"`)
	expectProblem(t, r, "duplicate connection lb -> web")
	expectProblem(t, r, "missing node")
}

func TestCollectMinimums(t *testing.T) {
	c, d := fixture(t)
	// Move the second subnet into the same AZ as the first: RDS and ALB need two AZs.
	d.Nodes[4].Props["az"] = "a"
	r := validate.Run(c, d)
	expectProblem(t, r, "RDS Instance needs Subnets in at least 2 different az values")
	expectProblem(t, r, "Application Load Balancer needs Subnets in at least 2 different az values")

	// Remove the second subnet entirely (and the db in it): count rule.
	c, d = fixture(t)
	var keep []document.Node
	for _, n := range d.Nodes {
		if n.ID != "sub2" && n.ID != "db" {
			keep = append(keep, n)
		}
	}
	d.Nodes = keep
	d.Edges = d.Edges[:1]
	r = validate.Run(c, d)
	expectProblem(t, r, "Application Load Balancer needs at least 2 Subnets in its VPC (found 1)")
}

func TestConnectionRequirements(t *testing.T) {
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	d := document.New("t")
	d.Nodes = []document.Node{
		{ID: "sub", Type: "azure.subscription", Name: "s", Props: map[string]any{"subscription_id": "00000000-0000-0000-0000-000000000000"}},
		{ID: "rg", Type: "azure.resource_group", Name: "rg", Parent: "sub", Props: map[string]any{"location": "westeurope"}},
		{ID: "vnet", Type: "azure.vnet", Name: "v", Parent: "rg", Props: map[string]any{"cidr": "10.0.0.0/16"}},
		{ID: "snet", Type: "azure.subnet", Name: "n", Parent: "vnet", Props: map[string]any{"cidr": "10.0.1.0/24"}},
		{ID: "vm", Type: "azure.virtual_machine", Name: "vm", Parent: "snet", Props: map[string]any{"size": "Standard_B1s", "public_ip": false}},
		{ID: "dns", Type: "azure.dns_zone", Name: "dns", Parent: "rg", Props: map[string]any{"domain": "example.com"}},
	}
	d.Edges = []document.Edge{{ID: "e1", Kind: "resolves_to", Source: "dns", Target: "vm"}}
	r := validate.Run(c, d)
	expectProblem(t, r, "dns -> vm needs vm.public_ip = true")
	d.Nodes[4].Props["public_ip"] = true
	if r := validate.Run(c, d); !r.OK() {
		t.Errorf("expected clean after enabling public ip: %s", messages(r))
	}
}

func TestTransparentGroupsUseLogicalParent(t *testing.T) {
	c, d := fixture(t)
	// A group inside the VPC: a subnet inside the group is fine (VPC is the
	// logical parent); an EC2 instance directly in a group on the canvas is not.
	vpc := ""
	for _, n := range d.Nodes {
		if n.Type == "aws.vpc" {
			vpc = n.ID
		}
	}
	d.Nodes = append(d.Nodes,
		document.Node{ID: "g1", Type: "common.group", Name: "tier", Parent: vpc, Props: map[string]any{}},
		document.Node{ID: "s9", Type: "aws.subnet", Name: "grouped", Parent: "g1", Props: map[string]any{"cidr": "10.0.9.0/24", "az": "c"}},
		document.Node{ID: "g2", Type: "common.group", Name: "loose", Props: map[string]any{}},
		document.Node{ID: "i9", Type: "aws.ec2_instance", Name: "orphan", Parent: "g2", Props: map[string]any{}},
		document.Node{ID: "u1", Type: "common.users", Name: "customers", Props: map[string]any{}},
	)
	d.Edges = append(d.Edges, document.Edge{ID: "f1", Kind: "flow", Source: "u1", Target: vpc})
	r := validate.Run(c, d)
	msgs := messages(r)
	if strings.Contains(msgs, "s9") {
		t.Errorf("subnet inside a group inside the VPC should be valid: %s", msgs)
	}
	if !strings.Contains(msgs, "i9 EC2 Instance cannot be placed") {
		t.Errorf("instance inside a group on the canvas should be rejected: %s", msgs)
	}
	if strings.Contains(msgs, "u1") || strings.Contains(msgs, "f1") {
		t.Errorf("actors and flow edges are always valid: %s", msgs)
	}
}
