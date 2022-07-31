package alerts

const (
	// TOPIC is the PubSub topic for alert messages.
	TOPIC = "promtheus-alerts"

	// Well known keys and values for Metric.
	TYPE         = "__name__" // The two valid values are ALERTS and HEALTHZ.
	TYPE_ALERTS  = "ALERTS"
	TYPE_HEALTHZ = "HEALTHZ"

	STATE          = "__state__"
	STATE_ACTIVE   = "active"
	STATE_RESOLVED = "resolved"

	// Where the alert came from, e.g. 'skia-public' or 'skolo'.
	LOCATION = "skia_location"
)
