// For a given time period, find all the bots that failed when a CL, that was
// later reverted, first landed. The count of failed bots does not include bots
// that failed at both the initial commit and at the revert. Note that "-All"
// is removed from bot names.
//
// Running this requires a client_secret.json file in the current directory that
// is good for accessing the swarming API.
package main

import (
	"context"
	"flag"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
)

var (
	repo              = flag.String("repo", ".", "Path to Git repo.")
	since             = flag.String("since", "6months", "How far back to search in git history.")
	findRevertString  = regexp.MustCompile("Revert")
	extractRevertHash = regexp.MustCompile("This reverts commit ([a-z0-9]+)")
	removeAll         = regexp.MustCompile("-All")
)

func failingTasksAtACommit(ctx context.Context, swarmApi swarming.ApiClient, hash string) map[string]bool {
	resp, err := swarmApi.ListTasks(ctx, time.Time{}, time.Time{}, []string{fmt.Sprintf("source_revision:%s", hash)}, "completed_failure")
	if err != nil {
		sklog.Fatal(err)
	}
	ret := map[string]bool{}
	names := []string{}
	for _, r := range resp {
		name := removeAll.ReplaceAllLiteralString(r.TaskResult.Name, "")
		ret[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	sklog.Infof("Failing: %v", names)
	return ret
}

func main() {
	common.Init()

	gd := git.GitDir(*repo)
	ts, err := auth.NewDefaultTokenSource(true, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	swarmApi, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		sklog.Fatal(err)
	}
	ctx := context.Background()

	// Get all the commits since *since with one line per commit.
	output, err := gd.Git(ctx, "log", "--format=oneline", fmt.Sprintf("--since=%s", *since))
	if err != nil {
		sklog.Fatal(err)
	}
	lines := strings.Split(output, "\n")
	counts := map[string]int{}
	numCommitsReverted := 0
	for _, line := range lines {
		// Only include commits that have an odd number of occurrences of the string "Revert" in their title.
		sklog.Infof("Line: %s", line)
		revertHash := strings.Split(line, " ")[0]
		numReverts := len(findRevertString.FindAllString(line, -1))
		sklog.Infof("numReverts: %d", numReverts)
		if numReverts%2 == 1 {
			// From the revert CL, find the git hash of the initial commit we are reverting.
			ci, err := gd.Details(ctx, revertHash)
			if err != nil {
				sklog.Fatal(err)
			}
			match := extractRevertHash.FindStringSubmatch(ci.Body)
			sklog.Infof("match: %#v", match)
			if len(match) == 2 {
				numCommitsReverted += 1
				badHash := match[1]
				sklog.Infof("%s was reverted by %s", badHash, line)
				sklog.Info("Which tasks failed?")
				failed := failingTasksAtACommit(ctx, swarmApi, badHash)
				sklog.Info("Which tasks were still failing upon revert?")
				failingEvenAfterRevert := failingTasksAtACommit(ctx, swarmApi, revertHash)
				// Only count bots that appear in failed and not in failingEvenAfterRevert.
				for k := range failed {
					if failingEvenAfterRevert[k] {
						continue
					}
					counts[k] = counts[k] + 1
				}
			} else {
				sklog.Infof("Failed to find revert hash in: %s", ci.Body)
			}
		}
	}
	for k, v := range counts {
		fmt.Printf("%d %s\n", v, k)
	}
	fmt.Printf("\n\nFound %d commits reverted in %d commits. %.2f%%\n", numCommitsReverted, len(lines), float32(100*numCommitsReverted)/float32(len(lines)))
}
