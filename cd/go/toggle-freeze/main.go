/*
Package contains toggle-freeze, a tool to automate the freeze procedure for deployments.

It introduces the infrastructure to seamlessly freeze deployments by managing a
freeze lock file. The tool takes an action ('freeze', 'unfreeze', or 'toggle')
and manages the creation or removal of the specified lock file locally.

It handles the entire Git workflow:
  - Committing the state change
  - Pushing to Gerrit for auto-submission
  - Polling the Gerrit API until the CL is successfully merged
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gerrit/rubberstamper"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/td"
	"golang.org/x/oauth2/google"
)

func main() {
	action := flag.String("action", "", "Action to perform: 'freeze', 'unfreeze', or 'toggle'. (Required)")
	freezeFile := flag.String("freeze-file", "", "Path to the freeze lock file. (Required)")
	email := flag.String("email", "louhi-service-account@example.com", "Email address for Git authentication.")

	fakeProjectId := ""
	fakeTaskId := ""
	fakeTaskName := ""
	output := "-"
	local := true
	ctx := td.StartRun(&fakeProjectId, &fakeTaskId, &fakeTaskName, &output, &local)
	defer td.EndRun(ctx)

	if *action == "" {
		err := skerr.Fmt("--action is required")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		td.Fatal(ctx, err)
	}
	if *freezeFile == "" {
		err := skerr.Fmt("--freeze-file is required")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		td.Fatal(ctx, err)
	}

	if err := run(ctx, *action, *freezeFile, *email, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		td.Fatal(ctx, err)
	}
}

func run(ctx context.Context, action, freezeFile, email string, g gerrit.GerritInterface) error {
	ctx = td.StartStep(ctx, td.Props("Toggle Freeze"))
	defer td.EndStep(ctx)

	cwd, err := os.Getwd()
	if err != nil {
		return td.FailStep(ctx, skerr.Wrap(err))
	}

	// Determine file state
	freezePath := filepath.Join(cwd, freezeFile)
	_, err = os.Stat(freezePath)
	exists := !os.IsNotExist(err)

	var shouldExist bool
	switch action {
	case "freeze":
		shouldExist = true
	case "unfreeze":
		shouldExist = false
	case "toggle":
		shouldExist = !exists
	default:
		return td.FailStep(ctx, skerr.Fmt("Invalid action %q. Must be freeze, unfreeze, or toggle.", action))
	}

	if exists == shouldExist {
		fmt.Printf("File %q already in desired state (exists=%v)\n", freezeFile, exists)
		return nil
	}

	// Apply change
	if shouldExist {
		if err := os.MkdirAll(filepath.Dir(freezePath), 0755); err != nil {
			return td.FailStep(ctx, skerr.Wrap(err))
		}
		if err := os.WriteFile(freezePath, []byte("FREEZE"), 0644); err != nil {
			return td.FailStep(ctx, skerr.Wrap(err))
		}
	} else {
		if err := os.Remove(freezePath); err != nil {
			return td.FailStep(ctx, skerr.Wrap(err))
		}
	}

	// Git operations
	ts, err := git_steps.Init(ctx, true)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	if _, err := gitauth.New(ctx, ts, "/tmp/.gitcookies", true, email); err != nil {
		return td.FailStep(ctx, err)
	}

	gitExec, err := git.Executable(ctx)
	if err != nil {
		return td.FailStep(ctx, skerr.Wrap(err))
	}

	if _, err := exec.RunCwd(ctx, cwd, gitExec, "add", freezeFile); err != nil {
		return td.FailStep(ctx, skerr.Wrap(err))
	}

	commitMsg := fmt.Sprintf("%s %s\n\n%s", "Toggle freeze", freezeFile, rubberstamper.RandomChangeID(ctx))
	if _, err := exec.RunCwd(ctx, cwd, gitExec, "commit", "-m", commitMsg); err != nil {
		return td.FailStep(ctx, skerr.Wrap(err))
	}

	out, err := exec.RunCwd(ctx, cwd, gitExec, "push", git.DefaultRemote, rubberstamper.PushRequestAutoSubmit)
	if err != nil {
		return td.FailStep(ctx, skerr.Wrap(err))
	}

	var uploadedCLRegex = regexp.MustCompile(`https://.*review\.googlesource\.com.*\d+`)
	match := uploadedCLRegex.FindString(out)
	if match == "" {
		return td.FailStep(ctx, skerr.Fmt("Failed to parse CL link from:\n%s", out))
	}
	parts := strings.Split(match, "/")
	issueStr := parts[len(parts)-1]
	fmt.Printf("Uploaded CL: %s\n", match)

	if g == nil {
		clientts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit)
		if err != nil {
			return td.FailStep(ctx, skerr.Wrap(err))
		}
		client := httputils.DefaultClientConfig().WithTokenSource(clientts).Client()
		g, err = gerrit.NewGerrit("https://skia-review.googlesource.com", client)
		if err != nil {
			return td.FailStep(ctx, skerr.Wrap(err))
		}
	}

	for {
		ci, err := g.GetChange(ctx, issueStr)
		if err != nil {
			fmt.Printf("Failed to get CL status: %v\n", err)
		} else {
			if ci.Status == gerrit.ChangeStatusMerged {
				fmt.Println("CL merged successfully.")
				return nil
			}
			if ci.Status == gerrit.ChangeStatusAbandoned {
				return td.FailStep(ctx, skerr.Fmt("CL %s was abandoned.", match))
			}
			fmt.Printf("CL status is %s. Waiting...\n", ci.Status)
		}
		select {
		case <-ctx.Done():
			return td.FailStep(ctx, ctx.Err())
		case <-time.After(5 * time.Second):
		}
	}
}
