package notify

import (
	"go.skia.org/infra/perf/go/git/formatter"
	"go.skia.org/infra/perf/go/git/provider"
)

// URLFromCommitRange returns a URL that points to commit.URL or the expansion
// of the commitRangeURLTemplate if appropriate.
func URLFromCommitRange(commit, previousCommit provider.Commit, commitRangeURLTemplate string) string {
	return formatter.URLFromCommitRange(commit, previousCommit, commitRangeURLTemplate)
}
