package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/server"
	"github.com/iagram/iagram/internal/web"
)

func runUp(args []string, stdout io.Writer) error {
	fs_ := flag.NewFlagSet("up", flag.ContinueOnError)
	file := fileFlag(fs_)
	port := fs_.Int("port", 7777, "listen port")
	fs_.IntVar(port, "p", 7777, "listen port")
	noOpen := fs_.Bool("no-open", false, "do not open the browser")
	if err := fs_.Parse(args); err != nil {
		return err
	}
	if !exists(*file) {
		return fmt.Errorf("%s not found; run `iagram init` first", *file)
	}

	cat, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		return fmt.Errorf("load catalog: %w", err)
	}
	icons, err := fs.Sub(iagram.CatalogFS, "catalog/icons")
	if err != nil {
		return err
	}
	srv := server.New(cat, *file, web.FS(), icons, Version)

	addr := net.JoinHostPort("127.0.0.1", fmt.Sprint(*port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	url := "http://" + addr
	fmt.Fprintf(stdout, "iagram %s\n  diagram: %s\n  canvas:  %s\n\nPress Ctrl+C to stop.\n", Version, *file, url)

	httpSrv := &http.Server{Handler: srv, ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- httpSrv.Serve(ln) }()

	if !*noOpen {
		go openBrowser(url)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	select {
	case <-ctx.Done():
		fmt.Fprintln(stdout, "\nshutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
