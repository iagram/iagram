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

func runApply(args []string, stdout io.Writer) (*document.Document, error) {
	fs_ := flag.NewFlagSet("apply", flag.ContinueOnError)
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
	res, err := workspace.For(c.file).Apply(ctx, cat, doc, stdout)
	if err != nil {
		return doc, err
	}
	byID := doc.Index()
	ids := make([]string, 0, len(res.Outputs))
	for id := range res.Outputs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		n := byID[id]
		fmt.Fprintf(stdout, "\n%s (%s)\n", n.Name, n.Type)
		keys := make([]string, 0, len(res.Outputs[id]))
		for k := range res.Outputs[id] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(stdout, "  %-20s %v\n", k, res.Outputs[id][k])
		}
	}
	fmt.Fprintf(stdout, "\nApplied in %.0fs; outputs written to %s.\n", res.DurationS, c.file)
	return doc, nil
}

func runDrift(args []string, stdout io.Writer) (*document.Document, error) {
	fs_ := flag.NewFlagSet("drift", flag.ContinueOnError)
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
	res, err := workspace.For(c.file).Drift(ctx, cat, doc, stdout)
	if err != nil {
		return doc, err
	}
	byID := doc.Index()
	for id, np := range res.Summary.Nodes {
		if np.Action == tofu.ActionNoop {
			continue
		}
		n := byID[id]
		fmt.Fprintf(stdout, "  drifted  %-22s %s\n", n.Type, n.Name)
		for _, r := range np.Resources {
			if r.Action != tofu.ActionNoop {
				fmt.Fprintf(stdout, "           %s: %v\n", r.Address, r.Changed)
			}
		}
	}
	if res.Drift {
		return doc, fmt.Errorf("drift detected")
	}
	return doc, nil
}

// runDestroy plans the teardown, shows it, asks for the diagram name (unless
// --yes) and applies. Outputs are cleared from the diagram afterwards.
func runDestroy(args []string, stdout io.Writer) (*document.Document, error) {
	fs_ := flag.NewFlagSet("destroy", flag.ContinueOnError)
	var c common
	c.bind(fs_)
	yes := fs_.Bool("yes", false, "skip the confirmation prompt")
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
	ws := workspace.For(c.file)
	res, err := ws.PlanDestroy(ctx, cat, doc, stdout)
	if err != nil {
		return doc, err
	}
	if !res.Changes {
		fmt.Fprintln(stdout, "\nNothing to destroy.")
		return doc, nil
	}
	byID := doc.Index()
	for id, np := range res.Summary.Nodes {
		if np.Action != tofu.ActionNoop {
			fmt.Fprintf(stdout, "  %-8s %-22s %s (%d resources)\n", np.Action, byID[id].Type, byID[id].Name, len(np.Resources))
		}
	}
	name := doc.Name
	if name == "" {
		name = "destroy"
	}
	if !*yes {
		fmt.Fprintf(stdout, "\nThis destroys %d resource(s). Type the diagram name (%q) to confirm: ", res.Summary.Destroy, name)
		var typed string
		fmt.Fscanln(os.Stdin, &typed)
		if typed != name {
			return doc, fmt.Errorf("aborted")
		}
	}
	if _, err := ws.Apply(ctx, cat, doc, stdout); err != nil {
		return doc, err
	}
	fmt.Fprintln(stdout, "\nDestroyed. Outputs cleared from the diagram.")
	return doc, nil
}
