package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "grantsy"

var (
	// HTTP metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status_code"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path", "status_code"},
	)

	// Entitlement checks
	entitlementChecksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "entitlement_checks_total",
			Help:      "Total number of entitlement checks",
		},
		[]string{"feature", "result"},
	)

	// Subscription state
	activeSubscriptions = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_subscriptions",
			Help:      "Number of active subscriptions by plan",
		},
		[]string{"plan_id"},
	)

	// Webhook metrics
	webhookMessagesQueued = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "webhook_messages_queued_total",
			Help:      "Total webhook messages queued for delivery",
		},
		[]string{"endpoint"},
	)

	webhookDeliveryAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "webhook_delivery_attempts_total",
			Help:      "Total webhook delivery attempts",
		},
		[]string{"endpoint", "status"},
	)

	webhookDeliveryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "webhook_delivery_duration_seconds",
			Help:      "Webhook delivery duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	registry *prometheus.Registry
)

// Init initializes the metrics registry and returns the handler.
// If goMetrics is true, Go runtime metrics are included.
func Init(goMetrics bool) http.Handler {
	registry = prometheus.NewRegistry()

	registry.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		entitlementChecksTotal,
		activeSubscriptions,
		webhookMessagesQueued,
		webhookDeliveryAttempts,
		webhookDeliveryDuration,
	)

	if goMetrics {
		registry.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		)
	}

	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		Registry: registry,
	})
}

// recordHTTPRequest records an HTTP request metric.
func recordHTTPRequest(method, path, statusCode string) {
	httpRequestsTotal.WithLabelValues(method, path, statusCode).Inc()
}

// recordHTTPDuration records an HTTP request duration metric.
func recordHTTPDuration(method, path, statusCode string, duration float64) {
	httpRequestDuration.WithLabelValues(method, path, statusCode).Observe(duration)
}

// RecordEntitlementCheck records an entitlement check result.
func RecordEntitlementCheck(feature string, allowed bool) {
	result := "denied"
	if allowed {
		result = "allowed"
	}
	entitlementChecksTotal.WithLabelValues(feature, result).Inc()
}

// SetActiveSubscriptions sets the gauge for active subscriptions by plan.
func SetActiveSubscriptions(planID string, count float64) {
	activeSubscriptions.WithLabelValues(planID).Set(count)
}

// RecordWebhookQueued records a webhook message being queued.
func RecordWebhookQueued(endpoint string) {
	webhookMessagesQueued.WithLabelValues(endpoint).Inc()
}

// RecordWebhookDelivery records a webhook delivery attempt.
func RecordWebhookDelivery(endpoint string, success bool, duration time.Duration) {
	status := "failure"
	if success {
		status = "success"
	}
	webhookDeliveryAttempts.WithLabelValues(endpoint, status).Inc()
	webhookDeliveryDuration.WithLabelValues(endpoint).Observe(duration.Seconds())
}
