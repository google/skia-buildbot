package main

// The webserver for demos.skia.org. It serves a main page and a set of js+html+css demos.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/oauth2/google"

	"go.skia.org/infra/demos/go/frontend"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	repoPollInterval = 3 * time.Minute
)

func main() {
	var (
		port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
		local        = flag.Bool("local", false, "Is this running locally for development (use gcloud for auth)")
		resourcesDir = flag.String("resources_dir", "./dist", "The directory to find templates, JS, and CSS files. If blank ./dist will be used.")
		repoURL      = flag.String("repo_url", "https://skia.googlesource.com/infra-internal", "The repo from where to fetch the demos. Defaults to https://skia.googlesource.com/infra-internal")
		demosDir     = flag.String("demos_dir", "demos/internal", "The top level directory in the repo that holds the demos.")
		branch       = flag.String("repo_default_branch", git.MainBranch, "The branch of the repo to sync (ie. master or main).")

		unsyncedRepoPath = flag.String("unsynced_repo_path", "unset", "If set, will use an already existing Skia checkout and not sync it.")
	)
	common.InitWithMust(
		"demos",
	)

	ctx := context.Background()
	if err := setupGit(ctx, *local); err != nil {
		sklog.Fatalf("Failed to setup git: %s", err)
	}
	// Create a threadsafe checkout to serve from.
	checkoutDir, err := ioutil.TempDir("", "demos_repo")
	if err != nil {
		sklog.Fatalf("Unable to create temporary directory for demos checkout: %s", err)
	}
	var demos *syncedDemos
	if *unsyncedRepoPath != "unset" {
		if empty, err := util.IsDirEmpty(*unsyncedRepoPath); err != nil || empty {
			sklog.Fatalf("If unsynced_repo_path is specified, it cannot be empty. Do you have the environment variable SKIA_ROOT set?")
		}
		demos = newUnsyncedDemos(*unsyncedRepoPath, *demosDir)
	} else {
		demos = newSyncedDemos(ctx, *repoURL, *branch, checkoutDir, *demosDir)
	}

	r := mux.NewRouter()
	r.PathPrefix("/demo/").HandlerFunc(demos.demoHandler())
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.FileServer(http.Dir(*resourcesDir))))
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(*resourcesDir, "main.html"))
	})

	h := httputils.LoggingGzipRequestResponse(r)
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)
	sklog.Info("Ready to serve on http://localhost" + *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

type syncedDemos struct {
	sync.RWMutex
	repo    *git.Checkout
	repoURL string
	branch  string
	// Absolute path where demos are located.
	demoPath string
}

// newSyncedDemos creates a *syncedDemos that maintains a thread compatible, updating repo.
//
// It periodically updates the repo under a lock, and writes a metadata file to demoPath listing
// the available subdirectories and other repo information. The repo files can be safely served
// by exposing DemoHandler().
func newSyncedDemos(ctx context.Context, repoURL, branch, checkoutDir, demoPath string) *syncedDemos {
	sklog.Infof("Creating new syncedDemos for %s at %s", repoURL, checkoutDir)
	s := &syncedDemos{
		branch:  branch,
		repoURL: repoURL,
	}
	var err error
	s.repo, err = git.NewCheckout(ctx, repoURL, checkoutDir)
	if err != nil {
		sklog.Fatal(err)
	}
	s.demoPath = filepath.Join(s.repo.Dir(), demoPath)
	sklog.Infof("Serving demos out of '%s'", s.demoPath)

	go util.RepeatCtx(ctx, repoPollInterval, s.Sync)
	return s
}

// newUnsyncedDemos creates a *syncedDemos pointing to an existing Skia checkout.
// It does not periodically update anything; this is meant to expedite local development.
func newUnsyncedDemos(checkoutDir, demoPath string) *syncedDemos {
	return &syncedDemos{
		demoPath: filepath.Join(checkoutDir, demoPath),
	}
}

// writeMetadata writes a json file containing the list of subdirectories in s.demoPath as well as
// the hash and URL of the current s.repo revision.
//
// This is used to generate a list of demo links on the skia-demos main page.
func (s *syncedDemos) writeMetadata(rev string) error {
	file, err := os.Open(s.demoPath)
	if err != nil {
		return skerr.Wrapf(err, "Failed to Open '%s'.", s.demoPath)
	}
	defer file.Close()
	metadata := frontend.Metadata{
		Rev: frontend.Revision{
			Hash: rev,
			URL:  fmt.Sprintf("%s/+/%s", s.repoURL, rev),
		},
	}
	metadata.DemoList, err = file.Readdirnames(0)
	if err != nil {
		return skerr.Wrapf(err, "Failed to Readdirnames of '%s'.", file.Name())
	}
	sklog.Infof("Available demos: %v", metadata.DemoList)

	obj, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return skerr.Wrap(err)
	}
	metadataPath := filepath.Join(s.demoPath, "metadata.json")
	err = ioutil.WriteFile(metadataPath, obj, 0644)
	if err != nil {
		return skerr.Wrapf(err, "Unable to write json to '%s'.", metadataPath)
	}
	return nil
}

// Sync performs a repo Update under s.Lock().
//
// If the HEAD revision changes it rewrites the metadata.json file with updated information.
func (s *syncedDemos) Sync(ctx context.Context) {
	sklog.Info("Syncing and rewriting metadata file.")
	s.Lock()
	defer s.Unlock()

	if err := s.repo.UpdateBranch(ctx, s.branch); err != nil {
		sklog.Errorf("Failed to update repo: %s", err)
	}

	hash, err := s.repo.FullHash(ctx, "HEAD")
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Checkout at %s.", hash)

	if err = s.writeMetadata(hash); err != nil {
		sklog.Fatalf("Unable to write metadata: %s", err)
	}
}

// demoHandler returns a fileserver handler for dir that won't serve while the underlying repo
// is being updated.
func (s *syncedDemos) demoHandler() func(http.ResponseWriter, *http.Request) {
	h := http.StripPrefix("/demo", http.FileServer(http.Dir(s.demoPath)))
	return func(w http.ResponseWriter, r *http.Request) {
		s.RLock()
		defer s.RUnlock()
		h.ServeHTTP(w, r)
	}
}

// setupGit acquires necessary credentials to clone the repo.
func setupGit(ctx context.Context, local bool) error {
	// Start the gitauth package because we will need to read from infra-internal.
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit)
	if err != nil {
		return err
	}
	if !local {
		if _, err := gitauth.New(ts, filepath.Join(os.TempDir(), "gitcookies"), true, ""); err != nil {
			return skerr.Wrapf(err, "Failed to create git cookie updater")
		}
	}
	return nil
}
