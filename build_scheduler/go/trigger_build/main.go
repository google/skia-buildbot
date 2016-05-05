package main

import (
	"flag"
	"path"

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

	builder = flag.String("builder", "", "Builder name.")
	commit  = flag.String("commit", "", "Commit")
	workdir = flag.String("workdir", "workdir", "Working directory to use.")
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

	// Find the builder.
	builders, err := buildbot.GetBuilders()
	if err != nil {
		glog.Fatal(err)
	}
	b, ok := builders[*builder]
	if !ok {
		glog.Fatalf("Unknown builder %s", *builder)
	}

	// Initialize the BuildBucket client.
	c, err := auth.NewClient(true, path.Join(*workdir, "oauth_token_cache"), buildbucket.DEFAULT_SCOPES...)
	if err != nil {
		glog.Fatal(err)
	}
	bb := buildbucket.NewClient(c)

	// Schedule the build.
	scheduled, err := bb.RequestBuild(b.Name, b.Master, *commit, repoName, author)
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Triggered %s. Builder page: %s%s/builders/%s", scheduled.Id, buildbot.BUILDBOT_URL, b.Master, b.Name)
}
