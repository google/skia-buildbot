package main

import (
	"bytes"
	"context"
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

	CONFIG_MAP_EXTERNAL = "autoroll-config"
	CONFIG_MAP_INTERNAL = "autoroll-config-internal"

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
	updateBeImage = flag.Bool("update-be-image", false, "If true, update to the most recently uploaded backend image.")
	updateFeImage = flag.Bool("update-fe-image", false, "If true, update to the most recently uploaded frontend image.")
	updateRoller  = flag.String("update-roller", "", "If set, apply the new backend image to this roller only. Implies --update-be-image.")
)

// kubeConfGen generates the given destination Kubernetes YAML config file
// based on the given source config file, template file, and additional
// variables. Returns true if dstConfig's content changed.
func kubeConfGen(ctx context.Context, tmpl, srcConfig, dstConfig string, extraVars map[string]string) (bool, error) {
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
	if srcConfig != "" {
		cmd = append(cmd, "-c", srcConfig)
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
func kubeConfGenBe(ctx context.Context, tmpl, srcConfig, dstConfig, configFileHash, configMapName, image string) (bool, error) {
	return kubeConfGen(ctx, tmpl, srcConfig, dstConfig, map[string]string{
		"configFile":     path.Base(srcConfig),
		"configFileHash": configFileHash,
		"configMapName":  configMapName,
		"image":          image,
	})
}

// kubeConfGenFe generates the Kubernetes YAML config file for the given
// frontend instance.
func kubeConfGenFe(ctx context.Context, tmpl, srcConfig, dstConfig, configDirHash, configMapName, image string) (bool, error) {
	return kubeConfGen(ctx, tmpl, srcConfig, dstConfig, map[string]string{
		"configDirHash": configDirHash,
		"configMapName": configMapName,
		"image":         image,
	})
}

// updateConfigMap updates the Autoroll config map in Kubernetes. Returns a
// function used to roll back the config map to its previous version, in case
// of subsequent problems.
func updateConfigMap(ctx context.Context, configMapName, configDir string) (func() error, error) {
	oldContents, err := exec.RunCwd(ctx, ".", "kubectl", "get", "configmap", "-o", "yaml", configMapName)
	if err != nil {
		return nil, err
	}
	contents, err := exec.RunCwd(ctx, ".", "kubectl", "create", "configmap", configMapName, fmt.Sprintf("--from-file=%s", configDir), "-o", "yaml", "--dry-run")
	if err != nil {
		return nil, err
	}
	_, err = exec.RunCommand(ctx, &exec.Command{
		Name:  "kubectl",
		Args:  []string{"replace", "-f", "-"},
		Stdin: strings.NewReader(contents),
	})
	if err != nil {
		return nil, err
	}
	return func() error {
		_, err := exec.RunCommand(ctx, &exec.Command{
			Name:  "kubectl",
			Args:  []string{"replace", "-f", "-"},
			Stdin: strings.NewReader(oldContents),
		})
		return err
	}, nil
}

// getConfigHashes returns the sha1 sum of the given config directory and the
// sha1 sums of the individual config files themselves.
func getConfigHashes(ctx context.Context, configDir string) (string, map[string]string, error) {
	// Obtain the sha1 sums of the individual config files.
	fileInfos, err := ioutil.ReadDir(configDir)
	if err != nil {
		return "", nil, err
	}
	files := make([]string, 0, len(fileInfos))
	for _, fi := range fileInfos {
		if !fi.IsDir() {
			files = append(files, fi.Name())
		}
	}
	cfgSums := make(map[string]string, len(files))
	for _, f := range files {
		fullPath := filepath.Join(configDir, f)
		sum, err := exec.RunCwd(ctx, ".", "sha1sum", fullPath)
		if err != nil {
			return "", nil, err
		}
		cfgSums[fullPath] = strings.Fields(sum)[0]
	}

	// Obtain the sha1 sum of the config dir.
	cmd := append([]string{"tar", "-cf", "-"}, files...)
	tar, err := exec.RunCwd(ctx, configDir, cmd...)
	if err != nil {
		return "", nil, err
	}
	dirSum, err := exec.RunCommand(ctx, &exec.Command{
		Name:  "sha1sum",
		Args:  []string{"-"},
		Stdin: strings.NewReader(tar),
	})
	dirSum = strings.Fields(dirSum)[0]

	return dirSum, cfgSums, nil
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
func updateConfigs(ctx context.Context, project, configDir, k8sConfigDir, configMapName, configFileFe, latestImageFe, latestImageBe string, updateWhitelist map[string]bool) (rvErr error) {
	if err := switchCluster(ctx, project); err != nil {
		return err
	}

	// Pull some information out of the frontend config.
	var configFe struct {
		AppName string `json:"appName"`
	}
	if err := util.WithReadFile(configFileFe, func(f io.Reader) error {
		return json5.NewDecoder(f).Decode(&configFe)
	}); err != nil {
		return fmt.Errorf("Failed to decode frontend config file %s: %s", configFileFe, err)
	}

	co := &git.Checkout{git.GitDir(k8sConfigDir)}

	// Update the configMap in Kubernetes.
	dirSum, cfgSums, err := getConfigHashes(ctx, configDir)
	if err != nil {
		return err
	}
	rollbackConfigMap, err := updateConfigMap(ctx, configMapName, configDir)
	if err != nil {
		return err
	}
	defer func() {
		if rvErr != nil {
			if err := rollbackConfigMap(); err != nil {
				rvErr = fmt.Errorf("Error: %s ...and failed to roll back config map with: %s", rvErr, err)
			}
		}
	}()

	// Write the new k8s config files for the frontends.
	modified := []string{}
	tmplFe := "./go/autoroll-fe/autoroll-fe.yaml.template"

	imageFe := latestImageFe
	dstFe := filepath.Join(k8sConfigDir, configFe.AppName+".yaml")
	if _, err := os.Stat(dstFe); err == nil && !*updateFeImage {
		imageFe, err = getActiveImage(ctx, dstFe)
		if err != nil {
			return err
		}
	}
	modifiedFe, err := kubeConfGenFe(ctx, tmplFe, configFileFe, dstFe, dirSum, configMapName, imageFe)
	if err != nil {
		return err
	}
	if modifiedFe {
		modified = append(modified, filepath.Base(dstFe))
	}

	// Write the new k8s config files for the backends.
	tmplBe := "./go/autoroll-be/autoroll-be.yaml.template"
	for cfg, sum := range cfgSums {
		baseCfg := filepath.Base(cfg)
		dst := filepath.Join(k8sConfigDir, fmt.Sprintf("autoroll-be-%s.yaml", strings.Split(baseCfg, ".")[0]))

		// If the k8s file doesn't exist yet, if the user supplied the
		// --update-image flag, or if the current config is whitelisted
		// via --update-roller, use the latest image, otherwise, use
		// the currently-active image.
		image := latestImageBe
		if _, err := os.Stat(dst); err == nil && (!*updateBeImage && updateWhitelist != nil && !updateWhitelist[baseCfg]) {
			image, err = getActiveImage(ctx, dst)
			if err != nil {
				return err
			}
		}

		// Regenerate the k8s config file.
		modifiedBe, err := kubeConfGenBe(ctx, tmplBe, cfg, dst, sum, configMapName, image)
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
		if _, err := exec.RunCwd(ctx, k8sConfigDir, cmd...); err != nil {
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

// findRoller searches for roller names matching the given regex within the
// given config dir. Returns the config file names matching the regex.
func findRoller(re *regexp.Regexp, configDir string) ([]string, error) {
	dirEntries, err := ioutil.ReadDir(configDir)
	if err != nil {
		return nil, err
	}
	var matches []string
	for _, entry := range dirEntries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			// Load the config.
			var cfg roller.AutoRollerConfig
			if err := util.WithReadFile(path.Join(configDir, entry.Name()), func(f io.Reader) error {
				return json5.NewDecoder(f).Decode(&cfg)
			}); err != nil {
				return nil, err
			}
			if re.MatchString(cfg.RollerName) {
				matches = append(matches, entry.Name())
			}
		}
	}
	return matches, nil
}

func main() {
	// TODO(borenet): Need support for pushing to a single roller.
	common.Init()

	// Derive paths to config files.
	_, thisFileName, _, ok := runtime.Caller(0)
	if !ok {
		sklog.Fatal("Unable to find path to current file.")
	}
	autorollDir := filepath.Dir(filepath.Dir(filepath.Dir(thisFileName)))
	configDirExternal := filepath.Join(autorollDir, "config")

	// Find the requested roller, if provided.
	var updateWhitelist map[string]bool
	if *updateRoller != "" {
		if *external || *internal || *updateBeImage {
			sklog.Fatal("--update-roller is incompatible with --external, --internal, and --update-be-image.")
		}
		re, err := regexp.Compile(*updateRoller)
		if err != nil {
			sklog.Fatal("Invalid regex for --update-roller: %s", err)
		}
		updateWhitelist = map[string]bool{}
		if found, err := findRoller(re, configDirExternal); err != nil {
			sklog.Fatal(err)
		} else if len(found) > 0 {
			*external = true
			for _, cfgFile := range found {
				updateWhitelist[cfgFile] = true
			}
		}
		if found, err := findRoller(re, CONFIG_DIR_INTERNAL); err != nil {
			sklog.Fatal(err)
		} else if len(found) > 0 {
			*internal = true
			for _, cfgFile := range found {
				updateWhitelist[cfgFile] = true
			}
		}
		if len(updateWhitelist) == 0 {
			sklog.Fatalf("No matching rollers found for %q", *updateRoller)
		}
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
	if *external {
		configDir := filepath.Join(autorollDir, "config")
		configFe := filepath.Join(autorollDir, "go", "autoroll-fe", "cfg-public.json")
		if err := updateConfigs(ctx, PROJECT_PUBLIC, configDir, K8S_CONFIG_DIR_EXTERNAL, CONFIG_MAP_EXTERNAL, configFe, latestImageFe, latestImageBe, updateWhitelist); err != nil {
			sklog.Fatal(err)
		}
	}
	if *internal {
		configFe := filepath.Join(autorollDir, "go", "autoroll-fe", "cfg-corp.json")
		if err := updateConfigs(ctx, PROJECT_CORP, CONFIG_DIR_INTERNAL, K8S_CONFIG_DIR_INTERNAL, CONFIG_MAP_INTERNAL, configFe, latestImageFe, latestImageBe, updateWhitelist); err != nil {
			sklog.Fatal(err)
		}
	}
}
