package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/templates"
)

func runInit(args []string, stdout io.Writer) error {
	fs_ := flag.NewFlagSet("init", flag.ContinueOnError)
	tpl := fs_.String("template", "", "start from a reference architecture (<provider>/<slug>, see `iagram templates list`)")
	var c common
	fs_.Var(&c.catalogs, "catalog", "extra catalog directory (repeatable)")
	if err := fs_.Parse(args); err != nil {
		return err
	}
	rest := fs_.Args()
	name := ""
	if len(rest) > 0 {
		name = rest[0]
	} else if wd, err := os.Getwd(); err == nil {
		name = filepath.Base(wd)
	}
	if exists(defaultFile) || exists(legacyFile) {
		return fmt.Errorf("a diagram already exists here (%s or %s)", defaultFile, legacyFile)
	}
	doc := document.New(name)
	if *tpl != "" {
		cat, err := c.loadCatalog()
		if err != nil {
			return err
		}
		list, err := templates.Load(iagram.TemplatesFS, cat)
		if err != nil {
			return err
		}
		found := false
		for _, t := range list {
			if t.ID == *tpl {
				doc, found = t.Document, true
				doc.Name = name
			}
		}
		if !found {
			return fmt.Errorf("unknown template %q (see `iagram templates list`)", *tpl)
		}
	}
	if err := doc.Save(defaultFile); err != nil {
		return err
	}
	if err := os.MkdirAll(".iagram", 0o755); err != nil {
		return err
	}
	// State, plans and downloaded binaries live here and never belong in git.
	if err := os.WriteFile(filepath.Join(".iagram", ".gitignore"), []byte("*\n!.gitignore\n"), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Created %s and .iagram/\n\nNext: iagram up\n", defaultFile)
	return nil
}
