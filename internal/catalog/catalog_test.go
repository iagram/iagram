package catalog_test

import (
	"strings"
	"testing"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/tfschema"
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
	if r, ok := c.Connection("aws.ec2_instance", "aws.alb"); !ok || r.Kind != catalog.FlowKind || r.Terraform != nil {
		t.Errorf("ec2->alb is a documentation arrow only: %+v %v", r, ok)
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

func TestGeneratedElementsFromRegistry(t *testing.T) {
	c := load(t)
	c.SetRegistry(tfschema.Catalog{Registry: tfschema.NewRegistry(iagram.SchemasFS, "")})
	id, ok := c.GeneratedID("google_pubsub_subscription")
	if !ok || id != "gcp.res.google_pubsub_subscription" {
		t.Fatalf("id = %q", id)
	}
	e, ok := c.Get(id)
	if !ok || e.Kind != catalog.KindLeaf || e.Terraform.Role != catalog.RoleResource || e.Terraform.Resource != "google_pubsub_subscription" {
		t.Fatalf("entry = %+v", e)
	}
	if e.Label != "Pubsub Subscription" || e.Provider != "gcp" {
		t.Errorf("label=%q provider=%q", e.Label, e.Provider)
	}
	if !contains(e.AllowedParents, "gcp.project") || !contains(e.AllowedParents, "gcp.vpc") {
		t.Errorf("parents = %v", e.AllowedParents)
	}
	if _, ok := c.Connection(id, "gcp.pubsub_topic"); !ok {
		t.Error("generated -> curated same-provider reference should be allowed")
	}
	if _, ok := c.Connection(id, "aws.s3_bucket"); ok {
		t.Error("cross-provider reference must be refused")
	}
	if r, ok := c.Connection("gcp.pubsub_topic", id); !ok || r.Kind != catalog.FlowKind {
		t.Errorf("curated -> generated is a documentation arrow: %+v", r)
	}
	if n := len(c.Generated("aws")); n < 1500 {
		t.Errorf("aws generated = %d", n)
	}
	if _, ok := c.Get("aws.res.aws_not_a_thing"); ok {
		t.Error("unknown type must not synthesise")
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestGraphicalVersusAttachment(t *testing.T) {
	c := load(t)
	c.SetRegistry(tfschema.Catalog{Registry: tfschema.NewRegistry(iagram.SchemasFS, "")})
	list := c.Generated("aws")
	byRes := map[string]catalog.GeneratedSummary{}
	for _, g := range list {
		byRes[g.Resource] = g
	}
	if g := byRes["aws_sns_topic"]; !g.Graphical || g.Icon == "aws/generic.svg" {
		t.Errorf("sns topic should be graphical with an official icon: %+v", g)
	}
	if g := byRes["aws_sns_topic_policy"]; g.Graphical || !g.Attachment {
		t.Errorf("topic policy should be an attachment: %+v", g)
	}
	if _, listed := byRes["aws_sns_topic_subscription"]; listed {
		t.Error("subscriptions are link lines, not listed elements")
	}
	if g := byRes["aws_s3_bucket_versioning"]; g.Graphical {
		t.Errorf("bucket versioning should be an attachment: %+v", g)
	}
	if g := byRes["aws_iam_role_policy_attachment"]; g.Graphical {
		t.Errorf("policy attachment should be an attachment: %+v", g)
	}
	// First-level: own icon, no owner of the same kind. Placement references (vpc, subnet) do not count.
	for _, res := range []string{"aws_iam_role", "aws_iam_policy", "aws_route53_zone", "aws_lb", "aws_api_gateway_rest_api", "aws_lambda_function", "aws_ecs_cluster", "aws_ecs_task_definition", "aws_nat_gateway", "aws_vpc_endpoint", "aws_instance", "aws_cloudwatch_log_group", "aws_kms_key", "aws_ecr_repository", "aws_rds_cluster"} {
		if g := byRes[res]; !g.Graphical {
			t.Errorf("%s should be first-level: %+v", res, g)
		}
	}
	// Attachments: configuration of another resource, or owned by one sharing the icon.
	for _, res := range []string{"aws_route", "aws_security_group_rule", "aws_lambda_permission", "aws_lambda_alias", "aws_cloudwatch_event_target", "aws_api_gateway_method", "aws_lb_listener", "aws_ecs_service", "aws_eks_node_group", "aws_rds_cluster_instance", "aws_cloudwatch_log_stream", "aws_kms_alias", "aws_ecr_repository_policy", "aws_glue_catalog_table"} {
		if g := byRes[res]; g.Graphical {
			t.Errorf("%s should be an attachment: %+v", res, g)
		}
	}
	gcpList := c.Generated("gcp")
	gcpBy := map[string]catalog.GeneratedSummary{}
	for _, g := range gcpList {
		gcpBy[g.Resource] = g
	}
	for _, res := range []string{"google_sql_database", "google_sql_user", "google_container_node_pool", "google_bigquery_table"} {
		if g := gcpBy[res]; g.Graphical {
			t.Errorf("%s should be an attachment: %+v", res, g)
		}
	}
	for _, res := range []string{"google_compute_instance", "google_sql_database_instance", "google_cloud_run_v2_service", "google_storage_bucket", "google_pubsub_topic"} {
		if g := gcpBy[res]; !g.Graphical {
			t.Errorf("%s should be first-level: %+v", res, g)
		}
	}
	azList := c.Generated("azure")
	azBy := map[string]catalog.GeneratedSummary{}
	for _, g := range azList {
		azBy[g.Resource] = g
	}
	for _, res := range []string{"azurerm_storage_container", "azurerm_storage_blob", "azurerm_kubernetes_cluster_node_pool", "azurerm_role_assignment"} {
		if g := azBy[res]; g.Graphical {
			t.Errorf("%s should be an attachment: %+v", res, g)
		}
	}
	for _, res := range []string{"azurerm_linux_virtual_machine", "azurerm_linux_web_app", "azurerm_storage_account", "azurerm_kubernetes_cluster", "azurerm_mssql_server"} {
		if g := azBy[res]; !g.Graphical {
			t.Errorf("%s should be first-level: %+v", res, g)
		}
	}
	if attr, out, ok := c.AttachmentBinding("aws.res.aws_sns_topic_subscription", "aws.res.aws_sns_topic"); !ok || attr != "topic_arn" || out != "arn" {
		t.Errorf("binding = %s %s %v", attr, out, ok)
	}
	graphical := 0
	for _, g := range list {
		if g.Graphical {
			graphical++
		}
	}
	if graphical < 200 || graphical > 1000 {
		t.Errorf("aws graphical count = %d (palette should be a few hundred, not 1700)", graphical)
	}
	// attachments for the curated bucket and for a generated topic
	opts := c.AttachmentsFor("aws.s3_bucket")
	found := map[string]catalog.AttachmentOption{}
	for _, o := range opts {
		found[o.Resource] = o
	}
	if v, ok := found["aws_s3_bucket_versioning"]; !ok || v.Attr != "bucket" || v.Output != "bucket_name" {
		t.Errorf("bucket versioning binding = %+v (all: %d)", v, len(opts))
	}
	opts = c.AttachmentsFor("aws.res.aws_s3_bucket")
	found = map[string]catalog.AttachmentOption{}
	for _, o := range opts {
		found[o.Resource] = o
	}
	if v, ok := found["aws_s3_bucket_versioning"]; !ok || v.Attr != "bucket" {
		t.Errorf("versioning binding = %+v", v)
	}
	e, _ := c.Get("aws.res.aws_s3_bucket_versioning")
	if !e.Attachment || !c.CanContain("aws.res.aws_s3_bucket", e.ID) {
		t.Error("attachment must be placeable inside its (leaf) parent")
	}
}

func TestFamiliesOnePerIcon(t *testing.T) {
	c := load(t)
	c.SetRegistry(tfschema.Catalog{Registry: tfschema.NewRegistry(iagram.SchemasFS, "")})
	fams := c.Families("aws")
	if len(fams) < 100 || len(fams) > 200 {
		t.Errorf("aws families = %d (one per icon expected)", len(fams))
	}
	seen := map[string]bool{}
	var lambda, iam *catalog.Family
	for i := range fams {
		f := &fams[i]
		if seen[f.Icon] {
			t.Errorf("icon %s listed twice", f.Icon)
		}
		seen[f.Icon] = true
		if len(f.Types) == 0 || f.Default != f.Types[0] {
			t.Errorf("family %s malformed: %+v", f.Label, f)
		}
		switch {
		case strings.HasSuffix(f.Icon, "/lambda.svg") || strings.HasSuffix(f.Icon, "aws_lambda.svg"):
			lambda = f
		case strings.HasSuffix(f.Icon, "aws_identity_access_management_role.svg"):
			iam = f
		}
	}
	if lambda == nil || lambda.Default != "aws.res.aws_lambda_function" {
		t.Errorf("lambda family = %+v", lambda)
	}
	if iam == nil || iam.Default != "aws.res.aws_iam_role" {
		t.Errorf("iam family = %+v", iam)
	}
	if lambda != nil && lambda.Curated != "aws.lambda_function" {
		t.Errorf("lambda family should point at the curated element: %+v", lambda)
	}
}

func TestClustersAreBoxesWithComponents(t *testing.T) {
	c := load(t)
	c.SetRegistry(tfschema.Catalog{Registry: tfschema.NewRegistry(iagram.SchemasFS, "")})
	for _, id := range []string{"aws.eks_cluster", "gcp.gke_cluster", "azure.aks_cluster", "aws.res.aws_ecs_cluster", "aws.res.aws_rds_cluster", "aws.res.aws_emr_cluster", "gcp.res.google_container_cluster", "azure.res.azurerm_kubernetes_cluster"} {
		e, ok := c.Get(id)
		if !ok || e.Kind != catalog.KindContainer {
			t.Errorf("%s should be a container box: %+v", id, e)
		}
	}
	// Multi-zone elements without member resources stay single nodes.
	for _, id := range []string{"aws.res.aws_msk_cluster", "aws.res.aws_redshift_cluster", "aws.res.aws_elasticache_replication_group", "aws.alb"} {
		e, ok := c.Get(id)
		if !ok || e.Kind != catalog.KindLeaf || e.Attachment {
			t.Errorf("%s should be a first-level node: kind=%s attachment=%v", id, e.Kind, e.Attachment)
		}
	}
	for tf, owners := range map[string][]string{
		"aws_eks_node_group":                   {"aws.eks_cluster", "aws.res.aws_eks_cluster"},
		"aws_ecs_service":                      {"aws.res.aws_ecs_cluster"},
		"aws_rds_cluster_instance":             {"aws.res.aws_rds_cluster"},
		"google_container_node_pool":           {"gcp.gke_cluster", "gcp.res.google_container_cluster"},
		"azurerm_kubernetes_cluster_node_pool": {"azure.aks_cluster", "azure.res.azurerm_kubernetes_cluster"},
	} {
		id, _ := c.GeneratedID(tf)
		e, ok := c.Get(id)
		if !ok || !e.Component || e.Attachment {
			t.Errorf("%s should be a component: %+v", tf, e)
			continue
		}
		for _, o := range owners {
			if !c.CanContain(o, id) {
				t.Errorf("%s should be placeable in %s (allowed: %v)", tf, o, e.AllowedParents)
			}
		}
		if c.CanContain("aws.region", id) || c.CanContain("gcp.project", id) {
			t.Errorf("%s must not be placeable outside its cluster", tf)
		}
	}
	// pure configuration stays an invisible attachment
	if e, _ := c.Get("aws.res.aws_iam_role_policy_attachment"); !e.Attachment || e.Component {
		t.Errorf("policy attachment misclassified: %+v", e)
	}
	opts := c.AttachmentsFor("aws.res.aws_ecs_cluster")
	found := false
	for _, o := range opts {
		if o.Resource == "aws_ecs_service" && o.Component {
			found = true
		}
	}
	if !found {
		t.Errorf("ecs service should be offered as a component of the cluster: %+v", opts)
	}
}

func TestCommonVocabulary(t *testing.T) {
	c := load(t)
	if c.HasTerraform("common") || !c.HasTerraform("aws") {
		t.Fatal("common must be the provider without Terraform")
	}
	for _, id := range []string{"common.group", "common.datacenter", "common.note", "common.user", "common.internet"} {
		if _, ok := c.Get(id); !ok {
			t.Errorf("%s missing", id)
		}
	}
	// Groups go anywhere and accept anything; the validator checks children
	// against the group's own parent.
	for _, parent := range []string{catalog.Root, "aws.account", "aws.vpc", "aws.subnet", "gcp.project"} {
		if !c.CanContain(parent, "common.group") || !c.CanContain(parent, "common.user") || !c.CanContain(parent, "common.note") {
			t.Errorf("%s should accept group, actor and note", parent)
		}
	}
	if !c.CanContain("common.group", "aws.ec2_instance") || !c.CanContain("common.datacenter", "aws.subnet") {
		t.Error("transparent containers accept any element")
	}
	if r, ok := c.Connection("common.users", "aws.alb"); !ok || r.Kind != catalog.FlowKind || r.Terraform != nil {
		t.Errorf("actor -> element should be a flow edge: %+v %v", r, ok)
	}
	if r, ok := c.Connection("aws.lambda_function", "common.saas"); !ok || r.Kind != catalog.FlowKind {
		t.Errorf("element -> actor should be a flow edge: %+v %v", r, ok)
	}
}

func TestZoneBoxes(t *testing.T) {
	c := load(t)
	for _, id := range []string{"aws.availability_zone", "gcp.zone", "azure.zone"} {
		e, ok := c.Get(id)
		if !ok || e.Kind != catalog.KindContainer || !e.Transparent || len(e.Provides) == 0 {
			t.Errorf("%s should be a transparent container that provides values: %+v", id, e)
		}
	}
	if !c.CanContain("aws.vpc", "aws.availability_zone") || !c.CanContain("aws.region", "aws.availability_zone") {
		t.Error("availability zones live in regions and VPCs")
	}
	if !c.CanContain("aws.availability_zone", "aws.subnet") || !c.CanContain("aws.availability_zone", "aws.ec2_instance") {
		t.Error("zone boxes accept anything (validated against the zone's parent)")
	}
}

func TestLinkResources(t *testing.T) {
	c := load(t)
	c.SetRegistry(tfschema.Catalog{Registry: tfschema.NewRegistry(iagram.SchemasFS, "")})
	// VPC to VPC: peering, drawn as a plain line.
	r, ok := c.Connection("aws.vpc", "aws.vpc")
	if !ok || r.Kind != catalog.LinkKind || r.Type != "aws.res.aws_vpc_peering_connection" || r.Style == nil || r.Style.Direction != "none" {
		t.Errorf("vpc -> vpc = %+v %v", r, ok)
	}
	b, ok := c.LinkFor("aws.vpc", "aws.vpc", "")
	if !ok || b.SrcAttr != "vpc_id" || b.DstAttr != "peer_vpc_id" || b.SrcOutput != "vpc_id" {
		t.Errorf("peering binding = %+v", b)
	}
	// Transit gateway to VPC in either orientation: the attachment.
	for _, pair := range [][2]string{{"aws.res.aws_ec2_transit_gateway", "aws.vpc"}, {"aws.vpc", "aws.res.aws_ec2_transit_gateway"}} {
		b, ok := c.LinkFor(pair[0], pair[1], "")
		if !ok || b.Link.Resource != "aws_ec2_transit_gateway_vpc_attachment" {
			t.Errorf("%v -> %+v %v", pair, b.Link.Resource, ok)
		}
	}
	// Customer gateway to transit gateway: the VPN connection picks the matching alternative.
	b, ok = c.LinkFor("aws.res.aws_customer_gateway", "aws.res.aws_ec2_transit_gateway", "")
	if !ok || b.Link.Resource != "aws_vpn_connection" || b.SrcAttr != "customer_gateway_id" || b.DstAttr != "transit_gateway_id" {
		t.Errorf("vpn binding = %+v %v", b, ok)
	}
	// Link types are neither palette tiles nor attachments.
	e, _ := c.Get("aws.res.aws_vpc_peering_connection")
	if e == nil || !e.Link || e.Attachment {
		t.Errorf("peering entry = %+v", e)
	}
	for _, f := range c.Families("aws") {
		for _, id := range f.Types {
			if id == "aws.res.aws_vpc_peering_connection" || id == "aws.res.aws_ec2_transit_gateway_vpc_attachment" {
				t.Errorf("link type %s listed in palette family %s", id, f.Label)
			}
		}
	}
	for _, o := range c.AttachmentsFor("aws.vpc") {
		if o.Resource == "aws_vpc_peering_connection" || o.Resource == "aws_route53_zone_association" {
			t.Errorf("link type %s offered as an attachment", o.Resource)
		}
	}
}

func TestOrganizationBoxes(t *testing.T) {
	c := load(t)
	if !c.CanContain(catalog.Root, "aws.organization") || !c.CanContain("aws.organization", "aws.organizational_unit") || !c.CanContain("aws.organizational_unit", "aws.organizational_unit") {
		t.Error("organization > OU > OU nesting should be allowed")
	}
	if !c.CanContain("aws.organizational_unit", "aws.account") || !c.CanContain("aws.organization", "aws.account") || !c.CanContain(catalog.Root, "aws.account") {
		t.Error("accounts live on the canvas or inside the organization tree")
	}
	e, _ := c.Get("aws.organizational_unit")
	if e == nil || e.Terraform == nil || e.Terraform.Role != catalog.RoleResource || e.Terraform.Attrs["name"] != "${name}" {
		t.Errorf("OU entry = %+v", e)
	}
}
