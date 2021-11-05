package main

/*
	Find results of a task across a range of commits given by a git log expression.

	This is essentially a CLI version of status.skia.org, but for a single task
	and with better support for git log expressions and the ability to go beyond
	the 200-commit limit.

	Example:

	$ go run ./find_task_results_by_git_log.go
		--task=Test-Debian10-Clang-NUC7i5BNK-GPU-IntelIris640-x86_64-Debug-All-DDL3_ASAN_Vulkan
		d9d9e21..817dba1
	Found 5 commits
	817dba1 SUCCESS
	f0b283e SUCCESS
	2739def SUCCESS
	569bf57 FAILURE
	a8d3807 FAILURE
*/

import (
	"context"
	"flag"
	"fmt"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
)

var (
	repo  = flag.String("repo", common.REPO_SKIA, "Git repository to search.")
	task  = flag.String("task", "", "Name of the task to search.")
	limit = flag.Int("limit", 100, "Limit to this many commits")
)

func main() {
	flag.Parse()

	if *task == "" {
		sklog.Fatal("--task is required.")
	}

	logExpr := flag.Arg(0)
	if logExpr == "" {
		logExpr = git.MainBranch
	}

	ctx := context.Background()

	ts, err := auth.NewDefaultTokenSource(true, auth.ScopeGerrit, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()

	d, err := firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, "production", ts)
	if err != nil {
		sklog.Fatal(err)
	}

	commits, err := gitiles.NewRepo(*repo, client).Log(ctx, logExpr, gitiles.LogLimit(*limit))
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println(fmt.Sprintf("Found %d commits", len(commits)))
	noIssue := ""
	for _, commit := range commits {
		tasks, err := d.SearchTasks(ctx, &db.TaskSearchParams{
			Name:     task,
			Repo:     repo,
			Revision: &commit.Hash,
			Issue:    &noIssue,
		})
		if err != nil {
			sklog.Fatal(err)
		}
		statusStr := ""
		for _, task := range tasks {
			statusStr += " " + string(task.Status)
		}
		fmt.Println(fmt.Sprintf("%s%s", commit.Hash[:7], statusStr))
	}
}
