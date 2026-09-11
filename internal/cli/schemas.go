package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/tfschema"
	"github.com/iagram/iagram/internal/tofu"
)

// runSchemas shows the provider schema snapshots or refreshes them from the
// real providers (`iagram schemas update`), writing to ~/.iagram/schemas or,
// with --write DIR, to a directory (used to refresh the repo's snapshots).
func runSchemas(args []string, stdout io.Writer) error {
	fs_ := flag.NewFlagSet("schemas", flag.ContinueOnError)
	write := fs_.String("write", "", "directory to write snapshots to (default ~/.iagram/schemas)")
	if err := fs_.Parse(args); err != nil {
		return err
	}
	home, _ := tofu.Home()
	userDir := filepath.Join(home, "schemas")
	reg := tfschema.NewRegistry(iagram.SchemasFS, userDir)

	if fs_.NArg() == 0 || fs_.Arg(0) == "status" {
		var c common
		cat, err := c.loadCatalog()
		if err != nil {
			return err
		}
		names := make([]string, 0, len(cat.Providers))
		for n := range cat.Providers {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			local := cat.Providers[n].LocalName()
			p, err := reg.Provider(local)
			if err != nil {
				fmt.Fprintf(stdout, "%-8s %-10s no snapshot\n", n, local)
				continue
			}
			src := "embedded"
			if _, err := os.Stat(filepath.Join(userDir, local+".json.gz")); err == nil {
				src = userDir
			}
			fmt.Fprintf(stdout, "%-8s %-10s v%-9s %5d resource types  (%s)\n", n, local, p.Version, len(p.Resources), src)
		}
		return nil
	}
	if fs_.Arg(0) != "update" {
		return fmt.Errorf("usage: iagram schemas [status|update] [--write DIR]")
	}
	var c common
	cat, err := c.loadCatalog()
	if err != nil {
		return err
	}
	providers := map[string]string{}
	for _, p := range cat.Providers {
		providers[p.LocalName()] = p.Source + "@" + p.Version
	}
	ctx := context.Background()
	bin, err := tofu.Ensure(ctx, stdout)
	if err != nil {
		return err
	}
	out, err := tfschema.Extract(ctx, bin, providers, stdout)
	if err != nil {
		return err
	}
	dir := *write
	if dir == "" {
		dir = userDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for local, p := range out {
		f, err := os.Create(filepath.Join(dir, local+".json.gz"))
		if err != nil {
			return err
		}
		if err := tfschema.Encode(p, f); err != nil {
			f.Close()
			return err
		}
		f.Close()
		fmt.Fprintf(stdout, "wrote %s (%d resource types, v%s)\n", filepath.Join(dir, local+".json.gz"), len(p.Resources), p.Version)
	}
	return nil
}
