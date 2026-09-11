package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/iagram/iagram"
	"github.com/iagram/iagram/internal/catalog"
	"github.com/iagram/iagram/internal/convert"
	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/importer"
	"github.com/iagram/iagram/internal/jobs"
	"github.com/iagram/iagram/internal/layout"
	"github.com/iagram/iagram/internal/validate"
	"github.com/iagram/iagram/internal/workspace"
)

// startPlan plans the document as saved on disk (save first from the UI), so
// what gets planned is exactly what is in the file, like terraform.
func (s *Server) startPlan(w http.ResponseWriter, r *http.Request) {
	s.startPlanKind(w, r, false)
}

// startDestroyPlan produces a destroy plan; Apply then runs it.
func (s *Server) startDestroyPlan(w http.ResponseWriter, r *http.Request) {
	s.startPlanKind(w, r, true)
}

func (s *Server) startPlanKind(w http.ResponseWriter, _ *http.Request, destroy bool) {
	s.mu.Lock()
	d, err := document.Load(s.DocPath)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job, err := s.Jobs.Start("plan", func(ctx context.Context, log *jobs.Log) (any, error) {
		start := time.Now()
		var res *workspace.PlanResult
		if destroy {
			res, err = s.Workspace.PlanDestroy(ctx, s.Catalog, d, log)
		} else {
			res, err = s.Workspace.Plan(ctx, s.Catalog, d, log)
		}
		outcome := "ok"
		if err != nil {
			outcome = "error"
			if ctx.Err() != nil {
				outcome = "cancelled"
			}
		}
		cmd := "plan"
		if destroy {
			cmd = "destroy"
		}
		props := map[string]any{"command": cmd, "outcome": outcome, "duration_s": int(time.Since(start).Seconds())}
		if res != nil {
			props["changes"] = res.Summary.Add + res.Summary.Change + res.Summary.Destroy
			s.mu.Lock()
			s.lastPlan = res
			s.lastPlanHash = hashDoc(d)
			s.mu.Unlock()
		}
		s.event("command", props, d)
		if err != nil {
			return nil, err
		}
		return res, nil
	})
	if errors.Is(err, jobs.ErrBusy) {
		writeError(w, http.StatusConflict, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) latestPlan(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	p, dr := s.lastPlan, s.lastDrift
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"plan": p, "drift": dr})
}

// startApply applies the last plan. It refuses if the saved diagram differs
// from the one that was planned, mirroring `terraform apply plan.tfplan`
// semantics: what you reviewed is what runs.
func (s *Server) startApply(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	plan, planHash := s.lastPlan, s.lastPlanHash
	s.mu.Unlock()
	if plan == nil {
		writeError(w, http.StatusConflict, errors.New("no plan to apply; run plan first"))
		return
	}
	s.mu.Lock()
	d, err := document.Load(s.DocPath)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if hashDoc(d) != planHash {
		writeError(w, http.StatusConflict, errors.New("the diagram changed since it was planned; plan again"))
		return
	}
	job, err := s.Jobs.Start("apply", func(ctx context.Context, log *jobs.Log) (any, error) {
		start := time.Now()
		res, err := s.Workspace.Apply(ctx, s.Catalog, d, log)
		outcome := "ok"
		if err != nil {
			outcome = "error"
			if ctx.Err() != nil {
				outcome = "cancelled"
			}
		}
		s.mu.Lock()
		s.lastPlan, s.lastPlanHash = nil, "" // the plan file is consumed either way
		s.mu.Unlock()
		s.event("command", map[string]any{"command": "apply", "outcome": outcome, "duration_s": int(time.Since(start).Seconds())}, d)
		if err != nil {
			return nil, err
		}
		return res, nil
	})
	if errors.Is(err, jobs.ErrBusy) {
		writeError(w, http.StatusConflict, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

// startDrift runs a refresh-only plan and stores the result for the overlay.
func (s *Server) startDrift(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	d, err := document.Load(s.DocPath)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job, err := s.Jobs.Start("drift", func(ctx context.Context, log *jobs.Log) (any, error) {
		start := time.Now()
		res, err := s.Workspace.Drift(ctx, s.Catalog, d, log)
		outcome := "ok"
		if err != nil {
			outcome = "error"
			if ctx.Err() != nil {
				outcome = "cancelled"
			}
		}
		if res != nil {
			s.mu.Lock()
			s.lastDrift = res
			s.mu.Unlock()
		}
		s.event("command", map[string]any{"command": "drift", "outcome": outcome, "duration_s": int(time.Since(start).Seconds())}, d)
		if err != nil {
			return nil, err
		}
		return res, nil
	})
	if errors.Is(err, jobs.ErrBusy) {
		writeError(w, http.StatusConflict, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) clearDrift(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.lastDrift = nil
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// hashDoc fingerprints the semantic content of a document (layout and
// outputs excluded) so cosmetic moves do not invalidate a plan.
func hashDoc(d *document.Document) string {
	h := sha256.New()
	for _, n := range d.Nodes {
		props, _ := json.Marshal(n.Props)
		fmt.Fprintf(h, "n|%s|%s|%s|%s|%s\n", n.ID, n.Type, n.Name, n.Parent, props)
	}
	for _, e := range d.Edges {
		fmt.Fprintf(h, "e|%s|%s|%s|%s\n", e.ID, e.Kind, e.Source, e.Target)
	}
	fmt.Fprintf(h, "name|%s", d.Name)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	j, ok := s.Jobs.Get(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	if !s.Jobs.Cancel(r.PathValue("id")) {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// streamJob sends the log as server-sent events: `line` events, then one
// `done` event carrying the final job.
func (s *Server) streamJob(w http.ResponseWriter, r *http.Request) {
	j, ok := s.Jobs.Get(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(event string, v any) {
		raw, _ := json.Marshal(v)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
		fl.Flush()
	}
	backlog, ch := j.Subscribe()
	for _, l := range backlog {
		send("line", l)
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case l, more := <-ch:
			if !more {
				send("done", j)
				return
			}
			send("line", l)
		}
	}
}

// generate renders Terraform for the saved document and returns it, so the
// UI can show exactly what a plan would run.
func (s *Server) generate(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := document.Load(s.DocPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.Workspace.Generate(s.Catalog, d)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	raw, _ := res.JSON()
	writeJSON(w, http.StatusOK, map[string]any{"path": s.Workspace.Dir + string(os.PathSeparator) + workspace.ConfigFile, "config": json.RawMessage(raw), "warnings": res.Warnings})
}

// event forwards a telemetry event with document-derived, bucketed props.
func (s *Server) event(name string, props map[string]any, d *document.Document) {
	if s.OnEvent == nil {
		return
	}
	s.OnEvent(name, props, d)
}

// importState turns a posted terraform.tfstate / `tofu show -json` body into a
// document (not saved: the UI shows it and the user decides to save).
func (s *Server) importState(w http.ResponseWriter, r *http.Request) {
	raw, err := readAll(r, 64<<20)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	res, err := importer.ParseState(raw)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "imported"
	}
	d, rep := importer.Build(s.Catalog, name, res)
	layout.Auto(s.Catalog, d)
	writeJSON(w, http.StatusOK, map[string]any{"document": d, "report": rep, "validation": validate.Run(s.Catalog, d)})
}

// generatedList serves the light listing of generated elements for a provider.
func (s *Server) generatedList(w http.ResponseWriter, r *http.Request) {
	prov := r.URL.Query().Get("provider")
	if prov == "" {
		writeError(w, http.StatusBadRequest, errors.New("provider is required"))
		return
	}
	list := s.Catalog.Generated(prov)
	if list == nil {
		list = []catalog.GeneratedSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"provider": prov, "elements": list})
}

// generatedEntry serves the full entry (settings schema included) of one element.
func (s *Server) generatedEntry(w http.ResponseWriter, r *http.Request) {
	e, ok := s.Catalog.Get(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

// resolveEntries returns full entries for a list of ids (used after loading a
// document that contains generated elements).
func (s *Server) resolveEntries(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	raw, err := readAll(r, 1<<20)
	if err != nil || json.Unmarshal(raw, &req) != nil {
		writeError(w, http.StatusBadRequest, errors.New("expected {\"ids\": [...]}"))
		return
	}
	out := map[string]*catalog.Entry{}
	for _, id := range req.IDs {
		if e, ok := s.Catalog.Get(id); ok {
			out[id] = e
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out})
}

// importHCL turns posted HCL (a single .tf body) into a document of generated elements.
func (s *Server) importHCL(w http.ResponseWriter, r *http.Request) {
	raw, err := readAll(r, 16<<20)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	dir, err := os.MkdirTemp("", "iagram-hcl-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer os.RemoveAll(dir)
	name := "main.tf"
	if r.URL.Query().Get("format") == "json" {
		name = "main.tf.json"
	}
	if err := os.WriteFile(dir+"/"+name, raw, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	parsed, err := importer.ParseHCL(dir)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	docName := r.URL.Query().Get("name")
	if docName == "" {
		docName = "imported"
	}
	d, rep := importer.BuildHCL(s.Catalog, docName, parsed)
	layout.Auto(s.Catalog, d)
	writeJSON(w, http.StatusOK, map[string]any{"document": d, "report": rep, "validation": validate.Run(s.Catalog, d)})
}

// convertDoc rewrites the saved diagram for another provider (not saved).
func (s *Server) convertDoc(w http.ResponseWriter, r *http.Request) {
	to := r.URL.Query().Get("to")
	if to == "" {
		writeError(w, http.StatusBadRequest, errors.New("to is required"))
		return
	}
	s.mu.Lock()
	d, err := document.Load(s.DocPath)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	tbl, err := convert.Load(iagram.CatalogFS)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out, rep, err := convert.Run(s.Catalog, tbl, d, to)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"document": out, "report": rep, "validation": validate.Run(s.Catalog, out)})
}
