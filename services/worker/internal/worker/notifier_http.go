package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type HTTPNotifier struct {
	BaseURL string
	Client  *http.Client
}

func NewHTTPNotifier(baseURL string, timeout time.Duration) *HTTPNotifier {
	client := &http.Client{
		Timeout: timeout,
	}
	return &HTTPNotifier{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  client,
	}
}

func (n *HTTPNotifier) NotifyOrderCreated(ctx context.Context, event OrdersCreatedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal notify payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.BaseURL+"/v1/notify", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create notify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("send notify request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("notify failed: unexpected status %d", resp.StatusCode)
	}
	return nil
}
