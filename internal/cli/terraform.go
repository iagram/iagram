package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"

	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/tofu"
	"github.com/iagram/iagram/internal/workspace"
)

func runGenerate(args []string, stdout io.Writer) (*document.Document, error) {
	fs_ := flag.NewFlagSet("generate", flag.ContinueOnError)
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
	ws := workspace.For(c.file)
	res, err := ws.Generate(cat, doc)
	if err != nil {
		return doc, err
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(stdout, "warning:", w)
	}
	fmt.Fprintf(stdout, "wrote %s (%d modules)\n", filepath.Join(ws.Dir, workspace.ConfigFile), len(res.ModuleToNode))
	return doc, nil
}

func runPlan(args []string, stdout io.Writer) (*document.Document, error) {
	fs_ := flag.NewFlagSet("plan", flag.ContinueOnError)
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	res, err := workspace.For(c.file).Plan(ctx, cat, doc, stdout)
	if err != nil {
		return doc, err
	}
	fmt.Fprintln(stdout)
	byID := doc.Index()
	ids := make([]string, 0, len(res.Summary.Nodes))
	for id := range res.Summary.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		np := res.Summary.Nodes[id]
		n := byID[id]
		fmt.Fprintf(stdout, "  %-8s %-22s %s (%d resources)\n", np.Action, n.Type, n.Name, len(np.Resources))
	}
	for _, o := range res.Summary.Orphans {
		fmt.Fprintf(stdout, "  %-8s %s (no longer in the diagram)\n", o.Action, o.Address)
	}
	if !res.Changes {
		fmt.Fprintln(stdout, "\nNo changes. Infrastructure matches the diagram.")
	} else {
		fmt.Fprintln(stdout, "\nRun `iagram apply` to apply this plan.")
	}
	return doc, nil
}

func runApply(args []string, stdout io.Writer) error {
	fs_ := flag.NewFlagSet("apply", flag.ContinueOnError)
	var c common
	c.bind(fs_)
	if err := fs_.Parse(args); err != nil {
		return err
	}
	if !exists(c.file) {
		return fmt.Errorf("%s not found", c.file)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ws := workspace.For(c.file)
	if err := ws.Apply(ctx, stdout); err != nil {
		return err
	}
	bin, err := tofu.Ensure(ctx, io.Discard)
	if err != nil {
		return nil
	}
	r := &tofu.Runner{Bin: bin, Dir: ws.Dir}
	outs, err := r.Outputs(ctx)
	if err != nil {
		return nil
	}
	if len(outs) > 0 {
		fmt.Fprintf(stdout, "\nOutputs are available with: %s -chdir=%s output -json\n", bin, ws.Dir)
	}
	return nil
}
