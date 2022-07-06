// alert-to-pubsub accepts POST messages from Prometheus for alerts and publishes them to Google PubSub.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/alerts"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// flags
var (
	location = flag.String("location", "skia-public", "The name where this prometheus server is running.")
	period   = flag.Duration("period", 15*time.Second, "How often to query for alerts.")
	port     = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	project  = flag.String("project", "skia-public", "The GCE project name.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	simulate = flag.Bool("simulate", false, "Send simulated alerts as opposed to polling Prometheus.")
)

// metrics
var (
	failureCounter = metrics2.GetCounter("pubsub_send_failure", nil)
	successCounter = metrics2.GetCounter("pubsub_send_success", nil)
	liveness       = metrics2.NewLiveness("alert_to_pubsub_incoming_alerts")
)

var (
	// Version can be changed via -ldflags.
	Version = "kubernetes"
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

	stateFromResolved = map[bool]string{
		true:  alerts.STATE_RESOLVED,
		false: alerts.STATE_ACTIVE,
	}
)

const (
	LINK_TO_SOURCE = "link_to_source"
)

// Optimally we would just use the prometheus code itself for the definition of
// Alert, but their code includes a different version of opencensus initialized
// in an init() call somewhere in their code.

// Alert is a generic representation of an alert in the Prometheus eco-system.
type Alert struct {
	// Label value pairs for purpose of aggregation, matching, and disposition
	// dispatching. This must minimally include an "alertname" label.
	Labels map[string]string `json:"labels"`

	// Extra key/value information which does not define alert identity.
	Annotations map[string]string `json:"annotations"`

	// The known time range for this alert. Both ends are optional.
	StartsAt     time.Time `json:"startsAt,omitempty"`
	EndsAt       time.Time `json:"endsAt,omitempty"`
	GeneratorURL string    `json:"generatorURL,omitempty"`
}

// Resolved returns true if the activity interval ended in the past.
func (a *Alert) Resolved() bool {
	resolved := a.ResolvedAt(time.Now())
	if resolved {
		sklog.Warningf("This received alert is already resolved: %+v", a)
	}
	return resolved
}

// ResolvedAt returns true if the activity interval ended before
// the given timestamp.
func (a *Alert) ResolvedAt(ts time.Time) bool {
	if a.EndsAt.IsZero() {
		return false
	}
	return !a.EndsAt.After(ts)
}

func sendPubSub(ctx context.Context, m map[string]string, topic *pubsub.Topic) {
	m[alerts.LOCATION] = *location
	b, err := json.Marshal(m)
	if err != nil {
		sklog.Errorf("Failed to encode message Data: %s: %#v", err, m)
		return
	}
	msg := &pubsub.Message{
		Data: b,
	}
	res := topic.Publish(ctx, msg)
	if _, err := res.Get(ctx); err != nil {
		failureCounter.Inc(1)
		sklog.Errorf("Failed to send message %+v: %s", msg, err)
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
	liveness.Reset()
	var incomingAlerts []Alert
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httputils.ReportError(w, err, "Failed to read JSON.", http.StatusInternalServerError)
		return
	}
	defer util.Close(r.Body)
	if err := json.Unmarshal(b, &incomingAlerts); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	sklog.Infof("Received %d incomingAlerts.", len(incomingAlerts))
	for _, alert := range incomingAlerts {
		m := map[string]string{
			alerts.STATE:   stateFromResolved[alert.Resolved()],
			LINK_TO_SOURCE: alert.GeneratorURL,
		}
		for k, v := range alert.Labels {
			m[k] = v
		}
		for k, v := range alert.Annotations {
			m[k] = v
		}
		// Do not use r.Context() here because we could run into "context deadline exceeded".
		sendPubSub(context.Background(), m, s.topic)
	}
}

func main() {
	common.InitWithMust(
		"alert-to-pubsub",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	sklog.Infof("Version: %s", Version)

	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, pubsub.ScopePubSub)
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
		for range time.Tick(*period) {
			if err := singleStep(ctx, httpClient, topic); err != nil {
				sklog.Errorf("Failed step: %s", err)
			}
		}
	}()

	server := NewServer(topic)

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/alerts", server.alertHandler)
	h := httputils.LoggingRequestResponse(r)
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
