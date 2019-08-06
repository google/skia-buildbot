package main

import (
	"bytes"
	"context"
	"encoding/base64"
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
	// Google Container Registry project and image names used by the
	// autoroller.
	GCR_PROJECT  = PROJECT_PUBLIC
	GCR_IMAGE_BE = "autoroll-be"
	GCR_IMAGE_FE = "autoroll-fe"

	// Path to internal autoroller configs.
	CONFIG_DIR_INTERNAL = "/tmp/skia-autoroll-internal-config"

	// Config maps are named using the roller name with a constant prefix.
	CONFIG_MAP_NAME_TMPL = "autoroll-config-%s"

	// Directories containing the k8s config files.
	// TODO(borenet): Look into moving these out of /tmp, possibly with
	// support for putting them wherever a developer wants.
	K8S_CONFIG_DIR_EXTERNAL = "/tmp/skia-public-config"
	K8S_CONFIG_DIR_INTERNAL = "/tmp/skia-corp-config"

	// API version used for Kubernetes.
	K8S_API_VERSION = "v1"

	// Google Cloud projects used by the autoroller.
	PROJECT_PUBLIC = "skia-public"
	PROJECT_CORP   = "google.com:skia-corp"
)

var (
	apply              = flag.Bool("apply", false, "If true, 'kubectl apply' the modified configs.")
	commitMsg          = flag.String("commit-with-msg", "", "If set, commit and push the modified configs with the given message.")
	external           = flag.Bool("external", false, "If true, update the external configs.")
	internal           = flag.Bool("internal", false, "If true, update the internal configs.")
	rollerRe           = flag.String("roller", "", "If set, only apply changes for rollers matching the given regex.")
	updateRollerConfig = flag.Bool("update-config", false, "If true, update the roller config(s).")
	updateBeImage      = flag.Bool("update-be-image", false, "If true, update to the most recently uploaded backend image.")
	updateFeImage      = flag.Bool("update-fe-image", false, "If true, update to the most recently uploaded frontend image.")
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
func kubeConfGenBe(ctx context.Context, tmpl, srcConfig, dstConfig, configFileBase64, image string) (bool, error) {
	// Generate the k8s config.
	return kubeConfGen(ctx, tmpl, dstConfig, map[string]string{
		"configBase64": configFileBase64,
		"image":        image,
	}, srcConfig)
}

type config struct {
	RollerName string `json:"rollerName"`
	Base64     string `json:"base64"`
}

// kubeConfGenFe generates the Kubernetes YAML config file for the given
// frontend instance.
func kubeConfGenFe(ctx context.Context, tmpl, srcConfig, dstConfig string, cfgBase64ByRollerName map[string]string, image string) (bool, error) {
	// Write the config info to a temporary file.
	rollerNames := make([]string, 0, len(cfgBase64ByRollerName))
	for name := range cfgBase64ByRollerName {
		rollerNames = append(rollerNames, name)
	}
	sort.Strings(rollerNames)
	cfgs := make([]config, 0, len(rollerNames))
	for _, name := range rollerNames {
		cfgs = append(cfgs, config{
			RollerName: name,
			Base64:     cfgBase64ByRollerName[name],
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

	// Generate the k8s config.
	return kubeConfGen(ctx, tmpl, dstConfig, map[string]string{
		"image": image,
	}, srcConfig, cfgsJson)
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

// switchCluster runs the gcloud commands to switch to the given cluster, using
// a kube config file in temporary dir to avoid clobbering the user's global
// kube config. Returns the path to the kube config file and a cleanup func, or
// any error which occurred.
func switchCluster(ctx context.Context, project string) (kubecfg string, cleanup func(), rvErr error) {
	// Use a temporary dir to avoid clobbering the global kube config.
	wd, err := ioutil.TempDir("", "")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() {
		util.RemoveAll(wd)
	}
	defer func() {
		if rvErr != nil {
			cleanup()
		}
	}()
	kubecfg = filepath.Join(wd, ".kubeconfig")

	// Obtain credentials for the cluster.
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Name:        "gcloud",
		Args:        []string{"container", "clusters", "get-credentials", strings.TrimPrefix(project, "google.com:"), "--zone", "us-central1-a", "--project", project},
		Env:         []string{fmt.Sprintf("KUBECONFIG=%s", kubecfg)},
		InheritEnv:  true,
		InheritPath: true,
	}); err != nil {
		return "", nil, err
	}
	return
}

// updateConfigs updates the Kubernetes config files in k8sConfigDir to reflect
// the current contents of configDir, inserting the roller configs into the
// given ConfigMap.
func updateConfigs(ctx context.Context, cfgDir *configDir, latestImageFe, latestImageBe string, configs map[string]*roller.AutoRollerConfig) (rvErr error) {
	kubecfg, cleanup, err := switchCluster(ctx, cfgDir.Project)
	if err != nil {
		return err
	}
	defer cleanup()

	// Pull some information out of the frontend config.
	var configFe struct {
		AppName string `json:"appName"`
	}
	if err := util.WithReadFile(cfgDir.FeConfigFile, func(f io.Reader) error {
		return json5.NewDecoder(f).Decode(&configFe)
	}); err != nil {
		return fmt.Errorf("Failed to decode frontend config file %s: %s", cfgDir.FeConfigFile, err)
	}

	co := &git.Checkout{GitDir: git.GitDir(cfgDir.K8sConfigDir)}

	// Read the existing frontend k8s config file (if it exists) and parse
	// out the currently-used roller configs.
	k8sFeConfigFile := filepath.Join(cfgDir.K8sConfigDir, configFe.AppName+".yaml")
	cfgBase64ByRollerName := map[string]string{}
	b, err := ioutil.ReadFile(k8sFeConfigFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil {
		// TODO(borenet): Should we parse the config as YAML?
		for _, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, "--config=") {
				split := strings.Split(line, "--config=")
				if len(split) != 2 {
					return fmt.Errorf("Failed to parse k8s config; invalid format --config line: %s", line)
				}
				cfgBase64 := strings.TrimSuffix(strings.TrimSpace(split[1]), "\"")
				dec, err := base64.StdEncoding.DecodeString(cfgBase64)
				if err != nil {
					return fmt.Errorf("Failed to decode existing roller config as base64: %s", err)
				}
				var cfg roller.AutoRollerConfig
				if err := json.Unmarshal(dec, &cfg); err != nil {
					return fmt.Errorf("Failed to decode existing roller config as JSON: %s", err)
				}
				cfgBase64ByRollerName[cfg.RollerName] = cfgBase64
			}
		}
	}

	// Update the roller config contents if requested.
	if *updateRollerConfig {
		for _, config := range configs {
			// Note that we could re-read the config file from disk
			// and base64-encode its contents. In practice, the
			// behavior of the autoroll frontend and backends would
			// be the same, so we consider it preferable to encode
			// the parsed config, which will strip things like
			// comments and whitespace that would otherwise produce
			// a "different" config.
			b, err := json.Marshal(config)
			if err != nil {
				return err
			}
			cfgBase64ByRollerName[config.RollerName] = base64.StdEncoding.EncodeToString(b)
		}
	}

	// Write the new k8s config file for the frontend.
	modified := []string{}
	if *updateFeImage || *updateRollerConfig {
		tmplFe := "./go/autoroll-fe/autoroll-fe.yaml.template"
		imageFe := latestImageFe
		dstFe := filepath.Join(cfgDir.K8sConfigDir, configFe.AppName+".yaml")
		if _, err := os.Stat(dstFe); err == nil && !*updateFeImage {
			imageFe, err = getActiveImage(ctx, dstFe)
			if err != nil {
				return err
			}
		}
		modifiedFe, err := kubeConfGenFe(ctx, tmplFe, cfgDir.FeConfigFile, dstFe, cfgBase64ByRollerName, imageFe)
		if err != nil {
			return err
		}
		if modifiedFe {
			modified = append(modified, filepath.Base(dstFe))
		}
	}

	// Write the new k8s config files for the backends.
	if *updateBeImage || *updateRollerConfig {
		tmplBe := "./go/autoroll-be/autoroll-be.yaml.template"
		for cfgFile, config := range configs {
			dst := filepath.Join(cfgDir.K8sConfigDir, fmt.Sprintf("autoroll-be-%s.yaml", strings.Split(cfgFile, ".")[0]))

			// If the k8s file doesn't exist yet or the user supplied the
			// --update-be-image flag, use the latest image. Otherwise use
			// the currently-active image.
			image := latestImageBe
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				// Do nothing, ie. use the latest image even if
				// --update-be-image was not provided.
				if !*updateBeImage {
					sklog.Warningf("--update-be-image was not provided, but destination config file %q does not exist. Defaulting to use the latest image: %s", dst, latestImageBe)
				}
			} else if err != nil {
				return err
			} else if !*updateBeImage {
				image, err = getActiveImage(ctx, dst)
				if err != nil {
					return err
				}
			}

			// Regenerate the k8s config file.
			cfgFileBase64 := cfgBase64ByRollerName[config.RollerName]
			cfgFilePath := filepath.Join(cfgDir.Dir, cfgFile)
			modifiedBe, err := kubeConfGenBe(ctx, tmplBe, cfgFilePath, dst, cfgFileBase64, image)
			if err != nil {
				return err
			}
			if modifiedBe {
				modified = append(modified, filepath.Base(dst))
			}
		}
	}

	if !*apply {
		return nil
	} else if len(modified) == 0 {
		sklog.Warningf("No configs modified in %s. Nothing to apply.", cfgDir.Project)
		return nil
	}

	// Apply the modified configs.
	// TODO(borenet): Support rolling back on error.
	args := []string{"apply"}
	for _, f := range modified {
		args = append(args, "-f", f)
	}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Name:        "kubectl",
		Args:        args,
		Dir:         cfgDir.K8sConfigDir,
		Env:         []string{fmt.Sprintf("KUBECONFIG=%s", kubecfg)},
		InheritEnv:  true,
		InheritPath: true,
	}); err != nil {
		return err
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

	if !*updateRollerConfig && !*updateBeImage && !*updateFeImage {
		sklog.Fatal("One or more of --update-config, --update-be-image, or --update-fe-image is required.")
	}
	if *rollerRe == "" && !*external && !*internal {
		sklog.Fatal("One of --roller, --external, or --internal is required.")
	}

	// Derive paths to config files.
	_, thisFileName, _, ok := runtime.Caller(0)
	if !ok {
		sklog.Fatal("Unable to find path to current file.")
	}
	autorollDir := filepath.Dir(filepath.Dir(filepath.Dir(thisFileName)))
	configDirExternal := filepath.Join(autorollDir, "config")

	// Determine where to look for roller configs.
	var rollerRegex *regexp.Regexp
	if *rollerRe != "" {
		if *external || *internal {
			sklog.Fatal("--roller is incompatible with --external and --internal.")
		}
		*external = true
		*internal = true
		var err error
		rollerRegex, err = regexp.Compile(*rollerRe)
		if err != nil {
			sklog.Fatalf("Invalid regex for --update-roller: %s", err)
		}
	}
	cfgDirs := []*configDir{}
	if *external {
		cfgDirs = append(cfgDirs, &configDir{
			Dir:          configDirExternal,
			FeConfigFile: filepath.Join(autorollDir, "go", "autoroll-fe", "cfg-public.json"),
			Project:      PROJECT_PUBLIC,
			K8sConfigDir: K8S_CONFIG_DIR_EXTERNAL,
		})
	}
	if *internal {
		cfgDirs = append(cfgDirs, &configDir{
			Dir:          CONFIG_DIR_INTERNAL,
			FeConfigFile: filepath.Join(autorollDir, "go", "autoroll-fe", "cfg-corp.json"),
			Project:      PROJECT_CORP,
			K8sConfigDir: K8S_CONFIG_DIR_INTERNAL,
		})
	}

	// Load all configs. This a nested map whose keys are config dir paths,
	// sub-map keys are config file names, and values are roller configs.
	configs := map[*configDir]map[string]*roller.AutoRollerConfig{}
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
				if rollerRegex == nil || rollerRegex.MatchString(cfg.RollerName) {
					cfgsInDir[filepath.Base(entry.Name())] = &cfg
				}
			}
		}
		if len(cfgsInDir) == 0 {
			sklog.Infof("No matching rollers in %s. Skipping.", dir.Dir)
		} else {
			configs[dir] = cfgsInDir
		}
	}
	if len(configs) == 0 {
		sklog.Errorf("Found no rollers matching %q", *rollerRe)
		os.Exit(1)
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
	for cfgDir, cfgs := range configs {
		if err := updateConfigs(ctx, cfgDir, latestImageFe, latestImageBe, cfgs); err != nil {
			sklog.Fatal(err)
		}
	}
}
