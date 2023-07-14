package notify

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/git/provider"
)

const uriTemplate = "https://example.org/from/{begin}/to/{end}/"

var (
	commit = provider.Commit{
		GitHash: "333333333333",
		URL:     "https://example.com/333333333333",
	}
	previousCommit = provider.Commit{
		GitHash: "111111111111",
		URL:     "https://example.com/111111111111",
	}
)

func TestURLFromCommitRange_NoTemplateProvided_ReturnsCommitURL(t *testing.T) {
	require.Equal(t, commit.URL, URLFromCommitRange(commit, previousCommit, ""))
}

func TestURLFromCommitRange_BothCommitsAreTheSame_ReturnsJustTheOneCommitURL(t *testing.T) {
	require.Equal(t, previousCommit.URL, URLFromCommitRange(previousCommit, previousCommit, uriTemplate))
}

func TestURLFromCommitRange_HappyPath(t *testing.T) {
	require.Equal(t, "https://example.org/from/111111111111/to/333333333333/", URLFromCommitRange(commit, previousCommit, uriTemplate))
}
