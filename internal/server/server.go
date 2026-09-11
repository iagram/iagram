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
	"strings"
	"sync"

	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/validate"
)

// Server serves the API and the embedded web UI.
type Server struct {
	Catalog *catalog.Catalog
	DocPath string
	WebFS   fs.FS // built frontend, rooted at index.html
	IconsFS fs.FS // catalog/icons
	Version string
	mu      sync.Mutex
	mux     *http.ServeMux
}

// New wires the routes.
func New(c *catalog.Catalog, docPath string, webFS, iconsFS fs.FS, version string) *Server {
	s := &Server{Catalog: c, DocPath: docPath, WebFS: webFS, IconsFS: iconsFS, Version: version}
	m := http.NewServeMux()
	m.HandleFunc("GET /api/health", s.health)
	m.HandleFunc("GET /api/catalog", s.catalog)
	m.HandleFunc("GET /api/document", s.getDocument)
	m.HandleFunc("PUT /api/document", s.putDocument)
	m.HandleFunc("POST /api/validate", s.validateDocument)
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
