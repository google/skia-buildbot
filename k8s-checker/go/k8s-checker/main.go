// k8s_checker is an application that checks for the following and alerts if necessary:
// * Dirty images checked into K8s config files.
// * Dirty configs running in K8s.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/kube/clusterconfig"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	IMAGE_DIRTY_SUFFIX = "-dirty"

	// Metric names.
	EVICTED_POD_METRIC                  = "evicted_pod_metric"
	DIRTY_COMMITTED_IMAGE_METRIC        = "dirty_committed_image_metric"
	DIRTY_CONFIG_METRIC                 = "dirty_config_metric"
	STALE_IMAGE_METRIC                  = "stale_image_metric"
	APP_RUNNING_METRIC                  = "app_running_metric"
	CONTAINER_RUNNING_METRIC            = "container_running_metric"
	RUNNING_APP_HAS_CONFIG_METRIC       = "running_app_has_config_metric"
	RUNNING_CONTAINER_HAS_CONFIG_METRIC = "running_container_has_config_metric"
	LIVENESS_METRIC                     = "k8s_checker"
)

var (
	// Flags.
	dirtyConfigChecksPeriod = flag.Duration("dirty_config_checks_period", 2*time.Minute, "How often to check for dirty configs/images in K8s.")
	configFile              = flag.String("config_file", "", "The location of the config.json file that describes all the clusters.")
	cluster                 = flag.String("cluster", "skia-public", "The k8s cluster name.")
	local                   = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort                = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	workdir                 = flag.String("workdir", "/tmp/", "Directory to use for scratch work.")

	// The format of the image is expected to be:
	// "gcr.io/${PROJECT}/${APPNAME}:${DATETIME}-${USER}-${HASH:0:7}-${REPO_STATE}" (from bash/docker_build.sh).
	imageRegex = regexp.MustCompile(`^.+:(.+)-.+-.+-.+$`)

	// k8sYamlRepo the repository where K8s yaml files are stored (eg:
	// https://skia.googlesource.com/k8s-config). Loaded from config file.
	k8sYamlRepo = ""
)

// getEvictedPods finds all pods in "Evicted" state and reports metrics.
// It puts all reported evictedMetrics into the specified metrics map.
func getEvictedPods(ctx context.Context, clientset *kubernetes.Clientset, metrics map[metrics2.Int64Metric]struct{}) error {
	pods, err := clientset.CoreV1().Pods("default").List(metav1.ListOptions{
		FieldSelector: "status.phase=Failed",
	})
	if err != nil {
		return fmt.Errorf("Error when listing running pods: %s", err)
	}

	for _, p := range pods.Items {
		evictedMetricTags := map[string]string{
			"pod":     p.ObjectMeta.Name,
			"reason":  p.Status.Reason,
			"message": p.Status.Message,
		}
		evictedMetric := metrics2.GetInt64Metric(EVICTED_POD_METRIC, evictedMetricTags)
		metrics[evictedMetric] = struct{}{}
		if strings.Contains(p.Status.Reason, "Evicted") {
			evictedMetric.Update(1)
		} else {
			evictedMetric.Update(0)
		}
	}

	return nil
}

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

type K8sConfig struct {
	Spec struct {
		Template struct {
			Metadata struct {
				Labels struct {
					App string `yaml:"app"`
				} `yaml:"labels"`
			} `yaml:"metadata"`
			TemplateSpec struct {
				Containers []struct {
					Name  string `yaml:"name"`
					Image string `yaml:"image"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// performChecks checks for:
// * Dirty images checked into K8s config files.
// * Dirty configs running in K8s.
// * How old the image running in K8s is.
// * Apps and containers running in K8s but not checked into the git repo.
// * Apps and containers checked into the git repo but not running in K8s.
// * Checks for evicted pods.
//
// It takes in a map of oldMetrics, any metrics from that map that are not encountered during this
// invocation of the function are deleted. This is done to handle the case when metric tags
// change. Eg: liveImage in dirtyConfigMetricTags.
// It returns a map of newMetrics, which are all the metrics that were used during this
// invocation of the function.
func performChecks(ctx context.Context, clientset *kubernetes.Clientset, g *gitiles.Repo, oldMetrics map[metrics2.Int64Metric]struct{}) (map[metrics2.Int64Metric]struct{}, error) {
	sklog.Info("---------- New round of checking k8s ----------")
	newMetrics := map[metrics2.Int64Metric]struct{}{}

	// Check for evicted pods.
	if err := getEvictedPods(ctx, clientset, newMetrics); err != nil {
		return nil, fmt.Errorf("Could not check for evicted pods from kubectl: %s", err)
	}

	// Get mapping from live apps to their containers and images.
	liveAppContainerToImages, err := getLiveAppContainersToImages(ctx, clientset)
	if err != nil {
		return nil, fmt.Errorf("Could not get live pods from kubectl: %s", err)
	}

	// Read files from the k8sYamlRepo using gitiles.
	files, _, err := g.ListDir(ctx, *cluster)
	if err != nil {
		return nil, fmt.Errorf("Error when listing files from %s: %s", k8sYamlRepo, err)
	}

	checkedInAppsToContainers := map[string]util.StringSet{}
	for _, f := range files {
		if filepath.Ext(f) != ".yaml" {
			// Only interested in YAML configs.
			continue
		}
		var buf bytes.Buffer
		if err := g.ReadFile(ctx, filepath.Join(*cluster, f), &buf); err != nil {
			return nil, fmt.Errorf("Could not read file %s from %s %s: %s", f, k8sYamlRepo, *cluster, err)
		}

		// There can be multiple YAML documents within a single YAML file.
		yamlDocs := strings.Split(buf.String(), "---")
		for _, yamlDoc := range yamlDocs {
			var config K8sConfig
			if err := yaml.Unmarshal([]byte(yamlDoc), &config); err != nil {
				sklog.Fatalf("Error when parsing %s: %s", yamlDoc, err)
			}
			app := config.Spec.Template.Metadata.Labels.App
			if app == "" {
				// This YAML config does not have an app. Continue.
				continue
			}
			checkedInAppsToContainers[app] = util.StringSet{}
			for _, c := range config.Spec.Template.TemplateSpec.Containers {
				container := c.Name
				committedImage := c.Image
				checkedInAppsToContainers[app][c.Name] = true

				// Check if the image in the config is dirty.
				dirtyCommittedMetricTags := map[string]string{
					"yaml":           f,
					"repo":           k8sYamlRepo,
					"cluster":        *cluster,
					"committedImage": committedImage,
				}
				dirtyCommittedMetric := metrics2.GetInt64Metric(DIRTY_COMMITTED_IMAGE_METRIC, dirtyCommittedMetricTags)
				newMetrics[dirtyCommittedMetric] = struct{}{}
				if strings.HasSuffix(committedImage, IMAGE_DIRTY_SUFFIX) {
					sklog.Infof("%s has a dirty committed image: %s", f, committedImage)
					dirtyCommittedMetric.Update(1)
				} else {
					dirtyCommittedMetric.Update(0)
				}

				// Create app_running metric.
				appRunningMetricTags := map[string]string{
					"app":  app,
					"yaml": f,
					"repo": k8sYamlRepo,
				}
				appRunningMetric := metrics2.GetInt64Metric(APP_RUNNING_METRIC, appRunningMetricTags)
				newMetrics[appRunningMetric] = struct{}{}

				// Check if the image running in k8s matches the checked in image.
				if liveContainersToImages, ok := liveAppContainerToImages[app]; ok {
					appRunningMetric.Update(1)

					// Create container_running metric.
					containerRunningMetricTags := map[string]string{
						"app":       app,
						"container": container,
						"yaml":      f,
						"repo":      k8sYamlRepo,
					}
					containerRunningMetric := metrics2.GetInt64Metric(CONTAINER_RUNNING_METRIC, containerRunningMetricTags)
					newMetrics[containerRunningMetric] = struct{}{}

					if liveImage, ok := liveContainersToImages[container]; ok {
						containerRunningMetric.Update(1)

						dirtyConfigMetricTags := map[string]string{
							"app":            app,
							"container":      container,
							"yaml":           f,
							"repo":           k8sYamlRepo,
							"committedImage": committedImage,
							"liveImage":      liveImage,
						}
						dirtyConfigMetric := metrics2.GetInt64Metric(DIRTY_CONFIG_METRIC, dirtyConfigMetricTags)
						newMetrics[dirtyConfigMetric] = struct{}{}
						if liveImage != committedImage {
							dirtyConfigMetric.Update(1)
							sklog.Infof("For app %s and container %s the running image differs from the image in config: %s != %s", app, container, liveImage, committedImage)
						} else {
							// The live image is the same as the committed image.
							dirtyConfigMetric.Update(0)

							// Now add a metric for how many days old the live/committed image is.
							m := imageRegex.FindStringSubmatch(liveImage)
							if len(m) == 2 {
								t, err := time.Parse(time.RFC3339, strings.ReplaceAll(m[1], "_", ":"))
								if err != nil {
									sklog.Errorf("Could not time.Parse %s from image %s: %s", m[1], liveImage, err)
								} else {
									staleImageMetricTags := map[string]string{
										"app":       app,
										"container": container,
										"yaml":      f,
										"repo":      k8sYamlRepo,
										"liveImage": liveImage,
									}
									staleImageMetric := metrics2.GetInt64Metric(STALE_IMAGE_METRIC, staleImageMetricTags)
									newMetrics[staleImageMetric] = struct{}{}
									numDaysOldImage := int64(time.Now().UTC().Sub(t).Hours() / 24)
									staleImageMetric.Update(numDaysOldImage)
								}
							}

						}
					} else {
						sklog.Infof("There is no running container %s for the config file %s", container, f)
						containerRunningMetric.Update(0)
					}
				} else {
					sklog.Infof("There is no running app %s for the config file %s", app, f)
					appRunningMetric.Update(0)
				}
			}
		}
	}

	// Find out which apps and containers are live but not found in git repo.
	for liveApp := range liveAppContainerToImages {
		runningAppHasConfigMetricTags := map[string]string{
			"app":  liveApp,
			"repo": k8sYamlRepo,
		}
		runningAppHasConfigMetric := metrics2.GetInt64Metric(RUNNING_APP_HAS_CONFIG_METRIC, runningAppHasConfigMetricTags)
		newMetrics[runningAppHasConfigMetric] = struct{}{}
		if checkedInApp, ok := checkedInAppsToContainers[liveApp]; ok {
			runningAppHasConfigMetric.Update(1)

			for liveContainer := range liveAppContainerToImages[liveApp] {
				runningContainerHasConfigMetricTags := map[string]string{
					"app":       liveApp,
					"container": liveContainer,
					"repo":      k8sYamlRepo,
				}
				runningContainerHasConfigMetric := metrics2.GetInt64Metric(RUNNING_CONTAINER_HAS_CONFIG_METRIC, runningContainerHasConfigMetricTags)
				newMetrics[runningContainerHasConfigMetric] = struct{}{}
				if _, ok := checkedInApp[liveContainer]; ok {
					runningContainerHasConfigMetric.Update(1)
				} else {
					sklog.Infof("The running container %s of app %s is not checked into %s", liveContainer, liveApp, k8sYamlRepo)
					runningContainerHasConfigMetric.Update(0)
				}
			}
		} else {
			sklog.Infof("The running app %s is not checked into %s", liveApp, k8sYamlRepo)
			runningAppHasConfigMetric.Update(0)
		}
	}

	// Delete unused old metrics.
	for m := range oldMetrics {
		if _, ok := newMetrics[m]; !ok {
			if err := m.Delete(); err != nil {
				sklog.Errorf("Failed to delete metric: %s", err)
				// Add the metric to newMetrics so that we'll
				// have the chance to delete it again on the
				// next cycle.
				newMetrics[m] = struct{}{}
			}
		}
	}
	return newMetrics, nil
}

func main() {
	common.InitWithMust("k8s_checker", common.PrometheusOpt(promPort))
	defer sklog.Flush()
	ctx := context.Background()

	clusterConfig, err := clusterconfig.New(*configFile)
	if err != nil {
		sklog.Fatalf("Failed to load cluster config: %s", err)
	}
	k8sYamlRepo = clusterConfig.Repo
	if _, ok := clusterConfig.Clusters[*cluster]; !ok {
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
	go util.RepeatCtx(ctx, *dirtyConfigChecksPeriod, func(ctx context.Context) {
		newMetrics, err := performChecks(ctx, clientset, gitiles.NewRepo(k8sYamlRepo, httpClient), oldMetrics)
		if err != nil {
			sklog.Errorf("Error when checking for dirty configs: %s", err)
		} else {
			liveness.Reset()
			oldMetrics = newMetrics
		}
	})

	select {}
}
