package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/telemetry"
)

func runTelemetry(args []string, stdout io.Writer, tel *telemetry.Client) error {
	if len(args) == 0 {
		args = []string{"status"}
	}
	switch args[0] {
	case "on":
		if err := tel.Set(true); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "telemetry on")
	case "off":
		if err := tel.Set(false); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "telemetry off")
	case "status":
		state := tel.Setting()
		if state == "on" && !tel.Enabled() {
			state = "on (disabled by environment: IAGRAM_TELEMETRY, DO_NOT_TRACK or CI)"
		}
		fmt.Fprintf(stdout, "telemetry: %s\nwhat is sent: https://github.com/iagram/iagram/blob/main/docs/telemetry.md\n", state)
	default:
		return fmt.Errorf("usage: iagram telemetry on|off|status")
	}
	return nil
}

// runCatalog implements `iagram catalog check DIR`: schema validation of every
// entry plus a full load layered over the built-in catalog, so references to
// built-in types resolve.
func runCatalog(args []string, stdout io.Writer) error {
	if len(args) != 2 || args[0] != "check" {
		return errors.New("usage: iagram catalog check DIR")
	}
	dir := args[1]
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	schema, err := iagram.CatalogFS.ReadFile("catalog/schema.json")
	if err != nil {
		return err
	}
	problems, err := catalog.CheckSchema(schema, os.DirFS(dir), ".")
	if err != nil {
		return err
	}
	for _, p := range problems {
		fmt.Fprintln(stdout, "schema:", p)
	}
	cat, err := catalog.LoadLayered(iagram.CatalogFS, os.DirFS(dir))
	if err != nil {
		fmt.Fprintln(stdout, "load:", err)
		return errors.New("catalog check failed")
	}
	if len(problems) > 0 {
		return errors.New("catalog check failed")
	}
	fmt.Fprintf(stdout, "ok: %d entries, %d connection rules, providers: ", len(cat.Entries), len(cat.Rules))
	sep := ""
	for name := range cat.Providers {
		fmt.Fprint(stdout, sep, name)
		sep = ", "
	}
	fmt.Fprintln(stdout)
	return nil
}
