package notify

import (
	"strings"

	"go.skia.org/infra/perf/go/git/provider"
)

// URLFromCommitRange returns a URL that points to commit.URL or the expansion
// of the commitRangeURLTemplate if appropriate.
func URLFromCommitRange(commit, previousCommit provider.Commit, commitRangeURLTemplate string) string {
	if commit.GitHash == previousCommit.GitHash {
		return commit.URL
	}
	if commitRangeURLTemplate == "" {
		return commit.URL
	}
	// Do template expansion.
	commitRangeURLTemplate = strings.ReplaceAll(commitRangeURLTemplate, "{begin}", previousCommit.GitHash)
	commitRangeURLTemplate = strings.ReplaceAll(commitRangeURLTemplate, "{end}", commit.GitHash)
	return commitRangeURLTemplate
}
