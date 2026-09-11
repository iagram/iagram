package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/validate"
)

func runValidate(args []string, stdout io.Writer) (*document.Document, error) {
	fs_ := flag.NewFlagSet("validate", flag.ContinueOnError)
	var c common
	c.bind(fs_)
	if err := fs_.Parse(args); err != nil {
		return nil, err
	}
	cat, err := c.loadCatalog()
	if err != nil {
		return nil, err
	}
	doc, err := c.loadDocument()
	if err != nil {
		return nil, err
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
		return doc, errors.New("validation failed")
	}
	fmt.Fprintf(stdout, "ok: %d nodes, %d connections\n", len(doc.Nodes), len(doc.Edges))
	return doc, nil
}
