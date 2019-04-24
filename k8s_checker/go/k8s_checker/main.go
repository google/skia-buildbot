// k8s_checker is an application that checks for the following and alerts if necessary:
// * Dirty images checked into K8s config files.
// * Dirty configs running in K8s.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

const (
	IMAGE_DIRTY_SUFFIX = "-dirty"

	// Metric names.
	DIRTY_COMMITTED_IMAGE_METRIC = "dirty_committed_image_metric"
	DIRTY_CONFIG_METRIC          = "dirty_config_metric"
	LIVENESS_METRIC              = "k8s_checker"
)

var (
	// Flags.
	k8sYamlRepo             = flag.String("k8s_yaml_repo", "https://skia.googlesource.com/skia-public-config", "The repository where K8s yaml files are stored (eg: https://skia.googlesource.com/skia-public-config)")
	kubeConfig              = flag.String("kube_config", "/var/secrets/kube-config/kube_config", "The kube config of the project kubectl will query against.")
	workdir                 = flag.String("workdir", "/tmp/", "Directory to use for scratch work.")
	promPort                = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	serviceAccountKey       = flag.String("service_account_key", "", "Should be set when running in K8s.")
	dirtyConfigChecksPeriod = flag.Duration("dirty_config_checks_period", 2*time.Minute, "How often to check for dirty configs/images in K8s.")
)

type K8sPodsJson struct {
	Items []struct {
		Metadata struct {
			Labels struct {
				App string `json:"app"`
			} `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Name  string `json:"name"`
				Image string `json:"image"`
			} `json:"containers"`
		} `json:"spec"`
	} `json:"items"`
}

// getLiveAppContainersToImages returns a map of app names to their containers to the images running on them.
func getLiveAppContainersToImages(ctx context.Context) (map[string]map[string]string, error) {
	// Get JSON output of pods running in K8s.
	getPodsCommand := fmt.Sprintf("kubectl get pods --kubeconfig=%s -o json --field-selector=status.phase=Running", *kubeConfig)
	output, err := exec.RunSimple(ctx, getPodsCommand)
	if err != nil {
		return nil, fmt.Errorf("Error when running \"%s\": %s", getPodsCommand, err)
	}

	liveAppContainersToImages := map[string]map[string]string{}
	var podsJson K8sPodsJson
	if err := json.Unmarshal([]byte(output), &podsJson); err != nil {
		return nil, fmt.Errorf("Error when unmarshalling JSON: %s", err)
	}
	for _, item := range podsJson.Items {
		app := item.Metadata.Labels.App
		liveAppContainersToImages[app] = map[string]string{}
		for _, container := range item.Spec.Containers {
			liveAppContainersToImages[app][container.Name] = container.Image
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
func checkForDirtyConfigs(ctx context.Context) error {

	// Get mapping from live apps to their containers and images.
	liveAppContainerToImages, err := getLiveAppContainersToImages(ctx)
	if err != nil {
		return fmt.Errorf("Could not get live pods from kubectl: %s", err)
	}

	// Checkout the K8s config repo.
	// Use gitiles if this ends up giving us any problems.
	g, err := git.NewCheckout(ctx, *k8sYamlRepo, *workdir)
	if err != nil {
		return fmt.Errorf("Error when checking out %s: %s", *k8sYamlRepo, err)
	}
	if err := g.Update(ctx); err != nil {
		return fmt.Errorf("Error when updating %s: %s", *k8sYamlRepo, err)
	}

	files, err := ioutil.ReadDir(g.Dir())
	if err != nil {
		return fmt.Errorf("Error when reading from %s: %s", g.Dir(), err)
	}

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
				sklog.Fatal(err)
			}
			app := config.Spec.Template.Metadata.Labels.App
			if app == "" {
				// This YAML config does not have an app. Continue.
				continue
			}
			for _, c := range config.Spec.Template.TemplateSpec.Containers {
				container := c.Name
				committedImage := c.Image
				metricTags := map[string]string{
					"container": container,
					"yaml":      f.Name(),
					"repo":      *k8sYamlRepo,
				}

				// Check if the image in the config is dirty.
				dirtyCommittedMetric := metrics2.GetInt64Metric(DIRTY_COMMITTED_IMAGE_METRIC, metricTags)
				if strings.HasSuffix(committedImage, IMAGE_DIRTY_SUFFIX) {
					sklog.Infof("%s has a dirty committed image: %s\n\n", f.Name(), committedImage)
					dirtyCommittedMetric.Update(1)
				} else {
					dirtyCommittedMetric.Update(0)
				}

				// Check if the image running in k8s matches the checked in image.
				dirtyConfigMetric := metrics2.GetInt64Metric(DIRTY_CONFIG_METRIC, metricTags)
				if liveContainersToImages, ok := liveAppContainerToImages[app]; ok {
					if liveImage, ok := liveContainersToImages[container]; ok {
						if liveImage != committedImage {
							dirtyConfigMetric.Update(1)
							sklog.Infof("For app %s and container %s the running image differs from the image in config: %s != %s\n\n", app, container, liveImage, committedImage)
						} else {
							dirtyConfigMetric.Update(0)
						}
					} else {
						sklog.Infof("There is no running container %s for the config file %s\n\n", container, f.Name())
						dirtyConfigMetric.Update(0)
					}
				} else {
					sklog.Infof("There is no running app %s for the config file %s\n\n", app, f.Name())
					dirtyConfigMetric.Update(0)
				}
			}
		}
	}
	return nil
}

func main() {
	common.InitWithMust("k8s_checker", common.PrometheusOpt(promPort))
	defer sklog.Flush()
	ctx := context.Background()

	if *serviceAccountKey != "" {
		activationCmd := fmt.Sprintf("gcloud auth activate-service-account --key-file %s", *serviceAccountKey)
		if _, err := exec.RunSimple(ctx, activationCmd); err != nil {
			sklog.Fatal(err)
		}
	}

	liveness := metrics2.NewLiveness(LIVENESS_METRIC)
	for range time.Tick(*dirtyConfigChecksPeriod) {
		if err := checkForDirtyConfigs(ctx); err != nil {
			sklog.Errorf("Error when checking for dirty configs: %s", err)
		} else {
			liveness.Reset()
		}
	}
}
