package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/golang/protobuf/proto"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/cq"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/specs"
)

var (
	configFile = flag.String("cfg-file", "", "commit-queue.cfg file to validate.")
	remote     = flag.String("remote", "origin", "Name of upstream remote to use.")
)

func main() {
	common.Init()

	if *configFile == "" {
		sklog.Fatal("--cfg-file is required.")
	}

	ctx := context.TODO()

	// Find the branch heads.
	cwd, err := os.Getwd()
	if err != nil {
		sklog.Fatalf("Failed to get current working directory: %s", err)
	}
	gd := git.GitDir(cwd)
	output, err := gd.Git(ctx, "ls-remote", *remote, "refs/heads/*")
	if err != nil {
		sklog.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	branches := make([]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			sklog.Warningf("Don't understand %q; skipping.", line)
			continue
		}
		branches = append(branches, fields[1])
	}

	// For each branch head, read the tasks.json file.
	taskCfgs := make(map[string]*specs.TasksCfg, len(branches))
	for _, branch := range branches {
		shortBranch := branch[len("refs/heads/"):]
		output, err = gd.Git(ctx, "show", fmt.Sprintf("refs/remotes/%s/%s:%s", *remote, shortBranch, specs.TASKS_CFG_FILE))
		if err != nil {
			if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "exists on disk, but not in") {
				// This is valid; there are no trybots on this branch.
				taskCfgs[branch] = &specs.TasksCfg{
					Jobs: map[string]*specs.JobSpec{},
				}
				continue
			}
			sklog.Fatalf("Failed to read tasks.json for %s; %s", branch, err)
		}
		cfg, err := specs.ParseTasksCfg(output)
		if err != nil {
			sklog.Fatal("Failed to parse tasks.json: %s", err)
		}
		taskCfgs[branch] = cfg
	}

	// Read the commit queue config.
	cfgBytes, err := ioutil.ReadFile(*configFile)
	if err != nil {
		sklog.Fatalf("Failed to read %s: %s", *configFile, err)
	}
	var cfg cq.Config
	if err := proto.UnmarshalText(string(cfgBytes), &cfg); err != nil {
		sklog.Fatalf("Failed to parse config proto: %s", err)
	}

	// For each branch, find the matching CQ config group. Ensure that the
	// tryjobs exist in the TasksCfg for that branch.
	badTryjobs := map[string][]string{}
	for _, branch := range branches {
		cg, _, _, err := cq.MatchConfigGroup(&cfg, branch)
		if err != nil {
			sklog.Fatal("Failed to find matching CQ config for %q: %s", branch, err)
		}
		taskCfg := taskCfgs[branch]
		if cg != nil && cg.Verifiers != nil && cg.Verifiers.Tryjob != nil {
			for _, tj := range cg.Verifiers.Tryjob.Builders {
				split := strings.Split(tj.Name, "/")
				shortName := split[len(split)-1]
				if strings.HasPrefix(tj.Name, "chromium") {
					if branch != "refs/heads/master" {
						// Chromium tryjobs only work on master.
						badTryjobs[branch] = append(badTryjobs[branch], shortName)
					}
				} else if _, ok := taskCfg.Jobs[shortName]; !ok {
					badTryjobs[branch] = append(badTryjobs[branch], shortName)
				}
			}
		}
	}

	// Report the results.
	if len(badTryjobs) > 0 {
		fmt.Println("Found try jobs which don't exist on their respective branches:")
		for branch, tryjobs := range badTryjobs {
			fmt.Println(fmt.Sprintf("%s:", branch))
			for _, tj := range tryjobs {
				fmt.Println(fmt.Sprintf("\t%s", tj))
			}
		}
		os.Exit(1)
	}
}
