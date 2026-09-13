package generate_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/generate"
	"github.com/iagram/iagram/internal/tfschema"
)

func fixture(t *testing.T) (*catalog.Catalog, *document.Document) {
	t.Helper()
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	d := document.New("demo")
	d.Nodes = []document.Node{
		{ID: "acct", Type: "aws.account", Name: "prod", Props: map[string]any{"profile": "prod", "account_id": "123456789012"}},
		{ID: "reg", Type: "aws.region", Name: "eu", Parent: "acct", Props: map[string]any{"region": "eu-west-1"}},
		{ID: "vpc", Type: "aws.vpc", Name: "main", Parent: "reg", Props: map[string]any{"cidr": "10.0.0.0/16"}},
		{ID: "sub-a", Type: "aws.subnet", Name: "a", Parent: "vpc", Props: map[string]any{"cidr": "10.0.1.0/24", "az": "a", "public": true}},
		{ID: "sub-b", Type: "aws.subnet", Name: "b", Parent: "vpc", Props: map[string]any{"cidr": "10.0.2.0/24", "az": "b"}},
		{ID: "web", Type: "aws.ec2_instance", Name: "web", Parent: "sub-a", Props: map[string]any{"instance_type": "t3.micro"}},
		{ID: "db", Type: "aws.rds_instance", Name: "db", Parent: "sub-b", Props: map[string]any{"engine": "postgres", "instance_class": "db.t3.micro"}},
		{ID: "lb", Type: "aws.alb", Name: "lb", Parent: "vpc", Props: map[string]any{"scheme": "internet-facing"}},
		{ID: "sg", Type: "aws.security_group", Name: "web-sg", Parent: "vpc", Props: map[string]any{}},
		{ID: "fn", Type: "aws.lambda_function", Name: "fn", Parent: "reg", Props: map[string]any{"runtime": "python3.12", "handler": "app.handler"}},
		{ID: "bkt", Type: "aws.s3_bucket", Name: "assets", Parent: "reg", Props: map[string]any{}},
	}
	d.Edges = []document.Edge{
		{ID: "e1", Kind: "routes_to", Source: "lb", Target: "web"},
		{ID: "e2", Kind: "connects_to", Source: "web", Target: "db"},
		{ID: "e3", Kind: "protects", Source: "sg", Target: "web"},
		{ID: "e4", Kind: "reads_writes", Source: "fn", Target: "bkt"},
	}
	return c, d
}

func TestGenerate(t *testing.T) {
	c, d := fixture(t)
	res, err := generate.Run(c, d)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := res.JSON()
	out := string(raw)

	// providers: default + aliased region, account args merged in
	prov := res.Config["provider"].(map[string][]map[string]any)["aws"]
	if len(prov) != 2 || prov[1]["alias"] != "reg" || prov[1]["region"] != "eu-west-1" || prov[1]["profile"] != "prod" {
		t.Errorf("providers = %v", prov)
	}
	if ids, _ := prov[1]["allowed_account_ids"].([]any); len(ids) != 1 || ids[0] != "123456789012" {
		t.Errorf("allowed_account_ids = %v", prov[1]["allowed_account_ids"])
	}
	// required providers from _provider.yaml, including extras
	if !strings.Contains(out, `"hashicorp/aws"`) || !strings.Contains(out, `"hashicorp/archive"`) {
		t.Error("required_providers missing")
	}

	mods := res.Config["module"].(map[string]map[string]any)
	web := mods["web"]
	if web["source"] != "./modules/aws/ec2_instance" || web["subnet_id"] != "${module.sub_a.subnet_id}" || web["vpc_id"] != "${module.vpc.vpc_id}" {
		t.Errorf("web = %v", web)
	}
	if web["providers"].(map[string]any)["aws"] != "aws.reg" {
		t.Errorf("web providers = %v", web["providers"])
	}
	// edge wiring on the right side
	if sgs, _ := web["security_group_ids"].([]any); len(sgs) != 1 || sgs[0] != "${module.sg.security_group_id}" {
		t.Errorf("web security_group_ids = %v", web["security_group_ids"])
	}
	if tg, _ := mods["lb"]["target_instance_ids"].([]any); len(tg) != 1 || tg[0] != "${module.web.instance_id}" {
		t.Errorf("lb targets = %v", mods["lb"]["target_instance_ids"])
	}
	if in, _ := mods["db"]["ingress_security_group_ids"].([]any); len(in) != 1 || in[0] != "${module.web.security_group_id}" {
		t.Errorf("db ingress = %v", mods["db"]["ingress_security_group_ids"])
	}
	if arns, _ := mods["fn"]["s3_bucket_arns"].([]any); len(arns) != 1 || arns[0] != "${module.bkt.bucket_arn}" {
		t.Errorf("fn s3 = %v", mods["fn"]["s3_bucket_arns"])
	}
	// collect: db subnet group gathers both subnets of the VPC
	if subs, _ := mods["db"]["subnet_ids"].([]any); len(subs) != 2 {
		t.Errorf("db subnet_ids = %v", mods["db"]["subnet_ids"])
	}
	// lambda in a region: empty subnet list, no vpc
	if l, _ := mods["fn"]["subnet_ids"].([]any); len(l) != 0 {
		t.Errorf("lambda in region should get subnet_ids = [], got %v", mods["fn"]["subnet_ids"])
	}
	if _, has := mods["fn"]["vpc_id"]; has {
		t.Error("lambda in region should not get vpc_id")
	}
	// tags carry the node id so plans map back
	if mods["bkt"]["tags"].(map[string]any)["iagram_node"] != "bkt" {
		t.Error("tags missing iagram_node")
	}
	if res.ModuleToNode["sub_a"] != "sub-a" {
		t.Errorf("module map = %v", res.ModuleToNode)
	}
	// outputs per module
	if _, ok := res.Config["output"].(map[string]any)["web"]; !ok {
		t.Error("outputs missing")
	}
	// deterministic
	raw2, _ := generate.Run(c, d)
	b2, _ := raw2.JSON()
	if string(b2) != out {
		t.Error("generation is not deterministic")
	}
	var check map[string]any
	if err := json.Unmarshal(raw, &check); err != nil {
		t.Fatal(err)
	}
}

func TestNoTerraformMappingIsAWarning(t *testing.T) {
	c, d := fixture(t)
	d.Nodes = d.Nodes[:2] // account + region only
	res, err := generate.Run(c, d)
	if err != nil {
		t.Fatal(err)
	}
	if _, has := res.Config["module"]; has {
		t.Error("no modules expected")
	}
	if len(res.Config["provider"].(map[string][]map[string]any)["aws"]) != 2 {
		t.Error("provider blocks expected even without modules")
	}
}

func TestProviderArgsKeepLiteralEmptyBlocks(t *testing.T) {
	c, d := fixture(t)
	// Simulate an azurerm-style template through the aws account entry.
	e, _ := c.Get("aws.account")
	saved := e.Terraform.ProviderArgs
	e.Terraform.ProviderArgs = map[string]any{"features": map[string]any{}, "profile": "${profile}", "gone": map[string]any{"x": "${missing}"}}
	defer func() { e.Terraform.ProviderArgs = saved }()
	res, err := generate.Run(c, d)
	if err != nil {
		t.Fatal(err)
	}
	prov := res.Config["provider"].(map[string][]map[string]any)["aws"][0]
	if _, ok := prov["features"]; !ok {
		t.Error("literal empty block dropped")
	}
	if _, ok := prov["gone"]; ok {
		t.Error("block with only unresolved placeholders kept")
	}
}

func TestBracketedParentRefYieldsOneElementList(t *testing.T) {
	c, d := fixture(t)
	d.Nodes = append(d.Nodes, document.Node{ID: "fn2", Type: "aws.lambda_function", Name: "fn2", Parent: "sub-a", Props: map[string]any{"runtime": "python3.12", "handler": "app.handler"}})
	res, err := generate.Run(c, d)
	if err != nil {
		t.Fatal(err)
	}
	fn2 := res.Config["module"].(map[string]map[string]any)["fn2"]
	if l, _ := fn2["subnet_ids"].([]any); len(l) != 1 || l[0] != "${module.sub_a.subnet_id}" {
		t.Errorf("subnet_ids = %v", fn2["subnet_ids"])
	}
	if fn2["vpc_id"] != "${module.vpc.vpc_id}" {
		t.Errorf("vpc_id = %v", fn2["vpc_id"])
	}
}

func TestCommonNodesAreIgnoredSilently(t *testing.T) {
	c, d := fixture(t)
	d.Nodes = append(d.Nodes,
		document.Node{ID: "g1", Type: "common.group", Name: "tier", Props: map[string]any{}},
		document.Node{ID: "n1", Type: "common.note", Name: "n", Parent: "g1", Props: map[string]any{"text": "hello"}},
		document.Node{ID: "u1", Type: "common.users", Name: "customers", Props: map[string]any{}},
	)
	d.Edges = append(d.Edges, document.Edge{ID: "f1", Kind: "flow", Source: "u1", Target: d.Nodes[0].ID})
	res, err := generate.Run(c, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("no warnings expected for common nodes: %v", res.Warnings)
	}
	for _, id := range []string{"g1", "n1", "u1"} {
		for name, node := range res.ModuleToNode {
			if node == id {
				t.Errorf("common node %s rendered as module %s", id, name)
			}
		}
	}
}

func TestGroupsAreTransparentForParentInputs(t *testing.T) {
	c, d := fixture(t)
	d.Nodes = append(d.Nodes,
		document.Node{ID: "tier", Type: "common.group", Name: "app tier", Parent: "vpc", Props: map[string]any{}},
		document.Node{ID: "sub_g", Type: "aws.subnet", Name: "grouped", Parent: "tier", Props: map[string]any{"cidr": "10.0.9.0/24", "az": "c"}},
	)
	res, err := generate.Run(c, d)
	if err != nil {
		t.Fatal(err)
	}
	mods := res.Config["module"].(map[string]map[string]any)
	if got := mods["sub_g"]["vpc_id"]; got != "${module.vpc.vpc_id}" {
		t.Errorf("subnet inside a group inside the VPC: vpc_id = %v", got)
	}
	if p, _ := mods["sub_g"]["providers"].(map[string]any); p["aws"] != "aws.reg" {
		t.Errorf("provider alias should come through the group: %v", mods["sub_g"]["providers"])
	}
}

func TestZoneBoxWinsOverDrawnValue(t *testing.T) {
	c, d := fixture(t)
	d.Nodes = append(d.Nodes,
		document.Node{ID: "az_b", Type: "aws.availability_zone", Name: "b", Parent: "vpc", Props: map[string]any{"zone": "b"}},
		document.Node{ID: "sub_z", Type: "aws.subnet", Name: "zoned", Parent: "az_b", Props: map[string]any{"cidr": "10.0.9.0/24", "az": "a"}},
	)
	res, err := generate.Run(c, d)
	if err != nil {
		t.Fatal(err)
	}
	mods := res.Config["module"].(map[string]map[string]any)
	if got := mods["sub_z"]["az"]; got != "b" {
		t.Errorf("subnet drawn in AZ b should get az=b, got %v", got)
	}
	if got := mods["sub_z"]["vpc_id"]; got != "${module.vpc.vpc_id}" {
		t.Errorf("vpc_id through the zone box = %v", got)
	}
}

func TestLinkEdgesRenderAsResources(t *testing.T) {
	c, d := fixture(t)
	c.SetRegistry(tfschema.Catalog{Registry: tfschema.NewRegistry(iagram.SchemasFS, "")})
	d.Nodes = append(d.Nodes, document.Node{ID: "vpc2", Type: "aws.vpc", Name: "second", Parent: "reg", Props: map[string]any{"cidr": "10.1.0.0/16"}})
	d.Edges = append(d.Edges, document.Edge{ID: "peer1", Kind: "link", Type: "aws.res.aws_vpc_peering_connection", Name: "main_second", Source: "vpc", Target: "vpc2", Props: map[string]any{"auto_accept": true}})
	res, err := generate.Run(c, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("warnings: %v", res.Warnings)
	}
	block := res.Config["resource"].(map[string]map[string]map[string]any)["aws_vpc_peering_connection"]["main_second"]
	if block["vpc_id"] != "${module.vpc.vpc_id}" || block["peer_vpc_id"] != "${module.vpc2.vpc_id}" || block["auto_accept"] != true {
		t.Errorf("peering block = %v", block)
	}
	if res.ModuleToNode["aws_vpc_peering_connection.main_second"] != "peer1" {
		t.Errorf("plan mapping should point at the edge: %v", res.ModuleToNode)
	}
}
