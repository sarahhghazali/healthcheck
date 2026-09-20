// Package web serves a live dashboard of the latest check results.
package web

import (
	_ "embed" // needed for the //go:embed directive below
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/sarahhghazali/healthcheck/internal/checker"
)

// The dashboard page lives in its own file, but //go:embed bakes it into the
// binary at build time, so the program still ships as a single file.
//
//go:embed dashboard.html
var dashboardHTML string

var tmpl = template.Must(template.New("dashboard").Parse(dashboardHTML))

// State holds the most recent results. The checker writes to it from one
// goroutine while HTTP handlers read from others, so every access is guarded
// by a mutex.
type State struct {
	mu      sync.RWMutex
	results []checker.Result
	updated time.Time
}

// NewState returns an empty State.
func NewState() *State {
	return &State{}
}

// Set replaces the stored results. Called after every round of checks.
func (s *State) Set(results []checker.Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = results
	s.updated = time.Now()
}

// Snapshot returns the stored results and when they were recorded.
// RLock allows many readers at the same time, as long as nobody is writing.
func (s *State) Snapshot() ([]checker.Result, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.results, s.updated
}

// serviceView is one row of the dashboard. checker.Result can't be sent as
// JSON directly because its Err field is an error, which encodes as {}.
type serviceView struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Up         bool   `json:"up"`
	StatusCode int    `json:"status_code"`
	LatencyMS  int64  `json:"latency_ms"`
	Error      string `json:"error"`
}

// pageData is everything the page (and the JSON API) needs.
type pageData struct {
	Services       []serviceView `json:"services"`
	UpCount        int           `json:"up_count"`
	Total          int           `json:"total"`
	AllUp          bool          `json:"all_up"`
	Updated        string        `json:"updated"`
	RefreshSeconds int           `json:"refresh_seconds"`
}

func (s *State) view(refresh time.Duration) pageData {
	results, updated := s.Snapshot()

	data := pageData{
		Services:       make([]serviceView, 0, len(results)),
		Total:          len(results),
		RefreshSeconds: int(refresh.Seconds()),
	}
	if !updated.IsZero() {
		data.Updated = updated.Format(time.TimeOnly)
	}

	for _, r := range results {
		v := serviceView{
			Name:       r.Name,
			URL:        r.URL,
			Up:         r.Up,
			StatusCode: r.StatusCode,
			LatencyMS:  r.Latency.Milliseconds(),
		}
		if r.Err != nil {
			v.Error = r.Err.Error()
		}
		if r.Up {
			data.UpCount++
		}
		data.Services = append(data.Services, v)
	}
	data.AllUp = data.Total > 0 && data.UpCount == data.Total

	return data
}

// NewHandler builds the dashboard routes. refresh tells the page how often to
// ask for fresh data; it should match the check interval.
func NewHandler(state *State, refresh time.Duration) http.Handler {
	if refresh < time.Second {
		refresh = time.Second
	}
	mux := http.NewServeMux()

	// The dashboard itself. "{$}" matches only the exact path "/",
	// so unknown paths still return 404.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.Execute(w, state.view(refresh)); err != nil {
			log.Printf("rendering dashboard: %v", err)
		}
	})

	// JSON for the page's auto-refresh, and for any other tool that wants it.
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		data := state.view(refresh)
		w.Header().Set("Content-Type", "application/json")
		// Status changes constantly, so browsers must never serve a cached copy.
		w.Header().Set("Cache-Control", "no-store")
		if !data.AllUp && data.Total > 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("writing status json: %v", err)
		}
	})

	// Liveness probe for the dashboard process itself.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok\n"))
	})

	return mux
}
