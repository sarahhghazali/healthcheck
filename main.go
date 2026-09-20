// Command healthcheck checks that a list of HTTP services are up.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sarahhghazali/healthcheck/internal/checker"
	"github.com/sarahhghazali/healthcheck/internal/config"
	"github.com/sarahhghazali/healthcheck/internal/notify"
	"github.com/sarahhghazali/healthcheck/internal/report"
	"github.com/sarahhghazali/healthcheck/internal/web"
)

// Used when -web is given without -interval, since a dashboard needs
// repeated checks to stay current.
const defaultWebInterval = 30 * time.Second

func main() {
	configPath := flag.String("config", "services.yaml", "path to the services config file")
	interval := flag.Duration("interval", 0, "how often to re-check (e.g. 30s); 0 runs once and exits")
	webhookURL := flag.String("webhook", os.Getenv("HEALTHCHECK_WEBHOOK_URL"), "Slack webhook URL for alerts (or set HEALTHCHECK_WEBHOOK_URL)")
	webAddr := flag.String("web", "", "serve a live dashboard on this address (e.g. :8080); empty disables it")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	var notifier notify.Notifier
	if *webhookURL != "" {
		notifier = notify.NewWebhook(*webhookURL)
	}

	// Stop cleanly on Ctrl+C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c := checker.New()

	// A dashboard only makes sense with repeated checks.
	if *webAddr != "" && *interval == 0 {
		*interval = defaultWebInterval
		log.Printf("-web given without -interval, using %s", defaultWebInterval)
	}

	// Run once: exit code 1 if anything is down (handy for cron jobs and CI).
	if *interval == 0 {
		results := c.CheckAll(ctx, cfg.Services)
		report.PrintTable(os.Stdout, results)
		fmt.Println(report.Summary(results))
		alertChanges(ctx, notifier, nil, results)
		for _, r := range results {
			if !r.Up {
				os.Exit(1)
			}
		}
		return
	}

	// Watch mode: check on every tick until interrupted.
	log.Printf("checking %d services every %s (Ctrl+C to stop)", len(cfg.Services), *interval)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	var state *web.State
	if *webAddr != "" {
		state = web.NewState()
		srv := startDashboard(*webAddr, state, *interval)
		// Give in-flight requests a moment to finish on shutdown.
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
		}()
	}

	previous := map[string]bool{}
	for {
		results := c.CheckAll(ctx, cfg.Services)
		fmt.Printf("\n[%s] %s\n", time.Now().Format(time.TimeOnly), report.Summary(results))
		report.PrintTable(os.Stdout, results)
		alertChanges(ctx, notifier, previous, results)
		if state != nil {
			state.Set(results)
		}

		select {
		case <-ctx.Done():
			log.Println("shutting down")
			return
		case <-ticker.C:
		}
	}
}

// startDashboard runs the web dashboard in the background and returns the
// server so it can be shut down later. Listening happens here (not inside the
// goroutine) so a busy port is reported immediately.
func startDashboard(addr string, state *web.State, refresh time.Duration) *http.Server {
	if !strings.Contains(addr, ":") {
		addr = ":" + addr // allow -web 8080 as well as -web :8080
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("starting dashboard: %v", err)
	}

	srv := &http.Server{
		Handler:           web.NewHandler(state, refresh),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("dashboard stopped: %v", err)
		}
	}()

	log.Printf("dashboard at http://localhost%s", displayAddr(addr))
	return srv
}

// displayAddr turns a listen address into something clickable.
func displayAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return ":" + port
	}
	return addr
}

// alertChanges sends a notification when a service goes down or recovers.
// previous maps service name -> last known Up state; pass nil for a single run.
// Only state changes trigger alerts, so a service that stays down isn't
// reported on every tick.
func alertChanges(ctx context.Context, n notify.Notifier, previous map[string]bool, results []checker.Result) {
	for _, r := range results {
		wasUp, seen := previous[r.Name]
		if previous != nil {
			previous[r.Name] = r.Up
		}
		if n == nil {
			continue
		}

		var msg string
		switch {
		case !r.Up && (!seen || wasUp):
			msg = fmt.Sprintf(":red_circle: *%s* is DOWN (%s): %v", r.Name, r.URL, r.Err)
		case r.Up && seen && !wasUp:
			msg = fmt.Sprintf(":large_green_circle: *%s* has RECOVERED (%s)", r.Name, r.URL)
		default:
			continue
		}

		if err := n.Notify(ctx, msg); err != nil {
			log.Printf("alert failed for %s: %v", r.Name, err)
		}
	}
}
