package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	TOPIC      = "promtheus-alerts"
	PATH_QUERY = "/api/v1/query?query=ALERTS%7Balertstate%3D%22firing%22%7D"
)

// flags
var (
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	project  = flag.String("project", "skia-public", "The GCE project name.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	promHost = flag.String("prom_host", "http://promtheus-0:9090", "The domain name and port to reach the prometheus query api.")
	period   = flag.Duration("period", 15*time.Second, "How often to query for alerts.")
	location = flag.String("location", "skia-public", "The name where this promethes server is running.")
)

type Result struct {
	Metric map[string]string `json:"metric"`
}

type Data struct {
	ResultType string   `json:"result_type"`
	Results    []Result `json:"results"`
}

type QueryResponse struct {
	Status string `json:"status"`
	Data   Data   `json:"data"`
}

func singleStep(ctx context.Context, client *http.Client, topic *pubsub.Topic) error {
	resp, err := client.Get(*promHost + PATH_QUERY)
	if err != nil {
		return fmt.Errorf("Failed to request alerts from %q: %s", *promHost, err)
	}
	defer util.Close(resp.Body)
	var queryResponse QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResponse); err != nil {
		return fmt.Errorf("Failed to decode promethes query response: %s", err)
	}
	if queryResponse.Status != "success" {
		return fmt.Errorf("Query was not successful")
	}
	for _, r := range queryResponse.Data.Results {
		b, err := json.Marshal(r.Metric)
		if err != nil {
			sklog.Errorf("Failed to encode message Data: %s: %#v", err, r.Metric)
			continue
		}
		msg := &pubsub.Message{
			Data: b,
			Attributes: map[string]string{
				"location": *location,
			},
		}
		res := topic.Publish(ctx, msg)
		// TODO(jcgregorio) Add metrics for success/fail and total msgs.
		if _, err := res.Get(ctx); err != nil {
			sklog.Errorf("Failed to send message: %s", err)
		}
	}
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
	topic := client.Topic(TOPIC)
	exists, err := topic.Exists(ctx)
	if err != nil {
		sklog.Fatal(err)
	}
	if !exists {
		topic, err := client.CreateTopic(ctx, TOPIC)
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
			sklog.Errorf("Failed first step: %s", err)
		}
	}

	// Loop on time
	// Request all alerts from prom instance
	// For each alert send a pubsub msg.
}
