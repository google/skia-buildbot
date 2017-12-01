// Run in a git repo it will print out all the hashes of git commits
// that contain an odd number of occurences of the word "Revert".
// The second hash emitted per line is the commit that follows the one
// with revert.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
)

var (
	repo              = flag.String("repo", ".", "Path to Git repo.")
	since             = flag.String("since", "6months", "How far back to search in git history.")
	findRevertString  = regexp.MustCompile("Revert")
	extractRevertHash = regexp.MustCompile("This reverts commit ([a-z0-9]+)")
)

func failingTasksAtACommit(swarmApi swarming.ApiClient, hash string) map[string]bool {
	resp, err := swarmApi.ListTasks(time.Time{}, time.Time{}, []string{fmt.Sprintf("source_revision:%s", hash)}, "completed_failure")
	if err != nil {
		sklog.Fatal(err)
	}
	ret := map[string]bool{}
	for _, r := range resp {
		ret[r.TaskResult.Name] = true
	}
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

	output := bytes.Buffer{}
	ctx := context.Background()
	err = exec.Run(ctx, &exec.Command{
		Name:           "git",
		Args:           []string{"log", "--format=oneline", fmt.Sprintf("--since=%s", *since)},
		CombinedOutput: &output,
	})
	if err != nil {
		sklog.Fatal(err)
	}
	lines := strings.Split(output.String(), "\n")
	counts := map[string]int{}
	for _, line := range lines {
		revertHash := strings.Split(line, " ")[0]
		numReverts := len(findRevertString.FindAllString(line, -1))
		if numReverts%2 == 1 {
			ci, err := gd.Details(ctx, revertHash)
			if err != nil {
				sklog.Fatal(err)
			}
			match := extractRevertHash.FindStringSubmatch(ci.Body)
			if len(match) == 1 {
				sklog.Infof("%s was reverted by %s", match[0], line)
				failed := failingTasksAtACommit(swarmApi, match[0])
				failingEvenAfterRevert := failingTasksAtACommit(swarmApi, revertHash)
				for k, _ := range failed {
					if failingEvenAfterRevert[k] {
						continue
					}
					counts[k] = counts[k] + 1
				}
			}
		}
	}
}
