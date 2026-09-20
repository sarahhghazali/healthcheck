package config

import (
	"testing"
	"time"
)

func TestParseAppliesDefaults(t *testing.T) {
	cfg, err := Parse([]byte(`
services:
  - url: https://example.com
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := cfg.Services[0]
	if s.Method != DefaultMethod {
		t.Errorf("Method = %q, want %q", s.Method, DefaultMethod)
	}
	if s.ExpectedStatus != DefaultExpectedStatus {
		t.Errorf("ExpectedStatus = %d, want %d", s.ExpectedStatus, DefaultExpectedStatus)
	}
	if s.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", s.Timeout, DefaultTimeout)
	}
	if s.Name != "https://example.com" {
		t.Errorf("Name = %q, want the URL", s.Name)
	}
}

func TestParseReadsAllFields(t *testing.T) {
	cfg, err := Parse([]byte(`
services:
  - name: API
    url: https://api.example.com/health
    method: head
    expected_status: 204
    timeout: 2s
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := cfg.Services[0]
	if s.Name != "API" || s.Method != "HEAD" || s.ExpectedStatus != 204 || s.Timeout != 2*time.Second {
		t.Errorf("unexpected service: %+v", s)
	}
}

func TestParseErrors(t *testing.T) {
	tests := map[string]string{
		"no services": `services: []`,
		"missing url": "services:\n  - name: x\n",
		"bad scheme":  "services:\n  - url: ftp://example.com\n",
		"not yaml":    "services: [",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(input)); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}
