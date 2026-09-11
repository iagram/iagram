package telemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestOnlyWhitelistedPropsAreSent(t *testing.T) {
	var mu sync.Mutex
	var got []Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var e Event
		_ = json.Unmarshal(raw, &e)
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
	}))
	defer srv.Close()
	Endpoint = srv.URL
	t.Setenv("IAGRAM_TELEMETRY", "")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("CI", "")

	home := t.TempDir()
	if Load(home, "test").Enabled() {
		t.Fatal("telemetry must be off by default")
	}
	c := Load(home, "test")
	if err := c.Set(true); err != nil {
		t.Fatal(err)
	}
	if !c.Enabled() {
		t.Fatal("expected enabled after opt-in")
	}
	c.Send("command", map[string]any{"command": "plan", "nodes": Bucket(7), "name": "SECRET", "account_id": "123456789012"})
	c.Flush()

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("got %d events", len(got))
	}
	e := got[0]
	if e.Props["command"] != "plan" || e.Props["nodes"] != "6-20" {
		t.Errorf("props = %v", e.Props)
	}
	if _, leaked := e.Props["name"]; leaked {
		t.Error("non-whitelisted prop was sent")
	}
	if _, leaked := e.Props["account_id"]; leaked {
		t.Error("account_id was sent")
	}
	if len(e.InstallID) != 32 || e.Version != "test" {
		t.Errorf("event = %+v", e)
	}
}

func TestDisableSwitches(t *testing.T) {
	Endpoint = "http://127.0.0.1:1" // must never be contacted
	for _, tc := range []struct{ k, v string }{{"IAGRAM_TELEMETRY", "0"}, {"IAGRAM_TELEMETRY", "off"}, {"DO_NOT_TRACK", "1"}, {"CI", "true"}} {
		t.Setenv("IAGRAM_TELEMETRY", "")
		t.Setenv("DO_NOT_TRACK", "")
		t.Setenv("CI", "")
		home := t.TempDir()
		_ = Load(home, "test").Set(true) // opted in...
		t.Setenv(tc.k, tc.v)
		if c := Load(home, "test"); c.Enabled() { // ...but the environment wins
			t.Errorf("%s=%s should disable", tc.k, tc.v)
		}
	}
	t.Setenv("IAGRAM_TELEMETRY", "")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("CI", "")
	home := t.TempDir()
	c := Load(home, "test")
	if err := c.Set(false); err != nil {
		t.Fatal(err)
	}
	if Load(home, "test").Enabled() {
		t.Error("persisted off was not honoured")
	}
	if Load(home, "test").Setting() != "off" {
		t.Error("setting should read off")
	}
}

func TestOptInPersistsAndCanBeRevoked(t *testing.T) {
	t.Setenv("IAGRAM_TELEMETRY", "")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("CI", "")
	home := t.TempDir()
	if err := Load(home, "test").Set(true); err != nil {
		t.Fatal(err)
	}
	if c := Load(home, "test"); !c.Enabled() || c.Setting() != "on" {
		t.Error("opt-in not persisted")
	}
	if err := Load(home, "test").Set(false); err != nil {
		t.Fatal(err)
	}
	if c := Load(home, "test"); c.Enabled() || c.Setting() != "off" {
		t.Error("opt-out not persisted")
	}
	if Load(home, "test").Notice() != "" {
		t.Error("no first-run notice for an opt-in feature")
	}
}
