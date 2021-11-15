package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/flynn/json5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gcr"
	"go.skia.org/infra/go/gerrit/rubberstamper"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
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

	// Directory containing the k8s config files.
	// TODO(borenet): Look into moving this out of /tmp, possibly with
	// support for putting it wherever a developer wants.
	DEFAULT_K8S_CONFIG_DIR = "/tmp/k8s-config"

	// Repo containing the k8s config files.
	K8S_CONFIG_REPO = "https://skia.googlesource.com/k8s-config.git"

	// Google Cloud projects used by the autoroller.
	PROJECT_PUBLIC = "skia-public"
	PROJECT_CORP   = "google.com:skia-corp"

	// Parent repo name for Google3 rollers.
	GOOGLE3_PARENT_NAME = "Google3"
)

var (
	apply              = flag.Bool("apply", false, "If true, 'kubectl apply' the modified configs.")
	commitMsg          = flag.String("commit-with-msg", "", "If set, commit and push the changes in Git, using the given message. Implies --apply.")
	rollerRe           = flag.String("roller", "", "If set, only apply changes for rollers matching the given regex.")
	updateRollerConfig = flag.Bool("update-config", false, "If true, update the roller config(s).")
	updateBeImage      = flag.Bool("update-be-image", false, "If true, update to the most recently uploaded backend image.")
	updateFeImage      = flag.Bool("update-fe-image", false, "If true, update to the most recently uploaded frontend image.")
	useTmpCheckout     = flag.Bool("use-tmp-checkout", false, "If true, use a temporary checkout of the k8s config repo. Only valid with --commit-with-msg")
)

// configDir contains information about an autoroller config dir.
type configDir struct {
	Dir          string
	FeConfigFile string
	Project      string
	ClusterName  string
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

type rollerConfig struct {
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
	cfgs := make([]rollerConfig, 0, len(rollerNames))
	for _, name := range rollerNames {
		cfgs = append(cfgs, rollerConfig{
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
			Configs []rollerConfig `json:"configs"`
		}{
			Configs: cfgs,
		})
	}); err != nil {
		return false, skerr.Wrapf(err, "failed kube-conf-gen")
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
		return "", skerr.Wrapf(err, "failed to read k8s config file %s", k8sCfg)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "image:") {
			fields := strings.Fields(line)
			if len(fields) == 2 {
				return fields[1], nil
			}
		}
	}
	return "", skerr.Fmt("Failed to find the image name from %s", k8sCfg)
}

// getLatestImage returns the most recently uploaded image.
func getLatestImage(image string) (string, error) {
	ts, err := auth.NewDefaultTokenSource(true, auth.ScopeUserinfoEmail)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to get latest image for %s; failed to get token source", image)
	}
	imageTags, err := gcr.NewClient(ts, GCR_PROJECT, image).Tags()
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to get latest image for %s; failed to get tags", image)
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
		return "", nil, skerr.Wrapf(err, "Failed to switch cluster; failed to create temp dir")
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
		return "", nil, skerr.Wrapf(err, "Failed to switch cluster")
	}
	return
}

// updateConfigs updates the Kubernetes config files in k8sConfigDir to reflect
// the current contents of configDir, inserting the roller configs into the
// given ConfigMap.
func updateConfigs(ctx context.Context, co *git.Checkout, cfgDir *configDir, latestImageFe, latestImageBe string, configs map[string]*config.Config) ([]string, error) {
	// This is the subdir for the current cluster.
	clusterCfgDir := filepath.Join(co.Dir(), cfgDir.ClusterName)

	// Pull some information out of the frontend config.
	var configFe struct {
		AppName string `json:"appName"`
	}
	if err := util.WithReadFile(cfgDir.FeConfigFile, func(f io.Reader) error {
		return json5.NewDecoder(f).Decode(&configFe)
	}); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode frontend config file %s", cfgDir.FeConfigFile)
	}

	// Read the existing frontend k8s config file (if it exists) and parse
	// out the currently-used roller configs.
	k8sFeConfigFile := filepath.Join(clusterCfgDir, configFe.AppName+".yaml")
	cfgBase64ByRollerName := map[string]string{}
	b, err := ioutil.ReadFile(k8sFeConfigFile)
	if err != nil && !os.IsNotExist(err) {
		return nil, skerr.Wrapf(err, "failed to read k8s config file for frontend")
	} else if err == nil {
		// TODO(borenet): Should we parse the config as YAML?
		for _, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, "--config=") {
				split := strings.Split(line, "--config=")
				if len(split) != 2 {
					return nil, skerr.Fmt("Failed to parse k8s config; invalid format --config line: %s", line)
				}
				cfgBase64 := strings.TrimSuffix(strings.TrimSpace(split[1]), "\"")
				dec, err := base64.StdEncoding.DecodeString(cfgBase64)
				if err != nil {
					return nil, skerr.Fmt("Failed to decode existing roller config as base64: %s", err)
				}
				opts := prototext.UnmarshalOptions{
					AllowPartial:   true,
					DiscardUnknown: true,
				}
				cfg := new(config.Config)
				if err := opts.Unmarshal(dec, cfg); err != nil {
					return nil, skerr.Wrapf(err, "failed to decode existing roller config")
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
			b, err := prototext.MarshalOptions{
				AllowPartial: true,
				EmitUnknown:  true,
			}.Marshal(config)
			if err != nil {
				return nil, skerr.Wrapf(err, "Failed to encode roller config as text proto")
			}
			cfgBase64ByRollerName[config.RollerName] = base64.StdEncoding.EncodeToString(b)
		}
	}

	// Write the new k8s config file for the frontend.
	modified := []string{}
	if *updateFeImage || *updateRollerConfig {
		tmplFe := "./go/autoroll-fe/autoroll-fe.yaml.template"
		imageFe := latestImageFe
		dstFe := filepath.Join(clusterCfgDir, configFe.AppName+".yaml")
		if _, err := os.Stat(dstFe); err == nil && !*updateFeImage {
			imageFe, err = getActiveImage(ctx, dstFe)
			if err != nil {
				return nil, skerr.Wrapf(err, "Failed to get active image for frontend")
			}
		}
		modifiedFe, err := kubeConfGenFe(ctx, tmplFe, cfgDir.FeConfigFile, dstFe, cfgBase64ByRollerName, imageFe)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to generate k8s config for frontend")
		}
		if modifiedFe {
			modified = append(modified, filepath.Base(dstFe))
		}
	}

	// Write the new k8s config files for the backends.
	if *updateBeImage || *updateRollerConfig {
		tmplBe := "./go/autoroll-be/autoroll-be.yaml.template"
		for cfgFile, config := range configs {
			// Google3 uses a different type of backend.
			if config.ParentDisplayName == GOOGLE3_PARENT_NAME {
				continue
			}
			dst := filepath.Join(clusterCfgDir, fmt.Sprintf("autoroll-be-%s.yaml", strings.Split(cfgFile, ".")[0]))

			// If the k8s file doesn't exist yet or the user supplied the
			// --update-be-image flag, use the latest image. Otherwise use
			// the currently-active image.
			image := latestImageBe
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				// Do nothing, ie. use the latest image even if
				// --update-be-image was not provided.
				if !*updateBeImage {
					fmt.Fprintf(os.Stderr, "--update-be-image was not provided, but destination config file %q does not exist. Defaulting to use the latest image: %s\n", dst, latestImageBe)
				}
			} else if err != nil {
				return nil, skerr.Wrapf(err, "Failed to read backend k8s config file %s", dst)
			} else if !*updateBeImage {
				image, err = getActiveImage(ctx, dst)
				if err != nil {
					return nil, skerr.Wrapf(err, "Failed to get active image for backend")
				}
			}

			// kube-conf-gen wants a JSON version of the config. Write it to a
			// temporary directory.
			tmp, err := ioutil.TempDir("", "")
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			defer func() {
				if err := os.RemoveAll(tmp); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to remove temp dir: %s", err)
				}
			}()
			configBytes, err := protojson.MarshalOptions{
				AllowPartial:    true,
				EmitUnpopulated: true,
			}.Marshal(config)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			cfgFilePath := filepath.Join(tmp, "cfg.json")
			if err := ioutil.WriteFile(cfgFilePath, configBytes, os.ModePerm); err != nil {
				return nil, skerr.Wrap(err)
			}

			// Regenerate the k8s config file.
			cfgFileBase64 := cfgBase64ByRollerName[config.RollerName]
			modifiedBe, err := kubeConfGenBe(ctx, tmplBe, cfgFilePath, dst, cfgFileBase64, image)
			if err != nil {
				return nil, skerr.Wrapf(err, "Failed to generate k8s config file for backend: %s", dst)
			}
			if modifiedBe {
				modified = append(modified, filepath.Base(dst))
			}
		}
	}

	return modified, nil
}

// flagWasSet returns true iff the given flag was set.
func flagWasSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func main() {
	common.Init()

	// Validate flags.
	if !*updateRollerConfig && !*updateBeImage && !*updateFeImage {
		log.Fatal("One or more of --update-config, --update-be-image, or --update-fe-image is required.")
	}
	if flagWasSet("roller") && *rollerRe == "" {
		// This is almost certainly a mistake.
		log.Fatal("--roller was set to an empty string.")
	}
	if flagWasSet("commit-with-msg") {
		if *commitMsg == "" {
			r := bufio.NewReader(os.Stdin)
			fmt.Println("--commit-with-msg was specified but is empty. Please enter a commit message, followed by EOF (ctrl+D):")
			msg, err := ioutil.ReadAll(r)
			if err != nil {
				log.Fatalf("Failed to read commit message from stdin: %s", err)
			}
			*commitMsg = string(msg)
		}
		// --commit-with-msg implies --apply.
		*apply = true
	} else if *useTmpCheckout {
		log.Fatal("--use-tmp-checkout is only valid with --commit-with-msg.")
	}

	// Derive paths to config files.
	_, thisFileName, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Unable to find path to current file.")
	}
	autorollDir := filepath.Dir(filepath.Dir(filepath.Dir(thisFileName)))
	configDirExternal := filepath.Join(autorollDir, "config")

	// Determine where to look for roller configs.
	var rollerRegex *regexp.Regexp
	if *rollerRe != "" {
		var err error
		rollerRegex, err = regexp.Compile(*rollerRe)
		if err != nil {
			log.Fatalf("Invalid regex for --roller: %s", err)
		}
	}
	// TODO(borenet): We should use the go/kube/clusterconfig package.
	cfgDirs := []*configDir{
		{
			Dir:          configDirExternal,
			FeConfigFile: filepath.Join(autorollDir, "go", "autoroll-fe", "cfg-public.json"),
			Project:      PROJECT_PUBLIC,
			ClusterName:  "skia-public",
		},
		{
			Dir:          filepath.Join(CONFIG_DIR_INTERNAL, "skia-corp"),
			FeConfigFile: filepath.Join(autorollDir, "go", "autoroll-fe", "cfg-corp.json"),
			Project:      PROJECT_CORP,
			ClusterName:  "skia-corp",
		},
	}

	// Load all configs. This a nested map whose keys are config dir paths,
	// sub-map keys are config file names, and values are roller configs.
	configs := map[*configDir]map[string]*config.Config{}
	for _, dir := range cfgDirs {
		dirEntries, err := ioutil.ReadDir(dir.Dir)
		if err != nil {
			log.Fatalf("Failed to read roller configs in %s: %s", dir, err)
		}
		cfgsInDir := make(map[string]*config.Config, len(dirEntries))
		for _, entry := range dirEntries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".cfg") {
				cfgPath := filepath.Join(dir.Dir, entry.Name())
				var cfg config.Config
				if err := util.WithReadFile(cfgPath, func(f io.Reader) error {
					configBytes, err := ioutil.ReadAll(f)
					if err != nil {
						return err
					}
					if err := prototext.Unmarshal(configBytes, &cfg); err != nil {
						return err
					}
					return nil
				}); err != nil {
					log.Fatalf("Failed to parse roller config %s: %s", cfgPath, err)
				}
				if rollerRegex == nil || rollerRegex.MatchString(cfg.RollerName) {
					if err := cfg.Validate(); err != nil {
						log.Fatalf("%s is invalid: %s", cfgPath, err)
					}
					cfgsInDir[filepath.Base(entry.Name())] = &cfg
				}
			}
		}
		if len(cfgsInDir) == 0 {
			fmt.Println(fmt.Sprintf("No matching rollers in %s. Skipping.", dir.Dir))
		} else {
			configs[dir] = cfgsInDir
		}
	}
	if len(configs) == 0 {
		log.Fatalf("Found no rollers matching %q", *rollerRe)
	}

	// Get the latest images for frontend and backend.
	latestImageFe, err := getLatestImage(GCR_IMAGE_FE)
	if err != nil {
		log.Fatalf("Failed to get latest image for %s: %s", GCR_IMAGE_FE, err)
	}
	latestImageBe, err := getLatestImage(GCR_IMAGE_BE)
	if err != nil {
		log.Fatalf("Failed to get latest image for %s: %s", GCR_IMAGE_BE, err)
	}

	// Find or create the checkout.
	ctx := context.Background()
	var co *git.Checkout
	if *useTmpCheckout {
		c, err := git.NewTempCheckout(ctx, K8S_CONFIG_REPO)
		if err != nil {
			log.Fatalf("Failed to create temporary checkout: %s", err)
		}
		defer c.Delete()
		co = (*git.Checkout)(c)
	} else {
		co = &git.Checkout{GitDir: git.GitDir(DEFAULT_K8S_CONFIG_DIR)}
	}

	// Update the configs.
	modByDir := make(map[*configDir][]string, len(configs))
	for cfgDir, cfgs := range configs {
		modified, err := updateConfigs(ctx, co, cfgDir, latestImageFe, latestImageBe, cfgs)
		if err != nil {
			log.Fatalf("Failed to update configs: %s", err)
		}
		if len(modified) > 0 {
			modFullPaths := make([]string, 0, len(modified))
			fmt.Println(fmt.Sprintf("Modified the following files in %s:", cfgDir.ClusterName))
			for _, f := range modified {
				fmt.Println(fmt.Sprintf("  %s", f))
				modFullPaths = append(modFullPaths, filepath.Join(cfgDir.ClusterName, f))
			}
			modByDir[cfgDir] = modFullPaths
		} else {
			fmt.Fprintf(os.Stderr, "No configs modified in %s.\n", cfgDir.ClusterName)
		}
	}

	// Apply the modified configs.
	if !*apply || len(modByDir) == 0 {
		return
	}

	// TODO(borenet): Support rolling back on error.
	for cfgDir, modified := range modByDir {
		kubecfg, cleanup, err := switchCluster(ctx, cfgDir.Project)
		if err != nil {
			log.Fatalf("Failed to update k8s configs: %s", err)
		}
		defer cleanup()
		args := []string{"apply"}
		for _, f := range modified {
			args = append(args, "-f", f)
		}
		if _, err := exec.RunCommand(ctx, &exec.Command{
			Name:        "kubectl",
			Args:        args,
			Dir:         co.Dir(),
			Env:         []string{fmt.Sprintf("KUBECONFIG=%s", kubecfg)},
			InheritEnv:  true,
			InheritPath: true,
		}); err != nil {
			log.Fatalf("Failed to apply k8s config file(s) in %s: %s", co.Dir(), err)
		}
	}

	// Commit and push the modified configs.
	if *commitMsg != "" {
		for _, modified := range modByDir {
			cmd := append([]string{"add"}, modified...)
			if _, err := co.Git(ctx, cmd...); err != nil {
				log.Fatalf("Failed to 'git add' k8s config file(s): %s", err)
			}
			msg := *commitMsg + "\n\n" + rubberstamper.RandomChangeID()
			if _, err := co.Git(ctx, "commit", "-m", msg); err != nil {
				log.Fatalf("Failed to 'git commit' k8s config file(s): %s", err)
			}
			if _, err := co.Git(ctx, "push", git.DefaultRemote, rubberstamper.PushRequestAutoSubmit); err != nil {
				// The upstream might have changed while we were
				// working. Rebase and try again.
				if err2 := co.Fetch(ctx); err2 != nil {
					log.Fatalf("Failed to push with %q and failed to fetch with %q", err, err2)
				}
				if _, err2 := co.Git(ctx, "rebase"); err2 != nil {
					log.Fatalf("Failed to push with %q and failed to rebase with %q", err, err2)
				}
			}
		}
	}
}
