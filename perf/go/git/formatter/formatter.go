package formatter

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/types"
)

// NewCommitHashRangeFormatter returns a standard CommitHashRangeFormatter that builds Git log URLs using the instance GitRepoConfig.
// Logic should be as close to commit-range-sk's _buildUrl as possible.
func NewCommitHashRangeFormatter(perfGit git.Git) types.CommitHashRangeFormatter {
	return func(ctx context.Context, startCommit, endCommit int64) string {
		startHash, err := perfGit.GitHashFromCommitNumber(ctx, types.CommitNumber(startCommit))
		if err != nil {
			return fmt.Sprintf("(%d..%d]", startCommit, endCommit)
		}
		endHash, err := perfGit.GitHashFromCommitNumber(ctx, types.CommitNumber(endCommit))
		if err != nil {
			return fmt.Sprintf("(%d..%d]", startCommit, endCommit)
		}

		startDisplayed := startHash[:min(len(startHash), 8)]
		endDisplayed := endHash[:min(len(endHash), 8)]

		basePath := config.Config.GitRepoConfig.URL
		var urlTemplate string

		switch config.Config.GitRepoConfig.Provider {
		case config.GitProviderGitiles:
			urlTemplate = basePath + "/+log/{begin}..{end}"
		case config.GitProviderCLI:
			urlTemplate = basePath + "/compare/{begin}...{end}"
		default:
			sklog.Errorf("unknown git provider %s", config.Config.GitRepoConfig.Provider)
			return fmt.Sprintf("(%s..%s]", startDisplayed, endDisplayed)
		}
		commitUrl := URLFromCommitRange(
			provider.Commit{GitHash: endHash},
			provider.Commit{GitHash: startHash},
			urlTemplate,
		)

		if startCommit+1 == endCommit {
			return fmt.Sprintf("[%s](%s)", endDisplayed, commitUrl)
		}
		return fmt.Sprintf("[\\(%s..%s\\]](%s)", startDisplayed, endDisplayed, commitUrl)
	}
}

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
