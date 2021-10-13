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
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/named-fiddles/go/parse"
)

// Server is the state of the server.
type Server struct {
	store   store.Store
	repo    *gitinfo.GitInfo
	repoDir string

	livenessExamples    metrics2.Liveness    // liveness of the naming the Skia examples.
	errorsInExamplesRun metrics2.Counter     // errorsInExamplesRun is the number of errors in a single examples run.
	numInvalidExamples  metrics2.Int64Metric // numInvalidExamples is the number of examples that are currently invalid.
}

func main() {
	local := flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort := flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	repoURL := flag.String("repo_url", "https://skia.googlesource.com/skia", "Repo url")
	repoDir := flag.String("repo_dir", "/tmp/skia_named_fiddles", "Directory the repo is checked out into.")

	common.InitWithMust(
		"named-fiddles",
		common.PrometheusOpt(promPort),
	)

	_, err := startSyncing(context.Background(), *local, *repoURL, *repoDir)
	if err != nil {
		sklog.Fatalf("Failed to create Server: %s", err)
	}
	select {}
}

// startSyncing creates a new Server in a goroutine and returns.
func startSyncing(ctx context.Context, local bool, repoURL, repoDir string) (*Server, error) {
	st, err := store.New(local)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating fiddle store")
	}

	if !local {
		ts, err := auth.NewDefaultTokenSource(false, auth.ScopeUserinfoEmail, auth.ScopeGerrit)
		if err != nil {
			sklog.Fatalf("Failed authentication: %s", err)
		}
		// Use the gitcookie created by the gitauth package.
		if _, err := gitauth.New(ts, "/tmp/gitcookies", true, ""); err != nil {
			sklog.Fatalf("Failed to create git cookie updater: %s", err)
		}
		sklog.Infof("Git authentication set up successfully.")
	}

	repo, err := gitinfo.CloneOrUpdate(ctx, repoURL, repoDir, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "Cloning git repo %s to %s", repoURL, repoDir)
	}
	sklog.Infof("git clone of %s to %s was successful", repoURL, repoDir)
	srv := &Server{
		store:   st,
		repo:    repo,
		repoDir: repoDir,

		livenessExamples:    metrics2.NewLiveness("named_fiddles_examples"),
		errorsInExamplesRun: metrics2.GetCounter("named_fiddles_errors_in_examples_run"),
		numInvalidExamples:  metrics2.GetInt64Metric("named_fiddles_examples_total_invalid"),
	}
	go util.RepeatCtx(ctx, time.Minute, srv.exampleStep)
	return srv, nil
}

// errorsInResults returns an empty string if there are no errors, either
// compile or runtime, found in the results. If there are errors then a string
// describing the error is returned.
func errorsInResults(runResults *types.RunResults) string {
	status := ""
	if runResults == nil {
		sklog.Infof("runResults are nil, so this was a timeout, count it as valid for now and try again on the next run.")
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
func (srv *Server) exampleStep(ctx context.Context) {
	srv.errorsInExamplesRun.Reset()
	sklog.Info("Starting exampleStep")
	if err := srv.repo.Update(ctx, true, false); err != nil {
		sklog.Errorf("Failed to sync git repo.")
		return
	}

	var numInvalid int64
	// Get a list of all examples.
	dir := filepath.Join(srv.repoDir, "docs", "examples")
	err := filepath.Walk(dir+"/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return skerr.Wrapf(err, "opening %q", path)
		}
		if info.IsDir() {
			return nil
		}
		name := filepath.Base(info.Name())
		if !strings.HasSuffix(name, ".cpp") {
			return nil
		}
		name = name[0 : len(name)-4]
		filename := filepath.Join(dir, info.Name())
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			sklog.Warningf("Failed to load file: %q", filename)
		}
		fc, err := parse.ParseCpp(string(b))
		if err == parse.ErrorInactiveExample {
			sklog.Infof("Inactive sample: %q", info.Name())
			return nil
		} else if err != nil {
			sklog.Infof("Invalid sample: %q\n%s", info.Name(), err)
			numInvalid++
			srv.numInvalidExamples.Update(numInvalid)
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
		status := errorsInResults(runResults)
		if status == "" {
			return nil
		}
		if !success {
			sklog.Errorf("Failed to run https://fiddle.skia.org/c/@%s: %s", name, status)
			srv.errorsInExamplesRun.Inc(1)
			return nil
		}
		if err := srv.store.WriteName(name, runResults.FiddleHash, "Skia example", status); err != nil {
			sklog.Errorf("Failed to write status for %s: %s", name, err)
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
