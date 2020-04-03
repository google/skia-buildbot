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
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/sktest"
)

type MockRepo struct {
	URLMock *mockhttpclient.URLMock
	repo    git.GitDir
	t       sktest.TestingT
	url     string
}

func NewMockRepo(t sktest.TestingT, url string, repo git.GitDir, c *mockhttpclient.URLMock) *MockRepo {
	return &MockRepo{
		URLMock: c,
		repo:    repo,
		t:       t,
		url:     url,
	}
}

func (mr *MockRepo) Empty() bool {
	return mr.URLMock.Empty()
}

func (mr *MockRepo) MockReadFile(ctx context.Context, srcPath, ref string) {
	contents, err := mr.repo.GetFile(ctx, srcPath, ref)
	assert.NoError(mr.t, err)
	body := make([]byte, base64.StdEncoding.EncodedLen(len([]byte(contents))))
	base64.StdEncoding.Encode(body, []byte(contents))
	url := fmt.Sprintf(gitiles.DOWNLOAD_URL, mr.url, ref, srcPath)
	mr.URLMock.MockOnce(url, mockhttpclient.MockGetDialogue(body))
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
	url := fmt.Sprintf(gitiles.COMMIT_URL_JSON, mr.url, ref)
	mr.URLMock.MockOnce(url, mockhttpclient.MockGetDialogue(b))
}

func (mr *MockRepo) MockBranches(ctx context.Context) {
	branches, err := mr.repo.Branches(ctx)
	assert.NoError(mr.t, err)
	res := make(gitiles.RefsMap, len(branches))
	for _, branch := range branches {
		res[branch.Name] = gitiles.Ref{
			Value: branch.Head,
		}
	}
	b, err := json.Marshal(res)
	assert.NoError(mr.t, err)
	b = append([]byte(")]}'\n"), b...)
	url := fmt.Sprintf(gitiles.REFS_URL, mr.url)
	mr.URLMock.MockOnce(url, mockhttpclient.MockGetDialogue(b))
}

func (mr *MockRepo) MockLog(ctx context.Context, logExpr string, opts ...gitiles.LogOption) {
	revlist, err := mr.repo.RevList(ctx, logExpr)
	assert.NoError(mr.t, err)
	log := &gitiles.Log{
		Log: make([]*gitiles.Commit, len(revlist)),
	}
	for idx, hash := range revlist {
		log.Log[idx] = mr.getCommit(ctx, hash)
	}
	b, err := json.Marshal(log)
	assert.NoError(mr.t, err)
	b = append([]byte(")]}'\n"), b...)
	url := fmt.Sprintf(gitiles.LOG_URL, mr.url, logExpr)
	query, _, err := gitiles.LogOptionsToQuery(opts)
	require.NoError(mr.t, err)
	if query != "" {
		url += "&" + query
	}
	mr.URLMock.MockOnce(url, mockhttpclient.MockGetDialogue(b))
}
