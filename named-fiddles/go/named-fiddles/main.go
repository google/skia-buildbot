package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/fiddlek/go/client"
	"go.skia.org/infra/fiddlek/go/store"
	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/named-fiddles/go/parse"
)

// flags
var (
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	period   = flag.Duration("period", time.Hour, "How often to check if the named fiddles are valid.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	repoURL  = flag.String("repo_url", "https://skia.googlesource.com/skia", "Repo url")
	repoDir  = flag.String("repo_dir", "/tmp/skia_named_fiddles", "Directory the repo is checked out into.")
)

// Server is the state of the server.
type Server struct {
	store store.Store
	repo  *gitinfo.GitInfo

	livenessExamples    metrics2.Liveness    // liveness of the naming the Skia examples.
	errorsInExamplesRun metrics2.Counter     // errorsInExamplesRun is the number of errors in a single examples run.
	numInvalidExamples  metrics2.Int64Metric // numInvalidExamples is the number of examples that are currently invalid.
}

// New creates a new Server.
func New() (*Server, error) {
	st, err := store.New(*local)
	if err != nil {
		return nil, fmt.Errorf("Failed to create client for GCS: %s", err)
	}

	if !*local {
		ts, err := auth.NewDefaultTokenSource(false, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
		if err != nil {
			sklog.Fatalf("Failed authentication: %s", err)
		}
		// Use the gitcookie created by the gitauth package.
		if _, err := gitauth.New(ts, "/tmp/gitcookies", true, ""); err != nil {
			sklog.Fatalf("Failed to create git cookie updater: %s", err)
		}
		sklog.Infof("Git authentication set up successfully.")
	}

	repo, err := gitinfo.CloneOrUpdate(context.Background(), *repoURL, *repoDir, false)
	if err != nil {
		return nil, fmt.Errorf("Failed to create git repo: %s", err)
	}

	srv := &Server{
		store: st,
		repo:  repo,

		livenessExamples:    metrics2.NewLiveness("named_fiddles_examples"),
		errorsInExamplesRun: metrics2.GetCounter("named_fiddles_errors_in_examples_run", nil),
		numInvalidExamples:  metrics2.GetInt64Metric("named_fiddles_examples_total_invalid"),
	}
	go srv.nameExamples()
	return srv, nil
}

// errorsInResults returns an empty string if there are no errors, either
// compile or runtime, found in the results. If there are errors then a string
// describing the error is returned.
func errorsInResults(runResults *types.RunResults, success bool) string {
	status := ""
	if runResults == nil {
		status = "Failed to run."
	} else if len(runResults.CompileErrors) > 0 || runResults.RunTimeError != "" {
		// update validity
		status = fmt.Sprintf("%v %s", runResults.CompileErrors, runResults.RunTimeError)
		if len(status) > 100 {
			status = status[:100]
		}
	}
	return status
}

// exampleStep is a single run through naming all the examples.
func (srv *Server) exampleStep() {
	srv.errorsInExamplesRun.Reset()
	sklog.Info("Starting exampleStep")
	if err := srv.repo.Update(context.Background(), true, false); err != nil {
		sklog.Errorf("Failed to sync git repo.")
		return
	}

	var numInvalid int64
	// Get a list of all examples.
	dir := filepath.Join(*repoDir, "docs", "examples")
	err := filepath.Walk(dir+"/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("Failed to open %q: %s", path, err)
		}
		if info.IsDir() {
			return nil
		}
		name := filepath.Base(info.Name())
		if !strings.HasSuffix(name, ".cpp") {
			return nil
		}
		name = name[0 : len(name)-4]
		b, err := ioutil.ReadFile(filepath.Join(dir, info.Name()))
		fc, err := parse.ParseCpp(string(b))
		if err == parse.ErrorInactiveExample {
			sklog.Infof("Inactive sample: %q", info.Name())
			return nil
		} else if err != nil {
			sklog.Infof("Invalid sample: %q", info.Name())
			numInvalid += 1
			return nil
		}
		// Now run it.
		sklog.Infof("About to run: %s", name)
		b, err = json.Marshal(fc)
		if err != nil {
			sklog.Errorf("Failed to encode example to JSON: %s", err)
			return nil
		}

		runResults, success := client.Do(b, false, "https://fiddle.skia.org", func(*types.RunResults) bool {
			return true
		})
		if !success {
			sklog.Errorf("Failed to run")
			srv.errorsInExamplesRun.Inc(1)
			return nil
		}
		status := errorsInResults(runResults, success)
		if err := srv.store.WriteName(name, runResults.FiddleHash, "Skia example", status); err != nil {
			sklog.Errorf("Failed to write status for %s: %s", name, err)
			srv.errorsInExamplesRun.Inc(1)
		}
		return nil
	})
	if err != nil {
		sklog.Errorf("Error walking the path %q: %v\n", dir, err)
		return
	}

	srv.numInvalidExamples.Update(numInvalid)
	srv.livenessExamples.Reset()
}

// nameExamples runs each Skia example and gives it a name.
func (srv *Server) nameExamples() {
	srv.exampleStep()
	for range time.Tick(time.Minute) {
		srv.exampleStep()
	}
}

func main() {
	common.InitWithMust(
		"named-fiddles",
		common.PrometheusOpt(promPort),
	)

	_, err := New()
	if err != nil {
		sklog.Fatalf("Failed to create Server: %s", err)
	}
	select {}
}
