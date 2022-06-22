package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
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
	"golang.org/x/oauth2/google"
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

	// Path to autoroller config repo.
	// TODO(borenet): Support an arbitrary location.
	CONFIG_REPO_LOCATION = "/tmp/skia-autoroll-internal-config"
	CONFIG_REPO_URL      = "https://skia.googlesource.com/skia-autoroll-internal-config.git"

	// Repo containing the k8s config files.
	K8S_CONFIG_REPO = "https://skia.googlesource.com/k8s-config.git"

	// Google Cloud projects used by the autoroller.
	PROJECT_PUBLIC = "skia-public"
	PROJECT_CORP   = "google.com:skia-corp"

	// Parent repo name for Google3 rollers.
	GOOGLE3_PARENT_NAME = "Google3"
)

var (
	// Flags.
	commitMsg          = flag.String("commit-msg", "", "If set, commit and push the changes in Git, using the given message. Implies --apply.")
	rollerRe           = flag.String("roller", "", "If set, only apply changes for rollers matching the given regex.")
	updateRollerConfig = flag.Bool("update-config", false, "If true, update the roller config(s).")
	updateBeImage      = flag.Bool("update-be-image", false, "If true, update to the most recently uploaded backend image.")
	updateFeImage      = flag.Bool("update-fe-image", false, "If true, update to the most recently uploaded frontend image.")

	// Regular expression used to replace the Docker image version in config
	// files.
	dockerImageRe = regexp.MustCompile(`(?m)^\s*image:\s*"\S+"\s*$`)

	oldClusters = []string{
		"skia-corp",
		"skia-public",
	}
)

// clusterCfg contains information about a cluster which runs autorollers.
type clusterCfg struct {
	FeConfigFile string
	Project      string
	Name         string
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
func kubeConfGenBe(ctx context.Context, tmpl, srcConfig, dstConfig, configFileBase64 string) (bool, error) {
	// Generate the k8s config.

	// Temporary measure to help transition over to the new cluster(s).
	isOldCluster := "false"
	splitRelPath := strings.Split(dstConfig, string(filepath.Separator))
	for _, oldCluster := range oldClusters {
		if util.In(oldCluster, splitRelPath) {
			isOldCluster = "true"
			break
		}
	}
	return kubeConfGen(ctx, tmpl, dstConfig, map[string]string{
		"configBase64": configFileBase64,
		"oldCluster":   isOldCluster,
	}, srcConfig)
}

type rollerConfig struct {
	RollerName string `json:"rollerName"`
	Base64     string `json:"base64"`
}

// kubeConfGenFe generates the Kubernetes YAML config file for the given
// frontend instance.
func kubeConfGenFe(ctx context.Context, tmpl, srcConfig, dstConfig string, image string) (bool, error) {
	// Generate the k8s config.
	return kubeConfGen(ctx, tmpl, dstConfig, map[string]string{
		"image": image,
	}, srcConfig)
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
func getLatestImage(ctx context.Context, image string) (string, error) {
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to get latest image for %s; failed to get token source", image)
	}
	tagsResp, err := gcr.NewClient(ts, GCR_PROJECT, image).Tags(ctx)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to get latest image for %s; failed to get tags", image)
	}
	imageTags := tagsResp.Tags
	sort.Strings(imageTags)
	if len(imageTags) == 0 {
		return "", skerr.Fmt("No image tags returned for %s", image)
	}
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
func updateConfigs(ctx context.Context, co *git.Checkout, cluster *clusterCfg, configs map[string]*config.Config) ([]string, error) {
	// This is the subdir for the current cluster.
	clusterCfgDir := filepath.Join(co.Dir(), cluster.Name)

	// Pull some information out of the frontend config.
	var configFe struct {
		AppName string `json:"appName"`
	}
	if err := util.WithReadFile(cluster.FeConfigFile, func(f io.Reader) error {
		return json5.NewDecoder(f).Decode(&configFe)
	}); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode frontend config file %s", cluster.FeConfigFile)
	}

	// Write the new k8s config file for the frontend.
	modified := []string{}
	if *updateFeImage {
		tmplFe := "./go/autoroll-fe/autoroll-fe.yaml.template"
		// Get the latest image for the frontend.
		latestImageFe, err := getLatestImage(ctx, GCR_IMAGE_FE)
		if err != nil {
			log.Fatalf("Failed to get latest image for %s: %s", configFe.AppName, err)
		}
		imageFe := latestImageFe
		dstFe := filepath.Join(clusterCfgDir, configFe.AppName+".yaml")
		if _, err := os.Stat(dstFe); err == nil && !*updateFeImage {
			imageFe, err = getActiveImage(ctx, dstFe)
			if err != nil {
				return nil, skerr.Wrapf(err, "Failed to get active image for frontend")
			}
		}
		modifiedFe, err := kubeConfGenFe(ctx, tmplFe, cluster.FeConfigFile, dstFe, imageFe)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to generate k8s config for frontend")
		}
		if modifiedFe {
			modified = append(modified, filepath.Base(dstFe))
		}
	}

	// Write the new k8s config files for the backends.
	if *updateBeImage || *updateRollerConfig {
		tmplBe := "./go/autoroll-config-converter/autoroll-be.yaml.template"
		for cfgFile, config := range configs {
			// Google3 uses a different type of backend.
			if config.ParentDisplayName == GOOGLE3_PARENT_NAME {
				continue
			}
			dst := filepath.Join(clusterCfgDir, fmt.Sprintf("autoroll-be-%s.yaml", strings.Split(cfgFile, ".")[0]))

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

			// Encode the roller config file.
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
			cfgFileBase64 := base64.StdEncoding.EncodeToString(b)

			modifiedBe, err := kubeConfGenBe(ctx, tmplBe, cfgFilePath, dst, cfgFileBase64)
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

	ctx := context.Background()

	// Validate flags.
	if !*updateRollerConfig && !*updateBeImage && !*updateFeImage {
		log.Fatal("One or more of --update-config, --update-be-image, or --update-fe-image is required.")
	}
	if flagWasSet("roller") && *rollerRe == "" {
		// This is almost certainly a mistake.
		log.Fatal("--roller was set to an empty string.")
	}
	if *commitMsg == "" {
		r := bufio.NewReader(os.Stdin)
		fmt.Println("--commit-msg was not provided. Please enter a commit message, followed by EOF (ctrl+D):")
		msg, err := ioutil.ReadAll(r)
		if err != nil {
			log.Fatalf("Failed to read commit message from stdin: %s", err)
		}
		*commitMsg = string(msg)
	}

	// Derive paths to config files.
	_, thisFileName, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Unable to find path to current file.")
	}
	autorollDir := filepath.Dir(filepath.Dir(filepath.Dir(thisFileName)))

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
	clusters := []*clusterCfg{
		{
			FeConfigFile: filepath.Join(autorollDir, "go", "autoroll-fe", "cfg-public.json"),
			Project:      PROJECT_PUBLIC,
			Name:         "skia-public",
		},
		{
			FeConfigFile: filepath.Join(autorollDir, "go", "autoroll-fe", "cfg-corp.json"),
			Project:      PROJECT_CORP,
			Name:         "skia-corp",
		},
	}

	// Ensure that the config repo is present.
	configCo := &git.Checkout{GitDir: git.GitDir(CONFIG_REPO_LOCATION)}
	if _, err := os.Stat(configCo.Dir()); os.IsNotExist(err) {
		if err := git.Clone(ctx, CONFIG_REPO_URL, configCo.Dir(), false); err != nil {
			log.Fatalf("Failed to clone config repo: %s", err)
		}
	}
	if err := configCo.Fetch(ctx); err != nil {
		log.Fatal(err)
	}
	configRepoIsDirty, configRepoStatus, err := configCo.IsDirty(ctx)
	if err != nil {
		log.Fatal(err)
	} else if configRepoIsDirty {
		fmt.Println(configRepoStatus)
		fmt.Println(fmt.Sprintf("Checkout in %s is dirty; are you sure you want to use it? (y/n): ", configCo.Dir()))
		reader := bufio.NewReader(os.Stdin)
		read, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		read = strings.TrimSpace(read)
		if read != "y" {
			os.Exit(1)
		}
	}

	// Load all configs. This a nested map whose keys are config dir paths,
	// sub-map keys are config file names, and values are roller configs.
	configs := map[*clusterCfg]map[string]*config.Config{}
	for _, cluster := range clusters {
		dirEntries, err := ioutil.ReadDir(filepath.Join(configCo.Dir(), cluster.Name))
		if err != nil {
			log.Fatalf("Failed to read roller configs in %s: %s", cluster, err)
		}
		cfgsInDir := make(map[string]*config.Config, len(dirEntries))
		for _, entry := range dirEntries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".cfg") {
				cfgPath := filepath.Join(configCo.Dir(), cluster.Name, entry.Name())
				cfgBytes, err := ioutil.ReadFile(cfgPath)
				if err != nil {
					log.Fatalf("Failed to read roller config %s: %s", cfgPath, err)
				}
				var cfg config.Config
				if err := prototext.Unmarshal(cfgBytes, &cfg); err != nil {
					log.Fatalf("Failed to parse roller config: %s", err)
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
			fmt.Println(fmt.Sprintf("No matching rollers in %s. Skipping.", cluster.Name))
		} else {
			configs[cluster] = cfgsInDir
		}
	}
	if len(configs) == 0 {
		log.Fatalf("Found no rollers matching %q", *rollerRe)
	}

	// Update the backend image if requested.
	if *updateBeImage {
		latestImageBe, err := getLatestImage(ctx, GCR_IMAGE_BE)
		if err != nil {
			log.Fatalf("Failed to get latest image for %s: %s", GCR_IMAGE_BE, err)
		}
		imageBytes := []byte(fmt.Sprintf("  image:  %q", latestImageBe))
		for cluster, byCluster := range configs {
			for filename, config := range byCluster {
				// Update the config files on disk.
				cfgPath := filepath.Join(configCo.Dir(), cluster.Name, filename)
				cfgBytes, err := ioutil.ReadFile(cfgPath)
				if err != nil {
					log.Fatalf("Failed to read roller config %s: %s", cfgPath, err)
				}
				cfgBytes = dockerImageRe.ReplaceAll(cfgBytes, imageBytes)
				if err := ioutil.WriteFile(cfgPath, cfgBytes, os.ModePerm); err != nil {
					log.Fatalf("Failed to write roller config %s: %s", cfgPath, err)
				}

				// Update the configs we already read.
				config.Kubernetes.Image = latestImageBe
			}
		}
		// If the repo is not dirty, commit and push the updated configs.
		if !configRepoIsDirty {
			msg := *commitMsg + "\n\n" + rubberstamper.RandomChangeID()
			if _, err := configCo.Git(ctx, "commit", "-a", "-m", msg); err != nil {
				log.Fatalf("Failed to 'git commit' k8s config file(s): %s", err)
			}
			if _, err := configCo.Git(ctx, "push", git.DefaultRemote, rubberstamper.PushRequestAutoSubmit); err != nil {
				// The upstream might have changed while we were
				// working. Rebase and try again.
				if err2 := configCo.Fetch(ctx); err2 != nil {
					log.Fatalf("Failed to push with %q and failed to fetch with %q", err, err2)
				}
				if _, err2 := configCo.Git(ctx, "rebase"); err2 != nil {
					log.Fatalf("Failed to push with %q and failed to rebase with %q", err, err2)
				}
			}
			// Remove the commit we just made, so that we aren't leaving the
			// checkout in a "dirty" state.
			if _, err := configCo.Git(ctx, "reset", "--hard", git.DefaultRemoteBranch); err != nil {
				log.Fatalf("Failed to cleanup: %s", err)
			}
		} else {
			fmt.Println(fmt.Sprintf("Updated config files in %s but not committing because of dirty checkout.", configCo.Dir()))
		}
	}

	// Create the checkout.
	co, err := git.NewTempCheckout(ctx, K8S_CONFIG_REPO)
	if err != nil {
		log.Fatalf("Failed to create temporary checkout: %s", err)
	}
	defer co.Delete()

	// Update the configs.
	modByCluster := make(map[*clusterCfg][]string, len(configs))
	for cluster, cfgs := range configs {
		modified, err := updateConfigs(ctx, co.Checkout, cluster, cfgs)
		if err != nil {
			log.Fatalf("Failed to update configs: %s", err)
		}
		if len(modified) > 0 {
			modFullPaths := make([]string, 0, len(modified))
			fmt.Println(fmt.Sprintf("Modified the following files in %s:", cluster.Name))
			for _, f := range modified {
				fmt.Println(fmt.Sprintf("  %s", f))
				modFullPaths = append(modFullPaths, filepath.Join(cluster.Name, f))
			}
			modByCluster[cluster] = modFullPaths
		} else {
			fmt.Fprintf(os.Stderr, "No configs modified in %s.\n", cluster.Name)
		}
	}

	// TODO(borenet): Support rolling back on error.
	for cluster, modified := range modByCluster {
		kubecfg, cleanup, err := switchCluster(ctx, cluster.Project)
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
		for _, modified := range modByCluster {
			cmd := append([]string{"add"}, modified...)
			if _, err := co.Git(ctx, cmd...); err != nil {
				log.Fatalf("Failed to 'git add' k8s config file(s): %s", err)
			}
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
