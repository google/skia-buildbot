package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitauth"
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
	common.InitWithMust("continuous-deploy",
		common.PrometheusOpt(promPort),
	)
	if len(flag.Args()) == 0 {
		sklog.Fatal("continuous-deploy must be passed in at least one package name to push.")
	}
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, pubsub.ScopePubSub)
	if err != nil {
		sklog.Fatal(err)
	}
	if !*local {
		gitauth.New(ts, "/tmp/git-cookie", true)
	}
	client, err := pubsub.NewClient(ctx, *project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}
	topic := client.Topic("gcr")
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	sub := client.Subscription(hostname)
	ok, err := sub.Exists(ctx)
	if err != nil {
		sklog.Fatalf("Failed checking subscription existence: %s", err)
	}
	if !ok {
		sub, err = client.CreateSubscription(ctx, hostname, pubsub.SubscriptionConfig{
			Topic: topic,
		})
		if err != nil {
			sklog.Fatalf("Failed creating subscription: %s", err)
		}
	}
	pushk := "/usr/local/bin/pushk"
	if *local {
		pushk = "pushk"
	}
	for {
		sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			sklog.Infof("Message Received %v", *msg)
			sklog.Infof("Attributes: %v", msg.Attributes)
			msg.Ack()
			cmd := fmt.Sprintf("%s --any_tag --logtostderr %s", pushk, strings.Join(flag.Args(), " "))
			sklog.Infof("About to execute: %q", cmd)
			output, err := exec.RunSimple(ctx, cmd)
			if err != nil {
				sklog.Errorf("Failed to run pushk: %s: %s", output, err)
				return
			}
			sklog.Info("Finished push")
		})
	}
}

// Subscribe to pubsub notifications of the builders.
// On event run

// pushk [list of things to push]

// Needs r/w access to the skia-public-config repo to record updates.
// Needs flag with list of images to push.
// Needs to be able to be paused.
