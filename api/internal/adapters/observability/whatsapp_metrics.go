package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// WhatsApp Cloud API integration metrics. Exposed via the same
// /metrics scrape endpoint as the HTTP middleware. Labels are kept
// low-cardinality (phone_number_id only — never include wamid /
// content / customer phone) so Prometheus doesn't run away.

var (
	WhatsAppInboundTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "whatsapp_inbound_total",
			Help: "Total inbound WhatsApp messages parsed by the webhook handler, labelled by message type.",
		},
		[]string{"phone_number_id", "type"},
	)

	WhatsAppOutboundTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "whatsapp_outbound_total",
			Help: "Total outbound WhatsApp send attempts, labelled by type and outcome (success / failure).",
		},
		[]string{"phone_number_id", "type", "outcome"},
	)

	WhatsAppSendDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "whatsapp_send_duration_seconds",
			Help:    "Latency of POST /messages calls to Meta, labelled by message type.",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 20},
		},
		[]string{"type"},
	)

	WhatsAppStatusUpdates = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "whatsapp_status_updates_total",
			Help: "Cloud API delivery status webhooks received, labelled by status (sent/delivered/read/failed).",
		},
		[]string{"phone_number_id", "status"},
	)

	WhatsAppMetaErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "whatsapp_meta_errors_total",
			Help: "Categorized Meta error responses, labelled by Kind() (rate-limited, token-expired, template-paused, ...).",
		},
		[]string{"kind", "code"},
	)

	WhatsAppOutboxPending = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "whatsapp_outbox_pending",
			Help: "Current count of wa_webhook_events rows in pending state. Sustained non-zero means the worker is falling behind.",
		},
	)

	WhatsAppAsyncInflight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "whatsapp_async_inflight",
			Help: "Current count of in-flight processAIAsync goroutines. Used to detect saturation and to drive the shutdown drain.",
		},
	)

	WhatsAppWebhookRejected = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "whatsapp_webhook_rejected_total",
			Help: "Webhook deliveries we refused before processing, labelled by reason (signature/replay/oversize/missing-secret).",
		},
		[]string{"reason"},
	)
)
