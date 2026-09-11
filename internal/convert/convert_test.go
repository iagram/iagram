package convert_test

import (
	"testing"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/convert"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/validate"
)

func TestConvertAWSExampleToGCPAndAzure(t *testing.T) {
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	tbl, err := convert.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	src, err := document.Load("../../examples/aws-three-tier/iagram.iad")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"gcp", "azure"} {
		out, rep, err := convert.Run(c, tbl, src, target)
		if err != nil {
			t.Fatal(err)
		}
		types := map[string]int{}
		for _, n := range out.Nodes {
			types[n.Type]++
			if n.Type[:len(target)] != target {
				t.Errorf("%s: foreign node %s", target, n.Type)
			}
		}
		if rep.Converted < 8 {
			t.Errorf("%s: only %d converted, dropped %v", target, rep.Converted, rep.Dropped)
		}
		// Only the root identifiers (project id, subscription id) may be missing.
		for _, p := range validate.Run(c, out).Problems {
			if p.Level == "error" && p.Field != "project" && p.Field != "subscription_id" {
				t.Errorf("%s: converted diagram invalid: %+v", target, p)
			}
		}
		byID := out.Index()
		for _, n := range out.Nodes {
			switch target + ":" + n.Type {
			case "gcp:gcp.compute_instance":
				if n.Props["machine_type"] != "e2-micro" {
					t.Errorf("gcp machine_type = %v", n.Props["machine_type"])
				}
				if byID[n.Parent].Type != "gcp.subnetwork" {
					t.Errorf("gcp instance parent = %s", byID[n.Parent].Type)
				}
			case "gcp:gcp.project":
				if n.Props["region"] != "europe-west1" {
					t.Errorf("gcp project region = %v", n.Props["region"])
				}
			case "azure:azure.resource_group":
				if n.Props["location"] != "westeurope" {
					t.Errorf("azure location = %v", n.Props["location"])
				}
			case "azure:azure.virtual_machine":
				if n.Props["size"] != "Standard_B1s" {
					t.Errorf("azure size = %v", n.Props["size"])
				}
			}
		}
		if target == "gcp" && types["gcp.subnetwork"] != 2 {
			t.Errorf("gcp subnetworks = %d", types["gcp.subnetwork"])
		}
	}
}
