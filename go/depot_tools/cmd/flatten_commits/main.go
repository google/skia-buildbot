package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

var (
	repoURL = flag.String("repo", "", "Git repository")
	commit  = flag.String("commit", "", "Git commit hash")
	tree    = flag.Bool("tree", false, "Print the full tree of commits, including roll-only commits.")
)

type commitInfo struct {
	*vcsinfo.LongCommit
	Repo     string
	Includes map[string][]*commitInfo
	RollOnly bool
}

func (c *commitInfo) strLine() string {
	const authorLen = 25
	const subjectLen = 50
	authorSplit := strings.Split(c.Author, "(")
	author := authorSplit[0]
	if len(authorSplit) > 1 {
		author = strings.Split(authorSplit[1], ")")[0]
	}
	author = util.Truncate(strings.TrimSpace(author), authorLen)
	author = author + strings.Repeat(" ", authorLen-len(author))
	subject := util.Truncate(c.Subject, subjectLen)
	subject = subject + strings.Repeat(" ", subjectLen-len(subject))
	line := fmt.Sprintf("%s %s %s %s/+/%s", c.Hash[:7], author, subject, c.Repo, c.Hash[:12])
	if len(c.Includes) > 0 && !c.RollOnly {
		line += " (contains additional non-DEPS changes)"
	}
	return line
}

func (c *commitInfo) recurseInner(fn func(*commitInfo, int), depth int) {
	fn(c, depth)
	if len(c.Includes) > 0 {
		repos := make([]string, 0, len(c.Includes))
		for repo := range c.Includes {
			repos = append(repos, repo)
		}
		sort.Strings(repos)
		for _, repo := range repos {
			for _, commit := range c.Includes[repo] {
				commit.recurseInner(fn, depth+1)
			}
		}
	}
}

func (c *commitInfo) recurse(fn func(*commitInfo, int)) {
	c.recurseInner(fn, 0)
}

func (c *commitInfo) Tree() string {
	var lines []string
	fn := func(c *commitInfo, depth int) {
		str := strings.Repeat("    ", depth) + c.strLine()
		lines = append(lines, str)
	}
	c.recurse(fn)
	return strings.Join(lines, "\n")
}

func (c *commitInfo) Flatten() string {
	var lines []string
	fn := func(c *commitInfo, depth int) {
		if !c.RollOnly {
			lines = append(lines, c.strLine())
		}
	}
	c.recurse(fn)
	return strings.Join(lines, "\n")
}

func getDEPS(ctx context.Context, repo *gitiles.Repo, commit string) (deps_parser.DepsEntries, error) {
	sklog.Infof("Retrieving %s from %s at %s", deps_parser.DepsFileName, repo.URL, commit)
	var buf bytes.Buffer
	if err := repo.ReadFileAtRef(ctx, deps_parser.DepsFileName, commit, &buf); err != nil {
		return nil, err
	}
	return deps_parser.ParseDeps(buf.String())
}

func getCommit(ctx context.Context, repo *gitiles.Repo, commit *vcsinfo.LongCommit, client *http.Client) (*commitInfo, error) {
	sklog.Infof("Retrieving %s at %s", repo.URL, commit.Hash)
	rv := &commitInfo{
		LongCommit: commit,
		Repo:       repo.URL,
		RollOnly:   true,
	}
	diffs, err := repo.GetTreeDiffs(ctx, commit.Hash)
	if err != nil {
		return nil, err
	}
	for _, diff := range diffs {
		if diff.NewPath == deps_parser.DepsFileName {
			depsBefore, err := getDEPS(ctx, repo, commit.Parents[0])
			if err != nil {
				return nil, err
			}
			depsAfter, err := getDEPS(ctx, repo, commit.Hash)
			if err != nil {
				return nil, err
			}
			for id, pinBefore := range depsBefore {
				if !strings.Contains(id, "googlesource") {
					// We can only handle repos with a Gitiles frontend.
					continue
				}
				pinAfter, ok := depsAfter[id]
				if !ok {
					// Just ignore added or removed deps.
					continue
				}
				if pinAfter.Version == pinBefore.Version {
					// Don't bother with unchanged dependencies.
					continue
				}
				depRepoURL := "https://" + id
				depRepo := gitiles.NewRepo(depRepoURL, client)
				log, err := depRepo.Log(ctx, git.LogFromTo(pinBefore.Version, pinAfter.Version))
				if err != nil {
					return nil, err
				}
				commits, err := getCommits(ctx, depRepo, log, client)
				if err != nil {
					return nil, err
				}
				if rv.Includes == nil {
					rv.Includes = map[string][]*commitInfo{}
				}
				rv.Includes[depRepoURL] = commits
			}
		} else {
			rv.RollOnly = false
		}
	}
	return rv, nil
}

func getCommits(ctx context.Context, repo *gitiles.Repo, commits []*vcsinfo.LongCommit, client *http.Client) ([]*commitInfo, error) {
	rv := make([]*commitInfo, 0, len(commits))
	for _, commit := range commits {
		ci, err := getCommit(ctx, repo, commit, client)
		if err != nil {
			return nil, err
		}
		rv = append(rv, ci)
	}
	return rv, nil
}

func main() {
	common.Init()
	if *repoURL == "" {
		sklog.Fatal("--repo is required.")
	}
	if *commit == "" {
		sklog.Fatal("--commit is required.")
	}
	ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	ctx := context.Background()
	repo := gitiles.NewRepo(*repoURL, client)
	details, err := repo.Details(ctx, *commit)
	if err != nil {
		sklog.Fatal(err)
	}
	result, err := getCommit(ctx, repo, details, client)
	if err != nil {
		sklog.Fatal(err)
	}
	var output string
	if *tree {
		output = result.Tree()
	} else {
		output = result.Flatten()
	}
	fmt.Println(output)
}
