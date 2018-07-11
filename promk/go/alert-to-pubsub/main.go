// alert-to-pubsub polls Prometheus for alerts and publishes them to Google PubSub.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/alerts"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

// flags
var (
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	simulate       = flag.Bool("simulate", false, "Send simulated alerts as opposed to polling Prometheus.")
	project        = flag.String("project", "skia-public", "The GCE project name.")
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	period         = flag.Duration("period", 15*time.Second, "How often to query for alerts.")
	location       = flag.String("location", "skia-public", "The name where this promethes server is running.")
	successCounter = metrics2.GetCounter("pubsub_send_success", nil)
	failureCounter = metrics2.GetCounter("pubsub_send_failure", nil)
)

var (
	sim1 = map[string]string{
		"__name__":   "ALERTS",
		"alertname":  "BotUnemployed",
		"alertstate": "firing",
		"bot":        "skia-rpi-064",
		"category":   "infra",
		"instance":   "skia-datahopper2:20000",
		"job":        "datahopper",
		"pool":       "Skia",
		"severity":   "critical",
		"swarming":   "chromium-swarm.appspot.com",
	}
	sim2 = map[string]string{
		"__name__":   "ALERTS",
		"alertname":  "BotMissing",
		"alertstate": "firing",
		"bot":        "skia-rpi-064",
		"category":   "infra",
		"instance":   "skia-datahopper2:20000",
		"job":        "datahopper",
		"pool":       "Skia",
		"severity":   "critical",
		"swarming":   "chromium-swarm.appspot.com",
	}
)

// The below is mostly copied from the Prometheus source code because
// we have conflict in tracing if we try to use the prometheus definitions
// directly. Yay init().

// Label is a key/value pair of strings.
type Label struct {
	Name, Value string
}

// Labels is a sorted set of labels. Order has to be guaranteed upon
// instantiation.
type Labels []Label

// Alert is a generic representation of an alert in the Prometheus eco-system.
type Alert struct {
	// Label value pairs for purpose of aggregation, matching, and disposition
	// dispatching. This must minimally include an "alertname" label.
	Labels Labels `json:"labels"`

	// Extra key/value information which does not define alert identity.
	Annotations Labels `json:"annotations"`

	// The known time range for this alert. Both ends are optional.
	StartsAt     time.Time `json:"startsAt,omitempty"`
	EndsAt       time.Time `json:"endsAt,omitempty"`
	GeneratorURL string    `json:"generatorURL,omitempty"`
}

// Resolved returns true iff the activity interval ended in the past.
func (a *Alert) Resolved() bool {
	return a.ResolvedAt(time.Now())
}

// ResolvedAt returns true off the activity interval ended before
// the given timestamp.
func (a *Alert) ResolvedAt(ts time.Time) bool {
	if a.EndsAt.IsZero() {
		return false
	}
	return !a.EndsAt.After(ts)
}

func sendPubSub(ctx context.Context, m map[string]string, topic *pubsub.Topic) {
	b, err := json.Marshal(m)
	if err != nil {
		sklog.Errorf("Failed to encode message Data: %s: %#v", err, m)
		return
	}

	m[alerts.LOCATION] = *location
	msg := &pubsub.Message{
		Data: b,
	}
	res := topic.Publish(ctx, msg)
	if _, err := res.Get(ctx); err != nil {
		failureCounter.Inc(1)
		sklog.Errorf("Failed to send message: %s", err)
	} else {
		successCounter.Inc(1)
	}
}

func singleStep(ctx context.Context, client *http.Client, topic *pubsub.Topic) error {
	if *simulate {
		sendPubSub(ctx, sim1, topic)
		sendPubSub(ctx, sim2, topic)
	}

	// Send a healthz alert to be used as a marker that pubsub is still working.
	m := map[string]string{
		"__name__": "HEALTHZ",
	}
	sklog.Info("healthz")
	sendPubSub(ctx, m, topic)

	return nil
}

type Server struct {
	topic *pubsub.Topic
}

func NewServer(topic *pubsub.Topic) *Server {
	return &Server{
		topic: topic,
	}
}

func (s *Server) alertHandler(w http.ResponseWriter, r *http.Request) {
	var alert Alert
	if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode JSON.")
		return
	}
	if alert.Resolved() {
		return
	}
	m := map[string]string{}
	for _, l := range alert.Labels {
		m[l.Name] = l.Value
	}
	for _, l := range alert.Annotations {
		m[l.Name] = l.Value
	}
	sendPubSub(r.Context(), m, s.topic)
}

func main() {
	common.InitWithMust(
		"alert-to-pubsub",
		common.PrometheusOpt(promPort),
	)
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, pubsub.ScopePubSub)
	if err != nil {
		sklog.Fatal(err)
	}
	client, err := pubsub.NewClient(ctx, *project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}
	topic := client.Topic(alerts.TOPIC)
	exists, err := topic.Exists(ctx)
	if err != nil {
		sklog.Fatal(err)
	}
	if !exists {
		topic, err = client.CreateTopic(ctx, alerts.TOPIC)
		if err != nil {
			sklog.Fatal(err)
		}
	}
	httpClient := httputils.NewTimeoutClient()
	go func() {
		for _ = range time.Tick(*period) {
			if err := singleStep(ctx, httpClient, topic); err != nil {
				sklog.Errorf("Failed step: %s", err)
			}
		}
	}()

	server := NewServer(topic)

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/alerts", server.alertHandler)
	http.Handle("/", httputils.LoggingRequestResponse(r))
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
