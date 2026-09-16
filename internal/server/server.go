// Package server is the local HTTP API and static host for the canvas. It runs
// in-process with the CLI; there is no remote component.
package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/jobs"
	"github.com/iagram/iagram/internal/layout"
	"github.com/iagram/iagram/internal/templates"
	"github.com/iagram/iagram/internal/validate"
	"github.com/iagram/iagram/internal/workspace"
)

// Server serves the API and the embedded web UI.
type Server struct {
	Catalog *catalog.Catalog
	DocPath string
	WebFS   fs.FS // built frontend, rooted at index.html
	IconsFS fs.FS // catalog/icons
	Version string
	// TemplatesFS holds templates/<provider>/<slug>/template.yaml (nil: none).
	TemplatesFS fs.FS
	tplOnce     sync.Once
	tpl         []templates.Template
	tplErr      error
	Workspace   *workspace.Workspace
	Jobs        *jobs.Manager
	// OnEvent receives telemetry events; nil disables. The document is passed
	// so the caller can derive bucketed counts, never content.
	OnEvent func(name string, props map[string]any, d *document.Document)

	mu           sync.Mutex
	mux          *http.ServeMux
	lastPlan     *workspace.PlanResult
	lastPlanHash string
	lastDrift    *workspace.DriftResult
}

// New wires the routes.
func New(c *catalog.Catalog, docPath string, webFS, iconsFS fs.FS, version string) *Server {
	s := &Server{Catalog: c, DocPath: docPath, WebFS: webFS, IconsFS: iconsFS, Version: version, Workspace: workspace.For(docPath), Jobs: jobs.New()}
	m := http.NewServeMux()
	m.HandleFunc("GET /api/health", s.health)
	m.HandleFunc("GET /api/catalog", s.catalog)
	m.HandleFunc("GET /api/templates", s.listTemplates)
	m.HandleFunc("GET /api/templates/{provider}/{slug}", s.getTemplate)
	m.HandleFunc("GET /api/templates/{provider}/{slug}/diagram", s.templateDiagram)
	m.HandleFunc("GET /api/document", s.getDocument)
	m.HandleFunc("PUT /api/document", s.putDocument)
	m.HandleFunc("POST /api/validate", s.validateDocument)
	m.HandleFunc("GET /api/generate", s.generate)
	m.HandleFunc("POST /api/import", s.importState)
	m.HandleFunc("POST /api/import/hcl", s.importHCL)
	m.HandleFunc("POST /api/convert", s.convertDoc)
	m.HandleFunc("POST /api/layout", s.layoutDoc)
	m.HandleFunc("GET /api/resources", s.generatedList)
	m.HandleFunc("GET /api/resources/attachments", s.attachmentsFor)
	m.HandleFunc("GET /api/resources/{id}", s.generatedEntry)
	m.HandleFunc("POST /api/resources/resolve", s.resolveEntries)
	m.HandleFunc("POST /api/plan", s.startPlan)
	m.HandleFunc("POST /api/plan/destroy", s.startDestroyPlan)
	m.HandleFunc("GET /api/plan/latest", s.latestPlan)
	m.HandleFunc("POST /api/apply", s.startApply)
	m.HandleFunc("POST /api/drift", s.startDrift)
	m.HandleFunc("DELETE /api/drift", s.clearDrift)
	m.HandleFunc("GET /api/jobs/{id}", s.getJob)
	m.HandleFunc("GET /api/jobs/{id}/stream", s.streamJob)
	m.HandleFunc("POST /api/jobs/{id}/cancel", s.cancelJob)
	m.Handle("GET /icons/", http.StripPrefix("/icons/", http.FileServerFS(iconsFS)))
	m.HandleFunc("/", s.static)
	s.mux = m
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Local tool: refuse anything that is not same-origin browser traffic.
	if o := r.Header.Get("Origin"); o != "" && !strings.HasPrefix(o, "http://localhost") && !strings.HasPrefix(o, "http://127.0.0.1") {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}
	s.mux.ServeHTTP(w, r)
}

// layoutDoc arranges the posted document (whole diagram, or the subtree under
// root) with the given options and returns it.
func (s *Server) layoutDoc(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Document document.Document `json:"document"`
		Options  layout.Options    `json:"options"`
		Root     string            `json:"root"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	d := req.Document
	o := req.Options
	if o.Algo == "" {
		o = layout.Defaults()
	}
	if req.Root != "" {
		// Arrange only the subtree: run on a copy holding the root's
		// descendants, then copy positions back (the root keeps its place).
		sub := document.New(d.Name)
		inside := map[string]bool{req.Root: true}
		for grew := true; grew; {
			grew = false
			for _, n := range d.Nodes {
				if !inside[n.ID] && inside[n.Parent] {
					inside[n.ID] = true
					grew = true
				}
			}
		}
		for _, n := range d.Nodes {
			if inside[n.ID] {
				sub.Nodes = append(sub.Nodes, n)
			}
		}
		for _, e := range d.Edges {
			if inside[e.Source] && inside[e.Target] {
				sub.Edges = append(sub.Edges, e)
			}
		}
		layout.Arrange(s.Catalog, sub, o)
		byID := sub.Index()
		for i := range d.Nodes {
			if m, ok := byID[d.Nodes[i].ID]; ok {
				if d.Nodes[i].ID == req.Root {
					m.Layout.X, m.Layout.Y = d.Nodes[i].Layout.X, d.Nodes[i].Layout.Y
				}
				d.Nodes[i].Layout = m.Layout
			}
		}
	} else {
		layout.Arrange(s.Catalog, &d, o)
	}
	writeJSON(w, http.StatusOK, map[string]any{"document": d})
}

// listTemplates lists the shipped reference architectures with their documents
// (small enough to preview client-side).
func (s *Server) listTemplates(w http.ResponseWriter, _ *http.Request) {
	list, err := s.templates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": list})
}

func (s *Server) getTemplate(w http.ResponseWriter, r *http.Request) {
	list, err := s.templates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	id := r.PathValue("provider") + "/" + r.PathValue("slug")
	for _, t := range list {
		if t.ID == id {
			writeJSON(w, http.StatusOK, t)
			return
		}
	}
	http.NotFound(w, r)
}

// templateDiagram serves the bundled vendor diagram of a template.
func (s *Server) templateDiagram(w http.ResponseWriter, r *http.Request) {
	list, err := s.templates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	id := r.PathValue("provider") + "/" + r.PathValue("slug")
	for _, t := range list {
		if t.ID != id || t.DiagramFile == "" {
			continue
		}
		data, err := fs.ReadFile(s.TemplatesFS, t.DiagramFile)
		if err != nil {
			break
		}
		ct := map[string]string{".jpg": "image/jpeg", ".svg": "image/svg+xml", ".png": "image/png", ".webp": "image/webp"}[path.Ext(t.DiagramFile)]
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) templates() ([]templates.Template, error) {
	s.tplOnce.Do(func() {
		if s.TemplatesFS != nil {
			s.tpl, s.tplErr = templates.Load(s.TemplatesFS, s.Catalog)
		}
	})
	return s.tpl, s.tplErr
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": s.Version, "document": s.DocPath})
}

func (s *Server) catalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Catalog)
}

func (s *Server) getDocument(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := document.Load(s.DocPath)
	if errors.Is(err, os.ErrNotExist) {
		d = document.New("")
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"document": d, "validation": validate.Run(s.Catalog, d)})
}

func (s *Server) putDocument(w http.ResponseWriter, r *http.Request) {
	d, err := decodeDoc(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := d.Save(s.DocPath); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"document": d, "validation": validate.Run(s.Catalog, d)})
}

func (s *Server) validateDocument(w http.ResponseWriter, r *http.Request) {
	d, err := decodeDoc(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, validate.Run(s.Catalog, d))
}

// static serves the embedded SPA with an index.html fallback.
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		p = "index.html"
	}
	if f, err := s.WebFS.Open(p); err == nil {
		f.Close()
		http.ServeFileFS(w, r, s.WebFS, p)
		return
	}
	http.ServeFileFS(w, r, s.WebFS, "index.html")
}

func decodeDoc(r *http.Request) (*document.Document, error) {
	raw, err := readAll(r, 8<<20)
	if err != nil {
		return nil, err
	}
	return document.Decode(raw)
}

func readAll(r *http.Request, limit int64) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(http.MaxBytesReader(nil, r.Body, limit))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
