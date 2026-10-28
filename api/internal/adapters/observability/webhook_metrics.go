package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// WebhookFailuresTotal counts inbound webhook deliveries that could not be
// processed (US-067), labelled by channel (whatsapp/telegram/instagram/
// unknown) and a low-cardinality reason. This is the cross-channel
// complement to the WhatsApp-specific whatsapp_webhook_rejected_total —
// it covers the generic inbound handler (channel resolution, secret
// validation, payload parsing and persistence). Reasons are fixed enum
// strings so the series count stays bounded.
var WebhookFailuresTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "webhook_failures_total",
		Help: "Inbound webhook deliveries that failed processing, labelled by channel and reason.",
	},
	[]string{"channel", "reason"},
)
