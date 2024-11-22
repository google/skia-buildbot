package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	"golang.org/x/oauth2/google"
)

// Server is the state of the server.
type Server struct {
	store   store.Store
	repo    *gitinfo.GitInfo
	repoDir string

	livenessExamples   metrics2.Liveness    // liveness of the naming the Skia examples.
	numInvalidExamples metrics2.Int64Metric // numInvalidExamples is the number of examples that are currently invalid.
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
	st, err := store.New(ctx, local)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating fiddle store")
	}

	if !local {
		ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit)
		if err != nil {
			sklog.Fatalf("Failed authentication: %s", err)
		}
		// Use the gitcookie created by the gitauth package.
		if _, err := gitauth.New(ctx, ts, "/tmp/gitcookies", true, ""); err != nil {
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

		livenessExamples:   metrics2.NewLiveness("named_fiddles_examples"),
		numInvalidExamples: metrics2.GetInt64Metric("named_fiddles_examples_total_invalid"),
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
	sklog.Info("Starting exampleStep")
	if err := srv.repo.Update(ctx, true, false); err != nil {
		sklog.Errorf("Failed to sync git repo.")
		return
	}

	var recentFiddles []string
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
		b, err := os.ReadFile(filename)
		if err != nil {
			sklog.Warningf("Failed to load file: %q", filename)
			return nil
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
		recentFiddles = append(recentFiddles, name)

		runResults, success := client.Do(b, false, "https://fiddle.skia.org", func(*types.RunResults) bool {
			return true
		})
		metric := metrics2.GetBoolMetric("named_fiddles_errors_in_examples_run", map[string]string{
			"name": name,
			"link": fmt.Sprintf("https://fiddle.skia.org/c/@%s", name),
		})
		status := errorsInResults(runResults)
		if success {
			metric.Update(false)
		} else {
			sklog.Errorf("Failed to run https://fiddle.skia.org/c/@%s: %s", name, status)
			metric.Update(true)
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

	sklog.Infof("Updated %d fiddles", len(recentFiddles))
	storedFiddles, err := srv.store.ListAllNames()
	if err != nil {
		sklog.Errorf("Error getting existing fiddles to clean out old ones: %v\n", err)
		return
	}
	sklog.Infof("There are now %d fiddles in the store. Going to delete %d of them that don't exist anymore.", len(storedFiddles), len(storedFiddles)-len(recentFiddles))
	for _, nf := range storedFiddles {
		if slices.Index(recentFiddles, nf.Name) == -1 {
			sklog.Infof("Deleting sample %q because it's not in version control anymore", nf.Name)
			if err := srv.store.DeleteName(nf.Name); err != nil {
				sklog.Warningf("Could not delete sample %q: %v", nf.Name, err)
			}
		}
	}

	srv.numInvalidExamples.Update(numInvalid)
	srv.livenessExamples.Reset()
}
