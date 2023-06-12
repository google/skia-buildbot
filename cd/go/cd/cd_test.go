package cd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/docker"
	docker_mocks "go.skia.org/infra/go/docker/mocks"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

func TestUploadedCLRegex(t *testing.T) {
	const logOutput = `
 path/to/my-file.txt  |   2 +
1 files changed, 376 insertions(+), 172 deletions(-)
Waiting for editor...

Enumerating objects: 68, done.
Counting objects: 100% (68/68), done.
Delta compression using up to 48 threads
Compressing objects: 100% (53/53), done.
Writing objects: 100% (54/54), 18.60 KiB | 3.72 MiB/s, done.
Total 54 (delta 29), reused 0 (delta 0), pack-reused 0
remote: Resolving deltas: 100% (29/29)
remote: Waiting for private key checker: 5/30 objects left
remote: Processing changes: refs: 1, new: 1, done
remote: commit 321f209: warning: subject >50 characters; use shorter first paragraph
remote: commit 321f209: warning: too many message lines longer than 72 characters; manually wrap lines
remote:
remote: SUCCESS
remote:
remote:   https://my-project-review.googlesource.com/c/my-repo/+/12345 Commit Message [WIP] [NEW]
remote:
To https://my-project.googlesource.com/my-repo.git
* [new reference]       321f209aecdaf8f0a276d7d8413fd5b524f1c985 -> refs/for/refs/heads/main%wip,m=Initial_upload,cc=reviews@my-project.org,l=Commit-Queue+1,hashtag=blah
`
	const expect = "https://my-project-review.googlesource.com/c/my-repo/+/12345"
	require.Equal(t, expect, uploadedCLRegex.FindString(logOutput))
}

func TestMatchDockerImagesToGitCommits(t *testing.T) {
	ctx := context.Background()
	dockerClient := &docker_mocks.Client{}
	urlmock := mockhttpclient.NewURLMock()
	repo := gitiles.NewRepo("https://my-repo.git", urlmock.Client())
	image := "gcr.io/skia-public/autoroll-be"
	limit := 3

	dockerClient.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/autoroll-be").Return(map[string]*docker.ImageInstance{
		"sha256:1111111111": {
			Digest: "sha256:1111111111",
			Tags:   []string{"git-1111111111111111111111111111111111111111"},
		},
		"sha256:2222222222": {
			Digest: "sha256:2222222222",
			Tags:   []string{"git-2222222222222222222222222222222222222222"},
		},
		"sha256:3333333333": {
			Digest: "sha256:3333333333",
			Tags:   []string{"git-3333333333333333333333333333333333333333"},
		},
		"sha256:4444444444": {
			Digest: "sha256:4444444444",
			Tags:   []string{"git-4444444444444444444444444444444444444444"},
		},
		"sha256:5555555555": {
			Digest: "sha256:5555555555",
			Tags:   []string{"git-5555555555555555555555555555555555555555"},
		},
	}, nil)
	ts := time.Unix(1684339486, 0).UTC()
	tsStr := ts.Format("Mon Jan 02 15:04:05 2006")
	b := append([]byte(")]}'\n"), []byte(testutils.MarshalJSON(t, &gitiles.Log{
		Log: []*gitiles.Commit{
			{
				Commit:  "1111111111111111111111111111111111111111",
				Parents: []string{"2222222222222222222222222222222222222222"},
				Author: &gitiles.Author{
					Name:  "1111",
					Email: "1111@google.com",
					Time:  tsStr,
				},
				Committer: &gitiles.Author{
					Name:  "1111",
					Email: "1111@google.com",
					Time:  tsStr,
				},
				Message: "",
			},
			{
				Commit:  "2222222222222222222222222222222222222222",
				Parents: []string{"3333333333333333333333333333333333333333"},
				Author: &gitiles.Author{
					Name:  "2222",
					Email: "2222@google.com",
					Time:  tsStr,
				},
				Committer: &gitiles.Author{
					Name:  "2222",
					Email: "2222@google.com",
					Time:  tsStr,
				},
				Message: "",
			},
			{
				Commit:  "3333333333333333333333333333333333333333",
				Parents: []string{"4444444444444444444444444444444444444444"},
				Author: &gitiles.Author{
					Name:  "3333",
					Email: "3333@google.com",
					Time:  tsStr,
				},
				Committer: &gitiles.Author{
					Name:  "3333",
					Email: "3333@google.com",
					Time:  tsStr,
				},
				Message: "",
			},
		},
		Next: "4444444444444444444444444444444444444444",
	}))...)
	urlmock.MockOnce("https://my-repo.git/+log/main?format=JSON&n=3", mockhttpclient.MockGetDialogue(b))

	images, err := MatchDockerImagesToGitCommits(ctx, dockerClient, repo, image, limit)
	require.NoError(t, err)
	require.Equal(t, []*DockerImageWithGitCommit{
		{
			Digest: "sha256:1111111111",
			Commit: &vcsinfo.LongCommit{
				ShortCommit: &vcsinfo.ShortCommit{
					Hash:   "1111111111111111111111111111111111111111",
					Author: "1111 (1111@google.com)",
				},
				Parents:   []string{"2222222222222222222222222222222222222222"},
				Timestamp: ts,
			},
			Tags: []string{"git-1111111111111111111111111111111111111111"},
		},
		{
			Digest: "sha256:2222222222",
			Commit: &vcsinfo.LongCommit{
				ShortCommit: &vcsinfo.ShortCommit{
					Hash:   "2222222222222222222222222222222222222222",
					Author: "2222 (2222@google.com)",
				},
				Parents:   []string{"3333333333333333333333333333333333333333"},
				Timestamp: ts,
			},
			Tags: []string{"git-2222222222222222222222222222222222222222"},
		},
		{
			Digest: "sha256:3333333333",
			Commit: &vcsinfo.LongCommit{
				ShortCommit: &vcsinfo.ShortCommit{
					Hash:   "3333333333333333333333333333333333333333",
					Author: "3333 (3333@google.com)",
				},
				Parents:   []string{"4444444444444444444444444444444444444444"},
				Timestamp: ts,
			},
			Tags: []string{"git-3333333333333333333333333333333333333333"},
		},
	}, images)
}
