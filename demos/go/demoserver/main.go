package main

// The webserver for demos.skia.org. It serves a main page and a set of js+html+css demos.

import (
	"context"
	"flag"
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
}

func newSyncedDemos(ctx context.Context, repoURL, dir string) *syncedDemos {
	sklog.Info("Creating new syncedDemos")
	s := new(syncedDemos)
	var err error
	s.repo, err = git.NewCheckout(ctx, repoURL, dir)
	sklog.Info("got repo")
	if err != nil {
		sklog.Fatal(err)
	}
	go util.RepeatCtx(ctx, 1*time.Minute, s.Sync)
	return s
}
func (s *syncedDemos) Sync(ctx context.Context) {
	sklog.Info("Syncing")
	s.Lock()
	defer s.Unlock()
	hash, err := s.repo.GetBranchHead(ctx, "master")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Info("pre update: branch head" + hash)
	hash, err = s.repo.FullHash(ctx, "HEAD")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Info("pre update: Full hash" + hash)
	s.repo.Update(ctx)
	hash, err = s.repo.GetBranchHead(ctx, "master")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Info("post update: branch head" + hash)
	hash, err = s.repo.FullHash(ctx, "HEAD")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Info("post update: Full hash" + hash)

	file, err := os.Open(s.repo.Dir())
	if err != nil {
		sklog.Fatalf("failed opening directory: %s", err)
	}
	defer file.Close()

	list, _ := file.Readdirnames(0)
	sklog.Infof("%#v", list)
}

func demoHandler(demos *syncedDemos, dir string) func(http.ResponseWriter, *http.Request) {
	h := http.StripPrefix("/demo", http.FileServer(http.Dir("tmp/infra-internal/demos/internal")))
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
	syncedDemos := newSyncedDemos(ctx, *demosRepo, checkoutDir)
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
