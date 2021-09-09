package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/flynn/json5"
	"go.skia.org/infra/gitsync/go/watcher"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/gitstore/pubsub"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/sync/errgroup"
)

// This server watches a list of git repos for changes and syncs the meta data of all commits
// to a BigTable backed datastore.

const APPNAME = "gitsync2"

// Default config/flag values
var defaultConf = gitSyncConfig{
	BTInstanceID:      "production",
	BTTableID:         "git-repos2",
	BTWriteGoroutines: bt_gitstore.DefaultWriteGoroutines,
	HttpPort:          ":9091",
	Local:             false,
	Mirrors:           []string{},
	ProjectID:         "skia-public",
	PromPort:          ":20000",
	RepoURLs:          []string{},
	RefreshInterval:   human.JSONDuration(10 * time.Minute),
}

func main() {
	// config holds the configuration values either from flags or from parsing the config files.
	config := defaultConf

	// Flags that cause the flags below to be disregarded.
	configFile := flag.String("config", "", "Disregard flags and load the configuration from this JSON5 config file. The keys and types of the config file match the flags.")
	runInit := flag.Bool("init", false, "Initialize the BigTable instance and quit. This should be run with a different different user who has admin rights.")
	gcsBucket := flag.String("gcs_bucket", "", "GCS bucket used for temporary storage during ingestion.")
	gcsPath := flag.String("gcs_path", "", "GCS path used for temporary storage during ingestion.")

	// Define flags that map to field in the configuration struct.
	flag.StringVar(&config.BTInstanceID, "bt_instance", defaultConf.BTInstanceID, "Big Table instance")
	flag.StringVar(&config.BTTableID, "bt_table", defaultConf.BTTableID, "BigTable table ID")
	flag.IntVar(&config.BTWriteGoroutines, "bt_write_goroutines", defaultConf.BTWriteGoroutines, "Number of goroutines to use when writing to BigTable.")
	flag.StringVar(&config.HttpPort, "http_port", defaultConf.HttpPort, "The http port where ready-ness endpoints are served.")
	flag.BoolVar(&config.Local, "local", defaultConf.Local, "Running locally if true. As opposed to in production.")
	flag.StringVar(&config.ProjectID, "project", defaultConf.ProjectID, "ID of the GCP project")
	flag.StringVar(&config.PromPort, "prom_port", defaultConf.PromPort, "Metrics service address (e.g., ':10110')")
	common.MultiStringFlagVar(&config.RepoURLs, "repo_url", defaultConf.RepoURLs, "Repo url")
	common.MultiStringFlagVar(&config.Mirrors, "mirror", defaultConf.Mirrors, "Obtain data for the given repo url from the given mirror, eg. --mirror=<repo URL>=<gitiles mirror URL>")
	common.MultiStringFlagVar(&config.IncludeBranches, "branches", defaultConf.IncludeBranches, "Restrict the given repo URL to the given branches, eg. --branches=<repo URL>=master,my-feature")
	common.MultiStringFlagVar(&config.ExcludeBranches, "exclude-branches", defaultConf.ExcludeBranches, "Exclude the given branches for the repo URL, eg. --exclude-branches=<repo URL>=master,my-feature")
	flag.DurationVar((*time.Duration)(&config.RefreshInterval), "refresh", time.Duration(defaultConf.RefreshInterval), "Interval in which to poll git and refresh the GitStore.")

	common.InitWithMust(
		"gitsync",
		common.PrometheusOpt(&config.PromPort),
		common.MetricsLoggingOpt(),
	)
	defer common.Defer()

	// If a configuration file was given we load it into config.
	if *configFile != "" {
		confBytes, err := ioutil.ReadFile(*configFile)
		if err != nil {
			sklog.Fatalf("Error reading config file %s: %s", *configFile, err)
		}

		if err := json5.Unmarshal(confBytes, &config); err != nil {
			sklog.Fatalf("Error parsing config file %s: %s", *configFile, err)
		}
	}

	// Dump the configuration since it might be different than the flags that are dumped by default.
	sklog.Infof("\n\n  Effective configuration: \n%s \n", config.String())

	// Configure the bigtable instance.
	btConfig := &bt_gitstore.BTConfig{
		ProjectID:       config.ProjectID,
		InstanceID:      config.BTInstanceID,
		TableID:         config.BTTableID,
		AppProfile:      APPNAME,
		WriteGoroutines: config.BTWriteGoroutines,
	}

	// Initialize bigtable if invoked with --init and quit.
	// This should be invoked with a user that has admin privileges, so that the production user that
	// wants to write to the instance does not need admin privileges.
	if *runInit {
		if err := bt_gitstore.InitBT(btConfig); err != nil {
			sklog.Fatalf("Error initializing BT: %s", err)
		}
		sklog.Infof("BigTable instance %s and table %s in project %s initialized.", btConfig.InstanceID, btConfig.TableID, btConfig.ProjectID)
		return
	}

	// Make sure we have at least one repo configured.
	if len(config.RepoURLs) == 0 {
		sklog.Fatalf("At least one repository URL must be configured.")
	}

	// Obtain the Gitiles URLs for each of the repos; by default, assume
	// that the Git repo URL is the Gitiles URL, but allow the user to
	// specify the Gitiles URL where that is not the case.
	gitilesURLs := make(map[string]string, len(config.RepoURLs))
	for _, url := range config.RepoURLs {
		gitilesURLs[url] = url
	}
	if len(config.Mirrors) > 0 {
		for _, mirror := range config.Mirrors {
			split := strings.Split(mirror, "=")
			if len(split) != 2 {
				sklog.Fatalf("Invalid value for --mirror: %q; must be separated by a single '='", mirror)
			}
			gitilesURLs[split[0]] = split[1]
		}
	}

	// Create token source.
	ts, err := auth.NewDefaultTokenSource(false, auth.ScopeUserinfoEmail, auth.ScopeGerrit, pubsub.AUTH_SCOPE)
	if err != nil {
		sklog.Fatalf("Problem setting up default token source: %s", err)
	}

	// Start all repo watchers.
	ctx := context.Background()
	includeBranches := make(map[string][]string, len(config.RepoURLs))
	for _, repo := range config.RepoURLs {
		includeBranches[repo] = []string{}
	}
	excludeBranches := make(map[string][]string, len(config.RepoURLs))
	for _, repo := range config.RepoURLs {
		excludeBranches[repo] = []string{}
	}
	for _, branchFlag := range config.IncludeBranches {
		split := strings.SplitN(branchFlag, "=", 2)
		if len(split) != 2 {
			sklog.Fatalf("Invalid value for --branch: %s", branchFlag)
		}
		repo := split[0]
		branches := strings.Split(split[1], ",")
		if _, ok := includeBranches[repo]; !ok {
			sklog.Fatalf("Invalid value for --branch; unknown repo %s", repo)
		}
		if len(branches) == 0 {
			sklog.Fatalf("Invalid value for --branch; no branches specified: %s", branchFlag)
		}
		includeBranches[repo] = branches
	}
	for _, branchFlag := range config.ExcludeBranches {
		split := strings.SplitN(branchFlag, "=", 2)
		if len(split) != 2 {
			sklog.Fatalf("Invalid value for --branch: %s", branchFlag)
		}
		repo := split[0]
		branches := strings.Split(split[1], ",")
		if _, ok := excludeBranches[repo]; !ok {
			sklog.Fatalf("Invalid value for --exclude-branch; unknown repo %s", repo)
		}
		if len(branches) == 0 {
			sklog.Fatalf("Invalid value for --exclude-branch; no branches specified: %s", branchFlag)
		}
		excludeBranches[repo] = branches
	}
	var egroup errgroup.Group
	for _, repoURL := range config.RepoURLs {
		repoURL := repoURL
		egroup.Go(func() error {
			return watcher.Start(ctx, btConfig, repoURL, includeBranches[repoURL], excludeBranches[repoURL], gitilesURLs[repoURL], *gcsBucket, *gcsPath, time.Duration(config.RefreshInterval), ts)
		})
	}
	if err := egroup.Wait(); err != nil {
		sklog.Fatal(err)
	}

	// Set up the http handler to indicate ready-ness and start serving.
	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	sklog.Infof("Listening on port: %s", config.HttpPort)
	log.Fatal(http.ListenAndServe(config.HttpPort, nil))
}
