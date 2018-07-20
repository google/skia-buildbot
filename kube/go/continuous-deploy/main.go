// continuous-deploy monitors pubsub events for the GCP Container Builder and
// pushes updated images when successful images are built.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	cloudbuild "google.golang.org/api/cloudbuild/v1"
	"google.golang.org/api/option"
)

// flags
var (
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	project  = flag.String("project", "skia-public", "The GCE project name.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

var (
	parseImageName *regexp.Regexp
)

func Init() {
	parseImageName = regexp.MustCompile("^gcr.io/" + *project + "/([^:]+).*$")
}

// baseImageName returns "fiddler" from "gcr.io/skia-public/fiddler:foo".
//
// If the image name doesn't start with "gcr.io" and the project name then "" is returned.
func baseImageName(s string) string {
	matches := parseImageName.FindStringSubmatch(s)
	if len(matches) != 2 {
		return ""
	} else {
		return matches[1]
	}
}

// imagesFromInfo parses the incoming PubSub Data 'b' as JSON and then returns
// the full image names of all the images that match 'shortImageNames'.
func imagesFromInfo(shortImageNames []string, buildInfo cloudbuild.Build) []string {
	imageNames := []string{}
	for _, im := range buildInfo.Results.Images {
		sklog.Infof("ImageName: %s", im.Name)
		// Is this one of the images we are pushing?
		for _, name := range shortImageNames {
			if baseImageName(im.Name) == name {
				imageNames = append(imageNames, im.Name)
				break
			}
		}
	}
	return imageNames
}

func main() {
	common.InitWithMust(
		"continuous-deploy",
		common.PrometheusOpt(promPort),
	)
	if len(flag.Args()) == 0 {
		sklog.Fatal("continuous-deploy must be passed in at least one package name to push.")
	}
	Init()
	sklog.Infof("Pushing to: %v", flag.Args())
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, pubsub.ScopePubSub, "https://www.googleapis.com/auth/gerritcodereview")
	if err != nil {
		sklog.Fatal(err)
	}
	if !*local {
		_, err := gitauth.New(ts, "/tmp/git-cookie", true, "skia-continuous-deploy@skia-public.iam.gserviceaccount.com")
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
	shortImageNames := flag.Args()

	pubSubReceive := metrics2.NewLiveness("ci_pubsub_receive", nil)
	for {
		err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			msg.Ack()
			sklog.Infof("Status: %s", msg.Attributes["status"])

			var buildInfo cloudbuild.Build
			if err := json.Unmarshal(msg.Data, &buildInfo); err != nil {
				sklog.Errorf("Failed to decode: %s: %q", err, string(msg.Data))
				return
			}

			// Record build failures so we can alert on them.
			if util.In(msg.Attributes["status"], []string{"FAILURE", "SUCCESS"}) {
				failure := 0
				if msg.Attributes["status"] == "FAILURE" {
					failure = 1
				}
				metrics2.GetInt64Metric("ci_build_failure", map[string]string{"trigger": buildInfo.Source.RepoSource.RepoName}).Update(int64(failure))
			}
			if msg.Attributes["status"] != "SUCCESS" {
				return
			}
			imageNames := imagesFromInfo(shortImageNames, buildInfo)
			if err != nil {
				sklog.Error(err)
				return
			}
			if len(imageNames) == 0 {
				sklog.Infof("No images to push.")
				return
			}
			cmd := fmt.Sprintf("%s --logtostderr %s", pushk, strings.Join(imageNames, " "))
			sklog.Infof("About to execute: %q", cmd)
			output, err := exec.RunSimple(ctx, cmd)
			pushFailure := metrics2.GetCounter("ci_push_failure", map[string]string{"trigger": buildInfo.Source.RepoSource.RepoName})
			if err != nil {
				sklog.Errorf("Failed to run pushk: %s: %s", output, err)
				pushFailure.Inc(1)
				return
			} else {
				sklog.Info(output)
			}
			pushFailure.Reset()
			pubSubReceive.Reset()
			sklog.Info("Finished push")
		})
		if err != nil {
			sklog.Errorf("Failed receiving pubsub message: %s", err)
		}
	}
}
