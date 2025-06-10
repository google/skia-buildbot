package chromiumbuilder

// Tests for code in service.go related to initial git checkout setup. This is
// kept separate from service_test.go due to the large amount of boilerplate
// code that is not relevant to other testing.

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	vcsinfo "go.skia.org/infra/go/vcsinfo"
	vfs_mocks "go.skia.org/infra/go/vfs/mocks"
)

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
func (m *MockCheckout) GetBranchHead(ctx context.Context, branchName string) (string, error) {
	args := m.Called(ctx, branchName)
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

func TestChromiumBuilderService_handleExistingDepotToolsCheckout(t *testing.T) {
	ctx := context.Background()
	const testDepotToolsPath = "/path/to/depot_tools"
	const testDepotToolsParentDir = "/path/to"

	setupService := func() *ChromiumBuilderService {
		return &ChromiumBuilderService{
			depotToolsPath: testDepotToolsPath,
		}
	}

	tests := []struct {
		name        string
		setupMocks  func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory
		expectError bool
		errorMsg    string
	}{
		{
			name: "happy path",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockDepotToolsFile := vfs_mocks.NewFile(t)
				mockDepotToolsFileInfo := &MockFileInfo{FName: "depot_tools", FIsDir: true}
				mockFS.On("Open", ctx, testDepotToolsPath).Return(mockDepotToolsFile, nil).Once()
				mockDepotToolsFile.On("Stat", ctx).Return(mockDepotToolsFileInfo, nil).Once()
				mockDepotToolsFile.On("Close", ctx).Return(nil).Once()

				mockLucicfgFile := vfs_mocks.NewFile(t)
				lucicfgPath := filepath.Join(testDepotToolsPath, "lucicfg")
				mockFS.On("Open", ctx, lucicfgPath).Return(mockLucicfgFile, nil).Once()
				mockLucicfgFile.On("Close", ctx).Return(nil).Once()

				mockDotGitFile := vfs_mocks.NewFile(t)
				dotGitPath := filepath.Join(testDepotToolsPath, ".git")
				mockDotGitFileInfo := &MockFileInfo{FName: ".git", FIsDir: true}
				mockFS.On("Open", ctx, dotGitPath).Return(mockDotGitFile, nil).Once()
				mockDotGitFile.On("Stat", ctx).Return(mockDotGitFileInfo, nil).Once()
				mockDotGitFile.On("Close", ctx).Return(nil).Once()

				factory := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					require.Equal(t, DepotToolsUrl, repoUrl)
					require.Equal(t, testDepotToolsParentDir, workdir)
					return mockCheckout, nil
				}
				mockCheckout.On("Update", ctx).Return(nil).Once()
				return factory
			},
		},
		{
			name: "depot_tools path is not a directory",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockDepotToolsFile := vfs_mocks.NewFile(t)
				mockDepotToolsFileInfo := &MockFileInfo{FName: "depot_tools", FIsDir: false}
				mockFS.On("Open", ctx, testDepotToolsPath).Return(mockDepotToolsFile, nil).Once()
				mockDepotToolsFile.On("Stat", ctx).Return(mockDepotToolsFileInfo, nil).Once()
				mockDepotToolsFile.On("Close", ctx).Return(nil).Once()
				return func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
			},
			expectError: true, errorMsg: "Path /path/to/depot_tools exists, but is not a directory.",
		},
		{
			name: "depot_tools path open fails",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockFS.On("Open", ctx, testDepotToolsPath).Return(nil, errors.New("open failed")).Once()
				return func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
			},
			expectError: true, errorMsg: "open failed",
		},
		{
			name: "lucicfg open fails",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockDepotToolsFile := vfs_mocks.NewFile(t)
				mockDepotToolsFileInfo := &MockFileInfo{FName: "depot_tools", FIsDir: true}
				mockFS.On("Open", ctx, testDepotToolsPath).Return(mockDepotToolsFile, nil).Once()
				mockDepotToolsFile.On("Stat", ctx).Return(mockDepotToolsFileInfo, nil).Once()
				mockDepotToolsFile.On("Close", ctx).Return(nil).Once()

				lucicfgPath := filepath.Join(testDepotToolsPath, "lucicfg")
				mockFS.On("Open", ctx, lucicfgPath).Return(nil, errors.New("lucicfg open failed")).Once()
				return func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
			},
			expectError: true, errorMsg: "lucicfg open failed",
		},
		{
			name: ".git path is not a directory",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockDepotToolsFile := vfs_mocks.NewFile(t)
				mockDepotToolsFileInfo := &MockFileInfo{FName: "depot_tools", FIsDir: true}
				mockFS.On("Open", ctx, testDepotToolsPath).Return(mockDepotToolsFile, nil).Once()
				mockDepotToolsFile.On("Stat", ctx).Return(mockDepotToolsFileInfo, nil).Once()
				mockDepotToolsFile.On("Close", ctx).Return(nil).Once()

				mockLucicfgFile := vfs_mocks.NewFile(t)
				lucicfgPath := filepath.Join(testDepotToolsPath, "lucicfg")
				mockFS.On("Open", ctx, lucicfgPath).Return(mockLucicfgFile, nil).Once()
				mockLucicfgFile.On("Close", ctx).Return(nil).Once()

				mockDotGitFile := vfs_mocks.NewFile(t)
				dotGitPath := filepath.Join(testDepotToolsPath, ".git")
				mockDotGitFileInfo := &MockFileInfo{FName: ".git", FIsDir: false}
				mockFS.On("Open", ctx, dotGitPath).Return(mockDotGitFile, nil).Once()
				mockDotGitFile.On("Stat", ctx).Return(mockDotGitFileInfo, nil).Once()
				mockDotGitFile.On("Close", ctx).Return(nil).Once()
				return func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
			},
			expectError: true, errorMsg: "Path /path/to/depot_tools/.git exists, but is not a directory.",
		},
		{
			name: "checkout factory fails",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockDepotToolsFile := vfs_mocks.NewFile(t)
				mockDepotToolsFileInfo := &MockFileInfo{FName: "depot_tools", FIsDir: true}
				mockFS.On("Open", ctx, testDepotToolsPath).Return(mockDepotToolsFile, nil).Once()
				mockDepotToolsFile.On("Stat", ctx).Return(mockDepotToolsFileInfo, nil).Once()
				mockDepotToolsFile.On("Close", ctx).Return(nil).Once()

				mockLucicfgFile := vfs_mocks.NewFile(t)
				lucicfgPath := filepath.Join(testDepotToolsPath, "lucicfg")
				mockFS.On("Open", ctx, lucicfgPath).Return(mockLucicfgFile, nil).Once()
				mockLucicfgFile.On("Close", ctx).Return(nil).Once()

				mockDotGitFile := vfs_mocks.NewFile(t)
				dotGitPath := filepath.Join(testDepotToolsPath, ".git")
				mockDotGitFileInfo := &MockFileInfo{FName: ".git", FIsDir: true}
				mockFS.On("Open", ctx, dotGitPath).Return(mockDotGitFile, nil).Once()
				mockDotGitFile.On("Stat", ctx).Return(mockDotGitFileInfo, nil).Once()
				mockDotGitFile.On("Close", ctx).Return(nil).Once()

				return func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return nil, errors.New("factory failed")
				}
			},
			expectError: true, errorMsg: "factory failed",
		},
		{
			name: "checkout update fails",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockDepotToolsFile := vfs_mocks.NewFile(t)
				mockDepotToolsFileInfo := &MockFileInfo{FName: "depot_tools", FIsDir: true}
				mockFS.On("Open", ctx, testDepotToolsPath).Return(mockDepotToolsFile, nil).Once()
				mockDepotToolsFile.On("Stat", ctx).Return(mockDepotToolsFileInfo, nil).Once()
				mockDepotToolsFile.On("Close", ctx).Return(nil).Once()

				mockLucicfgFile := vfs_mocks.NewFile(t)
				lucicfgPath := filepath.Join(testDepotToolsPath, "lucicfg")
				mockFS.On("Open", ctx, lucicfgPath).Return(mockLucicfgFile, nil).Once()
				mockLucicfgFile.On("Close", ctx).Return(nil).Once()

				mockDotGitFile := vfs_mocks.NewFile(t)
				dotGitPath := filepath.Join(testDepotToolsPath, ".git")
				mockDotGitFileInfo := &MockFileInfo{FName: ".git", FIsDir: true}
				mockFS.On("Open", ctx, dotGitPath).Return(mockDotGitFile, nil).Once()
				mockDotGitFile.On("Stat", ctx).Return(mockDotGitFileInfo, nil).Once()
				mockDotGitFile.On("Close", ctx).Return(nil).Once()

				factory := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
				mockCheckout.On("Update", ctx).Return(errors.New("update failed")).Once()
				return factory
			},
			expectError: true, errorMsg: "update failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupService()
			mockFS := vfs_mocks.NewFS(t)
			mockCheckout := NewMockCheckout(t, testDepotToolsParentDir)

			factory := tt.setupMocks(t, mockFS, mockCheckout)

			err := s.handleExistingDepotToolsCheckout(ctx, mockFS, factory)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					require.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, s.depotToolsCheckout)
			}
		})
	}
}

func TestChromiumBuilderService_handleExistingChromiumCheckout(t *testing.T) {
	ctx := context.Background()
	const testChromiumPath = "/path/to/chromium/src"
	const testChromiumParentDir = "/path/to/chromium"

	setupService := func() *ChromiumBuilderService {
		return &ChromiumBuilderService{
			chromiumPath: testChromiumPath,
		}
	}

	tests := []struct {
		name        string
		setupMocks  func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory
		expectError bool
		errorMsg    string
	}{
		{
			name: "happy path",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockChromiumFile := vfs_mocks.NewFile(t)
				mockChromiumFileInfo := &MockFileInfo{FName: "src", FIsDir: true}
				mockFS.On("Open", ctx, testChromiumPath).Return(mockChromiumFile, nil).Once()
				mockChromiumFile.On("Stat", ctx).Return(mockChromiumFileInfo, nil).Once()
				mockChromiumFile.On("Close", ctx).Return(nil).Once()

				mockDotGitFile := vfs_mocks.NewFile(t)
				dotGitPath := filepath.Join(testChromiumPath, ".git")
				mockDotGitFileInfo := &MockFileInfo{FName: ".git", FIsDir: true}
				mockFS.On("Open", ctx, dotGitPath).Return(mockDotGitFile, nil).Once()
				mockDotGitFile.On("Stat", ctx).Return(mockDotGitFileInfo, nil).Once()
				mockDotGitFile.On("Close", ctx).Return(nil).Once()

				factory := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					require.Equal(t, ChromiumUrl, repoUrl)
					require.Equal(t, testChromiumParentDir, workdir)
					return mockCheckout, nil
				}
				mockCheckout.On("Update", ctx).Return(nil).Once()
				return factory
			},
		},
		{
			name: "chromium path is not a directory",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockChromiumFile := vfs_mocks.NewFile(t)
				mockChromiumFileInfo := &MockFileInfo{FName: "src", FIsDir: false}
				mockFS.On("Open", ctx, testChromiumPath).Return(mockChromiumFile, nil).Once()
				mockChromiumFile.On("Stat", ctx).Return(mockChromiumFileInfo, nil).Once()
				mockChromiumFile.On("Close", ctx).Return(nil).Once()
				return func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
			},
			expectError: true, errorMsg: "Path /path/to/chromium/src exists, but is not a directory.",
		},
		{
			name: "chromium path open fails",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockFS.On("Open", ctx, testChromiumPath).Return(nil, errors.New("open failed")).Once()
				return func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
			},
			expectError: true, errorMsg: "open failed",
		},
		{
			name: ".git path is not a directory",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockChromiumFile := vfs_mocks.NewFile(t)
				mockChromiumFileInfo := &MockFileInfo{FName: "src", FIsDir: true}
				mockFS.On("Open", ctx, testChromiumPath).Return(mockChromiumFile, nil).Once()
				mockChromiumFile.On("Stat", ctx).Return(mockChromiumFileInfo, nil).Once()
				mockChromiumFile.On("Close", ctx).Return(nil).Once()

				mockDotGitFile := vfs_mocks.NewFile(t)
				dotGitPath := filepath.Join(testChromiumPath, ".git")
				mockDotGitFileInfo := &MockFileInfo{FName: ".git", FIsDir: false}
				mockFS.On("Open", ctx, dotGitPath).Return(mockDotGitFile, nil).Once()
				mockDotGitFile.On("Stat", ctx).Return(mockDotGitFileInfo, nil).Once()
				mockDotGitFile.On("Close", ctx).Return(nil).Once()
				return func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
			},
			expectError: true, errorMsg: "Path /path/to/chromium/src/.git exists, but is not a directory.",
		},
		{
			name: "checkout factory fails",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockChromiumFile := vfs_mocks.NewFile(t)
				mockChromiumFileInfo := &MockFileInfo{FName: "src", FIsDir: true}
				mockFS.On("Open", ctx, testChromiumPath).Return(mockChromiumFile, nil).Once()
				mockChromiumFile.On("Stat", ctx).Return(mockChromiumFileInfo, nil).Once()
				mockChromiumFile.On("Close", ctx).Return(nil).Once()

				mockDotGitFile := vfs_mocks.NewFile(t)
				dotGitPath := filepath.Join(testChromiumPath, ".git")
				mockDotGitFileInfo := &MockFileInfo{FName: ".git", FIsDir: true}
				mockFS.On("Open", ctx, dotGitPath).Return(mockDotGitFile, nil).Once()
				mockDotGitFile.On("Stat", ctx).Return(mockDotGitFileInfo, nil).Once()
				mockDotGitFile.On("Close", ctx).Return(nil).Once()

				return func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return nil, errors.New("factory failed")
				}
			},
			expectError: true, errorMsg: "factory failed",
		},
		{
			name: "checkout update fails",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCheckout *MockCheckout) checkoutFactory {
				mockChromiumFile := vfs_mocks.NewFile(t)
				mockChromiumFileInfo := &MockFileInfo{FName: "src", FIsDir: true}
				mockFS.On("Open", ctx, testChromiumPath).Return(mockChromiumFile, nil).Once()
				mockChromiumFile.On("Stat", ctx).Return(mockChromiumFileInfo, nil).Once()
				mockChromiumFile.On("Close", ctx).Return(nil).Once()

				mockDotGitFile := vfs_mocks.NewFile(t)
				dotGitPath := filepath.Join(testChromiumPath, ".git")
				mockDotGitFileInfo := &MockFileInfo{FName: ".git", FIsDir: true}
				mockFS.On("Open", ctx, dotGitPath).Return(mockDotGitFile, nil).Once()
				mockDotGitFile.On("Stat", ctx).Return(mockDotGitFileInfo, nil).Once()
				mockDotGitFile.On("Close", ctx).Return(nil).Once()

				factory := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
				mockCheckout.On("Update", ctx).Return(errors.New("update failed")).Once()
				return factory
			},
			expectError: true, errorMsg: "update failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupService()
			mockFS := vfs_mocks.NewFS(t)
			mockCheckout := NewMockCheckout(t, testChromiumParentDir)

			factory := tt.setupMocks(t, mockFS, mockCheckout)

			err := s.handleExistingChromiumCheckout(ctx, mockFS, factory)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					require.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, s.chromiumCheckout)
			}
		})
	}
}
