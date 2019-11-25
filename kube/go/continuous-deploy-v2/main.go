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
	"strings"
	"time"

	firestore_api "cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/continuous_deploy"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Flags
var (
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	project  = flag.String("project", "skia-public", "The GCE project name.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")

	tagProdImages = common.NewMultiStringFlag("tag_prod_image", nil, "Docker image that the continuous deploy app should tag as 'prod' if it is newer than the last hash tagged as 'prod'.")
	deployImages  = common.NewMultiStringFlag("deploy_image", nil, "Docker image that the continuous deploy app should deploy when it's docker image is built, if it is newer than the last encountered hash.")

	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'continuous-deploy-v2'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
)

var (
	parseImageName *regexp.Regexp

	// Binaries.
	pushk = "/usr/local/bin/pushk"
	// TODO(rmistry): Make this absolute path
	docker = "docker"
)

const (
	// For accessing Firestore.
	DEFAULT_ATTEMPTS      = 3
	PUT_SINGLE_TIMEOUT    = 10 * time.Second
	DELETE_SINGLE_TIMEOUT = 10 * time.Second
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

func addDockerProdTag(ctx context.Context, buildInfo continuous_deploy.BuildInfo) error {
	pullCmd := fmt.Sprintf("docker pull %s:%s", buildInfo.ImageName, buildInfo.Tag)
	if _, pullErr := exec.RunSimple(ctx, pullCmd); pullErr != nil {
		return fmt.Errorf("Error running docker pull: %s", pullErr)
	}

	tagCmd := fmt.Sprintf("docker tag %s %s:%s", "gcr.io/skia-public/infra-v2:test5", "gcr.io/skia-public/infra-v2", "test6")
	if _, tagErr := exec.RunSimple(ctx, tagCmd); tagErr != nil {
		return fmt.Errorf("Error running docker tag: %s", tagErr)
	}

	pushCmd := fmt.Sprintf("docker push %s:%s", "gcr.io/skia-public/infra-v2", "test6")
	if _, pushErr := exec.RunSimple(ctx, pushCmd); pushErr != nil {
		return fmt.Errorf("Error running docker push: %s", pushErr)
	}
	return nil
}

// TODO(rmistry): Add MUTEX for TAG PROD TO IMAGE.
// TODO(rmistry): Add MUTEX for DEPLOY IMAGE TO PROD.

// tagProdToImage adds the "prod" tag to docker image if:
// * It's commit hash is newer than the entry in Firestore.
// * There is no entry in Firestore and it is the first time we have seen this image.
func tagProdToImage(ctx context.Context, fsClient *firestore.Client, gitRepo *gitiles.Repo, buildInfo continuous_deploy.BuildInfo) error {

	// Query firstore for this image.
	col := fsClient.Collection(buildInfo.ImageName)
	id := buildInfo.ImageName
	var fromDB continuous_deploy.BuildInfo
	docs := []*firestore_api.DocumentSnapshot{}
	iter := col.Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return skerr.Wrap(err)
		}
		docs = append(docs, doc)
	}

	if len(docs) > 1 {
		return fmt.Errorf("For %s found %d entries in firestore", buildInfo.ImageName, len(docs))
	} else if len(docs) == 0 {
		// First time we have seen this image. Add it to firestore.
		if _, createErr := fsClient.Create(ctx, col.Doc(id), buildInfo, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT); createErr != nil {
			return skerr.Wrap(createErr)
		}
		sklog.Info("Going to apply the prod tag to %s:%s", buildInfo.ImageName, buildInfo.Tag)
		if err := addDockerProdTag(ctx, buildInfo); err != nil {
			return skerr.Wrap(err)
		}
	} else {
		// There is an existing prod hash for this image. See if the received image is newer.
		if err := docs[0].DataTo(&fromDB); err != nil {
			return skerr.Wrap(err)
		}
		log, err := gitRepo.LogLinear(ctx, fromDB.Tag, buildInfo.Tag)
		if err != nil {
			return fmt.Errorf("Could not query gitiles of %s: %s", common.REPO_SKIA, err)
		}
		if len(log) > 0 {
			// The image is newer. Replace entry in firestore.
			if _, deleteErr := fsClient.Delete(ctx, col.Doc(id), DEFAULT_ATTEMPTS, DELETE_SINGLE_TIMEOUT); deleteErr != nil {
				return fmt.Errorf("Could not delete %s in firestore: %s", buildInfo.ImageName, deleteErr)
			}
			if _, createErr := fsClient.Create(ctx, col.Doc(id), buildInfo, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT); createErr != nil {
				return skerr.Wrap(err)
			}
		}
		sklog.Info("%s is newer than %s for %s. Going to apply the tag", buildInfo.Tag, fromDB.Tag, buildInfo.ImageName)
		if err := addDockerProdTag(ctx, buildInfo); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// deployImage deploys the specified app using pushk.
func deployImage(ctx context.Context, appName string) error {
	// TODO(rmistry): Remove --dry-run from the below!
	pushCmd := fmt.Sprintf("%s --logtostderr --dry-run %s", pushk, appName)
	sklog.Infof("About to execute: %q", pushCmd)
	output, err := exec.RunSimple(ctx, pushCmd)
	if err != nil {
		return fmt.Errorf("Failed to run pushk: %s: %s", output, err)
	} else {
		sklog.Info(output)
	}
	return nil
}

func main() {
	common.InitWithMust(
		"continuous-deploy-v2",
		common.PrometheusOpt(promPort),
	)

	if *deployImages == nil && *tagProdImages == nil {
		sklog.Fatal("Must pass in atleast one of --tag-prod_image and --deploy_image")
	}
	if *fsNamespace == "" {
		sklog.Fatalf("--fs_namespace must be set")
	}

	Init()
	ctx := context.Background()

	if *local {
		pushk = "pushk"
		docker = "docker"
	}

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

	// Instantiate firestore.
	fsClient, err := firestore.NewClient(ctx, *fsProjectID, "continuous-deploy-v2", *fsNamespace, ts)
	if err != nil {
		sklog.Fatalf("Could not init firestore: %s", err)
	}

	// Instantiate gitrepo.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	gitRepo := gitiles.NewRepo(common.REPO_SKIA, httpClient)

	for {
		err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			msg.Ack()
			sklog.Infof("Status: %s", msg.Attributes["status"])

			var buildInfo continuous_deploy.BuildInfo
			if err := json.Unmarshal(msg.Data, &buildInfo); err != nil {
				sklog.Errorf("Failed to decode: %s: %q", err, string(msg.Data))
				return
			}

			// Extract the image name and tag.
			imageName := buildInfo.ImageName
			tag := buildInfo.Tag

			// Commit tags contain the commit hash. Trybot tags are are this format: ${CHANGE_NUM}/${PATCHSET_NUM}.
			// Ignore trybot tags and only apply prod tag to commit hashes.
			if strings.Index(tag, "_") != -1 {
				sklog.Infof("Found a trybot tag %s for %s. Ignoring.", tag, imageName)
				return
			}

			// See if the image is in the whitelist of images to be tagged.
			if util.In(baseImageName(imageName), *tagProdImages) {
				if err := tagProdToImage(ctx, fsClient, gitRepo, buildInfo); err != nil {
					sklog.Errorf("Failed to add the prod tag to %s", buildInfo)
					return
				}
			} else {
				sklog.Infof("Not going to tag %s with prod", buildInfo)
			}

			// See if the image is in the whitelist of images to be deployed by pushk.
			if util.In(baseImageName(imageName), *deployImages) {
				if err := deployImage(ctx, baseImageName(imageName)); err != nil {
					sklog.Errorf("Failed to deploy %s", buildInfo)
					return
				}
			} else {
				sklog.Infof("Not going to deploy %s", buildInfo)
			}

			fmt.Println("Done processing %s", buildInfo)
		})
		if err != nil {
			sklog.Errorf("Failed receiving pubsub message: %s", err)
		}
	}
}
