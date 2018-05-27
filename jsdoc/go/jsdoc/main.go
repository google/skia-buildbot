// Serves the jsdoc's for both the elements-sk and common libraries.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/iap"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	gitRepoURL = flag.String("git_repo_url", "https://skia.googlesource.com/buildbot", "The directory to check out the doc repo into.")
	local      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port       = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort   = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	refresh    = flag.Duration("refresh", 15*time.Minute, "The duration between doc git repo refreshes.")
	gitRepoDir = flag.String("git_repo_dir", "/tmp/buildbot", "The directory to check out the doc repo into.")
)

var (
	git      *gitinfo.GitInfo  = nil
	liveness metrics2.Liveness = metrics2.NewLiveness("build", nil)
)

func step() error {
	ctx := context.Background()
	var err error
	git, err = gitinfo.CloneOrUpdate(ctx, *gitRepoURL, *gitRepoDir, false)
	if err != nil {
		return fmt.Errorf("Failed to clone buildbot repo: %s", err)
	}

	// Build docs.
	buildDocsCmd := &exec.Command{
		Name:        "make",
		Args:        []string{"docs"},
		Dir:         path.Join(*gitRepoDir, "jsdoc"),
		InheritPath: false,
		LogStdout:   true,
	}

	if err := exec.Run(ctx, buildDocsCmd); err != nil {
		return fmt.Errorf("Failed building docs: %s", err)
	}

	// Build element demo.
	buildElementDemoCmd := &exec.Command{
		Name:        "make",
		Dir:         path.Join(*gitRepoDir, "ap"),
		InheritPath: false,
		LogStdout:   true,
	}

	if err := exec.Run(ctx, buildElementDemoCmd); err != nil {
		return fmt.Errorf("Failed building element demos: %s", err)
	}

	// Build common demo pages.
	buildCommonDemoCmd := &exec.Command{
		Name:        "make",
		Dir:         path.Join(*gitRepoDir, "common-sk"),
		InheritPath: false,
		LogStdout:   true,
	}

	if err := exec.Run(ctx, buildCommonDemoCmd); err != nil {
		return fmt.Errorf("Failed building common-sk demos: %s", err)
	}

	liveness.Reset()
	return nil
}

func periodic() {
	for _ = range time.Tick(*refresh) {
		if err := step(); err != nil {
			sklog.Errorf("Failed step: %s", err)
		}
	}
}

func main() {
	defer common.LogPanic()
	flag.Parse()
	opts := []common.Opt{
		common.PrometheusOpt(promPort),
	}
	common.InitWithMust(
		"jsdocserver",
		opts...,
	)
	if err := step(); err != nil {
		sklog.Fatalf("Failed initial checkout and doc build: %s", err)
	}
	go periodic()
	docsDir := path.Join(*gitRepoDir, "jsdoc", "out")
	elementsDemoDir := path.Join(*gitRepoDir, "ap", "dist")
	commonDemoDir := path.Join(*gitRepoDir, "common-sk", "dist")
	router := mux.NewRouter()
	router.PathPrefix("/common-sk/").Handler(http.StripPrefix("/common-sk/", http.HandlerFunc(httputils.MakeResourceHandler(commonDemoDir))))
	router.PathPrefix("/elements-sk/").Handler(http.StripPrefix("/elements-sk/", http.HandlerFunc(httputils.MakeResourceHandler(elementsDemoDir))))
	router.PathPrefix("/").Handler(http.HandlerFunc(httputils.MakeResourceHandler(docsDir)))

	h := httputils.LoggingGzipRequestResponse(router)
	if !*local {
		h = iap.None(h)
	}

	http.Handle("/", h)
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
