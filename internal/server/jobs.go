package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/iagram/iagram/internal/document"
	"github.com/iagram/iagram/internal/jobs"
	"github.com/iagram/iagram/internal/workspace"
)

// startPlan plans the document as saved on disk (save first from the UI), so
// what gets planned is exactly what is in the file, like terraform.
func (s *Server) startPlan(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	d, err := document.Load(s.DocPath)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job, err := s.Jobs.Start("plan", func(ctx context.Context, log *jobs.Log) (any, error) {
		start := time.Now()
		res, err := s.Workspace.Plan(ctx, s.Catalog, d, log)
		outcome := "ok"
		if err != nil {
			outcome = "error"
			if ctx.Err() != nil {
				outcome = "cancelled"
			}
		}
		props := map[string]any{"command": "plan", "outcome": outcome, "duration_s": int(time.Since(start).Seconds())}
		if res != nil {
			props["changes"] = res.Summary.Add + res.Summary.Change + res.Summary.Destroy
			s.mu.Lock()
			s.lastPlan = res
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
	p := s.lastPlan
	s.mu.Unlock()
	if p == nil {
		writeJSON(w, http.StatusOK, map[string]any{"plan": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": p})
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
