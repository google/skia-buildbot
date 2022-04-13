// Adds a single commit to a repo with the current data, e.g. 2022-04-01.
//
// Used in a kubernetes cron job to populate a repo that has one commit per day.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
)

const filename = "DATE"

var (
	authorEmail = flag.String("author_email", "perf-comp-ui@skia-public.iam.gserviceaccount.com", "Email address of the git author.")
	local       = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	repoURL     = flag.String("gitrepo", "https://skia.googlesource.com/perf-compui", "The repo that has commits associated with runs of the cron job.")
)

func main() {
	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeGerrit)
	if err != nil {
		sklog.Fatal(err)
	}

	// Do all the work in a temp directory.
	workDir, err := os.MkdirTemp("", "comp-ui-cron-job")
	if err != nil {
		sklog.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(workDir); err != nil {
			sklog.Error(err)
		}
	}()

	// We presume that if running locally that you've already authenticated to
	// Gerrit, otherwise write out a git cookie that enables R/W access to the
	// git repo.
	if !*local {
		if _, err := gitauth.New(ts, "/tmp/git-cookie", true, *authorEmail); err != nil {
			sklog.Fatal(err)
		}
	}

	// Checkout the repo.
	checkout, err := git.NewCheckout(ctx, *repoURL, workDir)
	if err != nil {
		sklog.Fatalf("Unable to create the checkout of %q at %q: %s", *repoURL, workDir, err)
	}
	if err := checkout.UpdateBranch(ctx, git.MainBranch); err != nil {
		sklog.Fatalf("Unable to update the checkout of %q at %q: %s", *repoURL, workDir, err)
	}

	// We write today as the data into a file called "DATE".
	timestamp := now.Now(ctx)
	dateAsString := timestamp.Format("2006.01.02")
	err = util.WithWriteFile(filepath.Join(checkout.Dir(), filename), func(w io.Writer) error {
		_, err := w.Write([]byte(dateAsString))
		return err
	})
	if err != nil {
		sklog.Fatal(err)
	}

	// Commit and push the update to the git repo.
	if msg, err := checkout.Git(ctx, "add", filename); err != nil {
		sklog.Fatal(err, msg)
	}
	if msg, err := checkout.Git(ctx, "commit", "-m", dateAsString, fmt.Sprintf("--date=%d", timestamp.Unix())); err != nil {
		sklog.Fatal(err, msg)
	}
	if msg, err := checkout.Git(ctx, "push", git.DefaultRemote, git.MainBranch); err != nil {
		sklog.Fatal(err, msg)
	}
	sklog.Infof("Success: %s", dateAsString)
}
