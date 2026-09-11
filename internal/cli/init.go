package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/iagram/iagram/internal/document"
)

func runInit(args []string, stdout io.Writer) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	} else if wd, err := os.Getwd(); err == nil {
		name = filepath.Base(wd)
	}
	if exists(defaultFile) || exists(legacyFile) {
		return fmt.Errorf("a diagram already exists here (%s or %s)", defaultFile, legacyFile)
	}
	if err := document.New(name).Save(defaultFile); err != nil {
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
