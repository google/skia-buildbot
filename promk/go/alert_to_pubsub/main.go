package main

import (
	"context"
	"flag"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

// flags
var (
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	project  = flag.String("project", "skia-public", "The GCE project name.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

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
	topic := client.Topic("prometheus-alerts")
	exists, err := topic.Exists(ctx)
	if err != nil {
		sklog.Fatal(err)
	}
	if !exists {
		topic, err := client.CreateTopic(ctx, "prometheus-alerts")
		if err != nil {
			sklog.Fatal(err)
		}
	}
	// Loop on time
	// Request all alerts from prom instance
	// For each alert send a pubsub msg.
	topic.Publish(ctx, msg)
}
