package main

import (
	"context"
	"flag"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
)

const (
	livenessMetric       = "k8s_deployer"
	gitstoreSubscriberID = "k8s-deployer"
)

func main() {
	configRepo := flag.String("config_repo", "https://skia.googlesource.com/k8s-config.git", "Repo containing Kubernetes configurations.")
	configSubdir := flag.String("config_subdir", "", "Subdirectory within the config repo to apply to this cluster.")
	interval := flag.Duration("interval", 10*time.Minute, "How often to re-apply configurations to the cluster")
	port := flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	promPort := flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	prune := flag.Bool("prune", false, "Whether to run 'kubectl apply' with '--prune'")
	kubectl := flag.String("kubectl", "kubectl", "Path to the kubectl executable.")
	k8sServer := flag.String("k8s_server", "", "Address of the Kubernetes server.")

	common.InitWithMust("k8s_deployer", common.PrometheusOpt(promPort))
	defer sklog.Flush()

	if *configRepo == "" {
		sklog.Fatal("config_repo is required.")
	}
	if *configSubdir == "" {
		// Note: this wouldn't be required if we had separate config repos per
		// cluster.
		sklog.Fatal("config_subdir is required.")
	}

	ctx := context.Background()

	// OAuth2.0 TokenSource.
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit)
	if err != nil {
		sklog.Fatal(err)
	}
	// Authenticated HTTP client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Gitiles repo.
	repo := gitiles.NewRepo(*configRepo, httpClient)

	// Apply configurations in a loop.  Note that we could respond directly to
	// commits in the repo via GitStore and PubSub, but that would require
	// access to BigTable, and with a relatively small interval we won't notice
	// too much of a delay.
	liveness := metrics2.NewLiveness(livenessMetric)
	go util.RepeatCtx(ctx, *interval, func(ctx context.Context) {
		if err := applyConfigs(ctx, repo, *kubectl, *k8sServer, *configSubdir, *prune); err != nil {
			sklog.Errorf("Failed to apply configs to cluster: %s", err)
		} else {
			liveness.Reset()
		}
	})

	// Run health check server.
	httputils.RunHealthCheckServer(*port)
}

func applyConfigs(ctx context.Context, repo *gitiles.Repo, kubectl, k8sServer, configSubdir string, prune bool) error {
	// Download the configs from Gitiles instead of maintaining a local Git
	// checkout, to avoid dealing with Git, persistent checkouts, etc.

	// Obtain the current set of configurations for the cluster.
	head, err := repo.Details(ctx, "main")
	if err != nil {
		return skerr.Wrapf(err, "failed to get most recent commit")
	}
	files, err := repo.ListFilesRecursiveAtRef(ctx, configSubdir, head.Hash)
	if err != nil {
		return skerr.Wrapf(err, "failed to list configs")
	}

	// Read the config contents in the given directory.
	eg := util.NewNamedErrGroup()
	contents := make(map[string][]byte, len(files))
	contentsMtx := sync.Mutex{}
	sklog.Infof("Downloading config files at %s", head.Hash)
	for _, file := range files {
		file := file // https://golang.org/doc/faq#closures_and_goroutines
		eg.Go(file, func() error {
			fullPath := path.Join(configSubdir, file)
			sklog.Infof("  %s", fullPath)
			fileContents, err := repo.ReadFileAtRef(ctx, fullPath, head.Hash)
			if err != nil {
				return skerr.Wrapf(err, "failed to retrieve contents of %s", fullPath)
			}
			contentsMtx.Lock()
			defer contentsMtx.Unlock()
			contents[file] = fileContents
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return skerr.Wrapf(err, "failed to download configs")
	}

	// Write the config contents to a temporary dir.
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return skerr.Wrapf(err, "failed to create temp dir")
	}
	defer util.RemoveAll(tmp)

	for path, fileContents := range contents {
		fullPath := filepath.Join(tmp, path)
		dir := filepath.Dir(fullPath)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return skerr.Wrapf(err, "failed to create %s", dir)
			}
		}
		if err := ioutil.WriteFile(fullPath, fileContents, os.ModePerm); err != nil {
			return skerr.Wrapf(err, "failed to create %s", fullPath)
		}
	}

	// Apply the configs to the cluster.
	// Note: this is a very naive approach.  We rely on the Kubernetes server to
	// determine when changes actually need to be made.  We could instead use
	// the REST API to load the full set of configuration which is actually
	// running on the server, diff that against the checked-in config files,
	// and then explicitly make only the changes we want to make.  That would be
	// a much more complicated and error-prone approach, but it would allow us
	// to partially apply configurations in the case of a failure and to alert
	// on specific components which fail to apply for whatever reason.
	cmd := []string{kubectl, "apply"}
	if k8sServer != "" {
		cmd = append(cmd, "--server", k8sServer)
	}
	if prune {
		cmd = append(cmd, "--prune", "--all")
	}
	cmd = append(cmd, "-f", ".")
	output, err := exec.RunCwd(ctx, tmp, cmd...)
	if err != nil {
		return skerr.Wrapf(err, "failed to apply configs: %s", output)
	}
	sklog.Infof("Output from kubectl: %s", output)
	return nil
}
