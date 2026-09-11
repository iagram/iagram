package importer_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/generate"
	"github.com/iagram/iagram/internal/importer"
	"github.com/iagram/iagram/internal/tfschema"
	"github.com/iagram/iagram/internal/validate"
)

const sampleHCL = `
provider "aws" {
  region  = "eu-west-1"
  profile = "prod"
}

resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  tags = { Name = "core" }
}

resource "aws_subnet" "a" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = "eu-west-1a"
  map_public_ip_on_launch = true
}

resource "aws_kms_key" "data" {
  description             = "data at rest"
  deletion_window_in_days = 7
  enable_key_rotation     = true
}

resource "aws_sns_topic" "alerts" {
  name              = "alerts"
  kms_master_key_id = aws_kms_key.data.id
}

resource "aws_instance" "web" {
  ami           = var.ami
  instance_type = "t3.micro"
  subnet_id     = aws_subnet.a.id
  root_block_device {
    volume_size = 20
    encrypted   = true
  }
}

variable "ami" { type = string }
output "vpc" { value = aws_vpc.main.id }
`

func loadCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	c.SetRegistry(tfschema.Catalog{Registry: tfschema.NewRegistry(iagram.SchemasFS, "")})
	return c
}

// HCL -> diagram -> Terraform JSON -> diagram must be a fixed point, and the
// generated Terraform must carry the same attributes and references.
func TestHCLRoundTripIsAFixedPoint(t *testing.T) {
	c := loadCatalog(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(sampleHCL), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := importer.ParseHCL(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Resources) != 5 || len(parsed.Providers) != 1 {
		t.Fatalf("parsed %d resources, %d providers", len(parsed.Resources), len(parsed.Providers))
	}
	doc, rep := importer.BuildHCL(c, "rt", parsed)
	if rep.Imported != 5 {
		t.Fatalf("report = %+v", rep)
	}
	// Every resource type must be known through the registry (KMS and SNS have no curated element).
	types := map[string]int{}
	for _, n := range doc.Nodes {
		types[n.Type]++
	}
	for _, want := range []string{"aws.res.aws_kms_key", "aws.res.aws_sns_topic", "aws.res.aws_instance", "aws.res.aws_vpc", "aws.res.aws_subnet", "aws.account", "aws.region"} {
		if types[want] == 0 {
			t.Errorf("missing %s in %v", want, types)
		}
	}
	// References became edges with the attribute and output recorded.
	refs := map[string]string{}
	byID := doc.Index()
	for _, e := range doc.Edges {
		refs[byID[e.Source].Name+"."+e.Attr] = byID[e.Target].Name + "." + e.Output
	}
	if refs["a.vpc_id"] != "main.id" || refs["web.subnet_id"] != "a.id" || refs["alerts.kms_master_key_id"] != "data.id" {
		t.Errorf("refs = %v", refs)
	}
	// Non-resource expressions are kept verbatim; nested blocks become JSON.
	var web, region *struct{ Props map[string]any }
	for _, n := range doc.Nodes {
		if n.Name == "web" {
			web = &struct{ Props map[string]any }{n.Props}
		}
		if n.Type == "aws.region" {
			region = &struct{ Props map[string]any }{n.Props}
		}
	}
	if web.Props["ami"] != "${var.ami}" {
		t.Errorf("ami = %v", web.Props["ami"])
	}
	if rbd, _ := web.Props["root_block_device"].([]any); len(rbd) != 1 || rbd[0].(map[string]any)["volume_size"] != 20.0 {
		t.Errorf("root_block_device = %v", web.Props["root_block_device"])
	}
	if region.Props["region"] != "eu-west-1" {
		t.Errorf("region = %v", region.Props)
	}
	if v := validate.Run(c, doc); !v.OK() {
		t.Errorf("imported doc invalid: %+v", v.Problems)
	}

	// Generate, then import the generated Terraform JSON and compare.
	res, err := generate.Run(c, doc)
	if err != nil {
		t.Fatal(err)
	}
	resources := res.Config["resource"].(map[string]map[string]map[string]any)
	if resources["aws_subnet"]["a"]["vpc_id"] != "${aws_vpc.main.id}" || resources["aws_instance"]["web"]["subnet_id"] != "${aws_subnet.a.id}" {
		t.Errorf("references not rendered: %v", resources["aws_subnet"]["a"])
	}
	if resources["aws_instance"]["web"]["provider"] == nil {
		t.Error("generated resources must bind the region provider alias")
	}
	raw, _ := res.JSON()
	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir2, "main.tf.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	parsed2, err := importer.ParseHCL(dir2)
	if err != nil {
		t.Fatal(err)
	}
	doc2, _ := importer.BuildHCL(c, "rt", parsed2)
	a := fingerprint(doc)
	b := fingerprint(doc2)
	if a != b {
		t.Errorf("round trip changed the diagram:\n%s\n---\n%s", a, b)
	}
}

// fingerprint renders the semantic content (types, names, props, references)
// independent of ids and layout, sorted, for equality checks.
func fingerprint(d *document.Document) string {
	byID := d.Index()
	var lines []string
	for _, n := range d.Nodes {
		parent := ""
		if p, ok := byID[n.Parent]; ok {
			parent = p.Type + ":" + p.Name
		}
		props, _ := json.Marshal(n.Props)
		lines = append(lines, fmt.Sprintf("N %s %s parent=%s %s", n.Type, n.Name, parent, props))
	}
	for _, e := range d.Edges {
		lines = append(lines, fmt.Sprintf("E %s.%s -> %s.%s", byID[e.Source].Name, e.Attr, byID[e.Target].Name, e.Output))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}
