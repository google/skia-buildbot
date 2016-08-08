package main

/*
	Program used for forcibly triggering builds.
*/

import (
	"flag"
	"path"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
)

var (
	// REPOS are the repositories to query.
	REPOS = []string{
		common.REPO_SKIA,
		common.REPO_SKIA_INFRA,
	}

	builder       = flag.String("builder", "", "Builder name, or comma-separated list of builder names.")
	commit        = flag.String("commit", "", "Commit")
	swarmingBotId = flag.String("swarming_bot_id", "", "Swarming bot ID (optional)")
	workdir       = flag.String("workdir", "workdir", "Working directory to use.")
)

func main() {
	defer common.LogPanic()
	common.Init()

	if *builder == "" {
		glog.Fatal("builder name is required.")
	}
	if *commit == "" {
		glog.Fatal("commit hash is required.")
	}

	// Sync the repos, find the desired commit.
	author := ""
	repoName := ""
	repos := gitinfo.NewRepoMap(*workdir)
	for _, r := range REPOS {
		repo, err := repos.Repo(r)
		if err != nil {
			glog.Fatal(err)
		}
		details, err := repo.Details(*commit, false)
		if err == nil {
			author = details.Author
			repoName = r
			break
		}
	}
	if repoName == "" {
		glog.Fatalf("Unable to find commit %s in any repo.", *commit)
	}

	// Obtain the list(s) of valid builders.
	builderNames := strings.Split(*builder, ",")
	builders := make([]*buildbot.Builder, 0, len(builderNames))
	buildbotBuilders, err := buildbot.GetBuilders()
	if err != nil {
		glog.Fatal(err)
	}
	swarmingBuildersList, err := buildbucket.GetBotsForRepo(repoName)
	if err != nil {
		glog.Fatal(err)
	}
	swarmingBuilders := make(map[string]bool, len(swarmingBuildersList))
	for _, b := range swarmingBuildersList {
		swarmingBuilders[b] = true
	}

	// Ensure that each of the requested builders is valid.
	for _, builderName := range builderNames {
		if b, ok := buildbotBuilders[builderName]; ok {
			builders = append(builders, b)
		} else if _, ok := swarmingBuilders[builderName]; ok {
			b = &buildbot.Builder{
				Name:          builderName,
				Master:        "client.skia.fyi",
				PendingBuilds: 0,
				Slaves:        []string{},
				State:         "",
			}
			builders = append(builders, b)
		} else {
			glog.Fatalf("Unknown builder %s", builderName)
		}
	}

	// Initialize the BuildBucket client.
	c, err := auth.NewClient(true, path.Join(*workdir, "oauth_token_cache"), buildbucket.DEFAULT_SCOPES...)
	if err != nil {
		glog.Fatal(err)
	}
	bb := buildbucket.NewClient(c)

	// Schedule the build.
	for _, b := range builders {
		scheduled, err := bb.RequestBuild(b.Name, b.Master, *commit, repoName, author, *swarmingBotId)
		if err != nil {
			glog.Fatal(err)
		}
		glog.Infof("Triggered %s : %s", scheduled.Id, scheduled.Url)
	}
}
