// Package notify sends alerts when a service changes state.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Notifier sends a text message somewhere.
type Notifier interface {
	Notify(ctx context.Context, message string) error
}

// Webhook posts {"text": "..."} to an incoming webhook URL.
// This format works with Slack incoming webhooks.
type Webhook struct {
	URL    string
	Client *http.Client
}

// NewWebhook creates a Webhook notifier with a 10 second timeout.
func NewWebhook(url string) *Webhook {
	return &Webhook{URL: url, Client: &http.Client{Timeout: 10 * time.Second}}
}

// Notify sends the message to the webhook.
func (w *Webhook) Notify(ctx context.Context, message string) error {
	body, err := json.Marshal(map[string]string{"text": message})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.Client.Do(req)
	if err != nil {
		return fmt.Errorf("sending webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
