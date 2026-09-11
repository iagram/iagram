package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/convert"
	"github.com/iagram/iagram/internal/layout"
)

// runConvert rewrites the diagram for another provider using the catalog's
// equivalence table and writes it to -o (default: <name>.<provider>.iad).
func runConvert(args []string, stdout io.Writer) error {
	fs_ := flag.NewFlagSet("convert", flag.ContinueOnError)
	var c common
	c.bind(fs_)
	to := fs_.String("to", "", "target provider (aws, gcp, azure)")
	out := fs_.String("o", "", "output file (default <diagram>.<provider>.iad)")
	relayout := fs_.Bool("relayout", false, "recompute the layout instead of keeping positions")
	if err := fs_.Parse(args); err != nil {
		return err
	}
	if *to == "" {
		return fmt.Errorf("usage: iagram convert --to PROVIDER [-o FILE]")
	}
	cat, err := c.loadCatalog()
	if err != nil {
		return err
	}
	doc, err := c.loadDocument()
	if err != nil {
		return err
	}
	tbl, err := convert.Load(iagram.CatalogFS)
	if err != nil {
		return err
	}
	res, rep, err := convert.Run(cat, tbl, doc, *to)
	if err != nil {
		return err
	}
	if *relayout {
		layout.Auto(cat, res)
	}
	target := *out
	if target == "" {
		base := c.file
		for _, ext := range []string{".iad", ".json"} {
			if len(base) > len(ext) && base[len(base)-len(ext):] == ext {
				base = base[:len(base)-len(ext)]
			}
		}
		target = base + "." + *to + ".iad"
	}
	if err := res.Save(target); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "converted %d element(s) to %s -> %s\n", rep.Converted, *to, target)
	for _, d := range rep.Dropped {
		fmt.Fprintln(stdout, "dropped:", d)
	}
	for _, e := range rep.Edges {
		fmt.Fprintln(stdout, "dropped connection:", e)
	}
	for _, n := range rep.Notes {
		fmt.Fprintln(stdout, "note:", n)
	}
	return nil
}
