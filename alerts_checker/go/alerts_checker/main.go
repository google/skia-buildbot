// alerts_checker is an application that checks for the following and alerts if necessary:
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
	"go.skia.org/infra/go/util"
)

const (
	IMAGE_DIRTY_SUFFIX = "-dirty"

	// Metric names.
	DIRTY_COMMITTED_IMAGE_METRIC = "dirty_committed_image_metric"
	DIRTY_CONFIG_METRIC          = "dirty_config_metric"
)

var (
	// Flags.
	k8sYamlRepo               = flag.String("k8_yaml_repo", "https://skia.googlesource.com/skia-public-config", "The repository where K8s yaml files are stored (eg: https://skia.googlesource.com/skia-public-config)")
	kubeConfig                = flag.String("kube_config", "/var/secrets/kube-config/kube_config", "The kube config of the project kubectl will query against.")
	workdir                   = flag.String("workdir", "/tmp/", "Directory to use for scratch work.")
	promPort                  = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	serviceAccountKey         = flag.String("service_account_key", "", "Should be set when running in K8s.")
	dirtyConfigChecksDuration = flag.Duration("dirty_config_checks_duration", 2*time.Minute, "How often to check for dirty configs/images in K8s.")
)

func getLivePodsToImages(ctx context.Context) (map[string][]string, error) {
	// Get JSON output of pods running in K8s.
	getPodsCommand := fmt.Sprintf("kubectl get pods --kubeconfig=%s -o json --field-selector=status.phase=Running", *kubeConfig)
	output, err := exec.RunSimple(ctx, getPodsCommand)
	if err != nil {
		return nil, fmt.Errorf("Error when running \"%s\": %s", getPodsCommand, err)
	}

	livePodToImages := map[string][]string{}
	// Parse the JSON output and populate livePodToImages dict.
	var result map[string][]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("Error when unmarshalling JSON: %s", err)
	}
	for _, i := range result["items"] {
		item := i.(map[string]interface{})
		containers := item["spec"].(map[string]interface{})["containers"].([]interface{})
		for _, container := range containers {
			name := container.(map[string]interface{})["name"].(string)
			image := container.(map[string]interface{})["image"].(string)
			if !util.In(image, livePodToImages[name]) {
				livePodToImages[name] = append(livePodToImages[name], image)
			}
		}
	}
	return livePodToImages, nil
}

type K8sConfig struct {
	Spec struct {
		Template struct {
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

	// Get mapping from live pods to their images.
	livePodToImages, err := getLivePodsToImages(ctx)
	if err != nil {
		return fmt.Errorf("Could not get live pods from kubectl: %s", err)
	}

	// Checkout the K8s config repo.
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
			for _, c := range config.Spec.Template.TemplateSpec.Containers {
				pod := c.Name
				committedImage := c.Image
				metricTags := map[string]string{
					"pod":  pod,
					"yaml": f.Name(),
					"repo": *k8sYamlRepo,
				}

				// Check if the image in the config is dirty.
				dirtyCommittedMetric := metrics2.GetInt64Metric(DIRTY_COMMITTED_IMAGE_METRIC, metricTags)
				if strings.HasSuffix(committedImage, IMAGE_DIRTY_SUFFIX) {
					fmt.Printf("%s has a dirty committed image: %s\n\n", f.Name(), committedImage)
					dirtyCommittedMetric.Update(0)
				} else {
					dirtyCommittedMetric.Update(1)
				}

				// Check if the image running in k8s matches the checked in image.
				dirtyConfigMetric := metrics2.GetInt64Metric(DIRTY_CONFIG_METRIC, metricTags)
				if liveImages, ok := livePodToImages[pod]; ok {
					if len(liveImages) > 1 {
						// I do not think this can happen.
						dirtyConfigMetric.Update(1)
						return fmt.Errorf("For %s found %d images: %s\n", pod, len(liveImages), liveImages)
					}
					if liveImages[0] != committedImage {
						dirtyConfigMetric.Update(0)
						fmt.Printf("For %s the running image differs from the image in config: %s != %s\n\n", pod, liveImages[0], committedImage)
					} else {
						dirtyConfigMetric.Update(1)
					}
				} else {
					fmt.Printf("There is no running pod for the %s config file\n\n", pod)
					dirtyConfigMetric.Update(1)
				}
			}
		}
	}
	return nil
}

func main() {
	common.InitWithMust("alerts_checker", common.PrometheusOpt(promPort))
	defer sklog.Flush()
	ctx := context.Background()

	if *serviceAccountKey != "" {
		activationCmd := fmt.Sprintf("gcloud auth activate-service-account --key-file %s", *serviceAccountKey)
		if _, err := exec.RunSimple(ctx, activationCmd); err != nil {
			sklog.Fatal(err)
		}
	}

	for range time.Tick(time.Duration(*dirtyConfigChecksDuration)) {
		if err := checkForDirtyConfigs(ctx); err != nil {
			sklog.Errorf("Error when checking for dirty configs: %s", err)
		}
	}
}
