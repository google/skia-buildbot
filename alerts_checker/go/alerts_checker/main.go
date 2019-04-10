// sheriff_emails is an application that emails the next sheriff every week.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	//"regexp"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// Turn into flags?
	CHECK_DIRTY_COMMITTED_CONFIGS_TIME = 2 * time.Minute

	METRIC_NAME = "alerts_watcher"

	IMAGE_DIRTY_SUFFIX = "-dirty"
)

var (
	// Flags.
	k8sYamlRepo       = flag.String("k8_yaml_repo", "https://skia.googlesource.com/skia-public-config", "The repository where K8s yaml files are stored (eg: https://skia.googlesource.com/skia-public-config)")
	kubeConfig        = flag.String("kube_config", "/var/secrets/kube-config/kube_config", "The kube config of the project kubectl will query against.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir           = flag.String("workdir", ".", "Directory to use for scratch work.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	serviceAccountKey = flag.String("service_account_key", "", "Should be set when running in K8s.")
)

func getLivePodsToImages(ctx context.Context) (map[string][]string, error) {
	livePodToImages := map[string][]string{}
	// Get JSON output of "kubectl get pods".
	getPodsCommand := fmt.Sprintf("kubectl get pods --kubeconfig=%s -o json", *kubeConfig)
	output, err := exec.RunSimple(ctx, getPodsCommand)
	if err != nil {
		return nil, fmt.Errorf("Error when running \"%s\": %s", getPodsCommand, err)
	}
	// Parse the JSON output of "kubectl get pods".
	var result map[string][]interface{}
	json.Unmarshal([]byte(output), &result)
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

type Containers struct {
	APIVersion string `yaml:"apiVersion"`
}

type Config struct {
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

type Spec struct {
	Template Template `yaml:"template"`
}

type Template struct {
	TemplateSpec TemplateSpec `yaml:"spec"`
}

type TemplateSpec struct {
	Containers []Container `yaml:"containers"`
}

type Container struct {
	Name  string `yaml:"name"`
	Image string `yaml:"image"`
}

// return errors and log them... only a few things should  be fatal.
// checkForDirtyConfigs checks of dirty configs in the K8s yaml files and
// if the images running in production are different than the checked in images.
func checkForDirtyConfigs(ctx context.Context) error {
	livePodToImages, err := getLivePodsToImages(ctx)
	fmt.Println(livePodToImages)
	if err != nil {
		return fmt.Errorf("Could not get live pods from kubectl: %s", err)
	}

	// Just check out what you need to here in skia-public-config or skia-corp-config specify via a flag.
	g, err := git.NewCheckout(ctx, *k8sYamlRepo, *workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	if err := g.Update(ctx); err != nil {
		return fmt.Errorf("Error when updating %s: %s", *k8sYamlRepo)
		sklog.Fatal(err)
	}

	files, err := ioutil.ReadDir(g.Dir())
	if err != nil {
		sklog.Fatal(err)
	}
	//re := regexp.MustCompile(`image: (.*)`)
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".yaml" {
			// Only interested in yaml configs.
			continue
		}
		b, err := ioutil.ReadFile(filepath.Join(g.Dir(), f.Name()))
		if err != nil {
			sklog.Fatal(err)
		}

		// TEST
		yamlDocs := strings.Split(string(b), "---")
		for _, yamlDoc := range yamlDocs {
			fmt.Println("TTTTTTTTTTTTTTTTTTTTTTTTTTTT")
			//fmt.Println(yamlDoc)
			var config Config
			if err := yaml.Unmarshal([]byte(yamlDoc), &config); err != nil {
				sklog.Fatal(err)
			}
			for _, c := range config.Spec.Template.TemplateSpec.Containers {
				pod := c.Name
				committedImage := c.Image
				// See if the image in the config is dirty.
				if strings.HasSuffix(committedImage, IMAGE_DIRTY_SUFFIX) {
					fmt.Println("Dirty")
					fmt.Println(f.Name())
				}

				// TODO(rmistry): MAKE A DATASTURCTURE HERE AND THEN COMPARE WITH THE OTHER DATASTRUCTRE!!!!!!!!!

				// See if the image running in k8s matches the checked in image.
				if liveImage, ok := livePodToImages[pod]; ok {
					fmt.Println("live IMAGE IS")
					fmt.Println(liveImage)
					fmt.Println("Checked in IMAGE IS")
					fmt.Println(committedImage)
					if liveImage[0] != committedImage {
						sklog.Fatalf("IMAGES DIFFER: %s != %s", liveImage, committedImage)
					}
				} else {
					fmt.Println("COULD NOT FIND!!!!!!!!!!!!")
					fmt.Println(pod)
				}
			}
			//fmt.Println(string(b))
			fmt.Println("TTTTTTTTTTTTTTTTTTTTTTTTTTTT")
		}
		// TEST2

		//m := re.FindStringSubmatch(string(b))
		//if len(m) > 1 {
		//	image := m[1]
		//	// See if the image in the config is dirty.
		//	if strings.HasSuffix(image, IMAGE_DIRTY_SUFFIX) {
		//		fmt.Println("Dirty")
		//		fmt.Println(f.Name())
		//	}

		//	// See if the image running in k8s matches the checked in image
		//	pod := strings.TrimSuffix(f.Name(), ".yaml")
		//	fmt.Println("xxxxxxxxx")
		//	if liveImage, ok := livePodToImages[pod]; ok {
		//		fmt.Println("live IMAGE IS")
		//		fmt.Println(liveImage)
		//	} else {
		//		fmt.Println("COULD NOT FIND!!!!!!!!!!!!")
		//		fmt.Println(pod)
		//	}

		//}

		//fmt.Println(f.Name())
	}
	return nil
}

func main() {
	common.InitWithMust(METRIC_NAME, common.PrometheusOpt(promPort))
	defer sklog.Flush()
	ctx := context.Background()

	if *serviceAccountKey != "" {
		activationCmd := fmt.Sprintf("gcloud auth activate-service-account --key-file %s", *serviceAccountKey)
		if _, err := exec.RunSimple(ctx, activationCmd); err != nil {
			sklog.Fatal(err)
		}
	}
	//livePodToImages := map[string][]string{}
	//// Get JSON output of "kubectl get pods".
	//getPodsCommand := fmt.Sprintf("kubectl get pods --kubeconfig=%s -o json", *kubeConfig)
	//output, err := exec.RunSimple(ctx, getPodsCommand)
	//if err != nil {
	//	sklog.Fatal(err)
	//}
	//// Parse the JSON output of "kubectl get pods".
	//var result map[string][]interface{}
	//json.Unmarshal([]byte(output), &result)
	//for _, i := range result["items"] {
	//	item := i.(map[string]interface{})
	//	containers := item["spec"].(map[string]interface{})["containers"].([]interface{})
	//	for _, container := range containers {
	//		name := container.(map[string]interface{})["name"].(string)
	//		image := container.(map[string]interface{})["image"].(string)
	//		if !util.In(image, livePodToImages[name]) {
	//			livePodToImages[name] = append(livePodToImages[name], image)
	//		}
	//	}
	//}

	//fmt.Println("XXXXXXXXXXXXXXXXXXXXXXX")
	//for k, v := range livePodToImages {
	//	fmt.Println(k)
	//	fmt.Println(v)
	//	fmt.Println("--------")
	//}

	if err := checkForDirtyConfigs(ctx); err != nil {
		sklog.Errorf("Error when checking for dirty configs: %s")
	}
	//for range time.Tick(time.Duration(CHECK_DIRTY_COMMITTED_CONFIGS_TIME)) {
	//	checkForDirtyConfigs(ctx)
	//}

}
