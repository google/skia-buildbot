package main

import (
	"testing"

	"github.com/stretchr/testify/require"
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
