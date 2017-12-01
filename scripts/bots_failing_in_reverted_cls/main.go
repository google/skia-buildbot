// Run in a git repo it will print out all the hashes of git commits
// that contain an odd number of occurences of the word "Revert".
// The second hash emitted per line is the commit that follows the one
// with revert.
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

func failingTasksAtACommit(swarmApi swarming.ApiClient, hash string) map[string]bool {
	resp, err := swarmApi.ListTasks(time.Time{}, time.Time{}, []string{fmt.Sprintf("source_revision:%s", hash)}, "completed_failure")
	if err != nil {
		sklog.Fatal(err)
	}
	ret := map[string]bool{}
	names := []string{}
	for _, r := range resp {
		ret[r.TaskResult.Name] = true
		names = append(names, removeAll.ReplaceAllLiteralString(r.TaskResult.Name, ""))
	}
	sort.Strings(names)
	sklog.Infof("Failing: %v", names)
	return ret
}

func main() {
	defer common.LogPanic()
	common.Init()

	gd := git.GitDir(*repo)
	httpClient, err := auth.NewDefaultClient(true, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	swarmApi, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		sklog.Fatal(err)
	}

	ctx := context.Background()
	output, err := gd.Git(ctx, "log", "--format=oneline", fmt.Sprintf("--since=%s", *since))
	if err != nil {
		sklog.Fatal(err)
	}
	lines := strings.Split(output, "\n")
	counts := map[string]int{}
	for _, line := range lines {
		sklog.Infof("Line: %s", line)
		revertHash := strings.Split(line, " ")[0]
		numReverts := len(findRevertString.FindAllString(line, -1))
		sklog.Infof("numReverts: %d", numReverts)
		if numReverts%2 == 1 {
			ci, err := gd.Details(ctx, revertHash)
			if err != nil {
				sklog.Fatal(err)
			}
			match := extractRevertHash.FindStringSubmatch(ci.Body)
			sklog.Infof("match: %#v", match)
			if len(match) == 2 {
				badHash := match[1]
				sklog.Infof("%s was reverted by %s", badHash, line)
				sklog.Info("Which tasks failed?")
				failed := failingTasksAtACommit(swarmApi, badHash)
				sklog.Info("Which tasks were still failing upon revert?")
				failingEvenAfterRevert := failingTasksAtACommit(swarmApi, revertHash)
				for k, _ := range failed {
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
}
