// alert-to-pubsub polls Prometheus for alerts and publishes them to Google PubSub.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/alerts"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	// PROM_QUERY is the path and query of the request we want to make to Prometheus.
	PROM_QUERY = "/api/v1/query?query=ALERTS%7Balertstate%3D%22firing%22%7D"
)

// flags
var (
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	project        = flag.String("project", "skia-public", "The GCE project name.")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	promHost       = flag.String("prom_host", "http://promtheus-0:9090", "The domain name and port to reach the prometheus query api.")
	period         = flag.Duration("period", 15*time.Second, "How often to query for alerts.")
	location       = flag.String("location", "skia-public", "The name where this promethes server is running.")
	successCounter = metrics2.GetCounter("pubsub_send_success", nil)
	failureCounter = metrics2.GetCounter("pubsub_send_failure", nil)
)

func sendPubSub(ctx context.Context, m map[string]string, topic *pubsub.Topic) {
	b, err := json.Marshal(m)
	if err != nil {
		sklog.Errorf("Failed to encode message Data: %s: %#v", err, m)
		return
	}
	msg := &pubsub.Message{
		Data: b,
		Attributes: map[string]string{
			"location": *location,
		},
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
	// Query for all ALERTS firing on the given Prometheus server.
	resp, err := client.Get(*promHost + PROM_QUERY)
	if err != nil {
		return fmt.Errorf("Failed to request alerts from %q: %s", *promHost, err)
	}
	defer util.Close(resp.Body)
	var queryResponse alerts.QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResponse); err != nil {
		return fmt.Errorf("Failed to decode promethes query response: %s", err)
	}
	if queryResponse.Status != "success" {
		return fmt.Errorf("Query was not successful")
	}
	// Send each alert as a PubSub message.
	for _, r := range queryResponse.Data.Results {
		sendPubSub(ctx, r.Metric, topic)
	}

	// Send a healthz alert to be used as a marker that pubsub is still working.
	m := map[string]string{
		"__name__": "HEALTHZ",
	}
	sklog.Info("healthz")
	sendPubSub(ctx, m, topic)

	return nil
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
	if err := singleStep(ctx, httpClient, topic); err != nil {
		sklog.Fatalf("Failed first step: %s", err)
	}
	for _ = range time.Tick(*period) {
		if err := singleStep(ctx, httpClient, topic); err != nil {
			sklog.Errorf("Failed step: %s", err)
		}
	}
}
