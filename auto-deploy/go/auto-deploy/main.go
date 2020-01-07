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
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Flags
var (
	// TODO(rmistry): Change the below to 5 mins???
	repoPollingPeriod = flag.Duration("repo_polling_period", 2*time.Minute, "How often to check the repo for new changes to deploy.")
	appToDir          = common.NewMultiStringFlag("app_to_dir", nil, "Name of the k8s apps and the directory to check for new changes, both are specified by a colon. Eg: ctfe:ct/go")
	clusterConfig     = flag.String("cluster_config", "", "Absolute filename of the config.json file.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	project           = flag.String("project", "skia-public", "The GCE project name.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	hang              = flag.Bool("hang", false, "If true, just hang and do nothing.")

	// The format of the image is expected to be:
	// "gcr.io/${PROJECT}/${APPNAME}:${DATETIME}-${USER}-${HASH:0:7}-${REPO_STATE}" (from bash/docker_build.sh).
	imageRegex = regexp.MustCompile(`^.+:(.+)-.+-.+-.+$`)

	// k8sYamlRepo the repository where K8s yaml files are stored (eg:
	// https://skia.googlesource.com/k8s-config). Loaded from config file.
	k8sYamlRepo = ""

	// Binaries.
	pushk = "/usr/local/bin/pushk"
)

const (
	IMAGE_DIRTY_SUFFIX = "-dirty"

	// Metric names.
	PUSH_FAILURE    = "auto_deploy_push_failure"
	LIVENESS_METRIC = "auto_deploy"
)

// getLiveAppContainersToImages returns a map of app names to their containers to the images running on them.
func getLiveAppContainersToImages(ctx context.Context, clientset *kubernetes.Clientset) (map[string]map[string]string, error) {
	// Get JSON output of pods running in K8s.
	pods, err := clientset.CoreV1().Pods("default").List(metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("Error when listing running pods: %s", err)
	}
	liveAppContainersToImages := map[string]map[string]string{}
	for _, p := range pods.Items {
		if app, ok := p.Labels["app"]; ok {
			liveAppContainersToImages[app] = map[string]string{}
			for _, container := range p.Spec.Containers {
				liveAppContainersToImages[app][container.Name] = container.Image
			}
		} else {
			sklog.Infof("No app label found for pod %+v", p)
		}
	}

	return liveAppContainersToImages, nil
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
//   Example of remote repository: https://console.cloud.google.com/gcr/images/skia-public/GLOBAL/infra
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
// Returns a bool that indicates whether this image has been tagged with "prod" or not.
func tagProdToImage(ctx context.Context, fsClient *firestore.Client, gitRepo *gitiles.Repo, ts oauth2.TokenSource, buildInfo docker_pubsub.BuildInfo) (bool, error) {
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
			return false, skerr.Wrap(err)
		}
		docs = append(docs, doc)
	}

	taggedWithProd := false
	if len(docs) > 1 {
		return false, fmt.Errorf("For %s found %d entries in firestore. There should be only 1 entry.", baseName, len(docs))
	} else if len(docs) == 0 {
		// First time we have seen this image. Add it to firestore.
		if _, createErr := fsClient.Create(ctx, col.Doc(id), buildInfo, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT); createErr != nil {
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
				return false, fmt.Errorf("Could not query gitiles of %s: %s", common.REPO_SKIA, err)
			}
			if len(log) > 0 {
				// This means that the commit hash in the received image is newer than the one in datastore.
				sklog.Infof("Applying the prod tag to %s:%s", buildInfo.ImageName, buildInfo.Tag)
				if err := addDockerProdTag(ctx, ts, buildInfo); err != nil {
					return false, skerr.Wrap(err)
				}
				sklog.Infof("%s is newer than %s for %s. Replacing the entry in firestore", buildInfo.Tag, fromDB.Tag, buildInfo.ImageName)
				if _, deleteErr := fsClient.Delete(ctx, col.Doc(id), DEFAULT_ATTEMPTS, DELETE_SINGLE_TIMEOUT); deleteErr != nil {
					return false, fmt.Errorf("Could not delete %s in firestore: %s", buildInfo.ImageName, deleteErr)
				}
				if _, createErr := fsClient.Create(ctx, col.Doc(id), buildInfo, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT); createErr != nil {
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
	cfgFile := ""
	if *clusterConfig != "" {
		cfgFile = fmt.Sprintf(" --config-file=%s", *clusterConfig)
	}
	runningInK8sArg := ""
	if !*local {
		runningInK8sArg = " --running-in-k8s"
	}
	pushCmd := fmt.Sprintf("%s --logtostderr%s%s %s", pushk, cfgFile, runningInK8sArg, fullyQualifiedImageName)
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
	common.InitWithMust("auto-deploy", common.PrometheusOpt(promPort))
	defer sklog.Flush()
	ctx := context.Background()

	clusterConfig, err := clusterconfig.New(*configFile)
	if err != nil {
		sklog.Fatalf("Failed to load cluster config: %s", err)
	}
	k8sYamlRepo = clusterConfig.GetString("repo")
	if _, ok := clusterConfig.GetStringMap("clusters")[*cluster]; !ok {
		sklog.Fatalf("Invalid cluster %q: %s", *cluster, err)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		sklog.Fatalf("Failed to get in-cluster config: %s", err)
	}
	sklog.Infof("Auth username: %s", config.Username)
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		sklog.Fatalf("Failed to get in-cluster clientset: %s", err)
	}

	// OAuth2.0 TokenSource.
	ts, err := auth.NewDefaultTokenSource(false, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatal(err)
	}
	// Authenticated HTTP client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	liveness := metrics2.NewLiveness(LIVENESS_METRIC)
	oldMetrics := map[metrics2.Int64Metric]struct{}{}
	go util.RepeatCtx(*dirtyConfigChecksPeriod, ctx, func(ctx context.Context) {
		newMetrics, err := checkForDirtyConfigs(ctx, clientset, gitiles.NewRepo(k8sYamlRepo, httpClient), oldMetrics)
		if err != nil {
			sklog.Errorf("Error when checking for dirty configs: %s", err)
		} else {
			liveness.Reset()
			oldMetrics = newMetrics
		}
	})

	select {}
}
