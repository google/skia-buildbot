// docker_pushes_watcher monitors pubsub events for docker pushes and looks at a
// whitelist of image names to do one or more of the following:
// * tag new images with "prod"
// * deploy images using pushk

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	firestore_api "cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	docker_pubsub "go.skia.org/infra/go/docker/build/pubsub"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Flags
var (
	clusterConfig = flag.String("cluster_config", "", "Absolute filename of the config.json file.")
	local         = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	project       = flag.String("project", "skia-public", "The GCE project name.")
	promPort      = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	hang          = flag.Bool("hang", false, "If true, just hang and do nothing.")

	tagProdImages = common.NewMultiStringFlag("tag_prod_image", nil, "Docker image that the docker_pushes_watcher app should tag as 'prod' if it is newer than the last hash tagged as 'prod'.")
	deployImages  = common.NewMultiStringFlag("deploy_image", nil, "Docker image that the docker_pushes_watcher app should deploy when it's docker image is built, if it is newer than the last encountered hash.")

	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'docker_pushes_watcher'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
)

var (
	parseImageName *regexp.Regexp

	// Binaries.
	pushk  = "/usr/local/bin/pushk"
	docker = "docker"

	// Mutex to ensure that only one goroutine is running tagProdToImage at a time.
	taggingMtx sync.Mutex

	// Mutex to ensure that only one goroutine is running deployImage at a time.
	deployingMtx sync.Mutex
)

const (
	// For accessing Firestore.
	DEFAULT_ATTEMPTS      = 3
	PUT_SINGLE_TIMEOUT    = 10 * time.Second
	DELETE_SINGLE_TIMEOUT = 10 * time.Second

	// Docker constants.
	PROD_TAG = "prod"
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

// addDockerProdTag adds the "prod" tag to the specified docker image in buildInfo.
//
// These steps are done here:
// * "docker login ..." This is done every time this function is run instead of once at startup because
//   the login seems to expire after sometime, maybe related to the oauth2 AccessToken expiration time.
// * "docker pull ..." To populate the local cache with the image we want to tag.
// * "docker tag ..." This tags the image.
// * "docker push ..."" This pushes the newly tagged image to the remote repository.
//   Example of remove repository: https://console.cloud.google.com/gcr/images/skia-public/GLOBAL/infra-v2
//
func addDockerProdTag(ctx context.Context, ts oauth2.TokenSource, buildInfo docker_pubsub.BuildInfo) error {
	token, err := ts.Token()
	if err != nil {
		return skerr.Wrap(err)
	}
	loginCmd := fmt.Sprintf("%s login -u oauth2accesstoken -p %s %s", docker, token.AccessToken, "https://gcr.io")
	sklog.Infof("Running %s", loginCmd)
	if _, loginErr := exec.RunSimple(ctx, loginCmd); loginErr != nil {
		return fmt.Errorf("Error running docker login: %s", loginErr)
	}

	pullCmd := fmt.Sprintf("docker pull %s:%s", buildInfo.ImageName, buildInfo.Tag)
	sklog.Infof("Running %s", pullCmd)
	if _, pullErr := exec.RunSimple(ctx, pullCmd); pullErr != nil {
		return fmt.Errorf("Error running docker pull: %s", pullErr)
	}

	tagCmd := fmt.Sprintf("docker tag %s:%s %s:%s", buildInfo.ImageName, buildInfo.Tag, buildInfo.ImageName, PROD_TAG)
	sklog.Infof("Running %s", tagCmd)
	if _, tagErr := exec.RunSimple(ctx, tagCmd); tagErr != nil {
		return fmt.Errorf("Error running docker tag: %s", tagErr)
	}

	pushCmd := fmt.Sprintf("docker push %s:%s", buildInfo.ImageName, PROD_TAG)
	sklog.Infof("Running %s", pushCmd)
	if _, pushErr := exec.RunSimple(ctx, pushCmd); pushErr != nil {
		return fmt.Errorf("Error running docker push: %s", pushErr)
	}
	return nil
}

// tagProdToImage adds the "prod" tag to docker image if:
// * It's commit hash is newer than the entry in Firestore for the specified image.
// * There is no entry in Firestore and it is the first time we have seen this image.
func tagProdToImage(ctx context.Context, fsClient *firestore.Client, gitRepo *gitiles.Repo, ts oauth2.TokenSource, buildInfo docker_pubsub.BuildInfo) error {
	taggingMtx.Lock()
	defer taggingMtx.Unlock()

	// Query firstore for this image.
	baseName := baseImageName(buildInfo.ImageName)
	col := fsClient.Collection(baseName)
	id := baseName
	var fromDB docker_pubsub.BuildInfo
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
		return fmt.Errorf("For %s found %d entries in firestore. There should be only 1 entry.", baseName, len(docs))
	} else if len(docs) == 0 {
		// First time we have seen this image. Add it to firestore.
		if _, createErr := fsClient.Create(ctx, col.Doc(id), buildInfo, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT); createErr != nil {
			return skerr.Wrap(createErr)
		}
		sklog.Infof("Going to apply the prod tag to %s:%s", buildInfo.ImageName, buildInfo.Tag)
		if err := addDockerProdTag(ctx, ts, buildInfo); err != nil {
			return skerr.Wrap(err)
		}
	} else {
		// There is an existing entry for this image. See if the commit hash in the received image is newer.
		if err := docs[0].DataTo(&fromDB); err != nil {
			return skerr.Wrap(err)
		}
		if fromDB.Tag == buildInfo.Tag {
			sklog.Infof("We have already in the past tagged %s:%s with prod", buildInfo.ImageName, buildInfo.Tag)
		} else {
			log, err := gitRepo.LogLinear(ctx, fromDB.Tag, buildInfo.Tag)
			if err != nil {
				return fmt.Errorf("Could not query gitiles of %s: %s", common.REPO_SKIA, err)
			}
			if len(log) > 0 {
				// This means that the commit hash in the received image is newer than the one in datastore.
				sklog.Infof("Applying the prod tag to %s:%s", buildInfo.ImageName, buildInfo.Tag)
				if err := addDockerProdTag(ctx, ts, buildInfo); err != nil {
					return skerr.Wrap(err)
				}
				sklog.Infof("%s is newer than %s for %s. Replacing the entry in firestore", buildInfo.Tag, fromDB.Tag, buildInfo.ImageName)
				if _, deleteErr := fsClient.Delete(ctx, col.Doc(id), DEFAULT_ATTEMPTS, DELETE_SINGLE_TIMEOUT); deleteErr != nil {
					return fmt.Errorf("Could not delete %s in firestore: %s", buildInfo.ImageName, deleteErr)
				}
				if _, createErr := fsClient.Create(ctx, col.Doc(id), buildInfo, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT); createErr != nil {
					return skerr.Wrap(err)
				}
			} else {
				sklog.Infof("Existing firestore entry %s is newer than %s for %s", fromDB.Tag, buildInfo.Tag, buildInfo.ImageName)
			}
		}
	}
	return nil
}

// deployImage deploys the specified app using pushk.
func deployImage(ctx context.Context, appName string) error {
	deployingMtx.Lock()
	defer deployingMtx.Unlock()

	// TODO(rmistry): Remove --dry-run from the below when we are ready to actually deploy images.
	pushCmd := fmt.Sprintf("%s --logtostderr --dry-run %s", pushk, appName)
	if *clusterConfig != "" {
		pushCmd += fmt.Sprintf(" --config-file=%s", *clusterConfig)
	}
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
		"docker_pushes_watcher",
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

	if *hang {
		sklog.Infof("--hang provided; doing nothing.")
		for {
		}
	}

	// Create token source.
	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL, auth.SCOPE_GERRIT, pubsub.ScopePubSub, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}

	// Setup pubsub.
	client, err := pubsub.NewClient(ctx, *project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}
	topic := client.Topic(docker_pubsub.TOPIC)
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	subName := fmt.Sprintf("%s-%s", docker_pubsub.TOPIC, hostname)
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
	fsClient, err := firestore.NewClient(ctx, *fsProjectID, "docker-pushes-watcher", *fsNamespace, ts)
	if err != nil {
		sklog.Fatalf("Could not init firestore: %s", err)
	}

	// Instantiate httpClient for gitiles.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	for {
		err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			msg.Ack()

			var buildInfo docker_pubsub.BuildInfo
			if err := json.Unmarshal(msg.Data, &buildInfo); err != nil {
				sklog.Errorf("Failed to decode: %s: %q", err, string(msg.Data))
				return
			}
			sklog.Infof("Reviewed msg for %+v", buildInfo)

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
				// Instantiate gitiles using the repo.
				gitRepo := gitiles.NewRepo(buildInfo.Repo, httpClient)

				if err := tagProdToImage(ctx, fsClient, gitRepo, ts, buildInfo); err != nil {
					sklog.Errorf("Failed to add the prod tag to %s: %s", buildInfo, err)
					return
				}
			} else {
				sklog.Infof("Not going to tag %s with prod. It is not in the whitelist of images to tag: %s", buildInfo, *tagProdImages)
			}

			// See if the image is in the whitelist of images to be deployed by pushk.
			if util.In(baseImageName(imageName), *deployImages) {
				if err := deployImage(ctx, baseImageName(imageName)); err != nil {
					sklog.Errorf("Failed to deploy %s: %s", buildInfo, err)
					return
				}
			} else {
				sklog.Infof("Not going to deploy %s. It is not in the whitelist of images to deploy: %s", buildInfo, *deployImages)
			}

			sklog.Infof("Done processing %s", buildInfo)
		})
		if err != nil {
			sklog.Errorf("Failed receiving pubsub message: %s", err)
		}
	}
}
