package testutils

/*
	Utilities for mocking requests to Gitiles.
*/

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

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
	url := fmt.Sprintf(gitiles.DownloadURL, mr.url, ref, srcPath)
	mr.URLMock.MockOnce(url, mockhttpclient.MockGetDialogue(body))
}

func (mr *MockRepo) getCommit(ctx context.Context, ref string) *gitiles.Commit {
	details, err := mr.repo.Details(ctx, ref)
	require.NoError(mr.t, err)
	rv, err := gitiles.LongCommitToCommit(details)
	require.NoError(mr.t, err)
	return rv
}

func (mr *MockRepo) MockGetCommit(ctx context.Context, ref string) {
	c := mr.getCommit(ctx, ref)
	b, err := json.Marshal(c)
	assert.NoError(mr.t, err)
	b = append([]byte(")]}'\n"), b...)
	url := fmt.Sprintf(gitiles.CommitURLJSON, mr.url, ref)
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
	url := fmt.Sprintf(gitiles.RefsURL, mr.url)
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
	url := fmt.Sprintf(gitiles.LogURL, mr.url, logExpr)
	query, _, err := gitiles.LogOptionsToQuery(opts)
	require.NoError(mr.t, err)
	if query != "" {
		url += "&" + query
	}
	mr.URLMock.MockOnce(url, mockhttpclient.MockGetDialogue(b))
}
