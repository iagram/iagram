// Package cli implements the iagram command. Subcommands are kept on the
// standard library on purpose: the binary should stay small and boring.
package cli

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/telemetry"
	"github.com/iagram/iagram/internal/tofu"
)

// Version is set by the linker (-X) in releases.
var Version = "dev"

// Run executes the command line and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	home, _ := tofu.Home()
	tel := telemetry.Load(home, Version)
	defer tel.Flush()

	cmd := args[0]
	start := time.Now()
	var err error
	var doc *document.Document
	switch cmd {
	case "init":
		err = runInit(args[1:], stdout)
	case "up":
		err = runUp(args[1:], stdout, tel)
	case "validate":
		doc, err = runValidate(args[1:], stdout)
	case "generate":
		doc, err = runGenerate(args[1:], stdout)
	case "plan":
		doc, err = runPlan(args[1:], stdout)
	case "apply":
		doc, err = runApply(args[1:], stdout)
	case "drift":
		doc, err = runDrift(args[1:], stdout)
	case "destroy":
		doc, err = runDestroy(args[1:], stdout)
	case "import":
		err = runImport(args[1:], stdout)
	case "catalog":
		err = runCatalog(args[1:], stdout)
	case "telemetry":
		err = runTelemetry(args[1:], stdout, tel)
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "iagram %s\n", Version)
	case "help", "-h", "--help":
		usage(stdout)
	default:
		fmt.Fprintf(stderr, "iagram: unknown command %q\n\n", args[0])
		usage(stderr)
		return 2
	}

	switch cmd {
	case "init", "validate", "generate", "plan", "apply", "drift", "destroy":
		outcome := "ok"
		if err != nil {
			outcome = "error"
		}
		tel.Send("command", eventProps(cmd, outcome, time.Since(start), doc))
	}

	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "iagram: %v\n", err)
		return 1
	}
	return 0
}

// eventProps builds the telemetry payload: command, outcome, duration and
// bucketed counts. Nothing from the document's content is included.
func eventProps(cmd, outcome string, dur time.Duration, d *document.Document) map[string]any {
	p := map[string]any{"command": cmd, "outcome": outcome, "duration_s": int(dur.Seconds())}
	if d != nil {
		p["nodes"] = telemetry.Bucket(len(d.Nodes))
		p["edges"] = telemetry.Bucket(len(d.Edges))
		p["providers"] = providersOf(d)
	}
	return p
}

func providersOf(d *document.Document) string {
	seen := map[string]bool{}
	out := ""
	for _, n := range d.Nodes {
		for i := 0; i < len(n.Type); i++ {
			if n.Type[i] == '.' {
				p := n.Type[:i]
				if !seen[p] {
					seen[p] = true
					if out != "" {
						out += ","
					}
					out += p
				}
				break
			}
		}
	}
	return out
}

func usage(w io.Writer) {
	fmt.Fprint(w, `iagram: infrastructure as diagram.

Usage:
  iagram init [name]           create iagram.json in the current directory
  iagram up [flags]            open the canvas at http://localhost:7777
  iagram validate [flags]      check iagram.json against the catalog
  iagram generate [flags]      write Terraform to .iagram/tf/main.tf.json
  iagram plan [flags]          generate, then run OpenTofu init + plan
  iagram apply [flags]         apply the last plan, write outputs back onto the diagram
  iagram drift [flags]         refresh-only plan: report infrastructure that no longer matches state
  iagram destroy [--yes]       tear down everything the diagram manages (asks for the diagram name)
  iagram import --state FILE   build a diagram from an existing Terraform state
  iagram catalog check DIR     validate an external catalog directory
  iagram telemetry on|off|status
  iagram version

Flags for up/validate/generate/plan/apply:
  -f, --file FILE      diagram file (default "iagram.json")
      --catalog DIR    layer an extra catalog directory (repeatable)
  -p, --port PORT      listen port for up (default 7777)
      --host ADDR      listen address for up (default 127.0.0.1; 0.0.0.0 inside Docker)
      --no-open        do not open the browser

iagram never stores or transmits cloud credentials; OpenTofu inherits your
shell's credential chain exactly as terraform does. See SECURITY.md.
`)
}

const defaultFile = "iagram.json"

// common flags shared by the diagram commands.
type common struct {
	file     string
	catalogs multiFlag
}

func (c *common) bind(fs_ *flag.FlagSet) {
	fs_.StringVar(&c.file, "file", defaultFile, "diagram file")
	fs_.StringVar(&c.file, "f", defaultFile, "diagram file")
	fs_.Var(&c.catalogs, "catalog", "extra catalog directory (repeatable)")
}

// loadCatalog layers --catalog directories over the built-in catalog.
func (c *common) loadCatalog() (*catalog.Catalog, error) {
	var extra []fs.FS
	for _, dir := range c.catalogs {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			return nil, fmt.Errorf("--catalog %s: not a directory", dir)
		}
		extra = append(extra, os.DirFS(dir))
	}
	cat, err := catalog.LoadLayered(iagram.CatalogFS, extra...)
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}
	return cat, nil
}

func (c *common) loadDocument() (*document.Document, error) {
	if !exists(c.file) {
		return nil, fmt.Errorf("%s not found; run `iagram init` first", c.file)
	}
	return document.Load(c.file)
}

type multiFlag []string

func (m *multiFlag) String() string { return fmt.Sprint([]string(*m)) }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, filepath.Clean(v))
	return nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
