// Package jobs runs one long operation at a time (plan, apply) and streams
// its log to any number of subscribers. State is in-memory: the process is
// the tool, there is nothing to persist across restarts.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Status of a job.
const (
	Running   = "running"
	Succeeded = "succeeded"
	Failed    = "failed"
	Cancelled = "cancelled"
)

// Job is one operation.
type Job struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	Status   string     `json:"status"`
	Started  time.Time  `json:"started"`
	Finished *time.Time `json:"finished,omitempty"`
	Error    string     `json:"error,omitempty"`
	Result   any        `json:"result,omitempty"`

	mu     sync.Mutex
	lines  []string
	subs   map[chan string]struct{}
	cancel context.CancelFunc
}

// Fn is the body of a job. It writes human-readable progress to log and
// returns a JSON-serialisable result.
type Fn func(ctx context.Context, log *Log) (any, error)

// Manager owns jobs.
type Manager struct {
	mu   sync.Mutex
	jobs map[string]*Job
	seq  int
	busy *Job
}

// New returns an empty manager.
func New() *Manager { return &Manager{jobs: map[string]*Job{}} }

// ErrBusy is returned when a job is already running.
var ErrBusy = errors.New("another operation is running")

// Start launches fn in the background. Only one job runs at a time because
// they all share the same working directory and state.
func (m *Manager) Start(kind string, fn Fn) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.busy != nil && m.busy.Status == Running {
		return nil, ErrBusy
	}
	m.seq++
	ctx, cancel := context.WithCancel(context.Background())
	j := &Job{ID: fmt.Sprintf("%s-%d", kind, m.seq), Kind: kind, Status: Running, Started: time.Now(), subs: map[chan string]struct{}{}, cancel: cancel}
	m.jobs[j.ID] = j
	m.busy = j
	go func() {
		defer cancel()
		res, err := fn(ctx, &Log{j: j})
		now := time.Now()
		j.mu.Lock()
		j.Finished = &now
		j.Result = res
		switch {
		case ctx.Err() != nil:
			j.Status = Cancelled
			j.Error = "cancelled"
		case err != nil:
			j.Status = Failed
			j.Error = err.Error()
		default:
			j.Status = Succeeded
		}
		for ch := range j.subs {
			close(ch)
		}
		j.subs = nil
		j.mu.Unlock()
	}()
	return j, nil
}

// Get returns a job by id.
func (m *Manager) Get(id string) (*Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	return j, ok
}

// Cancel stops a running job.
func (m *Manager) Cancel(id string) bool {
	j, ok := m.Get(id)
	if !ok {
		return false
	}
	j.cancel()
	return true
}

// Lines returns the log so far.
func (j *Job) Lines() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string(nil), j.lines...)
}

// Subscribe returns the backlog and a channel of subsequent lines; the
// channel is closed when the job ends.
func (j *Job) Subscribe() ([]string, <-chan string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	backlog := append([]string(nil), j.lines...)
	ch := make(chan string, 256)
	if j.Status != Running {
		close(ch)
		return backlog, ch
	}
	j.subs[ch] = struct{}{}
	return backlog, ch
}

// Log is an io.Writer that splits into lines and fans them out.
type Log struct {
	j   *Job
	buf strings.Builder
}

func (l *Log) Write(p []byte) (int, error) {
	l.buf.Write(p)
	for {
		s := l.buf.String()
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			break
		}
		l.emit(s[:i])
		l.buf.Reset()
		l.buf.WriteString(s[i+1:])
	}
	return len(p), nil
}

// Printf writes one formatted line.
func (l *Log) Printf(format string, args ...any) {
	l.emit(fmt.Sprintf(format, args...))
}

func (l *Log) emit(line string) {
	j := l.j
	j.mu.Lock()
	defer j.mu.Unlock()
	j.lines = append(j.lines, line)
	for ch := range j.subs {
		select {
		case ch <- line:
		default: // slow subscriber: drop rather than block the job
		}
	}
}
