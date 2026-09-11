package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/importer"
	"github.com/iagram/iagram/internal/layout"
)

// runImport builds a diagram from an existing Terraform state:
//
//	iagram import --state terraform.tfstate [-o iagram.json] [--name NAME]
//	tofu show -json | iagram import --state - -o iagram.json
func runImport(args []string, stdout io.Writer) error {
	fs_ := flag.NewFlagSet("import", flag.ContinueOnError)
	var c common
	c.bind(fs_)
	state := fs_.String("state", "", "terraform.tfstate or `tofu show -json` output ('-' for stdin)")
	hclDir := fs_.String("hcl", "", "directory of Terraform configuration (.tf / .tf.json) to import")
	out := fs_.String("o", defaultFile, "output diagram file")
	name := fs_.String("name", "imported", "diagram name")
	force := fs_.Bool("force", false, "overwrite the output file if it exists")
	if err := fs_.Parse(args); err != nil {
		return err
	}
	if *state == "" && *hclDir == "" {
		return fmt.Errorf("usage: iagram import (--state FILE | --hcl DIR) [-o iagram.iad]")
	}
	if exists(*out) && !*force {
		return fmt.Errorf("%s already exists (use --force to overwrite)", *out)
	}
	cat, err := c.loadCatalog()
	if err != nil {
		return err
	}
	var doc *document.Document
	var rep importer.Report
	if *hclDir != "" {
		parsed, err := importer.ParseHCL(*hclDir)
		if err != nil {
			return err
		}
		doc, rep = importer.BuildHCL(cat, *name, parsed)
	} else {
		var raw []byte
		if *state == "-" {
			raw, err = io.ReadAll(os.Stdin)
		} else {
			raw, err = os.ReadFile(*state)
		}
		if err != nil {
			return err
		}
		res, err := importer.ParseState(raw)
		if err != nil {
			return err
		}
		doc, rep = importer.Build(cat, *name, res)
	}
	layout.Auto(cat, doc)
	if err := doc.Save(*out); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "imported %d resource(s) into %d node(s) -> %s\n", rep.Imported, len(doc.Nodes), *out)
	if len(rep.Skipped) > 0 {
		fmt.Fprintf(stdout, "skipped %d resource type(s) with no catalog mapping: %v\n", len(rep.Skipped), rep.Skipped)
	}
	for _, u := range rep.Unplaced {
		fmt.Fprintln(stdout, "note:", u)
	}
	if *hclDir == "" {
		fmt.Fprintln(stdout, "\nArrows between curated elements are not recovered from state; references between generated elements are. Run `iagram plan` to see what differs.")
	}
	return nil
}
