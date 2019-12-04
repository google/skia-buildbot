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
	docker_util "go.skia.org/infra/docker_pushes_watcher/go/util"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/firestore"
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
	hang     = flag.Bool("hang", false, "If true, just hang and do nothing.")

	tagProdImages = common.NewMultiStringFlag("tag_prod_image", nil, "Docker image that the docker_pushes_watcher app should tag as 'prod' if it is newer than the last hash tagged as 'prod'.")
	deployImages  = common.NewMultiStringFlag("deploy_image", nil, "Docker image that the docker_pushes_watcher app should deploy when it's docker image is built, if it is newer than the last encountered hash.")

	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'docker_pushes_watcher'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
)

var (
	parseImageName *regexp.Regexp

	// Binaries.
	pushk = "/usr/local/bin/pushk"
	// TODO(rmistry): Make this absolute path
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

func addDockerProdTag(ctx context.Context, buildInfo docker_util.BuildInfo) error {
	pullCmd := fmt.Sprintf("docker pull %s:%s", buildInfo.ImageName, buildInfo.Tag)
	fmt.Printf("RUNNING THIS COMMAND: %s\n\n", pullCmd)
	if _, pullErr := exec.RunSimple(ctx, pullCmd); pullErr != nil {
		return fmt.Errorf("Error running docker pull: %s", pullErr)
	}

	// TODO(rmistry): Make prod a constant or a flag
	tagCmd := fmt.Sprintf("docker tag %s:%s %s:%s", buildInfo.ImageName, buildInfo.Tag, buildInfo.ImageName, "prod")
	fmt.Printf("RUNNING THIS COMMAND: %s\n\n", tagCmd)
	if _, tagErr := exec.RunSimple(ctx, tagCmd); tagErr != nil {
		return fmt.Errorf("Error running docker tag: %s", tagErr)
	}

	pushCmd := fmt.Sprintf("docker push %s:%s", buildInfo.ImageName, "prod")
	fmt.Printf("RUNNING THIS COMMAND: %s\n\n", pushCmd)
	if _, pushErr := exec.RunSimple(ctx, pushCmd); pushErr != nil {
		return fmt.Errorf("Error running docker push: %s", pushErr)
	}
	return nil
}

// tagProdToImage adds the "prod" tag to docker image if:
// * It's commit hash is newer than the entry in Firestore.
// * There is no entry in Firestore and it is the first time we have seen this image.
func tagProdToImage(ctx context.Context, fsClient *firestore.Client, gitRepo *gitiles.Repo, buildInfo docker_util.BuildInfo) error {
	taggingMtx.Lock()
	defer taggingMtx.Unlock()

	// Query firstore for this image.
	baseName := baseImageName(buildInfo.ImageName)
	col := fsClient.Collection(baseName)
	id := baseName
	var fromDB docker_util.BuildInfo
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
		return fmt.Errorf("For %s found %d entries in firestore", baseName, len(docs))
	} else if len(docs) == 0 {
		// First time we have seen this image. Add it to firestore.
		if _, createErr := fsClient.Create(ctx, col.Doc(id), buildInfo, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT); createErr != nil {
			return skerr.Wrap(createErr)
		}
		sklog.Infof("Going to apply the prod tag to %s:%s", buildInfo.ImageName, buildInfo.Tag)
		if err := addDockerProdTag(ctx, buildInfo); err != nil {
			return skerr.Wrap(err)
		}
	} else {
		// There is an existing prod hash for this image. See if the received image is newer.
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
				sklog.Infof("%s is newer than %s for %s. Replacing the entry in firestore", buildInfo.Tag, fromDB.Tag, buildInfo.ImageName)
				// The image is newer. Replace entry in firestore.
				if _, deleteErr := fsClient.Delete(ctx, col.Doc(id), DEFAULT_ATTEMPTS, DELETE_SINGLE_TIMEOUT); deleteErr != nil {
					return fmt.Errorf("Could not delete %s in firestore: %s", buildInfo.ImageName, deleteErr)
				}
				if _, createErr := fsClient.Create(ctx, col.Doc(id), buildInfo, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT); createErr != nil {
					return skerr.Wrap(err)
				}
				sklog.Infof("Applying the prod tag to %s:%s", buildInfo.ImageName, buildInfo.Tag)
				if err := addDockerProdTag(ctx, buildInfo); err != nil {
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

	// Login to docker.
	token, err := ts.Token()
	if err != nil {
		sklog.Fatal(err)
	}
	loginCmd := fmt.Sprintf("%s login -u oauth2accesstoken -p %s %s", docker, token.AccessToken, "https://gcr.io")
	if _, loginErr := exec.RunSimple(ctx, loginCmd); loginErr != nil {
		sklog.Fatalf("Error running docker login: %s", loginErr)
	}

	// Setup pubsub.
	client, err := pubsub.NewClient(ctx, *project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}
	topic := client.Topic(docker_util.TOPIC)
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	subName := fmt.Sprintf("%s-%s", docker_util.TOPIC, hostname)
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

			var buildInfo docker_util.BuildInfo
			if err := json.Unmarshal(msg.Data, &buildInfo); err != nil {
				sklog.Errorf("Failed to decode: %s: %q", err, string(msg.Data))
				return
			}
			sklog.Infof("Reviewed msg %+v", buildInfo)

			// Extract the image name and tag.
			imageName := buildInfo.ImageName
			tag := buildInfo.Tag

			// Instantiate gitiles using the repo.
			gitRepo := gitiles.NewRepo(buildInfo.Repo, httpClient)

			// Commit tags contain the commit hash. Trybot tags are are this format: ${CHANGE_NUM}/${PATCHSET_NUM}.
			// Ignore trybot tags and only apply prod tag to commit hashes.
			if strings.Index(tag, "_") != -1 {
				sklog.Infof("Found a trybot tag %s for %s. Ignoring.", tag, imageName)
				return
			}

			// See if the image is in the whitelist of images to be tagged.
			fmt.Println("DEBUGGING")
			fmt.Println(baseImageName(imageName))
			fmt.Println(*tagProdImages)
			if util.In(baseImageName(imageName), *tagProdImages) {
				if err := tagProdToImage(ctx, fsClient, gitRepo, buildInfo); err != nil {
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
