package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/golang/protobuf/proto"
	"go.chromium.org/luci/cv/api/config/v2"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/cq"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/specs"
)

var (
	configFile = flag.String("cfg-file", "", "commit-queue.cfg file to validate.")
	remote     = flag.String("remote", git.DefaultRemote, "Name of upstream remote to use.")
	repoUrl    = flag.String("repo", "", "Repo URL. Required if not currently inside a git checkout.")
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
	if *repoUrl == "" {
		*repoUrl, err = gd.Git(ctx, "remote", "get-url", *remote)
		if err != nil {
			sklog.Fatalf("--repo not provided and failed to obtain repo url for cwd: %s", err)
		}
		*repoUrl = strings.TrimSpace(*repoUrl)
	}
	repo := gitiles.NewRepo(*repoUrl, nil)
	branches, err := repo.Branches(ctx)
	if err != nil {
		sklog.Fatal(err)
	}

	// For each branch head, read the tasks.json file.
	taskCfgs := make(map[string]*specs.TasksCfg, len(branches))
	for _, branch := range branches {
		contents, err := repo.ReadFileAtRef(ctx, specs.TASKS_CFG_FILE, branch.Head)
		if err != nil {
			if strings.Contains(err.Error(), "404 Not Found") {
				sklog.Warningf("Could not find %s on %s", specs.TASKS_CFG_FILE, branch.Name)
				// This is valid; there are no trybots on this branch.
				taskCfgs[branch.Name] = &specs.TasksCfg{
					Jobs: map[string]*specs.JobSpec{},
				}
				continue
			}
			sklog.Fatalf("Failed to read tasks.json for %s; %s", branch.Name, err)
		}
		cfg, err := specs.ParseTasksCfg(string(contents))
		if err != nil {
			sklog.Fatalf("Failed to parse tasks.json: %s", err)
		}
		taskCfgs[branch.Name] = cfg
	}

	// Read the commit queue config.
	cfgBytes, err := ioutil.ReadFile(*configFile)
	if err != nil {
		sklog.Fatalf("Failed to read %s: %s", *configFile, err)
	}
	var cfg config.Config
	if err := proto.UnmarshalText(string(cfgBytes), &cfg); err != nil {
		sklog.Fatalf("Failed to parse config proto: %s", err)
	}

	// For each branch, find the matching CQ config group. Ensure that the
	// tryjobs exist in the TasksCfg for that branch.
	badTryjobs := map[string][]string{}
	for _, branch := range branches {
		cg, _, _, err := cq.MatchConfigGroup(&cfg, fmt.Sprintf("refs/heads/%s", branch.Name))
		if err != nil {
			sklog.Fatalf("Failed to find matching CQ config for %q: %s", branch.Name, err)
		}
		taskCfg := taskCfgs[branch.Name]
		if cg != nil && cg.Verifiers != nil && cg.Verifiers.Tryjob != nil {
			for _, tj := range cg.Verifiers.Tryjob.Builders {
				split := strings.Split(tj.Name, "/")
				shortName := split[len(split)-1]
				if strings.HasPrefix(tj.Name, "chromium") {
					if branch.Name != git.DefaultBranch {
						// Chromium tryjobs only work on the main branch.
						badTryjobs[branch.Name] = append(badTryjobs[branch.Name], shortName)
					}
				} else if _, ok := taskCfg.Jobs[shortName]; !ok {
					badTryjobs[branch.Name] = append(badTryjobs[branch.Name], shortName)
				}
			}
		} else if cg == nil {
			sklog.Warningf("No matching CQ config for %s", branch.Name)
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
