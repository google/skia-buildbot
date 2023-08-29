// docker_pushes_watcher monitors pubsub events for docker pushes and looks at a
// list of image names to do one or more of the following:
// * tag new images with "prod"
// * deploy images using pushk

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/pubsub/sub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
)

// Flags
var (
	clusterConfig     = flag.String("cluster_config", "", "Absolute filename of the config.json file.")
	gcImagesThreshold = flag.Int("gc_images_threshold", 5, "After this many images have been deployed, clean up docker resources.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	project           = flag.String("project", "skia-public", "The GCE project name.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	hang              = flag.Bool("hang", false, "If true, just hang and do nothing.")

	tagProdImages = common.NewMultiStringFlag("tag_prod_image", nil, "Docker image that the docker_pushes_watcher app should tag as 'prod' if it is newer than the last hash tagged as 'prod'.")
	deployImages  = common.NewMultiStringFlag("deploy_image", nil, "Docker image that the docker_pushes_watcher app should deploy when it's docker image is built, if it is has been tagged as 'prod' using the tag_prod_image flag.")

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
	defaultAttempts     = 3
	putSingleTimeout    = 10 * time.Second
	deleteSingleTimeout = 10 * time.Second

	// Docker constants.
	prodTag = "prod"

	// Name of metrics.
	tagFailureMetric  = "docker_watcher_tag_failure"
	pushFailureMetric = "docker_watcher_push_failure"
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
//   - "docker login ..." This is done every time this function is run instead of once at startup because
//     the login seems to expire after sometime, maybe related to the oauth2 AccessToken expiration time.
//   - "docker pull ..." To populate the local cache with the image we want to tag.
//   - "docker tag ..." This tags the image.
//   - "docker push ..."" This pushes the newly tagged image to the remote repository.
//     Example of remote repository: https://console.cloud.google.com/gcr/images/skia-public/GLOBAL/infra
func addDockerProdTag(ctx context.Context, ts oauth2.TokenSource, buildInfo docker_pubsub.BuildInfo) error {
	// Retry a few times if there are errors. Sometimes the access token expires between the login and the push.
	const maxNumAttempts = 2
	var err error
	for i := 0; i < maxNumAttempts; i++ {
		// Do not retry on e.g. a cancelled context.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			sklog.Warningf("Retrying because of %s", err)
			err = nil
		}

		token, tokenErr := ts.Token()
		if tokenErr != nil {
			err = skerr.Wrap(tokenErr)
			continue
		}
		loginCmd := fmt.Sprintf("%s login -u oauth2accesstoken -p %s %s", docker, token.AccessToken, "https://gcr.io")
		sklog.Infof("Running %s", loginCmd)
		if _, loginErr := exec.RunSimple(ctx, loginCmd); loginErr != nil {
			err = skerr.Wrapf(loginErr, "running docker login")
			continue
		}

		pullCmd := fmt.Sprintf("%s pull %s:%s", docker, buildInfo.ImageName, buildInfo.Tag)
		sklog.Infof("Running %s", pullCmd)
		if _, pullErr := exec.RunSimple(ctx, pullCmd); pullErr != nil {
			err = skerr.Wrapf(pullErr, "running docker pull")
			continue
		}

		tagCmd := fmt.Sprintf("%s tag %s:%s %s:%s", docker, buildInfo.ImageName, buildInfo.Tag, buildInfo.ImageName, prodTag)
		sklog.Infof("Running %s", tagCmd)
		if _, tagErr := exec.RunSimple(ctx, tagCmd); tagErr != nil {
			err = skerr.Wrapf(tagErr, "running docker tag")
			continue
		}

		pushCmd := fmt.Sprintf("%s push %s:%s", docker, buildInfo.ImageName, prodTag)
		sklog.Infof("Running %s", pushCmd)
		if _, pushErr := exec.RunSimple(ctx, pushCmd); pushErr != nil {
			err = skerr.Wrapf(pushErr, "running docker push")
			continue
		}

		// Docker cmds were successful, break out of the retry loop.
		break
	}
	return err
}

// cleanupImages removes all unused images to avoid filling up the shared Docker cache and causing k8s evictions
// b/296862664
func cleanupImages(ctx context.Context) error {
	pushCmd := fmt.Sprintf("%s image prune --all --force", docker)
	sklog.Infof("Cleaning up all unused docker images")
	if out, err := exec.RunSimple(ctx, pushCmd); err != nil {
		sklog.Error(out)
		return skerr.Wrap(err)
	}
	sklog.Infof("Docker image cleanup success")
	return nil
}

// tagProdToImage adds the "prod" tag to docker image if:
// * It's commit hash is newer than the entry in Firestore for the specified image.
// * There is no entry in Firestore and it is the first time we have seen this image.
// Returns a bool that indicates whether this image has been tagged with "prod" or not.
func tagProdToImage(ctx context.Context, fsClient *firestore.Client, gitRepo *gitiles.Repo, ts oauth2.TokenSource, buildInfo docker_pubsub.BuildInfo) (bool, error) {
	taggingMtx.Lock()
	defer taggingMtx.Unlock()

	// Query firestore for this image.
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
			return false, skerr.Wrap(err)
		}
		docs = append(docs, doc)
	}

	taggedWithProd := false
	if len(docs) > 1 {
		return false, skerr.Fmt("For %s found %d entries in firestore. There should be only 1 entry.", baseName, len(docs))
	} else if len(docs) == 0 {
		// First time we have seen this image. Add it to firestore.
		if _, createErr := fsClient.Create(ctx, col.Doc(id), buildInfo, defaultAttempts, putSingleTimeout); createErr != nil {
			return false, skerr.Wrap(createErr)
		}
		sklog.Infof("Going to apply the prod tag to %s:%s", buildInfo.ImageName, buildInfo.Tag)
		if err := addDockerProdTag(ctx, ts, buildInfo); err != nil {
			return false, skerr.Wrap(err)
		}
		taggedWithProd = true
	} else {
		// There is an existing entry for this image. See if the commit hash in the received image is newer.
		if err := docs[0].DataTo(&fromDB); err != nil {
			return false, skerr.Wrap(err)
		}
		if fromDB.Tag == buildInfo.Tag {
			sklog.Infof("We have already in the past tagged %s:%s with prod", buildInfo.ImageName, buildInfo.Tag)
		} else {
			log, err := gitRepo.LogLinear(ctx, fromDB.Tag, buildInfo.Tag)
			if err != nil {
				return false, skerr.Wrapf(err, "Could not query gitiles of %s", common.REPO_SKIA)
			}
			if len(log) > 0 {
				// This means that the commit hash in the received image is newer than the one in datastore.
				sklog.Infof("Applying the prod tag to %s:%s", buildInfo.ImageName, buildInfo.Tag)
				if err := addDockerProdTag(ctx, ts, buildInfo); err != nil {
					return false, skerr.Wrap(err)
				}
				sklog.Infof("%s is newer than %s for %s. Replacing the entry in firestore", buildInfo.Tag, fromDB.Tag, buildInfo.ImageName)
				if _, deleteErr := fsClient.Delete(ctx, col.Doc(id), defaultAttempts, deleteSingleTimeout); deleteErr != nil {
					return false, skerr.Wrapf(deleteErr, "Could not delete %s in firestore", buildInfo.ImageName)
				}
				if _, createErr := fsClient.Create(ctx, col.Doc(id), buildInfo, defaultAttempts, putSingleTimeout); createErr != nil {
					return false, skerr.Wrap(err)
				}
				taggedWithProd = true
			} else {
				sklog.Infof("Existing firestore entry %s is newer than %s for %s", fromDB.Tag, buildInfo.Tag, buildInfo.ImageName)
			}
		}
	}
	return taggedWithProd, nil
}

// deployImage deploys the specified fully qualified image name using pushk.
// fullyQualifiedImageName should look like this: gcr.io/skia-public/fiddler:840ee5a432444a504020e1ec3b25e2e3f4763e7b
func deployImage(ctx context.Context, fullyQualifiedImageName string) error {
	deployingMtx.Lock()
	defer deployingMtx.Unlock()

	cfgFile := ""
	if *clusterConfig != "" {
		cfgFile = fmt.Sprintf(" --config-file=%s", *clusterConfig)
	}
	runningInK8sArg := ""
	if !*local {
		runningInK8sArg = " --running-in-k8s"
	}
	pushCmd := fmt.Sprintf("%s --do-not-override-dirty-image%s%s %s", pushk, cfgFile, runningInK8sArg, fullyQualifiedImageName)
	sklog.Infof("About to execute: %q", pushCmd)

	// Retry pushk command if there is an error due to concurrent pushes (skbug.com/10261).
	const maxAttempts = 3
	var pushkErr error
	for i := 0; i < maxAttempts; i++ {
		if pushkErr != nil {
			sklog.Warning("Retrying")
			pushkErr = nil
		}
		output, err := exec.RunSimple(ctx, pushCmd)
		if err != nil {
			pushkErr = err
			sklog.Warningf("Pushk failed with %s. Output: %s", pushkErr, output)
			continue
		}
		// Pushk cmd was successful, log and break out of the retry loop.
		sklog.Info(output)
		break
	}
	return pushkErr
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
		select {}
	}

	// Add dummy app metrics so that missing data alerts do not show up every time
	// this app is restarted.
	dummyTags := map[string]string{"image": "dummyImage", "repo": "dummyRepo"}
	dummyTagFailure := metrics2.GetCounter(tagFailureMetric, dummyTags)
	dummyTagFailure.Reset()
	dummyPushFailure := metrics2.GetCounter(pushFailureMetric, dummyTags)
	dummyPushFailure.Reset()

	// Create token source.
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeFullControl, auth.ScopeGerrit, pubsub.ScopePubSub, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create git-cookie if not local.
	if !*local {
		_, err := gitauth.New(ctx, ts, "/tmp/git-cookie", true, "skia-docker-pushes-watcher@skia-public.iam.gserviceaccount.com")
		if err != nil {
			sklog.Fatal(err)
		}
	}

	pubSubClient, err := sub.New(ctx, *local, *project, docker_pubsub.TOPIC, 1)
	if err != nil {
		sklog.Fatal(err)
	}

	// Instantiate firestore.
	fsClient, err := firestore.NewClient(ctx, *fsProjectID, "docker-pushes-watcher", *fsNamespace, ts)
	if err != nil {
		sklog.Fatalf("Could not init firestore: %s", err)
	}

	// Instantiate httpClient for gitiles.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	pubSubReceive := metrics2.NewLiveness("docker_watcher_pubsub_receive", nil)
	sinceLastGC := 0
	for {
		err := pubSubClient.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
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

			// Commit tags contain the commit hash. Tryjob tags are in this format: ${CHANGE_NUM}/${PATCHSET_NUM}.
			// Ignore Tryjob tags and only apply prod tag to commit hashes.
			if strings.Index(tag, "_") != -1 {
				sklog.Infof("Found a tryjob tag %s for %s. Ignoring.", tag, imageName)
				return
			}

			// See if the image is in the list of images to be tagged.
			if util.In(baseImageName(imageName), *tagProdImages) {
				// Instantiate gitiles using the repo.
				gitRepo := gitiles.NewRepo(buildInfo.Repo, httpClient)

				tagFailure := metrics2.GetCounter(tagFailureMetric, map[string]string{"image": baseImageName(imageName), "repo": buildInfo.Repo})
				taggedWithProd, err := tagProdToImage(ctx, fsClient, gitRepo, ts, buildInfo)
				if err != nil {
					sklog.Errorf("Failed to add the prod tag to %s: %s", buildInfo, err)
					tagFailure.Inc(1)
					return
				}
				tagFailure.Reset()
				if taggedWithProd {
					// See if the image is in the list of images to be deployed by pushk.
					if util.In(baseImageName(imageName), *deployImages) {
						pushFailure := metrics2.GetCounter(pushFailureMetric, map[string]string{"image": baseImageName(imageName), "repo": buildInfo.Repo})
						fullyQualifiedImageName := fmt.Sprintf("%s:%s", imageName, tag)
						sinceLastGC++
						if err := deployImage(ctx, fullyQualifiedImageName); err != nil {
							sklog.Errorf("Failed to deploy %s: %s", buildInfo, err)
							pushFailure.Inc(1)
							return
						}
						if sinceLastGC >= *gcImagesThreshold {
							if err := cleanupImages(ctx); err != nil {
								sklog.Errorf("Failed to clean up Docker images: %s", err)
							} else {
								sinceLastGC = 0
							}
						}
						pushFailure.Reset()
					} else {
						sklog.Infof("Not going to deploy %s. It is not in the list of images to deploy: %s", buildInfo, *deployImages)
					}
				}
			} else {
				sklog.Infof("Not going to tag %s with prod. It is not in the list of images to tag: %s", buildInfo, *tagProdImages)
			}

			pubSubReceive.Reset()
			sklog.Infof("Done processing %s", buildInfo)
		})
		if err != nil {
			sklog.Errorf("Failed receiving pubsub message: %s", err)
		}
	}
}
