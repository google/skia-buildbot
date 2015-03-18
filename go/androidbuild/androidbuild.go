// androidbuild implements a simple interface to look up skia git commit
// hashes from android buildIDs.
package androidbuild

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/gitinfo"
)

// GitHashLookup wraps around the androidbuildinternal service.
type GitHashLookup struct {
	service *androidbuildinternal.Service
}

func New(client *http.Client) (*GitHashLookup, error) {
	service, err := androidbuildinternal.New(client)
	if err != nil {
		return nil, err
	}

	return &GitHashLookup{
		service: service,
	}, nil
}

// FindCommit looks up a skia git commit from the branch, target and buildID.
// It queries the last 50 before the target buildID (branch and target help
// narrow down the search) and then iterates backwards through the builds
// until it finds a pattern that was generated as part of a skia depsroll.
func (g *GitHashLookup) FindCommit(branch, target, buildID string, logResponse bool) (*gitinfo.ShortCommit, error) {
	resp, err := g.service.Build.List().BuildType("submitted").Branch(branch).Target(target).ExtraFields("changeInfo").MaxResults(50).StartBuildId(buildID).Do()
	if err != nil {
		return nil, err
	}

	// This should only be used by test clients to make narrowing issues quick.
	if logResponse {
		for _, buildInfo := range resp.Builds {
			glog.Infof("BuildID: %s : %s", buildInfo.BuildId, buildInfo.Target.Name)
		}
	}

	automated_commit_message := ""
	for _, build := range resp.Builds {
		for _, changes := range build.Changes {
			if changes.Project == "platform/external/skia" {
				for _, revision := range changes.Revisions {
					authorName := revision.Commit.Author.Name
					if authorName == "Skia_Android Canary Bot" {
						automated_commit_message = revision.Commit.CommitMessage
					} else if strings.Contains(automated_commit_message, revision.GitRevision) {
						return &gitinfo.ShortCommit{
							Hash:    revision.GitRevision,
							Author:  authorName,
							Subject: revision.Commit.Subject,
						}, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("Unable to find commit infor for branch/target/buildID: %s %s %s", branch, target, buildID)
}
