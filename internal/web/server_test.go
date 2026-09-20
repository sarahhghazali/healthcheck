package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sarahhghazali/healthcheck/internal/checker"
)

func sampleState() *State {
	s := NewState()
	s.Set([]checker.Result{
		{Name: "Good API", URL: "https://good.example", Up: true, StatusCode: 200, Latency: 120 * time.Millisecond},
		{Name: "Bad API", URL: "https://bad.example", StatusCode: 500, Latency: 30 * time.Millisecond, Err: errors.New("expected status 200, got 500")},
	})
	return s
}

func TestDashboardRendersServices(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	NewHandler(sampleState(), 5*time.Second).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Good API", "Bad API", "1/2 up", "expected status 200, got 500"} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

func TestAPIStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	NewHandler(sampleState(), 5*time.Second).ServeHTTP(rec, req)

	// One service is down, so the API reports 503 for monitoring tools.
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}

	var got pageData
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding json: %v", err)
	}
	if got.Total != 2 || got.UpCount != 1 || got.AllUp {
		t.Errorf("unexpected summary: %+v", got)
	}
	if got.Services[1].Error == "" || got.Services[1].LatencyMS != 30 {
		t.Errorf("unexpected service: %+v", got.Services[1])
	}
}

func TestAPIStatusAllUp(t *testing.T) {
	s := NewState()
	s.Set([]checker.Result{{Name: "OK", Up: true, StatusCode: 200}})

	rec := httptest.NewRecorder()
	NewHandler(s, time.Second).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestEmptyStateAndHealthz(t *testing.T) {
	h := NewHandler(NewState(), time.Second)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Waiting for the first check") {
		t.Errorf("empty dashboard not rendered: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("healthz status = %d, want 200", rec.Code)
	}
}

func TestUnknownPathIs404(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(NewState(), time.Second).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
