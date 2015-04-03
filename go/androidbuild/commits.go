package androidbuild

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/gitinfo"
)

// commits is an interface for querying for commits, it is used in Info.
type commits interface {
	// List returns a list of ShortCommit's that match the branch, target and are in a build that is newer than endBuildID.
	List(branch, target, endBuildID string) (map[string]*gitinfo.ShortCommit, error)

	// Get returns a single ShortCommit for the specified branch, target and buildID.
	Get(branch, target, buildID string) (*gitinfo.ShortCommit, error)
}

// androidCommits implements commits.
type androidCommits struct {
	service *androidbuildinternal.Service
}

// newAndroidCommits creates a new commits.
func newAndroidCommits(client *http.Client) (commits, error) {
	service, err := androidbuildinternal.New(client)
	if err != nil {
		return nil, err
	}

	return &androidCommits{
		service: service,
	}, nil
}

// Find all buildIDs with Skia commits from latest build back to endBuildID.
// Pass in the empty string for endBuildID if you just want a range of IDs.
func (a *androidCommits) List(branch, target, endBuildID string) (map[string]*gitinfo.ShortCommit, error) {
	pageToken := ""
	var err error
	ret := map[string]*gitinfo.ShortCommit{}
	for {
		pageToken, err = a.findCommitsPage(branch, target, endBuildID, ret, pageToken)
		if err != nil {
			return nil, err
		}
		// If we aren't looking a specific endpoint then stop when we've gathered
		// enough commits.
		if endBuildID == "" && len(ret) > 10 {
			break
		}
		// We've reached the last page when no pageToken is returned.
		if pageToken == "" {
			break
		}
	}
	return ret, nil
}

// findCommitsPage requests a single page of the results and looks for commit info in each build returned.
//
// New commits are added to ret.
//
// A page token is returned along with an error. If there are no more pages of data
// then the page token is the empty string.
func (a *androidCommits) findCommitsPage(branch, target, endBuildID string, ret map[string]*gitinfo.ShortCommit, pageToken string) (string, error) {
	// We explicitly don't use exponential backoff since that increases the
	// likelihood of getting a bad response.
	for i := 0; i < NUM_RETRIES; i++ {
		glog.Infof("Querying for %q %q %q", branch, target, endBuildID)
		request := a.service.Build.List().Successful(true).BuildType("submitted").Branch(branch).Target(target).ExtraFields("changeInfo").MaxResults(PAGE_SIZE)
		if pageToken != "" {
			request.PageToken(pageToken)
		}
		if endBuildID != "" {
			request.EndBuildId(endBuildID)
		}
		resp, err := request.Do()
		if err != nil {
			glog.Infof("Call failed: %s", err)
			time.Sleep(SLEEP_DURATION * time.Second)
			continue
		}
		if len(resp.Builds) == 0 {
			glog.Infof("No builds in response.")
			time.Sleep(SLEEP_DURATION * time.Second)
			continue
		}
		if len(resp.Builds[0].Changes) == 0 {
			glog.Infof("No Changes in builds.")
			time.Sleep(SLEEP_DURATION * time.Second)
			continue
		}
		glog.Infof("Success after %d attempts.", i+1)

		for _, buildInfo := range resp.Builds {
			glog.Infof("BuildID: %s : %s", buildInfo.BuildId, buildInfo.Target.Name)
		}

		for _, build := range resp.Builds {
			for _, change := range commitsFromChanges(build.Changes) {
				ret[build.BuildId] = change
			}
		}
		return resp.NextPageToken, nil
	}
	return "", fmt.Errorf("No valid responses from API after %d requests.", NUM_RETRIES)
}

func commitsFromChanges(changes []*androidbuildinternal.Change) []*gitinfo.ShortCommit {
	ret := []*gitinfo.ShortCommit{}
	automated_commit_message := ""
	for _, changes := range changes {
		if changes.Project == "platform/external/skia" {
			for _, revision := range changes.Revisions {
				authorName := revision.Commit.Author.Name
				if authorName == "Skia_Android Canary Bot" {
					automated_commit_message = revision.Commit.CommitMessage
				} else if strings.Contains(automated_commit_message, revision.GitRevision) {
					ret = append(ret, &gitinfo.ShortCommit{
						Hash:    revision.GitRevision,
						Author:  authorName,
						Subject: revision.Commit.Subject,
					})
				}
			}
		}
	}
	return ret
}

func (a *androidCommits) Get(branch, target, buildID string) (*gitinfo.ShortCommit, error) {
	build, err := a.service.Build.Get(buildID, target).ExtraFields("changeInfo").Do()
	if err != nil {
		return nil, err
	}
	if len(build.Changes) > 1 {
		changes := commitsFromChanges(build.Changes)
		if len(changes) > 1 {
			return changes[0], nil
		}
	}
	return nil, fmt.Errorf("Didn't find a Skia commit in the response.")
}
