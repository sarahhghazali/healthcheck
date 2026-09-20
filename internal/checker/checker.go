// Package checker performs HTTP health checks against services.
package checker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sarahhghazali/healthcheck/internal/config"
)

// Result is the outcome of checking one service.
type Result struct {
	Name       string
	URL        string
	Up         bool
	StatusCode int
	Latency    time.Duration
	Err        error
}

// Checker runs health checks using a shared HTTP client.
type Checker struct {
	Client *http.Client
}

// New returns a Checker with a sensible default HTTP client.
func New() *Checker {
	return &Checker{Client: &http.Client{}}
}

// Check sends one request to the service and reports whether it is healthy.
func (c *Checker) Check(ctx context.Context, s config.Service) Result {
	res := Result{Name: s.Name, URL: s.URL}

	// Each service has its own timeout.
	ctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, s.Method, s.URL, nil)
	if err != nil {
		res.Err = err
		return res
	}

	start := time.Now()
	resp, err := c.Client.Do(req)
	res.Latency = time.Since(start)
	if err != nil {
		res.Err = err
		return res
	}
	defer resp.Body.Close()
	// Drain the body so the connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	res.StatusCode = resp.StatusCode
	if resp.StatusCode != s.ExpectedStatus {
		res.Err = fmt.Errorf("expected status %d, got %d", s.ExpectedStatus, resp.StatusCode)
		return res
	}

	res.Up = true
	return res
}

// CheckAll checks every service concurrently and returns results
// in the same order as the input.
func (c *Checker) CheckAll(ctx context.Context, services []config.Service) []Result {
	results := make([]Result, len(services))

	var wg sync.WaitGroup
	for i, s := range services {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each goroutine writes to its own index, so no mutex is needed.
			results[i] = c.Check(ctx, s)
		}()
	}
	wg.Wait()

	return results
}
