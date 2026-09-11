// Package tofu runs OpenTofu. The binary is downloaded once, pinned by
// version and verified against the release SHA256SUMS, into ~/.iagram/bin.
// IAGRAM_TOFU_PATH overrides this with any terraform-compatible binary.
package tofu

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Version of OpenTofu iagram pins. Bump deliberately; the SHA256SUMS check
// means a wrong version simply fails to install.
const Version = "1.12.6"

const releaseBase = "https://github.com/opentofu/opentofu/releases/download/v"

// Home is the per-user iagram directory (~/.iagram unless IAGRAM_HOME is set).
func Home() (string, error) {
	if h := os.Getenv("IAGRAM_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".iagram"), nil
}

// Ensure returns the path of a usable binary, downloading OpenTofu if needed.
// Progress lines are written to out.
func Ensure(ctx context.Context, out io.Writer) (string, error) {
	if p := os.Getenv("IAGRAM_TOFU_PATH"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("IAGRAM_TOFU_PATH: %w", err)
		}
		return p, nil
	}
	home, err := Home()
	if err != nil {
		return "", err
	}
	bin := filepath.Join(home, "bin", "tofu-"+Version)
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		return "", err
	}
	fmt.Fprintf(out, "Downloading OpenTofu %s (%s/%s)...\n", Version, runtime.GOOS, runtime.GOARCH)
	if err := download(ctx, bin); err != nil {
		return "", fmt.Errorf("install OpenTofu %s: %w", Version, err)
	}
	fmt.Fprintf(out, "Installed %s\n", bin)
	return bin, nil
}

func download(ctx context.Context, dest string) error {
	asset := fmt.Sprintf("tofu_%s_%s_%s.zip", Version, runtime.GOOS, runtime.GOARCH)
	client := &http.Client{Timeout: 5 * time.Minute}

	sums, err := fetch(ctx, client, releaseBase+Version+"/tofu_"+Version+"_SHA256SUMS")
	if err != nil {
		return err
	}
	want := ""
	sc := bufio.NewScanner(strings.NewReader(string(sums)))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && f[1] == asset {
			want = f[0]
		}
	}
	if want == "" {
		return fmt.Errorf("no checksum for %s (unsupported platform?)", asset)
	}

	body, err := fetch(ctx, client, releaseBase+Version+"/"+asset)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("checksum mismatch for %s: got %s want %s", asset, got, want)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), "tofu-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		return err
	}
	tmp.Close()

	zr, err := zip.OpenReader(tmp.Name())
	if err != nil {
		return err
	}
	defer zr.Close()
	name := "tofu"
	if runtime.GOOS == "windows" {
		name = "tofu.exe"
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		w, err := os.OpenFile(dest+".tmp", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, rc); err != nil {
			w.Close()
			return err
		}
		w.Close()
		return os.Rename(dest+".tmp", dest)
	}
	return errors.New("binary not found in archive")
}

func fetch(ctx context.Context, c *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, 200<<20))
}
