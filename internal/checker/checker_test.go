package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sarahhghazali/healthcheck/internal/config"
)

func service(url string) config.Service {
	return config.Service{
		Name:           "test",
		URL:            url,
		Method:         http.MethodGet,
		ExpectedStatus: http.StatusOK,
		Timeout:        time.Second,
	}
}

func TestCheckUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := New().Check(context.Background(), service(srv.URL))
	if !res.Up || res.StatusCode != http.StatusOK || res.Err != nil {
		t.Errorf("expected service to be up, got %+v", res)
	}
}

func TestCheckWrongStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	res := New().Check(context.Background(), service(srv.URL))
	if res.Up || res.StatusCode != http.StatusInternalServerError || res.Err == nil {
		t.Errorf("expected service to be down with 500, got %+v", res)
	}
}

func TestCheckTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	s := service(srv.URL)
	s.Timeout = 50 * time.Millisecond

	res := New().Check(context.Background(), s)
	if res.Up || res.Err == nil {
		t.Errorf("expected timeout error, got %+v", res)
	}
}

func TestCheckUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening anymore

	res := New().Check(context.Background(), service(url))
	if res.Up || res.Err == nil {
		t.Errorf("expected connection error, got %+v", res)
	}
}

func TestCheckAllKeepsOrder(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer up.Close()
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer down.Close()

	results := New().CheckAll(context.Background(), []config.Service{
		service(up.URL), service(down.URL), service(up.URL),
	})

	want := []bool{true, false, true}
	for i, r := range results {
		if r.Up != want[i] {
			t.Errorf("result %d: Up = %v, want %v", i, r.Up, want[i])
		}
	}
}
