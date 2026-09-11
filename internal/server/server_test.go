package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/server"
)

func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	c, err := catalog.Load(iagram.CatalogFS)
	if err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(t.TempDir(), "iagram.json")
	web := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}, "assets/app.js": {Data: []byte("1")}}
	icons := fstest.MapFS{"aws/ec2.svg": {Data: []byte("<svg/>")}}
	ts := httptest.NewServer(server.New(c, doc, web, icons, "test"))
	t.Cleanup(ts.Close)
	return ts, doc
}

func TestRoutes(t *testing.T) {
	ts, _ := newTestServer(t)
	get := func(p string) (*http.Response, string) {
		res, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		_, _ = strings.NewReader("").WriteTo(&b)
		buf := make([]byte, 1<<16)
		n, _ := res.Body.Read(buf)
		res.Body.Close()
		return res, string(buf[:n])
	}
	if res, body := get("/api/health"); res.StatusCode != 200 || !strings.Contains(body, `"ok":true`) {
		t.Errorf("health: %d %s", res.StatusCode, body)
	}
	if res, body := get("/api/catalog"); res.StatusCode != 200 || !strings.Contains(body, `"aws.ec2_instance"`) {
		t.Errorf("catalog: %d", res.StatusCode)
	}
	if res, body := get("/api/document"); res.StatusCode != 200 || !strings.Contains(body, `"version":1`) {
		t.Errorf("document (missing file -> empty doc): %d %s", res.StatusCode, body)
	}
	if _, body := get("/"); body != "<html>ui</html>" {
		t.Errorf("index: %q", body)
	}
	if _, body := get("/some/spa/route"); body != "<html>ui</html>" {
		t.Errorf("spa fallback: %q", body)
	}
	if _, body := get("/assets/app.js"); body != "1" {
		t.Errorf("asset: %q", body)
	}
	if _, body := get("/icons/aws/ec2.svg"); body != "<svg/>" {
		t.Errorf("icon: %q", body)
	}
	if res, _ := get("/api/nothing"); res.StatusCode != 404 {
		t.Errorf("unknown api: %d", res.StatusCode)
	}
}

func TestPutDocumentSavesAndValidates(t *testing.T) {
	ts, docPath := newTestServer(t)
	body := `{"version":1,"nodes":[{"id":"v","type":"aws.vpc","name":"x","props":{"cidr":"10.0.0.0/16"},"layout":{"x":0,"y":0}}],"edges":[]}`
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/document", strings.NewReader(body))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Validation struct {
			Problems []struct{ Message string } `json:"problems"`
		} `json:"validation"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Validation.Problems) != 1 || !strings.Contains(out.Validation.Problems[0].Message, "cannot be placed on the canvas") {
		t.Errorf("validation = %+v", out.Validation)
	}
	if _, err := http.Get("file://" + docPath); err == nil {
		t.Log("saved")
	}
}

func TestForeignOriginRejected(t *testing.T) {
	ts, _ := newTestServer(t)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/health", nil)
	req.Header.Set("Origin", "https://evil.example")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d", res.StatusCode)
	}
}

func TestApplyRequiresAPlan(t *testing.T) {
	ts, _ := newTestServer(t)
	res, err := http.Post(ts.URL+"/api/apply", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", res.StatusCode)
	}
	res, err = http.Get(ts.URL + "/api/plan/latest")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	if out["plan"] != nil || out["drift"] != nil {
		t.Errorf("latest = %v", out)
	}
}

func TestImportEndpointBuildsADocumentWithoutSaving(t *testing.T) {
	ts, docPath := newTestServer(t)
	state := `{"version":4,"resources":[{"mode":"managed","type":"aws_vpc","name":"v","instances":[{"attributes":{"id":"vpc-1","arn":"arn:aws:ec2:eu-west-1:123456789012:vpc/vpc-1","cidr_block":"10.0.0.0/16","tags":{"Name":"core"}}}]}]}`
	res, err := http.Post(ts.URL+"/api/import?name=x", "application/json", strings.NewReader(state))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Document struct {
			Name  string                  `json:"name"`
			Nodes []struct{ Type string } `json:"nodes"`
		} `json:"document"`
		Report struct{ Imported int } `json:"report"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || out.Report.Imported != 1 || len(out.Document.Nodes) != 3 || out.Document.Name != "x" { // vpc + synthesized account + region
		t.Errorf("status=%d out=%+v", res.StatusCode, out)
	}
	if _, err := os.Stat(docPath); err == nil {
		t.Error("import must not write the document")
	}
}
