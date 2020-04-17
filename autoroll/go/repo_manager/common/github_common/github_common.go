package github_common

import (
	"strings"

	"go.skia.org/infra/go/skerr"
)

// SplitGithubUserAndRepo splits the given Github repo URL into user and repo
// names.
func SplitGithubUserAndRepo(githubRepo string) (string, string, error) {
	split := strings.Split(githubRepo, ":")
	if len(split) != 2 {
		return "", "", skerr.Fmt("invalid Github repo URL; expected \"git@github.com:<user>/<repo>\" but got: %s", githubRepo)
	}
	split = strings.Split(split[1], "/")
	if len(split) != 2 {
		return "", "", skerr.Fmt("invalid Github repo URL; expected \"git@github.com:<user>/<repo>\" but got: %s", githubRepo)
	}
	user := split[0]
	repo := strings.TrimRight(split[1], ".git")
	return user, repo, nil
}
