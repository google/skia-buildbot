// continuous-deploy monitors pubsub events for the GCP Container Builder and
// pushes updated images when successful images are built.
package main

import (
	"context"
	"encoding/json"
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
	cloudbuild "google.golang.org/api/cloudbuild/v1"
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
		"continuous-deploy",
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
		_, err := gitauth.New(ts, "/tmp/git-cookie", true)
		if err != nil {
			sklog.Fatal(err)
		}
	}
	client, err := pubsub.NewClient(ctx, *project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}
	topic := client.Topic("cloud-builds")
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	subName := fmt.Sprintf("continuous-deploy-%s", hostname)
	sub := client.Subscription(subName)
	ok, err := sub.Exists(ctx)
	if err != nil {
		sklog.Fatalf("Failed checking subscription existence: %s", err)
	}
	if !ok {
		sub, err = client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
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
		err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			msg.Ack()
			sklog.Infof("msg.Data: %s", string(msg.Data))
			if msg.Attributes["status"] == "SUCCESS" {
				var buildInfo cloudbuild.Build
				if err := json.Unmarshal(msg.Data, &buildInfo); err != nil {
					sklog.Errorf("Failed to decode: %s: %q", err, string(msg.Data))
					return
				}
				imageName := buildInfo.Results.Images[0].Name
				// Is this one of the images we are pushing?
				found := false
				for _, name := range flag.Args() {
					if strings.Contains(imageName, name) {
						found = true
						break
					}
				}
				if !found {
					return
				}
				cmd := fmt.Sprintf("%s --logtostderr %s", pushk, imageName)
				sklog.Infof("About to execute: %q", cmd)
				output, err := exec.RunSimple(ctx, cmd)
				if err != nil {
					sklog.Errorf("Failed to run pushk: %s: %s", output, err)
					return
				} else {
					sklog.Info(output)
				}
				sklog.Info("Finished push")
			}
		})
		if err != nil {
			sklog.Errorf("Failed receiving pubsub message: %s", err)
		}
	}
}
