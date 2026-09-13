package importer_test

import (
	"fmt"
	"testing"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/generate"
	"github.com/iagram/iagram/internal/importer"
	"github.com/iagram/iagram/internal/layout"
	"github.com/iagram/iagram/internal/tfschema"
	"github.com/iagram/iagram/internal/validate"
)

const tfstate = `{
 "version": 4,
 "terraform_version": "1.9.0",
 "resources": [
  {"mode":"managed","type":"aws_vpc","name":"main","instances":[{"attributes":{"id":"vpc-1","arn":"arn:aws:ec2:eu-west-1:123456789012:vpc/vpc-1","cidr_block":"10.0.0.0/16","enable_dns_hostnames":true,"tags":{"Name":"core"}}}]},
  {"mode":"managed","type":"aws_subnet","name":"a","instances":[{"attributes":{"id":"subnet-1","arn":"arn:aws:ec2:eu-west-1:123456789012:subnet/subnet-1","vpc_id":"vpc-1","cidr_block":"10.0.1.0/24","availability_zone":"eu-west-1a","map_public_ip_on_launch":true,"tags":{"Name":"public-a"}}}]},
  {"mode":"managed","type":"aws_subnet","name":"b","instances":[{"attributes":{"id":"subnet-2","arn":"arn:aws:ec2:eu-west-1:123456789012:subnet/subnet-2","vpc_id":"vpc-1","cidr_block":"10.0.2.0/24","availability_zone":"eu-west-1b","map_public_ip_on_launch":false,"tags":{}}}]},
  {"module":"module.web","mode":"managed","type":"aws_instance","name":"this","instances":[{"attributes":{"id":"i-1","arn":"arn:aws:ec2:eu-west-1:123456789012:instance/i-1","subnet_id":"subnet-1","instance_type":"t3.small","ami":"ami-123","associate_public_ip_address":true,"tags":{"Name":"web-1"}}}]},
  {"mode":"managed","type":"aws_lb","name":"edge","instances":[{"attributes":{"id":"arn:aws:elasticloadbalancing:eu-west-1:123456789012:loadbalancer/app/edge/abc","arn":"arn:aws:elasticloadbalancing:eu-west-1:123456789012:loadbalancer/app/edge/abc","vpc_id":"vpc-1","name":"edge","internal":false}}]},
  {"mode":"managed","type":"aws_s3_bucket","name":"assets","instances":[{"attributes":{"id":"assets-123","arn":"arn:aws:s3:::assets-123","bucket":"assets-123"}}]},
  {"mode":"managed","type":"aws_db_instance","name":"db","instances":[{"attributes":{"id":"db-1","arn":"arn:aws:rds:eu-west-1:123456789012:db:db-1","identifier":"db-1","engine":"postgres","engine_version":"16.3","instance_class":"db.t3.micro","allocated_storage":20,"multi_az":false}}]},
  {"mode":"managed","type":"aws_iam_role","name":"x","instances":[{"attributes":{"id":"x","arn":"arn:aws:iam::123456789012:role/x"}}]},
  {"mode":"data","type":"aws_ami","name":"al","instances":[{"attributes":{"id":"ami-999"}}]}
 ]
}`

func TestImportRebuildsContainmentFromState(t *testing.T) {
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	res, err := importer.ParseState([]byte(tfstate))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 8 { // data resource excluded
		t.Fatalf("resources = %d", len(res))
	}
	d, rep := importer.Build(c, "imported", res)
	layout.Auto(c, d)

	byType := map[string][]string{}
	byID := d.Index()
	for _, n := range d.Nodes {
		byType[n.Type] = append(byType[n.Type], n.ID)
	}
	if len(byType["aws.account"]) != 1 || len(byType["aws.region"]) != 1 {
		t.Fatalf("account/region synthesis: %v", byType)
	}
	acct := byID[byType["aws.account"][0]]
	reg := byID[byType["aws.region"][0]]
	if acct.Props["account_id"] != "123456789012" || reg.Props["region"] != "eu-west-1" || reg.Parent != acct.ID {
		t.Errorf("acct=%+v reg=%+v", acct, reg)
	}
	vpc := byID[byType["aws.vpc"][0]]
	if vpc.Parent != reg.ID || vpc.Name != "core" || vpc.Props["cidr"] != "10.0.0.0/16" {
		t.Errorf("vpc = %+v", vpc)
	}
	var web *struct{ parent, name string }
	for _, id := range byType["aws.ec2_instance"] {
		n := byID[id]
		web = &struct{ parent, name string }{n.Parent, n.Name}
		if n.Props["instance_type"] != "t3.small" || n.Props["public_ip"] != true {
			t.Errorf("web props = %v", n.Props)
		}
	}
	if web == nil || byID[web.parent].Type != "aws.subnet" || web.name != "web-1" {
		t.Errorf("web = %+v", web)
	}
	for _, id := range byType["aws.subnet"] {
		n := byID[id]
		// Two zones in the VPC: the importer draws Availability Zone boxes.
		az := byID[n.Parent]
		if az == nil || az.Type != "aws.availability_zone" || az.Parent != vpc.ID || az.Props["zone"] != n.Props["az"] || (n.Props["az"] != "a" && n.Props["az"] != "b") {
			t.Errorf("subnet = %+v (parent %+v)", n, az)
		}
	}
	lb := byID[byType["aws.alb"][0]]
	if lb.Props["scheme"] != "internet-facing" || lb.Parent != vpc.ID {
		t.Errorf("alb = %+v", lb)
	}
	bucket := byID[byType["aws.s3_bucket"][0]]
	if bucket.Parent != reg.ID || bucket.Name != "assets-123" { // no region in S3 ARN: sole region
		t.Errorf("bucket = %+v", bucket)
	}
	db := byID[byType["aws.rds_instance"][0]]
	if byID[db.Parent].Type != "aws.subnet" { // fallback placement
		t.Errorf("db parent = %+v", byID[db.Parent])
	}
	if rep.Imported != 7 || len(rep.Skipped) != 1 || rep.Skipped[0] != "aws_iam_role" {
		t.Errorf("report = %+v", rep)
	}
	// The imported document must validate and have a usable layout.
	if v := validate.Run(c, d); !v.OK() {
		t.Errorf("imported document invalid: %+v", v.Problems)
	}
	if vpc.Layout.W < 240 || byID[web.parent].Layout.W < 120 {
		t.Errorf("layout: vpc=%+v", vpc.Layout)
	}
}

func TestParseShowJSON(t *testing.T) {
	show := `{"format_version":"1.0","values":{"root_module":{"resources":[{"address":"aws_vpc.v","mode":"managed","type":"aws_vpc","name":"v","values":{"id":"vpc-9"}}],"child_modules":[{"address":"module.m","resources":[{"address":"module.m.aws_subnet.s","mode":"managed","type":"aws_subnet","name":"s","values":{"id":"subnet-9","vpc_id":"vpc-9"}}]}]}}}`
	res, err := importer.ParseState([]byte(show))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[1].Module != "module.m" || res[1].Attrs["vpc_id"] != "vpc-9" {
		t.Errorf("res = %+v", res)
	}
}

func TestStateImportFallsBackToGeneratedElementsAndRecoversReferences(t *testing.T) {
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	c.SetRegistry(tfschema.Catalog{Registry: tfschema.NewRegistry(iagram.SchemasFS, "")})
	state := `{"version":4,"resources":[
	 {"mode":"managed","type":"aws_kms_key","name":"data","instances":[{"attributes":{"id":"key-1","arn":"arn:aws:kms:eu-west-1:123456789012:key/key-1","description":"data at rest","enable_key_rotation":true,"key_id":"key-1"}}]},
	 {"mode":"managed","type":"aws_sns_topic","name":"alerts","instances":[{"attributes":{"id":"arn:aws:sns:eu-west-1:123456789012:alerts","arn":"arn:aws:sns:eu-west-1:123456789012:alerts","name":"alerts","kms_master_key_id":"key-1"}}]}
	]}`
	res, err := importer.ParseState([]byte(state))
	if err != nil {
		t.Fatal(err)
	}
	d, rep := importer.Build(c, "s", res)
	if rep.Imported != 2 || len(rep.Skipped) != 0 {
		t.Fatalf("report = %+v", rep)
	}
	byID := d.Index()
	var topic, key *document.Node
	for i := range d.Nodes {
		switch d.Nodes[i].Type {
		case "aws.res.aws_sns_topic":
			topic = &d.Nodes[i]
		case "aws.res.aws_kms_key":
			key = &d.Nodes[i]
		}
	}
	if topic == nil || key == nil {
		t.Fatalf("generated nodes missing: %+v", d.Nodes)
	}
	if byID[topic.Parent].Type != "aws.region" || key.Props["enable_key_rotation"] != true {
		t.Errorf("topic parent=%v key props=%v", byID[topic.Parent], key.Props)
	}
	if _, still := topic.Props["kms_master_key_id"]; still {
		t.Error("reference value should have moved into an edge")
	}
	if len(d.Edges) != 1 || d.Edges[0].Attr != "kms_master_key_id" || d.Edges[0].Target != key.ID {
		t.Errorf("edges = %+v", d.Edges)
	}
	if v := validate.Run(c, d); !v.OK() {
		t.Errorf("invalid: %+v", v.Problems)
	}
	// And it renders: the reference comes back as an expression.
	out, err := generate.Run(c, d)
	if err != nil {
		t.Fatal(err)
	}
	r := out.Config["resource"].(map[string]map[string]map[string]any)
	if r["aws_sns_topic"]["alerts"]["kms_master_key_id"] != "${aws_kms_key.data.id}" {
		t.Errorf("rendered = %v", r["aws_sns_topic"]["alerts"])
	}
}

func TestImportGroupsSubnetsIntoAvailabilityZones(t *testing.T) {
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	res, err := importer.ParseState([]byte(tfstate))
	if err != nil {
		t.Fatal(err)
	}
	d, _ := importer.Build(c, "imported", res)
	zones := map[string]string{}
	for _, n := range d.Nodes {
		if n.Type == "aws.availability_zone" {
			zones[n.ID] = fmt.Sprint(n.Props["zone"])
		}
	}
	if len(zones) != 2 {
		t.Fatalf("expected two availability zone boxes, got %v", zones)
	}
	for _, n := range d.Nodes {
		if n.Type == "aws.subnet" {
			if z, ok := zones[n.Parent]; !ok || z != fmt.Sprint(n.Props["az"]) {
				t.Errorf("subnet %s should sit in the AZ box of zone %v (parent %s)", n.Name, n.Props["az"], n.Parent)
			}
		}
	}
}

func TestImportTurnsAttachmentsIntoLinkEdges(t *testing.T) {
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	c.SetRegistry(tfschema.Catalog{Registry: tfschema.NewRegistry(iagram.SchemasFS, "")})
	state := `{"version":4,"terraform_version":"1.9.0","resources":[
	 {"mode":"managed","type":"aws_vpc","name":"main","instances":[{"attributes":{"id":"vpc-1","arn":"arn:aws:ec2:eu-west-1:123456789012:vpc/vpc-1","cidr_block":"10.0.0.0/16","tags":{"Name":"main"}}}]},
	 {"mode":"managed","type":"aws_ec2_transit_gateway","name":"hub","instances":[{"attributes":{"id":"tgw-1","arn":"arn:aws:ec2:eu-west-1:123456789012:transit-gateway/tgw-1","description":"hub","amazon_side_asn":64512,"tags":{"Name":"hub"}}}]},
	 {"mode":"managed","type":"aws_ec2_transit_gateway_vpc_attachment","name":"main","instances":[{"attributes":{"id":"tgw-attach-1","transit_gateway_id":"tgw-1","vpc_id":"vpc-1","subnet_ids":["subnet-x"],"dns_support":"enable","tags":{}}}]}
	]}`
	res, err := importer.ParseState([]byte(state))
	if err != nil {
		t.Fatal(err)
	}
	d, _ := importer.Build(c, "links", res)
	for _, n := range d.Nodes {
		if n.Type == "aws.res.aws_ec2_transit_gateway_vpc_attachment" {
			t.Errorf("attachment should be an edge, not a node: %+v", n)
		}
	}
	var link *document.Edge
	for i := range d.Edges {
		if d.Edges[i].Kind == "link" {
			link = &d.Edges[i]
		}
	}
	if link == nil {
		t.Fatalf("no link edge: %+v", d.Edges)
	}
	byID := d.Index()
	if link.Type != "aws.res.aws_ec2_transit_gateway_vpc_attachment" || byID[link.Source].Type != "aws.res.aws_ec2_transit_gateway" || byID[link.Target].Type != "aws.vpc" || link.Props["dns_support"] != "enable" {
		t.Errorf("link = %+v", link)
	}
	// And it generates back as the same resource.
	gen, err := generate.Run(c, d)
	if err != nil {
		t.Fatal(err)
	}
	blocks := gen.Config["resource"].(map[string]map[string]map[string]any)["aws_ec2_transit_gateway_vpc_attachment"]
	if len(blocks) != 1 {
		t.Errorf("attachment blocks = %v", blocks)
	}
}
