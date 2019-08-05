package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/flynn/json5"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gcr"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	GCR_PROJECT  = PROJECT_PUBLIC
	GCR_IMAGE_BE = "autoroll-be"
	GCR_IMAGE_FE = "autoroll-fe"

	CONFIG_DIR_INTERNAL = "/tmp/skia-autoroll-internal-config"

	CONFIG_MAP_NAME_TMPL = "autoroll-config-%s"

	K8S_CONFIG_DIR_EXTERNAL = "/tmp/skia-public-config"
	K8S_CONFIG_DIR_INTERNAL = "/tmp/skia-corp-config"

	PROJECT_PUBLIC = "skia-public"
	PROJECT_CORP   = "google.com:skia-corp"
)

var (
	apply         = flag.Bool("apply", false, "If true, 'kubectl apply' the modified configs.")
	commitMsg     = flag.String("commit-with-msg", "", "If set, commit and push the modified configs with the given message.")
	external      = flag.Bool("external", false, "If true, update the external configs.")
	internal      = flag.Bool("internal", false, "If true, update the internal configs.")
	rollerRe      = flag.String("roller", "", "If set, only apply changes for rollers matching the given regex.")
	updateBeImage = flag.Bool("update-be-image", false, "If true, update to the most recently uploaded backend image.")
	updateFeImage = flag.Bool("update-fe-image", false, "If true, update to the most recently uploaded frontend image.")
)

// configDir contains information about an autoroller config dir.
type configDir struct {
	Dir          string
	FeConfigFile string
	Project      string
	K8sConfigDir string
}

// kubeConfGen generates the given destination Kubernetes YAML config file
// based on the given source config file, template file, and additional
// variables. Returns true if dstConfig's content changed.
func kubeConfGen(ctx context.Context, tmpl, dstConfig string, extraVars map[string]string, cfgFiles ...string) (bool, error) {
	oldContent, err := ioutil.ReadFile(dstConfig)
	if os.IsNotExist(err) {
		oldContent = []byte{}
	} else if err != nil {
		return false, err
	}
	cmd := []string{
		"kube-conf-gen", "-t", tmpl,
		"-o", dstConfig,
	}
	for _, cfgFile := range cfgFiles {
		cmd = append(cmd, "-c", cfgFile)
	}
	for k, v := range extraVars {
		cmd = append(cmd, "--extra", fmt.Sprintf("%s:%s", k, v))
	}
	_, err = exec.RunCwd(ctx, ".", cmd...)
	if err != nil {
		return false, err
	}
	newContent, err := ioutil.ReadFile(dstConfig)
	if os.IsNotExist(err) {
		newContent = []byte{}
	} else if err != nil {
		return false, err
	}
	return bytes.Compare(oldContent, newContent) != 0, err
}

// kubeConfGenBe generates the Kubernetes YAML config file for the given backend
// instance.
func kubeConfGenBe(ctx context.Context, tmpl, srcConfig, dstConfig, configMapName, configFileHash, image string) (bool, error) {
	return kubeConfGen(ctx, tmpl, dstConfig, map[string]string{
		"configFile":     path.Base(srcConfig),
		"configFileHash": configFileHash,
		"configMapName":  configMapName,
		"image":          image,
	}, srcConfig)
}

type config struct {
	RollerName string `json:"rollerName"`
	Hash       string `json:"hash"`
}

// kubeConfGenFe generates the Kubernetes YAML config file for the given
// frontend instance.
func kubeConfGenFe(ctx context.Context, tmpl, srcConfig, dstConfig string, configs map[string]*roller.AutoRollerConfig, cfgSums map[string]string, image string) (bool, error) {
	sumsByRollerName := map[string]string{}
	if *rollerRe != "" {
		// Read the existing dstConfig (if it exists) and parse out the
		// currently-used config hashes. This prevents rollers from
		// disappearing from the front end when we're only updating
		// configs for a subset of rollers.
		b, err := ioutil.ReadFile(dstConfig)
		if err != nil && !os.IsNotExist(err) {
			return false, err
		} else if err == nil {
			// TODO(borenet): Should we parse the config as YAML?
			for _, line := range strings.Split(string(b), "\n") {
				if strings.Contains(line, "--config=") {
					split := strings.Split(line, "/")
					if len(split) != 5 {
						return false, fmt.Errorf("Failed to parse k8s config; invalid format --config line: %s", line)
					}
					roller := split[3]
					hash := strings.Split(split[4], ".")[0]
					sumsByRollerName[roller] = hash
				}
			}
		}
	}

	// Write the config hashes to a temporary file.
	rollerNameMap := map[string]struct{}{}
	for name, _ := range sumsByRollerName {
		rollerNameMap[name] = struct{}{}
	}
	for cfgFile, cfg := range configs {
		rollerNameMap[cfg.RollerName] = struct{}{}
		sumsByRollerName[cfg.RollerName] = cfgSums[cfgFile]
	}
	rollerNames := make([]string, 0, len(rollerNameMap))
	for name := range rollerNameMap {
		rollerNames = append(rollerNames, name)
	}
	sort.Strings(rollerNames)

	cfgs := make([]config, 0, len(rollerNames))
	for _, name := range rollerNames {
		cfgs = append(cfgs, config{
			RollerName: name,
			Hash:       sumsByRollerName[name],
		})
	}
	d, err := ioutil.TempDir("", "")
	if err != nil {
		return false, err
	}
	defer util.RemoveAll(d)
	cfgsJson := filepath.Join(d, "configs.json")
	if err := util.WithWriteFile(cfgsJson, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(&struct {
			Configs []config `json:"configs"`
		}{
			Configs: cfgs,
		})
	}); err != nil {
		return false, err
	}

	return kubeConfGen(ctx, tmpl, dstConfig, map[string]string{
		"image": image,
	}, srcConfig, cfgsJson)
}

// configMap represents a config map in Kubernetes.
type configMap struct {
	ApiVersion string            `json:"apiVersion,omitempty"`
	Data       map[string]string `json:"data"`
	Kind       string            `json:"kind"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// updateConfigMap updates the Autoroll config map in Kubernetes. Previous
// versions of the config file are left in the config map to prevent unexpected
// config changes in the case of pod restarts following partially-successful
// config updates.
func updateConfigMap(ctx context.Context, configMapName, configFile, configFileHash string) error {
	// Get the existing config map.
	verb := "replace"
	var newContents []byte
	oldContents, err := exec.RunCwd(ctx, ".", "kubectl", "get", "configmap", "-o", "json", configMapName)
	if err == nil {
		// The config map already exists; insert the new data into the
		// map, using the hash as the file name.
		var cm configMap
		if err := json.Unmarshal([]byte(oldContents), &cm); err != nil {
			return err
		}
		configContent, err := ioutil.ReadFile(configFile)
		if err != nil {
			return err
		}
		cm.Data[configFileHash+".json"] = string(configContent)
		newContents, err = json.Marshal(&cm)
		if err != nil {
			return err
		}
	} else if strings.Contains(err.Error(), "not found") {
		// This is a new config map, eg. for a new roller. Generate its
		// contents using `kubectl create configmap --dry-run`.
		verb = "create"
		out, err := exec.RunCwd(ctx, ".", "kubectl", "create", "configmap", configMapName, fmt.Sprintf("--from-file=%s.json=%s", configFileHash, configFile), "-o", "json", "--dry-run")
		if err != nil {
			return err
		}
		newContents = []byte(out)
	} else {
		return err
	}

	// Apply the new config map.
	sklog.Infof("New contents:\n%s", string(newContents))
	_, err = exec.RunCommand(ctx, &exec.Command{
		Name:  "kubectl",
		Args:  []string{verb, "-f", "-"},
		Stdin: bytes.NewReader(newContents),
	})
	return err
}

// getConfigHash returns the sha1 sum of the given config file.
func getConfigHash(ctx context.Context, configFile string) (string, error) {
	out, err := exec.RunCwd(ctx, ".", "sha1sum", configFile)
	if err != nil {
		return "", err
	}
	return strings.Fields(out)[0], nil
}

// getActiveImage returns the image currently used in the given Kubernetes
// config file.
func getActiveImage(ctx context.Context, k8sCfg string) (string, error) {
	// TODO(borenet): Should we parse the config as YAML?
	b, err := ioutil.ReadFile(k8sCfg)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "image:") {
			fields := strings.Fields(line)
			if len(fields) == 2 {
				return fields[1], nil
			}
		}
	}
	return "", fmt.Errorf("Failed to find the image name from %s", k8sCfg)
}

// getLatestImage returns the most recently uploaded image.
func getLatestImage(image string) (string, error) {
	ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		return "", err
	}
	imageTags, err := gcr.NewClient(ts, GCR_PROJECT, image).Tags()
	if err != nil {
		return "", err
	}
	sort.Strings(imageTags)
	return fmt.Sprintf("gcr.io/%s/%s:%s", GCR_PROJECT, image, imageTags[len(imageTags)-1]), nil
}

// switchCluster runs the gcloud commands to switch to the given cluster.
// TODO(borenet): Share this code with pushk.
func switchCluster(ctx context.Context, project string) error {
	if _, err := exec.RunCwd(ctx, ".", "gcloud", "config", "set", "project", project); err != nil {
		return err
	}
	if _, err := exec.RunCwd(ctx, ".", "gcloud", "container", "clusters", "get-credentials", strings.TrimPrefix(project, "google.com:"), "--zone", "us-central1-a", "--project", project); err != nil {
		return err
	}
	return nil
}

// updateConfigs updates the Kubernetes config files in k8sConfigDir to reflect
// the current contents of configDir, inserting the roller configs into the
// given ConfigMap.
func updateConfigs(ctx context.Context, cfgDir configDir, latestImageFe, latestImageBe string, configs map[string]*roller.AutoRollerConfig) (rvErr error) {
	if err := switchCluster(ctx, cfgDir.Project); err != nil {
		return err
	}

	// Pull some information out of the frontend config.
	var configFe struct {
		AppName string `json:"appName"`
	}
	if err := util.WithReadFile(cfgDir.FeConfigFile, func(f io.Reader) error {
		return json5.NewDecoder(f).Decode(&configFe)
	}); err != nil {
		return fmt.Errorf("Failed to decode frontend config file %s: %s", cfgDir.FeConfigFile, err)
	}

	co := &git.Checkout{git.GitDir(cfgDir.K8sConfigDir)}

	// Update the config maps in Kubernetes.
	cfgSums := make(map[string]string, len(configs))
	for cfgFile, config := range configs {
		cfgPath := filepath.Join(cfgDir.Dir, cfgFile)
		hash, err := getConfigHash(ctx, cfgPath)
		if err != nil {
			return err
		}
		cfgSums[cfgFile] = hash
		if err := updateConfigMap(ctx, fmt.Sprintf(CONFIG_MAP_NAME_TMPL, config.RollerName), cfgPath, hash); err != nil {
			return err
		}
	}

	// Write the new k8s config files for the frontends.
	modified := []string{}
	tmplFe := "./go/autoroll-fe/autoroll-fe.yaml.template"

	imageFe := latestImageFe
	dstFe := filepath.Join(cfgDir.K8sConfigDir, configFe.AppName+".yaml")
	if _, err := os.Stat(dstFe); err == nil && !*updateFeImage {
		imageFe, err = getActiveImage(ctx, dstFe)
		if err != nil {
			return err
		}
	}
	modifiedFe, err := kubeConfGenFe(ctx, tmplFe, cfgDir.FeConfigFile, dstFe, configs, cfgSums, imageFe)
	if err != nil {
		return err
	}
	if modifiedFe {
		modified = append(modified, filepath.Base(dstFe))
	}

	// Write the new k8s config files for the backends.
	tmplBe := "./go/autoroll-be/autoroll-be.yaml.template"
	for cfgFile, config := range configs {
		dst := filepath.Join(cfgDir.K8sConfigDir, fmt.Sprintf("autoroll-be-%s.yaml", strings.Split(cfgFile, ".")[0]))

		// If the k8s file doesn't exist yet, if the user supplied the
		// --update-image flag, or if the current config is whitelisted
		// via --update-roller, use the latest image, otherwise, use
		// the currently-active image.
		image := latestImageBe
		if _, err := os.Stat(dst); err == nil {
			image, err = getActiveImage(ctx, dst)
			if err != nil {
				return err
			}
		}

		// Regenerate the k8s config file.
		cfgFilePath := filepath.Join(cfgDir.Dir, cfgFile)
		modifiedBe, err := kubeConfGenBe(ctx, tmplBe, cfgFilePath, dst, fmt.Sprintf(CONFIG_MAP_NAME_TMPL, config.RollerName), cfgSums[cfgFile], image)
		if err != nil {
			return err
		}
		if modifiedBe {
			modified = append(modified, filepath.Base(dst))
		}
	}

	if len(modified) == 0 {
		sklog.Errorf("No configs modified. Nothing to apply.")
		return nil
	}

	// Apply the modified configs.
	// TODO(borenet): Support rolling back on error.
	if *apply {
		cmd := []string{"kubectl", "apply"}
		for _, f := range modified {
			cmd = append(cmd, "-f", f)
		}
		if _, err := exec.RunCwd(ctx, cfgDir.K8sConfigDir, cmd...); err != nil {
			return err
		}
	}

	// Commit and push the modified configs.
	if *commitMsg != "" {
		cmd := append([]string{"add"}, modified...)
		if _, err := co.Git(ctx, cmd...); err != nil {
			return err
		}
		if _, err := co.Git(ctx, "commit", "-m", *commitMsg); err != nil {
			return err
		}
		if _, err := co.Git(ctx, "push", "origin", "HEAD:master"); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	common.Init()

	// Derive paths to config files.
	_, thisFileName, _, ok := runtime.Caller(0)
	if !ok {
		sklog.Fatal("Unable to find path to current file.")
	}
	autorollDir := filepath.Dir(filepath.Dir(filepath.Dir(thisFileName)))
	configDirExternal := filepath.Join(autorollDir, "config")

	// Load all configs. This a nested map whose keys are config dir paths,
	// sub-map keys are config file names, and values are roller configs.
	// Respect --roller, --internal, and --external.
	cfgDirs := []configDir{}
	rollerRegex, err := regexp.Compile(".+")
	if err != nil {
		sklog.Fatal(err)
	}
	if *rollerRe != "" {
		if *external || *internal {
			sklog.Fatal("--roller is incompatible with --external and --internal.")
		}
		*external = true
		*internal = true
		rollerRegex, err = regexp.Compile(*rollerRe)
		if err != nil {
			sklog.Fatalf("Invalid regex for --update-roller: %s", err)
		}
	}
	if *external {
		cfgDirs = append(cfgDirs, configDir{
			Dir:          configDirExternal,
			FeConfigFile: filepath.Join(autorollDir, "go", "autoroll-fe", "cfg-public.json"),
			Project:      PROJECT_PUBLIC,
			K8sConfigDir: K8S_CONFIG_DIR_EXTERNAL,
		})
	}
	if *internal {
		cfgDirs = append(cfgDirs, configDir{
			Dir:          CONFIG_DIR_INTERNAL,
			FeConfigFile: filepath.Join(autorollDir, "go", "autoroll-fe", "cfg-corp.json"),
			Project:      PROJECT_CORP,
			K8sConfigDir: K8S_CONFIG_DIR_INTERNAL,
		})
	}
	configs := map[string]map[string]*roller.AutoRollerConfig{}
	for _, dir := range cfgDirs {
		dirEntries, err := ioutil.ReadDir(dir.Dir)
		if err != nil {
			sklog.Fatal(err)
		}
		cfgsInDir := make(map[string]*roller.AutoRollerConfig, len(dirEntries))
		for _, entry := range dirEntries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				var cfg roller.AutoRollerConfig
				if err := util.WithReadFile(path.Join(dir.Dir, entry.Name()), func(f io.Reader) error {
					return json5.NewDecoder(f).Decode(&cfg)
				}); err != nil {
					sklog.Fatal(err)
				}
				if rollerRegex.MatchString(cfg.RollerName) {
					cfgsInDir[filepath.Base(entry.Name())] = &cfg
				}
			}
		}
		configs[dir.Dir] = cfgsInDir
	}

	// Get the latest images for frontend and backend.
	latestImageFe, err := getLatestImage(GCR_IMAGE_FE)
	if err != nil {
		sklog.Fatal(err)
	}
	latestImageBe, err := getLatestImage(GCR_IMAGE_BE)
	if err != nil {
		sklog.Fatal(err)
	}

	ctx := context.Background()
	for _, cfgDir := range cfgDirs {
		if err := updateConfigs(ctx, cfgDir, latestImageFe, latestImageBe, configs[cfgDir.Dir]); err != nil {
			sklog.Fatal(err)
		}
	}
}
