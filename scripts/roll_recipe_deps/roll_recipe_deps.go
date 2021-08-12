package main

/*
	Program used for updating recipe DEPS.

	Follows a dependency graph of repos and creates roll CLs for those which
	are not up-to-date. If any dependency of a repo is not up-to-date, that
	repo is skipped. Therefore, run this script repeatedly as CLs are
	created and land, until all repos are up-to-date.

	For example:

        // Upload recipe roll CLs for infra and skia-recipes repos:
	$ go run scripts/roll_recipe_deps/roll_recipe_deps.go

	// After the skia-recipes roll above lands, the following will upload
	// a roll CL for the skia repo:
	$ go run scripts/roll_recipe_deps/roll_recipe_deps.go

	// After the skia roll above lands, the following is a no-op:
        $ go run scripts/roll_recipe_deps/roll_recipe_deps.go

	Note that if you run this script again before an uploaded roll lands,
	the script will upload another roll.
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	// RECIPES_PY_PATH indicates where in the given repo the recipes.py
	// file lives.
	RECIPES_PY_PATH = map[string]string{
		common.REPO_SKIA:       "infra/bots/recipes.py",
		common.REPO_SKIA_INFRA: "infra/bots/recipes.py",
	}

	// REPOS maps out the recipe dependency relationship between
	// repositories.
	REPOS = map[string][]string{
		common.REPO_SKIA_INFRA: {},
		common.REPO_SKIA:       {},
	}
)

// issueJson is a struct which matches the format of the JSON output of
// "git cl issue".
type issueJson struct {
	Issue    int    `json:"issue"`
	IssueUrl string `json:"issue_url"`
}

// rollJson is a struct which matches the format of the JSON output of
// "recipes.py autoroll".
type rollJson struct {
	PickedRollDetails struct {
		CommitInfos map[string][]struct {
			Author   string `json:"author"`
			Message  string `json:"message"`
			RepoId   string `json:"repo_id"`
			Revision string `json:"revision"`
		} `json:"commit_infos"`
	} `json:"picked_roll_details"`
}

// rollOnce performs a single recipe roll and returns the commits in the roll.
// The result will be empty if the repo is up-to-date.
func rollOnce(ctx context.Context, repoUrl, cwd string) (map[string][]string, error) {
	tmpDir, err := ioutil.TempDir("", "recipe_roll_")
	if err != nil {
		return nil, err
	}
	defer util.RemoveAll(tmpDir)
	outJson := path.Join(tmpDir, "roll.json")
	if _, err := exec.RunCwd(ctx, cwd, "python", RECIPES_PY_PATH[repoUrl], "autoroll", "--output-json", outJson); err != nil {
		return nil, err
	}
	f, err := os.Open(outJson)
	if err != nil {
		return nil, err
	}
	defer util.Close(f)
	var js rollJson
	if err := json.NewDecoder(f).Decode(&js); err != nil {
		return nil, err
	}
	rv := make(map[string][]string, len(js.PickedRollDetails.CommitInfos))
	for repo, commits := range js.PickedRollDetails.CommitInfos {
		shortCommits := make([]string, 0, len(commits))
		for _, c := range commits {
			msg := strings.Split(c.Message, "\n")[0]
			if len(msg) > 64 {
				msg = msg[:64]
			}
			shortCommits = append(shortCommits, fmt.Sprintf("%s %s", c.Revision[:7], msg))
		}
		rv[repo] = shortCommits
	}

	return rv, nil
}

// rollRepo performs a DEPS roll and uploads a CL if the repo is not up-to-date.
// Returns the URL of the uploaded CL, if any.
func rollRepo(ctx context.Context, repoUrl string) (string, error) {
	sklog.Infof("  Creating checkout...")
	tmpDir, err := ioutil.TempDir("", "recipe_roll_")
	if err != nil {
		return "", err
	}
	defer util.RemoveAll(tmpDir)
	repo, err := git.NewCheckout(ctx, repoUrl, tmpDir)
	if err != nil {
		return "", err
	}
	sklog.Infof("  Rolling recipe DEPS...")
	details, err := rollOnce(ctx, repoUrl, repo.Dir())
	if err != nil {
		return "", err
	}
	if len(details) == 0 {
		return "", nil
	}
	if _, err := repo.Git(ctx, "commit", "-a", "-m", "Roll Recipe DEPS"); err != nil {
		return "", err
	}

	commitMsg := "Roll Recipe DEPS\n\n"
	repoNames := make([]string, 0, len(details))
	for repo := range details {
		repoNames = append(repoNames, repo)
	}
	sort.Strings(repoNames)
	for _, repo := range repoNames {
		commitMsg += fmt.Sprintf("%s:\n", repo)
		for _, c := range details[repo] {
			commitMsg += fmt.Sprintf("\t%s\n", c)
		}
		commitMsg += "\n"
	}

	sklog.Infof("  Uploading roll CL...")
	if _, err := repo.Git(ctx, "cl", "upload", "--gerrit", "--bypass-hooks", "--cq-dry-run", "-f", "-m", commitMsg); err != nil {
		return "", err
	}
	issueFile := path.Join(tmpDir, "issue.json")
	if _, err := repo.Git(ctx, "cl", "issue", fmt.Sprintf("--json=%s", issueFile)); err != nil {
		return "", err
	}
	f, err := os.Open(issueFile)
	if err != nil {
		return "", err
	}
	defer util.Close(f)
	var issue issueJson
	if err := json.NewDecoder(f).Decode(&issue); err != nil {
		return "", err
	}
	return issue.IssueUrl, nil
}

func main() {
	common.Init()

	uploaded := []string{}
	ctx := context.Background()

	// Traverse the dependency graph of repos. If any repo is not
	// up-to-date, create a recipe roll for that repo. If any of a repo's
	// dependencies is not up-to-date, do not roll that repo.
	cachedResult := map[string]bool{}
	var recurse func(string) bool
	recurse = func(repoUrl string) bool {
		if result, ok := cachedResult[repoUrl]; ok {
			return result
		}
		for _, dep := range REPOS[repoUrl] {
			if !recurse(dep) {
				sklog.Infof("Not rolling %s; dependency %s is not up to date.", repoUrl, dep)
				return false
			}
		}
		sklog.Infof("Rolling %s...", repoUrl)
		rollIssue, err := rollRepo(ctx, repoUrl)
		if err != nil {
			sklog.Fatal(err)
		}
		if rollIssue == "" {
			cachedResult[repoUrl] = true
		} else {
			uploaded = append(uploaded, rollIssue)
			cachedResult[repoUrl] = false
		}
		return cachedResult[repoUrl]
	}

	for repo := range REPOS {
		recurse(repo)
	}

	if len(uploaded) > 0 {
		sklog.Infof("Uploaded CLs:")
		for _, cl := range uploaded {
			sklog.Infof("\t%s", cl)
		}
	}
}
