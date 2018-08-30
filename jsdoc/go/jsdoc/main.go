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
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	infraGitRepoURL    = flag.String("infra_git_repo_url", "https://skia.googlesource.com/buildbot", "Git repo of the main infra repo.")
	elementsGitRepoURL = flag.String("elements_git_repo_url", "https://github.com/google/elements-sk/", "Auxillary repo for elements-sk.")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port               = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	refresh            = flag.Duration("refresh", 15*time.Minute, "The duration between doc git repo refreshes.")
	infraGitRepoDir    = flag.String("infra_git_repo_dir", "/tmp/buildbot", "The directory to check out the doc repo into.")
	elementsGitRepoDir = flag.String("elements_git_repo_dir", "/tmp/elements-sk", "The directory to check out the auxillary repo to.")
)

var (
	liveness metrics2.Liveness = metrics2.NewLiveness("build", nil)
)

func step() error {
	ctx := context.Background()

	if _, err := gitinfo.CloneOrUpdate(ctx, *infraGitRepoURL, *infraGitRepoDir, false); err != nil {
		return fmt.Errorf("Failed to clone buildbot repo: %s", err)
	}
	if _, err := gitinfo.CloneOrUpdate(ctx, *elementsGitRepoURL, *elementsGitRepoDir, false); err != nil {
		return fmt.Errorf("Failed to clone elements-sk repo: %s", err)
	}

	// Build docs.
	buildDocsCmd := &exec.Command{
		Name:        "make",
		Args:        []string{"docs"},
		Dir:         path.Join(*infraGitRepoDir, "jsdoc"),
		InheritPath: false,
		LogStdout:   true,
	}

	if err := exec.Run(ctx, buildDocsCmd); err != nil {
		return fmt.Errorf("Failed building docs: %s", err)
	}

	// Build element-sk demo.
	buildElementDemoCmd := &exec.Command{
		Name:        "make",
		Args:        []string{"release"},
		Dir:         path.Join(*elementsGitRepoDir),
		InheritPath: false,
		LogStdout:   true,
	}

	if err := exec.Run(ctx, buildElementDemoCmd); err != nil {
		return fmt.Errorf("Failed building element demos: %s", err)
	}

	// Build common-sk demo pages.
	buildCommonDemoCmd := &exec.Command{
		Name:        "make",
		Args:        []string{"demos"},
		Dir:         path.Join(*infraGitRepoDir, "common-sk"),
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
	docsDir := path.Join(*infraGitRepoDir, "jsdoc", "out")
	elementsDemoDir := path.Join(*elementsGitRepoDir, "dist")
	commonDemoDir := path.Join(*infraGitRepoDir, "common-sk", "dist")
	router := mux.NewRouter()
	router.PathPrefix("/common-sk/").Handler(http.StripPrefix("/common-sk/", http.HandlerFunc(httputils.MakeResourceHandler(commonDemoDir))))
	router.PathPrefix("/elements-sk/").Handler(http.StripPrefix("/elements-sk/", http.HandlerFunc(httputils.MakeResourceHandler(elementsDemoDir))))
	router.PathPrefix("/").Handler(http.HandlerFunc(httputils.MakeResourceHandler(docsDir)))

	h := httputils.LoggingGzipRequestResponse(router)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}

	http.Handle("/", h)
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
