// k8s_checker is an application that checks for the following and alerts if necessary:
// * Dirty images checked into K8s config files.
// * Dirty configs running in K8s.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
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
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	IMAGE_DIRTY_SUFFIX = "-dirty"

	// Metric names.
	DIRTY_COMMITTED_IMAGE_METRIC = "dirty_committed_image_metric"
	DIRTY_CONFIG_METRIC          = "dirty_config_metric"
	STALE_IMAGE_METRIC           = "stale_image_metric"
	LIVENESS_METRIC              = "k8s_checker"
)

var (
	// Flags.
	k8sYamlRepo             = flag.String("k8s_yaml_repo", "https://skia.googlesource.com/skia-public-config", "The repository where K8s yaml files are stored (eg: https://skia.googlesource.com/skia-public-config)")
	workdir                 = flag.String("workdir", "/tmp/", "Directory to use for scratch work.")
	promPort                = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	dirtyConfigChecksPeriod = flag.Duration("dirty_config_checks_period", 2*time.Minute, "How often to check for dirty configs/images in K8s.")
	local                   = flag.Bool("local", false, "Running locally if true. As opposed to in production.")

	// The format of the image is expected to be:
	// "gcr.io/${PROJECT}/${APPNAME}:${DATETIME}-${USER}-${HASH:0:7}-${REPO_STATE}" (from bash/docker_build.sh).
	imageRegex = regexp.MustCompile(`^.+:(.+)-.+-.+-.+$`)
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

// checkForDirtyConfigs checks for:
// * Dirty images checked into K8s config files.
// * Dirty configs running in K8s.
// It takes in a map of oldMetrics, any metrics from that map that are not encountered during this
// invocation of the function are deleted. This is done to handle the case when metric tags
// change. Eg: liveImage in dirtyConfigMetricTags.
// It returns a map of newMetrics, which are all the metrics that were used during this
// invocation of the function.
func checkForDirtyConfigs(ctx context.Context, clientset *kubernetes.Clientset, oldMetrics map[metrics2.Int64Metric]struct{}) (map[metrics2.Int64Metric]struct{}, error) {
	sklog.Info("\n\n---------- New round of checking k8 dirty configs ----------\n\n")

	// Get mapping from live apps to their containers and images.
	liveAppContainerToImages, err := getLiveAppContainersToImages(ctx, clientset)
	if err != nil {
		return nil, fmt.Errorf("Could not get live pods from kubectl: %s", err)
	}

	// Checkout the K8s config repo.
	// Use gitiles if this ends up giving us any problems.
	g, err := git.NewCheckout(ctx, *k8sYamlRepo, *workdir)
	if err != nil {
		return nil, fmt.Errorf("Error when checking out %s: %s", *k8sYamlRepo, err)
	}
	if err := g.Update(ctx); err != nil {
		return nil, fmt.Errorf("Error when updating %s: %s", *k8sYamlRepo, err)
	}

	files, err := ioutil.ReadDir(g.Dir())
	if err != nil {
		return nil, fmt.Errorf("Error when reading from %s: %s", g.Dir(), err)
	}

	newMetrics := map[metrics2.Int64Metric]struct{}{}
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".yaml" {
			// Only interested in YAML configs.
			continue
		}
		b, err := ioutil.ReadFile(filepath.Join(g.Dir(), f.Name()))
		if err != nil {
			sklog.Fatal(err)
		}

		// There can be multiple YAML documents within a single YAML file.
		yamlDocs := strings.Split(string(b), "---")
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
			for _, c := range config.Spec.Template.TemplateSpec.Containers {
				container := c.Name
				committedImage := c.Image

				// Check if the image in the config is dirty.
				dirtyCommittedMetricTags := map[string]string{
					"yaml":           f.Name(),
					"repo":           *k8sYamlRepo,
					"committedImage": committedImage,
				}
				dirtyCommittedMetric := metrics2.GetInt64Metric(DIRTY_COMMITTED_IMAGE_METRIC, dirtyCommittedMetricTags)
				newMetrics[dirtyCommittedMetric] = struct{}{}
				if strings.HasSuffix(committedImage, IMAGE_DIRTY_SUFFIX) {
					sklog.Infof("%s has a dirty committed image: %s\n\n", f.Name(), committedImage)
					dirtyCommittedMetric.Update(1)
				} else {
					dirtyCommittedMetric.Update(0)
				}

				// Check if the image running in k8s matches the checked in image.
				if liveContainersToImages, ok := liveAppContainerToImages[app]; ok {
					if liveImage, ok := liveContainersToImages[container]; ok {
						dirtyConfigMetricTags := map[string]string{
							"app":            app,
							"container":      container,
							"yaml":           f.Name(),
							"repo":           *k8sYamlRepo,
							"committedImage": committedImage,
							"liveImage":      liveImage,
						}
						dirtyConfigMetric := metrics2.GetInt64Metric(DIRTY_CONFIG_METRIC, dirtyConfigMetricTags)
						newMetrics[dirtyConfigMetric] = struct{}{}
						if liveImage != committedImage {
							dirtyConfigMetric.Update(1)
							sklog.Infof("For app %s and container %s the running image differs from the image in config: %s != %s\n\n", app, container, liveImage, committedImage)
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
									stateImageMetricTags := map[string]string{
										"app":       app,
										"container": container,
										"yaml":      f.Name(),
										"repo":      *k8sYamlRepo,
										"liveImage": liveImage,
									}
									staleImageMetric := metrics2.GetInt64Metric(STALE_IMAGE_METRIC, stateImageMetricTags)
									newMetrics[staleImageMetric] = struct{}{}
									numDaysOldImage := int64(time.Now().UTC().Sub(t).Hours() / 24)
									staleImageMetric.Update(numDaysOldImage)
								}
							}

						}
					} else {
						sklog.Infof("There is no running container %s for the config file %s\n\n", container, f.Name())
					}
				} else {
					sklog.Infof("There is no running app %s for the config file %s\n\n", app, f.Name())
				}
			}
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

	config, err := rest.InClusterConfig()
	if err != nil {
		sklog.Fatalf("Failed to get in-cluster config: %s", err)
	}
	sklog.Infof("Auth username: %s", config.Username)
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		sklog.Fatalf("Failed to get in-cluster clientset: %s", err)
	}

	if !*local {
		// Use the gitcookie created by gitauth package.
		ts, err := auth.NewDefaultTokenSource(false, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
		if err != nil {
			sklog.Fatal(err)
		}
		gitcookiesPath := filepath.Join(*workdir, ".gitcookies")
		if _, err := gitauth.New(ts, gitcookiesPath, true, ""); err != nil {
			sklog.Fatalf("Failed to create git cookie updater: %s", err)
		}
	}

	liveness := metrics2.NewLiveness(LIVENESS_METRIC)
	oldMetrics := map[metrics2.Int64Metric]struct{}{}
	go util.RepeatCtx(*dirtyConfigChecksPeriod, ctx, func(ctx context.Context) {
		newMetrics, err := checkForDirtyConfigs(ctx, clientset, oldMetrics)
		if err != nil {
			sklog.Errorf("Error when checking for dirty configs: %s", err)
		} else {
			liveness.Reset()
			oldMetrics = newMetrics
		}
	})

	select {}
}
