package chromiumbuilder

// Code that is useful for multiple test files, mostly mocks. Note that despite
// the _test suffix of this file, there are no actual tests. The suffix is
// necesasry in order for the file to be included as a source file for go_test
// targets.

import (
	"context"
	"io/fs"
	"time"

	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	vcsinfo "go.skia.org/infra/go/vcsinfo"
)

/*******************************************************************************
* Mock Checkout
*******************************************************************************/

// MockCheckout is a mock implementation of git.Checkout.
type MockCheckout struct {
	mock.Mock
	workdir string
}

func NewMockCheckout(t mock.TestingT, workdir string) *MockCheckout {
	m := &MockCheckout{workdir: workdir}
	m.Mock.Test(t)
	if tHelper, ok := t.(interface{ Helper() }); ok {
		tHelper.Helper()
	}
	if tCleanup, ok := t.(interface{ Cleanup(func()) }); ok {
		tCleanup.Cleanup(func() { m.AssertExpectations(t) })
	}
	return m
}

func (m *MockCheckout) Dir() string { return m.workdir }
func (m *MockCheckout) Git(ctx context.Context, cmd ...string) (string, error) {
	args := m.Called(ctx, cmd)
	return args.String(0), args.Error(1)
}
func (m *MockCheckout) Details(ctx context.Context, name string) (*vcsinfo.LongCommit, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*vcsinfo.LongCommit), args.Error(1)
}
func (m *MockCheckout) RevParse(ctx context.Context, revParseArgs ...string) (string, error) {
	args := m.Called(ctx, revParseArgs)
	return args.String(0), args.Error(1)
}
func (m *MockCheckout) RevList(ctx context.Context, revListArgs ...string) ([]string, error) {
	args := m.Called(ctx, revListArgs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}
func (m *MockCheckout) ResolveRef(ctx context.Context, ref string) (string, error) {
	args := m.Called(ctx, ref)
	return args.String(0), args.Error(1)
}
func (m *MockCheckout) Branches(ctx context.Context) ([]*git.Branch, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*git.Branch), args.Error(1)
}
func (m *MockCheckout) GetFile(ctx context.Context, fileName, commit string) (string, error) {
	args := m.Called(ctx, fileName, commit)
	return args.String(0), args.Error(1)
}
func (m *MockCheckout) IsSubmodule(ctx context.Context, path, commit string) (bool, error) {
	args := m.Called(ctx, path, commit)
	return args.Bool(0), args.Error(1)
}
func (m *MockCheckout) ReadSubmodule(ctx context.Context, path, commit string) (string, error) {
	args := m.Called(ctx, path, commit)
	return args.String(0), args.Error(1)
}
func (m *MockCheckout) UpdateSubmodule(ctx context.Context, path, commit string) error {
	args := m.Called(ctx, path, commit)
	return args.Error(0)
}
func (m *MockCheckout) NumCommits(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockCheckout) IsAncestor(ctx context.Context, a, b string) (bool, error) {
	args := m.Called(ctx, a, b)
	return args.Bool(0), args.Error(1)
}
func (m *MockCheckout) Version(ctx context.Context) (int, int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Int(1), args.Error(2)
}
func (m *MockCheckout) FullHash(ctx context.Context, ref string) (string, error) {
	args := m.Called(ctx, ref)
	return args.String(0), args.Error(1)
}
func (m *MockCheckout) CatFile(ctx context.Context, ref, path string) ([]byte, error) {
	args := m.Called(ctx, ref, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}
func (m *MockCheckout) ReadDir(ctx context.Context, ref, path string) ([]fs.FileInfo, error) {
	args := m.Called(ctx, ref, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]fs.FileInfo), args.Error(1)
}
func (m *MockCheckout) GetRemotes(ctx context.Context) (map[string]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}
func (m *MockCheckout) VFS(ctx context.Context, ref string) (*git.FS, error) {
	args := m.Called(ctx, ref)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*git.FS), args.Error(1)
}
func (m *MockCheckout) FetchRefFromRepo(ctx context.Context, repo, ref string) error {
	args := m.Called(ctx, repo, ref)
	return args.Error(0)
}
func (m *MockCheckout) Fetch(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
func (m *MockCheckout) AddRemote(ctx context.Context, remote, repoUrl string) error {
	args := m.Called(ctx, remote, repoUrl)
	return args.Error(0)
}
func (m *MockCheckout) CleanupBranch(ctx context.Context, branch string) error {
	args := m.Called(ctx, branch)
	return args.Error(0)
}
func (m *MockCheckout) Cleanup(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
func (m *MockCheckout) UpdateBranch(ctx context.Context, branch string) error {
	args := m.Called(ctx, branch)
	return args.Error(0)
}
func (m *MockCheckout) Update(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
func (m *MockCheckout) IsDirty(ctx context.Context) (bool, string, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.String(1), args.Error(2)
}

// Assert that MockCheckout implements git.Checkout.
var _ git.Checkout = &MockCheckout{}

/*******************************************************************************
* Mock FileInfo
*******************************************************************************/

// MockFileInfo is a mock implementation of fs.FileInfo.
type MockFileInfo struct {
	mock.Mock
	FName    string
	FIsDir   bool
	FSize    int64
	FMode    fs.FileMode
	FModTime time.Time
}

func (m *MockFileInfo) Name() string       { return m.FName }
func (m *MockFileInfo) Size() int64        { return m.FSize }
func (m *MockFileInfo) Mode() fs.FileMode  { return m.FMode }
func (m *MockFileInfo) ModTime() time.Time { return m.FModTime }
func (m *MockFileInfo) IsDir() bool        { return m.FIsDir }
func (m *MockFileInfo) Sys() interface{}   { return nil }

// Assert that MockFileInfo implements fs.FileInfo.
var _ fs.FileInfo = &MockFileInfo{}

// MockProcess is a mock implementation of exec.Process.
type MockProcess struct {
	mock.Mock
}

// NewMockProcess creates a new MockProcess.
func NewMockProcess(t mock.TestingT) *MockProcess {
	m := &MockProcess{}
	m.Mock.Test(t)
	if tHelper, ok := t.(interface{ Helper() }); ok {
		tHelper.Helper()
	}
	if tCleanup, ok := t.(interface{ Cleanup(func()) }); ok {
		tCleanup.Cleanup(func() { m.AssertExpectations(t) })
	}
	return m
}

func (m *MockProcess) Kill() error {
	args := m.Called()
	return args.Error(0)
}

// Assert that MockProcess implements exec.Process.
var _ exec.Process = &MockProcess{}

/*******************************************************************************
* Mock directoryRemover
*******************************************************************************/

type mockDirectoryRemover struct {
	mock.Mock
}

func (m *mockDirectoryRemover) Execute(path string) error {
	args := m.Called(path)
	return args.Error(0)
}
