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

var (
	port          = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	local         = flag.Bool("local", false, "Is this running locally for development (use gcloud for auth)")
	resourcesDir  = flag.String("resources_dir", "./dist", "The directory to find templates, JS, and CSS files. If blank ./dist will be used.")
	demosRepo     = flag.String("repo_url", "https://skia.googlesource.com/infra-internal", "The repo from where to fetch the demos. Defaults to https://skia.googlesource.com/infra-internal")
	demosRepoPath = flag.String("demos_dir", "demos/internal", "The top level directory in the repo that holds the demos.")
)

type syncedDemos struct {
	sync.RWMutex
	repo    *git.Checkout
	repoURL string
	// Absolute path where demos are located.
	demoPath string
}

// newSyncedDemos creates a *syncedDemos that maintains a thread compatible, updating repo.
//
// It periodically updates the repo under a lock, and writes a metadata file to demoPath listing
// the available subdirectories and other repo information. The repo files can be safely served
// by exposing DemoHandler().
func newSyncedDemos(ctx context.Context, repoURL, checkoutDir, demoPath string) *syncedDemos {
	sklog.Infof("Creating new syncedDemos for %s at %s", repoURL, checkoutDir)
	s := new(syncedDemos)
	var err error
	s.repo, err = git.NewCheckout(ctx, repoURL, checkoutDir)
	if err != nil {
		sklog.Fatal(err)
	}
	s.repoURL = repoURL
	s.demoPath = filepath.Join(s.repo.Dir(), demoPath)
	sklog.Infof("Serving demos out of '%s'", s.demoPath)

	go util.RepeatCtx(ctx, repoPollInterval, s.Sync)
	return s
}

// Revision represents repo HEAD info for storage as json.
type Revision struct {
	Hash string `json:"hash"`
	URL  string `json:"url"`
}

// Metadata represents repo metadata and list of demos, for storage as json.
type Metadata struct {
	Rev Revision `json:"revision"`
	// In the future we may include actual author information etc, but for now we just list the
	// available demos.
	DemoList []string `json:"demos"`
}

// writeMetadata writes a json file containing the list of subdirectories in s.demoPath as well as
// the hash and URL of the current s.repo revision.
//
// This is used to generate a list of demo links on the skia-demos main page.
func (s *syncedDemos) writeMetadata(ctx context.Context, rev string) error {
	file, err := os.Open(s.demoPath)
	if err != nil {
		return skerr.Wrapf(err, "Failed to Open '%s'.", s.demoPath)
	}
	defer file.Close()
	metadata := Metadata{
		Rev: Revision{
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

	if err := s.repo.Update(ctx); err != nil {
		sklog.Errorf("Failed to update repo: %s", err)
	}

	hash, err := s.repo.FullHash(ctx, "HEAD")
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Checkout at %s.", hash)

	if err = s.writeMetadata(ctx, hash); err != nil {
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
func setupGit() error {
	// Start the gitauth package because we will need to read from infra-internal.
	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
	if err != nil {
		return err
	}
	if !*local {
		if _, err := gitauth.New(ts, filepath.Join(os.TempDir(), "gitcookies"), true, ""); err != nil {
			return skerr.Wrapf(err, "Failed to create git cookie updater")
		}
	}
	return nil
}

func main() {
	common.InitWithMust(
		"demos",
	)

	if err := setupGit(); err != nil {
		sklog.Fatalf("Failed to setup git: %s", err)
	}
	// Create a threadsafe checkout to serve from.
	ctx := context.Background()
	checkoutDir, err := ioutil.TempDir("", "demos_repo")
	if err != nil {
		sklog.Fatalf("Unable to create temporary directory for demos checkout: %s", err)
	}
	syncedDemos := newSyncedDemos(ctx, *demosRepo, checkoutDir, *demosRepoPath)

	r := mux.NewRouter()
	r.PathPrefix("/demo/").HandlerFunc(syncedDemos.demoHandler())
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
