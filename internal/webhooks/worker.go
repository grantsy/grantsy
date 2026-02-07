package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-http-utils/headers"
	"github.com/google/uuid"
	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"

	"github.com/grantsy/grantsy/internal/infra/config"
	"github.com/grantsy/grantsy/internal/infra/metrics"
)

// Worker processes webhook jobs and sends them to endpoints
type Worker struct {
	endpoints []config.WebhookEndpoint
	client    *http.Client
}

// NewWorker creates a new webhook worker
func NewWorker(endpoints []config.WebhookEndpoint) *Worker {
	return &Worker{
		endpoints: endpoints,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Handle processes a webhook job from the queue
func (w *Worker) Handle(ctx context.Context, body []byte) error {
	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	endpoint := w.findEndpoint(payload.Endpoint)
	if endpoint == nil {
		slog.Warn("endpoint not found in config, skipping", "url", payload.Endpoint)
		return nil
	}

	return w.send(ctx, *endpoint, body)
}

func (w *Worker) findEndpoint(url string) *config.WebhookEndpoint {
	for _, ep := range w.endpoints {
		if ep.URL == url {
			return &ep
		}
	}
	return nil
}

func (w *Worker) send(
	ctx context.Context,
	endpoint config.WebhookEndpoint,
	body []byte,
) error {
	wh, err := standardwebhooks.NewWebhook(endpoint.Secret)
	if err != nil {
		return fmt.Errorf("failed to create webhook signer: %w", err)
	}

	msgID := uuid.New().String()
	ts := time.Now()
	signature, err := wh.Sign(msgID, ts, body)
	if err != nil {
		return fmt.Errorf("failed to sign webhook: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint.URL,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set(headers.ContentType, "application/json")
	req.Header.Set("webhook-id", msgID)
	req.Header.Set("webhook-timestamp", fmt.Sprint(ts.Unix()))
	req.Header.Set("webhook-signature", signature)

	start := time.Now()
	resp, err := w.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		metrics.RecordWebhookDelivery(endpoint.URL, false, duration)
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	metrics.RecordWebhookDelivery(endpoint.URL, success, duration)

	if success {
		slog.Debug(
			"webhook sent successfully",
			"url",
			endpoint.URL,
			"status",
			resp.StatusCode,
		)
		return nil
	}

	return fmt.Errorf("webhook failed: status %d", resp.StatusCode)
}
