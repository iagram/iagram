// Package telemetry sends a few anonymous usage events so the project can
// tell whether anyone uses it. Everything about it is deliberately boring:
//
//   - It is ON by default, disclosed on first run, and off with any of
//     `iagram telemetry off`, IAGRAM_TELEMETRY=0, DO_NOT_TRACK=1, or CI=true.
//   - The payload is the fixed set of fields in Event and nothing else. Keys in
//     Props are whitelisted; values are numbers, booleans or short enums.
//     Names, properties, ids, account ids, paths, IPs and error text are
//     never sent. The code path is this file; there is no other.
//   - The install id is a random UUID stored in ~/.iagram/config.json. It is
//     not derived from anything about the machine or the user.
//   - Sending is fire-and-forget with a 2s timeout and never affects a
//     command's outcome.
package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Endpoint receiving events. Overridable for self-hosted collectors.
var Endpoint = "https://telemetry.iagram.dev/v1/events"

// Config is ~/.iagram/config.json.
type Config struct {
	InstallID   string `json:"install_id"`
	Telemetry   string `json:"telemetry,omitempty"` // "on" | "off" | "" (default on)
	NoticeShown bool   `json:"telemetry_notice_shown,omitempty"`
}

// Event is the complete wire format.
type Event struct {
	Event     string         `json:"event"`
	InstallID string         `json:"install_id"`
	Version   string         `json:"version"`
	OS        string         `json:"os"`
	Arch      string         `json:"arch"`
	Time      time.Time      `json:"time"`
	Props     map[string]any `json:"props,omitempty"`
}

// allowedProps is the closed list of property keys that may be sent.
var allowedProps = map[string]bool{
	"command":    true, // init, up, validate, generate, plan, apply
	"nodes":      true, // bucketed count
	"edges":      true, // bucketed count
	"providers":  true, // e.g. "aws" or "aws,gcp"
	"outcome":    true, // ok | error | cancelled
	"duration_s": true, // rounded
	"changes":    true, // plan: add+change+destroy, bucketed
}

// Client sends events.
type Client struct {
	Version string
	cfg     Config
	home    string
	enabled bool
	wg      sync.WaitGroup
}

// Load reads the config (creating an install id if needed) and decides
// whether telemetry is enabled.
func Load(home, version string) *Client {
	c := &Client{Version: version, home: home}
	c.cfg = readConfig(home)
	if c.cfg.InstallID == "" {
		c.cfg.InstallID = newID()
		_ = c.save()
	}
	c.enabled = c.cfg.Telemetry != "off" && !envDisabled()
	return c
}

// Enabled reports whether events will be sent.
func (c *Client) Enabled() bool { return c.enabled }

// Setting returns "on" or "off" as configured (ignoring env overrides).
func (c *Client) Setting() string {
	if c.cfg.Telemetry == "off" {
		return "off"
	}
	return "on"
}

// Set persists on/off.
func (c *Client) Set(on bool) error {
	if on {
		c.cfg.Telemetry = "on"
	} else {
		c.cfg.Telemetry = "off"
	}
	c.enabled = on && !envDisabled()
	return c.save()
}

// Notice returns the first-run disclosure once, and records that it was shown.
func (c *Client) Notice() string {
	if c.cfg.NoticeShown {
		return ""
	}
	c.cfg.NoticeShown = true
	_ = c.save()
	if !c.enabled {
		return ""
	}
	return "iagram sends anonymous usage events (command name, version, OS, counts; never names,\n" +
		"properties, account ids or paths). Details: https://github.com/iagram/iagram/blob/main/docs/telemetry.md\n" +
		"Turn off with: iagram telemetry off   (or IAGRAM_TELEMETRY=0)\n"
}

// Send queues an event. Unknown prop keys are dropped, not sent.
func (c *Client) Send(event string, props map[string]any) {
	if !c.enabled {
		return
	}
	clean := map[string]any{}
	for k, v := range props {
		if allowedProps[k] {
			clean[k] = v
		}
	}
	e := Event{Event: event, InstallID: c.cfg.InstallID, Version: c.Version, OS: runtime.GOOS, Arch: runtime.GOARCH, Time: time.Now().UTC(), Props: clean}
	body, err := json.Marshal(e)
	if err != nil {
		return
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, Endpoint, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "iagram/"+c.Version)
		res, err := http.DefaultClient.Do(req)
		if err == nil {
			res.Body.Close()
		}
	}()
}

// Flush waits briefly for in-flight events (call before process exit).
func (c *Client) Flush() {
	done := make(chan struct{})
	go func() { c.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

// Bucket coarsens a count so it cannot fingerprint a diagram.
func Bucket(n int) string {
	switch {
	case n == 0:
		return "0"
	case n <= 5:
		return "1-5"
	case n <= 20:
		return "6-20"
	case n <= 100:
		return "21-100"
	default:
		return "100+"
	}
}

func envDisabled() bool {
	if v := strings.ToLower(os.Getenv("IAGRAM_TELEMETRY")); v == "0" || v == "off" || v == "false" {
		return true
	}
	if v := os.Getenv("DO_NOT_TRACK"); v == "1" || strings.EqualFold(v, "true") {
		return true
	}
	if v := strings.ToLower(os.Getenv("CI")); v == "1" || v == "true" {
		return true
	}
	return false
}

func configPath(home string) string { return filepath.Join(home, "config.json") }

func readConfig(home string) Config {
	var c Config
	raw, err := os.ReadFile(configPath(home))
	if err == nil {
		_ = json.Unmarshal(raw, &c)
	}
	return c
}

func (c *Client) save() error {
	if err := os.MkdirAll(c.home, 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(c.cfg, "", "  ")
	return os.WriteFile(configPath(c.home), append(raw, '\n'), 0o600)
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}
