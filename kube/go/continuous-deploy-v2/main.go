// continuous-deploy-v2 monitors pubsub events for the GCP Container Builder and
// pushes updated images when successful images are built.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	//"strings"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/continuous_deploy"
	//"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitauth"
	//"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	//cloudbuild "google.golang.org/api/cloudbuild/v1"
	"google.golang.org/api/option"
)

// flags
var (
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	project  = flag.String("project", "skia-public", "The GCE project name.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")

	deployImages  = common.NewMultiStringFlag("deploy_image", nil, "Docker image that the continuous deploy app should deploy when it's docker image is built, if it is newer than the last encountered hash.")
	tagProdImages = common.NewMultiStringFlag("tag_prod_image", nil, "Docker image that the continuous deploy app should tag as 'prod' if it is newer than the last encountered hash.")
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

// TODO(rmistry): Update this doc
// imagesFromInfo parses the incoming PubSub Data 'b' as JSON and then returns
// the full image names of all the images that match 'shortImageNames'.
func imagesFromInfo(shortImageNames []string, buildInfo continuous_deploy.BuildInfo) []string {
	imageNames := []string{}
	sklog.Infof("ImageName: %s", buildInfo.ImageName)
	// Is this one of the images we are pushing?
	for _, name := range shortImageNames {
		if baseImageName(buildInfo.ImageName) == name {
			imageNames = append(imageNames, buildInfo.ImageName)
			break
		}
	}
	return imageNames
}

func main() {
	common.InitWithMust(
		"continuous-deploy-v2",
		common.PrometheusOpt(promPort),
	)

	if *deployImages == nil && *tagProdImages == nil {
		sklog.Fatal("Must pass in atleast one of --deploy_image and --tag_prod_image")
	}

	Init()
	//sklog.Infof("Pushing to: %v", flag.Args())
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
	topic := client.Topic(continuous_deploy.TOPIC)
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	subName := fmt.Sprintf("%s-%s", continuous_deploy.TOPIC, hostname)
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
	fmt.Println(pushk)

	//pubSubReceive := metrics2.NewLiveness("ci_pubsub_receive_v2", nil)
	for {
		err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			fmt.Println("GOT SOMETHING!!!!!!")
			// Check here to see what the shortImage name is.
			// If it is skia-release-v2 or skia-wasm-release-v2 or skia-infra-v2 then do the prod thingy.
			// For everything do the pushk thingy.

			msg.Ack()
			sklog.Infof("Status: %s", msg.Attributes["status"])

			var buildInfo continuous_deploy.BuildInfo
			if err := json.Unmarshal(msg.Data, &buildInfo); err != nil {
				sklog.Errorf("Failed to decode: %s: %q", err, string(msg.Data))
				return
			}
			//repoName := "--unknown--"
			//if buildInfo.Source != nil && buildInfo.Source.RepoSource != nil {
			//	repoName = buildInfo.Source.RepoSource.RepoName
			//}

			// No need because it will be a bot visible on the tree.
			//// Record build failures so we can alert on them.
			//counter := metrics2.GetCounter("ci_build_failure_v2", map[string]string{"image_name": buildInfo.ImageName})
			//if msg.Attributes["status"] == "FAILURE" {
			//	counter.Inc(1)
			//} else if msg.Attributes["status"] == "SUCCESS" {
			//	counter.Reset()
			//}

			//if msg.Attributes["status"] != "SUCCESS" {
			//	return
			//}
			imageNames := imagesFromInfo(*deployImages, buildInfo)
			if err != nil {
				sklog.Error(err)
				return
			}
			if len(imageNames) == 0 {
				sklog.Infof("No images to push.")
				return
			}
			// UNCOMMENT THE BELOW TO ACTUALLY PUSH TO SOMETHING!!!!!!
			//cmd := fmt.Sprintf("%s --logtostderr %s", pushk, strings.Join(imageNames, " "))
			//sklog.Infof("About to execute: %q", cmd)
			//output, err := exec.RunSimple(ctx, cmd)
			//pushFailure := metrics2.GetCounter("ci_push_failure_v2", map[string]string{"image_name": buildInfo.ImageName})
			//if err != nil {
			//	sklog.Errorf("Failed to run pushk: %s: %s", output, err)
			//	pushFailure.Inc(1)
			//	return
			//} else {
			//	sklog.Info(output)
			//}
			//pushFailure.Reset()
			//pubSubReceive.Reset()
			sklog.Info("Finished push")

			fmt.Println("DONE DONE DONE")
		})
		if err != nil {
			sklog.Errorf("Failed receiving pubsub message: %s", err)
		}
	}
}
