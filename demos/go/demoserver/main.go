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
	"strings"
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
	// Path relative to the checkout root where demos are located.
	demoPath string
}

func newSyncedDemos(ctx context.Context, repoURL, checkoutDir, demoPath string) *syncedDemos {
	sklog.Info("Creating new syncedDemos")
	s := new(syncedDemos)
	var err error
	s.repo, err = git.NewCheckout(ctx, repoURL, checkoutDir)
	if err != nil {
		sklog.Fatal(err)
	}
	s.repoURL = repoURL
	s.demoPath = demoPath
	go util.RepeatCtx(ctx, 1*time.Minute, s.Sync)
	return s
}
func (s *syncedDemos) writeMetadata(ctx context.Context, rev string) error {
	demoPath := filepath.Join(s.repo.Dir(), s.demoPath)
	file, err := os.Open(demoPath)
	if err != nil {
		return skerr.Wrapf(err, "Failed to Open '%s'.", demoPath)
	}
	defer file.Close()

	type Revision struct {
		Hash string `json:"hash"`
		URL  string `json:"url"`
	}
	type Metadata struct {
		Rev Revision `json:"revision"`
		// In the future we may include actual author information etc, but for now we just list the available demos.
		DemoList []string `json:"demos"`
	}
	var metadata Metadata
	metadata.Rev = Revision{rev, fmt.Sprintf("%s/+/%s", s.repoURL, rev)}
	metadata.DemoList, err = file.Readdirnames(0)
	if err != nil {
		return skerr.Wrapf(err, "Failed to Readdirnames of '%s'.", file.Name())
	}
	sklog.Infof("Available demos: %v", metadata.DemoList)

	obj, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	metadataPath := filepath.Join(filepath.Join(s.repo.Dir(), s.demoPath), "metadata.json")
	err = ioutil.WriteFile(metadataPath, obj, 0644)
	if err != nil {
		return skerr.Wrapf(err, "Unable to write json to '%s'.", metadataPath)
	}
	return nil
}

func (s *syncedDemos) Sync(ctx context.Context) {
	sklog.Info("Syncing")
	s.Lock()
	defer s.Unlock()
	var oldHash, newHash string
	var err error
	oldHash, err = s.repo.FullHash(ctx, "HEAD")
	if err != nil {
		sklog.Fatal(err)
	}

	if err = s.repo.Update(ctx); err != nil {
		sklog.Errorf("Failed to update repo: %s", err)
	}

	newHash, err = s.repo.FullHash(ctx, "HEAD")
	if err != nil {
		sklog.Fatal(err)
	}
	if oldHash == newHash {
		return
	}
	sklog.Infof("Updated checkout from %s to %s. Rewriting metadata file.", oldHash, newHash)
	if err = s.writeMetadata(ctx, newHash); err != nil {
		sklog.Fatalf("Unable to write metadata: %s", err)
	}
}

func demoHandler(demos *syncedDemos, dir string) func(http.ResponseWriter, *http.Request) {
	h := http.StripPrefix("/demo", http.FileServer(http.Dir(dir)))
	return func(w http.ResponseWriter, r *http.Request) {
		demos.RLock()
		defer demos.RUnlock()
		h.ServeHTTP(w, r)
	}
}
func setupGit() error {
	// Start the gitauth package because we will need to read from infra-internal.
	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
	if err != nil {
		return err
	}
	if _, err := gitauth.New(ts, filepath.Join(os.TempDir(), "gitcookies"), true, ""); err != nil {
		return fmt.Errorf("Failed to create git cookie updater: %s", err)
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

	// Build the path to serve demos from (<checkout_dir>/<repo_name>/<demos_path>)
	repoURLParts := strings.Split(*demosRepo, "/")
	if len(repoURLParts) <= 1 {
		sklog.Fatalf("Unable to derive repo name from '%s'", *demosRepo)
	}
	servingDir := filepath.Join(checkoutDir, repoURLParts[len(repoURLParts)-1], *demosRepoPath)
	sklog.Infof("Serving demos out of '%s'", servingDir)

	r := mux.NewRouter()
	r.PathPrefix("/demo/").HandlerFunc(demoHandler(syncedDemos, servingDir))
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
