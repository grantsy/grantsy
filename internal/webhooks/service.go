package webhooks

import (
	"context"
	"encoding/json"

	"github.com/iamolegga/goqite"
	"github.com/iamolegga/goqite/jobs"

	"github.com/grantsy/grantsy/internal/infra/config"
	"github.com/grantsy/grantsy/internal/infra/metrics"
)

// Service handles queueing webhook notifications
type Service struct {
	queue     *goqite.Queue
	endpoints []config.WebhookEndpoint
}

// NewService creates a new webhook service
func NewService(queue *goqite.Queue, endpoints []config.WebhookEndpoint) *Service {
	return &Service{queue: queue, endpoints: endpoints}
}

// NotifyPlanUpdated queues a webhook notification for a plan update.
// One message is enqueued per endpoint for independent retry handling.
func (s *Service) NotifyPlanUpdated(
	ctx context.Context,
	userID, activePlan, prevPlan string,
	subscription any,
) error {
	for _, endpoint := range s.endpoints {
		payload := Payload{
			Endpoint:   endpoint.URL,
			UserID:     userID,
			ActivePlan: activePlan,
			Meta: Meta{
				PrevPlan:     prevPlan,
				Subscription: subscription,
			},
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		if _, err := jobs.Create(ctx, s.queue, "webhooks", goqite.Message{Body: body}); err != nil {
			return err
		}

		metrics.RecordWebhookQueued(endpoint.URL)
	}

	return nil
}
