package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/templates"
	"github.com/iagram/iagram/internal/validate"
)

// runTemplates: `iagram templates list` prints the shipped reference
// architectures; `iagram templates build DIR` writes each as
// DIR/<provider>/<slug>/iagram.iad (validated), for people who want the files.
func runTemplates(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: iagram templates list | build DIR")
	}
	var c common
	fs_ := flag.NewFlagSet("templates", flag.ContinueOnError)
	fs_.Var(&c.catalogs, "catalog", "extra catalog directory (repeatable)")
	if err := fs_.Parse(args[1:]); err != nil {
		return err
	}
	cat, err := c.loadCatalog()
	if err != nil {
		return err
	}
	list, err := templates.Load(iagram.TemplatesFS, cat)
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tCATEGORY\tELEMENTS\tTITLE")
		for _, t := range list {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", t.ID, t.Category, t.Elements, t.Title)
		}
		return tw.Flush()
	case "build":
		rest := fs_.Args()
		if len(rest) != 1 {
			return fmt.Errorf("usage: iagram templates build DIR")
		}
		var failures []string
		for _, t := range list {
			if r := validate.Run(cat, t.Document); !r.OK() {
				var msgs []string
				for _, p := range r.Problems {
					if p.Level == validate.Error {
						msgs = append(msgs, fmt.Sprintf("%s%s: %s", p.Node, p.Edge, p.Message))
					}
				}
				failures = append(failures, fmt.Sprintf("%s: %s", t.ID, strings.Join(msgs, "; ")))
				continue
			}
			dir := filepath.Join(rest[0], t.Provider, t.Slug)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			if err := t.Document.Save(filepath.Join(dir, "iagram.iad")); err != nil {
				return err
			}
			meta, _ := json.MarshalIndent(t.Meta, "", "  ")
			if err := os.WriteFile(filepath.Join(dir, "template.json"), append(meta, '\n'), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(stdout, "wrote %s (%d elements)\n", filepath.Join(dir, "iagram.iad"), t.Elements)
		}
		if len(failures) > 0 {
			return fmt.Errorf("%d template(s) invalid:\n  %s", len(failures), strings.Join(failures, "\n  "))
		}
		return nil
	}
	return fmt.Errorf("unknown templates command %q", args[0])
}
