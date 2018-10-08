package testutils

/*
	Utilities for mocking requests to Gitiles.
*/

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

type MockRepo struct {
	c    *mockhttpclient.URLMock
	repo git.GitDir
	t    testutils.TestingT
	url  string
}

func NewMockRepo(t testutils.TestingT, url string, repo git.GitDir, c *mockhttpclient.URLMock) *MockRepo {
	return &MockRepo{
		c:    c,
		repo: repo,
		t:    t,
		url:  url,
	}
}

func (mr *MockRepo) MockReadFile(ctx context.Context, srcPath, ref string) {
	contents, err := mr.repo.GetFile(ctx, srcPath, ref)
	assert.NoError(mr.t, err)
	body := make([]byte, base64.StdEncoding.EncodedLen(len([]byte(contents))))
	base64.StdEncoding.Encode(body, []byte(contents))
	url := fmt.Sprintf(gitiles.DOWNLOAD_URL, mr.url, ref, srcPath)
	mr.c.MockOnce(url, mockhttpclient.MockGetDialogue(body))
}

func (mr *MockRepo) getCommit(ctx context.Context, ref string) *gitiles.Commit {
	details, err := mr.repo.Details(ctx, ref)
	assert.NoError(mr.t, err)
	// vcsinfo.LongCommit expresses authors in the form: "Author Name (author@email.com)"
	split := strings.Split(details.Author, "(")
	if len(split) != 2 {
		mr.t.Fatalf("Bad author format: %q", details.Author)
	}
	authorName := strings.TrimSpace(split[0])
	authorEmail := strings.TrimSpace(strings.TrimRight(split[1], ")"))
	return &gitiles.Commit{
		Commit:  details.Hash,
		Parents: details.Parents,
		Author: &gitiles.Author{
			Name:  authorName,
			Email: authorEmail,
			Time:  details.Timestamp.Format(gitiles.DATE_FORMAT_TZ),
		},
		Committer: &gitiles.Author{
			Name:  authorName,
			Email: authorEmail,
			Time:  details.Timestamp.Format(gitiles.DATE_FORMAT_TZ),
		},
		Message: details.Subject + "\n\n" + details.Body,
	}
}

func (mr *MockRepo) MockGetCommit(ctx context.Context, ref string) {
	c := mr.getCommit(ctx, ref)
	b, err := json.Marshal(c)
	assert.NoError(mr.t, err)
	b = append([]byte(")]}'\n"), b...)
	url := fmt.Sprintf(gitiles.COMMIT_URL, mr.url, ref)
	mr.c.MockOnce(url, mockhttpclient.MockGetDialogue(b))
}

func (mr *MockRepo) MockLog(ctx context.Context, from, to string) {
	revlist, err := mr.repo.RevList(ctx, fmt.Sprintf("%s..%s", from, to))
	assert.NoError(mr.t, err)
	log := &gitiles.Log{
		Log: []*gitiles.Commit{},
	}
	for _, hash := range revlist {
		log.Log = append(log.Log, mr.getCommit(ctx, hash))
	}
	b, err := json.Marshal(log)
	assert.NoError(mr.t, err)
	b = append([]byte(")]}'\n"), b...)
	url := fmt.Sprintf(gitiles.LOG_URL, mr.url, from, to)
	mr.c.MockOnce(url, mockhttpclient.MockGetDialogue(b))
}
