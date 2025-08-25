package testutils

/*
	Utilities for mocking requests to Gitiles.
*/

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vfs"
)

// XSS protection line which is added by the Gitiles server and removed by our
// client code.
const XSSLine = ")]}'\n"

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

func MockReadFile(t sktest.TestingT, c *mockhttpclient.URLMock, repoUrl, srcPath, ref string, contents []byte, stat fs.FileInfo) {
	body := make([]byte, base64.StdEncoding.EncodedLen(len(contents)))
	base64.StdEncoding.Encode(body, contents)
	url := fmt.Sprintf(gitiles.DownloadURL, repoUrl, ref, srcPath)
	md := mockhttpclient.MockGetDialogue(body)
	typ := git.ObjectTypeBlob
	if stat.IsDir() {
		typ = git.ObjectTypeTree
	}
	md.ResponseHeader(gitiles.ModeHeader, strconv.FormatInt(int64(stat.Mode()), 8))
	md.ResponseHeader(gitiles.TypeHeader, string(typ))
	c.MockOnce(url, md)
}

func (mr *MockRepo) MockReadFile(ctx context.Context, srcPath, ref string) {
	fs, err := mr.repo.VFS(ctx, ref)
	require.NoError(mr.t, err)
	defer func() {
		require.NoError(mr.t, fs.Close(ctx))
	}()
	f, err := fs.Open(ctx, srcPath)
	require.NoError(mr.t, err)
	contents, err := io.ReadAll(vfs.WithContext(ctx, f))
	require.NoError(mr.t, err)
	st, err := f.Stat(ctx)
	require.NoError(mr.t, err)
	MockReadFile(mr.t, mr.URLMock, mr.url, srcPath, ref, contents, st)
}

func getCommit(t sktest.TestingT, ctx context.Context, repo git.GitDir, ref string) *gitiles.Commit {
	details, err := repo.Details(ctx, ref)
	require.NoError(t, err)
	rv, err := gitiles.LongCommitToCommit(details)
	require.NoError(t, err)
	return rv
}

func MockGetCommit(t sktest.TestingT, c *mockhttpclient.URLMock, repoUrl, ref string, commit *gitiles.Commit) {
	b, err := json.Marshal(commit)
	require.NoError(t, err)
	b = append([]byte(XSSLine), b...)
	url := fmt.Sprintf(gitiles.CommitURLJSON, repoUrl, ref)
	c.MockOnce(url, mockhttpclient.MockGetDialogue(b))
}

func (mr *MockRepo) MockGetCommit(ctx context.Context, ref string) {
	c := getCommit(mr.t, ctx, mr.repo, ref)
	MockGetCommit(mr.t, mr.URLMock, mr.url, ref, c)
}

func MockBranches(t sktest.TestingT, c *mockhttpclient.URLMock, repoUrl string, branches []*git.Branch) {
	res := make(gitiles.RefsMap, len(branches))
	for _, branch := range branches {
		res[branch.Name] = gitiles.Ref{
			Value: branch.Head,
		}
	}
	b, err := json.Marshal(res)
	require.NoError(t, err)
	b = append([]byte(XSSLine), b...)
	url := fmt.Sprintf(gitiles.RefsURL, repoUrl)
	c.MockOnce(url, mockhttpclient.MockGetDialogue(b))
}

func (mr *MockRepo) MockBranches(ctx context.Context) {
	branches, err := mr.repo.Branches(ctx)
	require.NoError(mr.t, err)
	MockBranches(mr.t, mr.URLMock, mr.url, branches)
}

func MockLog(t sktest.TestingT, c *mockhttpclient.URLMock, repoUrl, logExpr string, log *gitiles.Log, opts ...gitiles.LogOption) {
	path, query, _, err := gitiles.LogOptionsToQuery(opts)
	require.NoError(t, err)
	b, err := json.Marshal(log)
	require.NoError(t, err)
	b = append([]byte(XSSLine), b...)
	if path != "" {
		logExpr += "/" + path
	}
	url := fmt.Sprintf(gitiles.LogURL, repoUrl, logExpr)
	if query != "" {
		url += "&" + query
	}
	c.MockOnce(url, mockhttpclient.MockGetDialogue(b))
}

func (mr *MockRepo) MockLog(ctx context.Context, logExpr string, opts ...gitiles.LogOption) {
	path, _, _, err := gitiles.LogOptionsToQuery(opts)
	require.NoError(mr.t, err)
	args := []string{logExpr}
	if path != "" {
		args = append(args, "--", path)
	}
	revlist, err := mr.repo.RevList(ctx, args...)
	require.NoError(mr.t, err)
	log := &gitiles.Log{
		Log: make([]*gitiles.Commit, len(revlist)),
	}
	for idx, hash := range revlist {
		log.Log[idx] = getCommit(mr.t, ctx, mr.repo, hash)
	}
	MockLog(mr.t, mr.URLMock, mr.url, logExpr, log, opts...)
}

type MockReadObjectInfo struct {
	path     string
	fi       fs.FileInfo
	contents []byte
}

func MockReadObject(g *mocks.GitilesRepo, revision string, obj *MockReadObjectInfo) {
	g.On("ReadObject", testutils.AnyContext, obj.path, revision).Return(obj.fi, obj.contents, nil).Once()
}

func MockReadObject_File(g *mocks.GitilesRepo, revision, name, contents string) {
	contentsBytes := []byte(contents)
	MockReadObject(g, revision, &MockReadObjectInfo{
		path: name,
		fi: vfs.FileInfo{
			Name:  path.Base(name),
			Size:  int64(len(contentsBytes)),
			Mode:  0644,
			IsDir: false,
		}.Get(),
		contents: contentsBytes,
	})
}

func MockReadObject_Dir(g *mocks.GitilesRepo, revision, name string, contents []string) {
	contentsStr := ""
	for _, name := range contents {
		contentsStr += fmt.Sprintf("0644 blob somehash %s\n", name)
	}
	contentsBytes := []byte(contentsStr)
	MockReadObject(g, revision, &MockReadObjectInfo{
		path: name,
		fi: vfs.FileInfo{
			Name:  path.Base(name),
			Size:  int64(len(contentsBytes)),
			Mode:  0644 | os.ModeDir,
			IsDir: true,
		}.Get(),
		contents: contentsBytes,
	})
}
