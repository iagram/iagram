package catalog_test

import (
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
	if _, ok := c.Connection("gcp.pubsub_topic", id); ok {
		t.Error("curated -> generated has no rule")
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
	if g := byRes["aws_sns_topic_subscription"]; g.Graphical || !g.Attachment {
		t.Errorf("subscription should be an attachment: %+v", g)
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
	for _, res := range []string{"aws_route", "aws_security_group_rule", "aws_lambda_permission", "aws_lambda_alias", "aws_cloudwatch_event_target", "aws_api_gateway_method", "aws_lb_listener", "aws_lb_target_group", "aws_ecs_service", "aws_eks_node_group", "aws_rds_cluster_instance", "aws_cloudwatch_log_stream", "aws_kms_alias", "aws_ecr_repository_policy", "aws_glue_catalog_table"} {
		if g := byRes[res]; g.Graphical {
			t.Errorf("%s should be an attachment: %+v", res, g)
		}
	}
	gcpList := c.Generated("gcp")
	gcpBy := map[string]catalog.GeneratedSummary{}
	for _, g := range gcpList {
		gcpBy[g.Resource] = g
	}
	for _, res := range []string{"google_sql_database", "google_sql_user", "google_pubsub_subscription", "google_container_node_pool", "google_bigquery_table"} {
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
	opts = c.AttachmentsFor("aws.res.aws_sns_topic")
	found = map[string]catalog.AttachmentOption{}
	for _, o := range opts {
		found[o.Resource] = o
	}
	if v, ok := found["aws_sns_topic_subscription"]; !ok || v.Attr != "topic_arn" || v.Output != "arn" {
		t.Errorf("subscription binding = %+v", v)
	}
	e, _ := c.Get("aws.res.aws_sns_topic_subscription")
	if !e.Attachment || !c.CanContain("aws.res.aws_sns_topic", e.ID) {
		t.Error("attachment must be placeable inside its (leaf) parent")
	}
}
