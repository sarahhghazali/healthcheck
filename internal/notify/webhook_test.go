package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookSendsText(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decoding body: %v", err)
		}
	}))
	defer srv.Close()

	if err := NewWebhook(srv.URL).Notify(context.Background(), "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["text"] != "hello" {
		t.Errorf("text = %q, want %q", got["text"], "hello")
	}
}

func TestWebhookErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	if err := NewWebhook(srv.URL).Notify(context.Background(), "hello"); err == nil {
		t.Error("expected an error for status 400, got nil")
	}
}
