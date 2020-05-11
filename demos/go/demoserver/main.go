package main

// The webserver for demos.skia.org. It serves a main page and a set of js+html+css demos.

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	port = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	//demosDir      = flag.String("demos_dir", "./demos/public", "The directory to find named subdirectories for each demo. If blank ./demos/public")
	resourcesDir  = flag.String("resources_dir", "./dist", "The directory to find templates, JS, and CSS files. If blank ./dist will be used.")
	demosRepo     = flag.String("repo_url", "https://skia.googlesource.com/infra-internal", "The repo from where to fetch the demos. Defaults to https://skia.googlesource.com/infra-internal")
	demosRepoPath = flag.String("demos_dir", "demos/internal", "The top level directory in the repo that holds the demos.")
)

type syncedDemos struct {
	sync.RWMutex
	repo *git.Checkout
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
	s.demoPath = demoPath
	go util.RepeatCtx(ctx, 1*time.Minute, s.Sync)
	return s
}
func (s *syncedDemos) writeMetadata(ctx context.Context) {
	file, err := os.Open(filepath.Join(s.repo.Dir(), s.demoPath))
	if err != nil {
		sklog.Fatalf("failed opening directory: %s", err)
	}
	defer file.Close()

	list, _ := file.Readdirnames(0)
	sklog.Infof("%#v", list)
	type Demo struct {
		Name string `json:"name"`
		//Author string			`json:"commit"`
	}
	var demolist []Demo
	for _, demoname := range list {
		demolist = append(demolist, Demo{demoname})

	}
	obj, err := json.MarshalIndent(demolist, "", "  ")
	if err != nil {
		sklog.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(filepath.Join(s.repo.Dir(), s.demoPath), "metadata.json"), obj, 0644)
	if err != nil {
		sklog.Fatal(err)
	}
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

	s.repo.Update(ctx)

	newHash, err = s.repo.FullHash(ctx, "HEAD")
	if err != nil {
		sklog.Fatal(err)
	}
	if oldHash == newHash {
		//	return
	}
	sklog.Infof("Updated checkout from %s to %s. Rewriting metadata file.", oldHash, newHash)
	s.writeMetadata(ctx)
}

func demoHandler(demos *syncedDemos, dir string) func(http.ResponseWriter, *http.Request) {
	h := http.StripPrefix("/demo", http.FileServer(http.Dir(dir)))
	return func(w http.ResponseWriter, r *http.Request) {
		demos.RLock()
		defer demos.RUnlock()
		h.ServeHTTP(w, r)
	}
}
func main() {
	common.InitWithMust(
		"demos",
	)

	ctx := context.Background()
	checkoutDir := "tmp"
	repoURLParts := strings.Split(*demosRepo, "/")
	repoName := repoURLParts[len(repoURLParts)-1]
	servingDir := filepath.Join(checkoutDir, repoName, *demosRepoPath)
	syncedDemos := newSyncedDemos(ctx, *demosRepo, checkoutDir, *demosRepoPath)
	sklog.Infof("Serving demos out of '%s'", servingDir)
	r := mux.NewRouter()
	r.PathPrefix("/demo/").HandlerFunc(demoHandler(syncedDemos, servingDir))
	//r.PathPrefix("/demo/").Handler(http.StripPrefix("/demo", http.FileServer(http.Dir(*demosDir))))

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
