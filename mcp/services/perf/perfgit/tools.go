package perfgit

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/pinpoint/go/backends"
)

const kGetCommitInfoForRevisionRangeToolName = "GetCommitInfoForRevisionRange"
const kGetCommitInfoForRevisionRangeToolDescription = `
Get the Git Commit Info for a given Chromium repo Revision Number range (inclusive).
Revision Numbers are given to Git Commits in the Chromium repo sequentially in the order
that they appear in the main branch. The result is a table of CSV data each row has the
following format:
  RevisionNumber,RepoURL,CommitHash,CommitSummary

If a Git Commmit's Summary starts with "Roll " or "Manual roll " then that Commit
was generated either to:
1. Bring in changes from another repo that this repo depends on. ("DEPS roll")
2. Update a data (eg. PGO) file in the repo. ("file roll")

If it is a DEPS roll then looking in that Commit's Body (see tool 'GetBodyForCommitHash')
will tell you which Commits from that other repo were included in the roll, as well as the
GoogleSource Git Repo URL of the other repo. Note that the Commit Hash range in a roll
description is exclusive of the start Commit Hash and inclusive of the end Commit Hash, so
for example given the description contains:
  https://chrome-internal.googlesource.com/chrome/browser/platform_experience/win.git/+log/35632e2e829b..e71c7abe3c0c
We should understand that the roll includes commit "e71c7abe3c0c" and not "35632e2e829b".
And furthermore we should understand that the Repo URL where we will find the named
commit is https://chrome-internal.googlesource.com/chrome/browser/platform_experience/win.git/

If it is a file roll then looking in that Commit's Body will tell you which file was changed
and how it was changed.

If a Git Commit's CommitSummary starts with "Revert " then that Commit was generated to
revert the commit with the quoted Summary.
`
const kStartRevisionNumberArgumentName = "StartRevisionNumber"
const kEndRevisionNumberArgumentName = "EndRevisionNumber"

const kGetBodyForCommitHashToolName = "GetBodyForCommitHash"
const kGetBodyForCommitHashToolDescription = `
Get the body of the Git Commit for a given Git Commit Hash, at the given GoogleSource
Git Repo URL (eg. https://chromium.googlesource.com/chromium/src/). The result is a
string containing the body of the Git Commit.

The first line of the body is considered the Git Commit's Summary.

If it is a DEPS roll then looking in that Commit's Body (see tool 'GetBodyForCommitHash')
will tell you which Commits from that other repo were included in the roll, as well as the
GoogleSource Git Repo URL of the other repo. Note that the Commit Hash range in a roll
description is exclusive of the start Commit Hash and inclusive of the end Commit Hash, so
for example given the description contains:
  https://chrome-internal.googlesource.com/chrome/browser/platform_experience/win.git/+log/35632e2e829b..e71c7abe3c0c
We should understand that the roll includes commit "e71c7abe3c0c" and not "35632e2e829b".
And furthermore we should understand that the Repo URL where we will find the named
commit is https://chrome-internal.googlesource.com/chrome/browser/platform_experience/win.git/

If it is a file roll then looking in that Commit's Body will tell you which file was changed
and how it was changed.

If a Git Commit's Summary starts with "Revert " then that Commit was generated to revert
the commit with the quoted Summary.
`
const kGitCommitHashArgumentName = "GitCommitHash"
const kGitRepoURLArgumentName = "GitRepoURL"

func GetTools(httpClient *http.Client, crrevClient *backends.CrrevClientImpl) []common.Tool {
	getCommitInfoForRevisionRangeTool := common.Tool{
		Name:        kGetCommitInfoForRevisionRangeToolName,
		Description: kGetCommitInfoForRevisionRangeToolDescription,
		Arguments: []common.ToolArgument{
			{
				Name:         kStartRevisionNumberArgumentName,
				Description:  "Start of Chromium repo Revision Number range (inclusive) for which to get the Git Commit Info.",
				Required:     true,
				ArgumentType: common.NumberArgument,
			},
			{
				Name:         kEndRevisionNumberArgumentName,
				Description:  "End of Chromium repo Revision Number range (inclusive) for which to get the Git Commit Info.",
				Required:     true,
				ArgumentType: common.NumberArgument,
			},
		},
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			startRevisionNumber, err := request.RequireInt(kStartRevisionNumberArgumentName)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			endRevisionNumber, err := request.RequireInt(kEndRevisionNumberArgumentName)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			csvString := ""
			for i := startRevisionNumber; i <= endRevisionNumber; i++ {
				crrevResponse, err := crrevClient.GetCommitInfo(ctx, fmt.Sprintf("%d", i))
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				repoURL := fmt.Sprintf("https://%s.googlesource.com/%s/", crrevResponse.Project, crrevResponse.Repo)
				commitHash := crrevResponse.GitHash
				repo := gitiles.NewRepo(repoURL, httpClient)
				longCommits, err := repo.Log(ctx, commitHash, gitiles.LogLimit(1))
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				if len(longCommits) == 0 {
					return mcp.NewToolResultError(fmt.Sprintf("CommitHash=%s was missing from repo=%s", commitHash, repoURL)), nil
				}
				commitSummary := longCommits[0].Subject
				csvString += fmt.Sprintf("%d,'%s','%s','%s'\n", i, repoURL, commitHash, commitSummary)
			}
			return mcp.NewToolResultText(csvString), nil
		},
	}
	getBodyForCommitHashTool := common.Tool{
		Name:        kGetBodyForCommitHashToolName,
		Description: kGetBodyForCommitHashToolDescription,
		Arguments: []common.ToolArgument{
			{
				Name:         kGitCommitHashArgumentName,
				Description:  "Git Commit Hash for which to get the Git Commit body.",
				Required:     true,
				ArgumentType: common.StringArgument,
			},
			{
				Name:         kGitRepoURLArgumentName,
				Description:  "URL of GoogleSource Git repo in which to look for the commit.",
				Required:     true,
				ArgumentType: common.StringArgument,
			},
		},
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			commitHash, err := request.RequireString(kGitCommitHashArgumentName)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repoURL, err := request.RequireString(kGitRepoURLArgumentName)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo := gitiles.NewRepo(repoURL, httpClient)
			longCommits, err := repo.Log(ctx, commitHash, gitiles.LogLimit(1))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if len(longCommits) == 0 {
				return mcp.NewToolResultError(fmt.Sprintf("CommitHash=%s was missing from repo=%s", commitHash, repoURL)), nil
			}
			return mcp.NewToolResultText(longCommits[0].Body), nil
		},
	}
	return []common.Tool{getCommitInfoForRevisionRangeTool, getBodyForCommitHashTool}
}
