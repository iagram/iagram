package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/validate"
)

func runValidate(args []string, stdout io.Writer) error {
	fs_ := flag.NewFlagSet("validate", flag.ContinueOnError)
	file := fileFlag(fs_)
	if err := fs_.Parse(args); err != nil {
		return err
	}
	cat, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		return fmt.Errorf("load catalog: %w", err)
	}
	doc, err := document.Load(*file)
	if err != nil {
		return err
	}
	res := validate.Run(cat, doc)
	byID := doc.Index()
	for _, p := range res.Problems {
		where := ""
		if p.Node != "" {
			if n, ok := byID[p.Node]; ok {
				where = fmt.Sprintf("%s %q", n.Type, n.Name)
			} else {
				where = p.Node
			}
		} else if p.Edge != "" {
			where = "edge " + p.Edge
		}
		fmt.Fprintf(stdout, "%-7s %s: %s\n", p.Level, where, p.Message)
	}
	if !res.OK() {
		return errors.New("validation failed")
	}
	fmt.Fprintf(stdout, "ok: %d nodes, %d connections\n", len(doc.Nodes), len(doc.Edges))
	return nil
}
