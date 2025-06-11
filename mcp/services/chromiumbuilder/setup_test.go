package chromiumbuilder

// Tests for code in service.go related to initial git checkout setup. This is
// kept separate from service_test.go due to the large amount of boilerplate
// code that is not relevant to other testing.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
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

func TestChromiumBuilderService_handleMissingDepotToolsCheckout(t *testing.T) {
	ctx := context.Background()
	const testDepotToolsPath = "/path/to/depot_tools"
	const testDepotToolsParentDir = "/path/to"

	setupService := func() *ChromiumBuilderService {
		return &ChromiumBuilderService{
			depotToolsPath: testDepotToolsPath,
		}
	}

	tests := []struct {
		name             string
		setupMocks       func(t *testing.T, mockCheckout *MockCheckout) (checkoutFactory, directoryCreator)
		expectError      bool
		expectedCheckout bool // whether s.depotToolsCheckout should be set
		errorMsgContains string
	}{
		{
			name: "happy path",
			setupMocks: func(t *testing.T, mockCheckout *MockCheckout) (checkoutFactory, directoryCreator) {
				dc := func(path string, perm os.FileMode) error {
					require.Equal(t, testDepotToolsParentDir, path)
					require.Equal(t, os.FileMode(0o750), perm)
					return nil
				}
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					require.Equal(t, DepotToolsUrl, repoUrl)
					require.Equal(t, testDepotToolsParentDir, workdir)
					return mockCheckout, nil
				}
				mockCheckout.On("Update", ctx).Return(nil).Once()
				return cf, dc
			},
			expectError:      false,
			expectedCheckout: true,
		},
		{
			name: "directory creator fails",
			setupMocks: func(t *testing.T, mockCheckout *MockCheckout) (checkoutFactory, directoryCreator) {
				dc := func(path string, perm os.FileMode) error {
					return errors.New("mkdir failed")
				}
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return nil, errors.New("cf should not be called") // Should not be called
				}
				return cf, dc
			},
			expectError:      true,
			expectedCheckout: false,
			errorMsgContains: "mkdir failed",
		},
		{
			name: "checkout factory fails",
			setupMocks: func(t *testing.T, mockCheckout *MockCheckout) (checkoutFactory, directoryCreator) {
				dc := func(path string, perm os.FileMode) error {
					return nil
				}
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return nil, errors.New("checkout factory failed")
				}
				return cf, dc
			},
			expectError:      true,
			expectedCheckout: false,
			errorMsgContains: "checkout factory failed",
		},
		{
			name: "checkout update fails",
			setupMocks: func(t *testing.T, mockCheckout *MockCheckout) (checkoutFactory, directoryCreator) {
				dc := func(path string, perm os.FileMode) error {
					return nil
				}
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
				mockCheckout.On("Update", ctx).Return(errors.New("update failed")).Once()
				return cf, dc
			},
			expectError:      true,
			expectedCheckout: true, // Checkout is assigned before update is called
			errorMsgContains: "update failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupService()
			mockFS := vfs_mocks.NewFS(t) // Not directly used by handleMissingDepotToolsCheckout
			mockCheckout := NewMockCheckout(t, testDepotToolsParentDir)

			cf, dc := tt.setupMocks(t, mockCheckout)

			err := s.handleMissingDepotToolsCheckout(ctx, mockFS, cf, dc)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsgContains != "" {
					require.Contains(t, err.Error(), tt.errorMsgContains)
				}
			} else {
				require.NoError(t, err)
			}

			if tt.expectedCheckout {
				require.NotNil(t, s.depotToolsCheckout)
				if !tt.expectError { // If no error and checkout expected, it should be the mockCheckout
					require.Equal(t, mockCheckout, s.depotToolsCheckout)
				}
			} else {
				require.Nil(t, s.depotToolsCheckout)
			}
		})
	}
}

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

func TestChromiumBuilderService_handleMissingChromiumCheckout(t *testing.T) {
	const testChromiumPath = "/path/to/chromium/src"
	const testChromiumParentDir = "/path/to/chromium"
	const testDepotToolsPath = "/fake/depot_tools"

	setupService := func() *ChromiumBuilderService {
		return &ChromiumBuilderService{
			chromiumPath:   testChromiumPath,
			depotToolsPath: testDepotToolsPath,
		}
	}

	tests := []struct {
		name             string
		setupMocks       func(t *testing.T, mockCmdCollector *exec.CommandCollector, mockCheckout *MockCheckout) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error)
		expectError      bool
		expectedCheckout bool
		errorMsgContains string
		expectedCmdName  string
		expectedCmdArgs  []string
		expectedCmdDir   string
	}{
		{
			name: "happy path",
			setupMocks: func(t *testing.T, mockCmdCollector *exec.CommandCollector, mockCheckout *MockCheckout) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error) {
				dc := func(path string, perm os.FileMode) error {
					require.Equal(t, testChromiumParentDir, path)
					require.Equal(t, os.FileMode(0o750), perm)
					return nil
				}
				execRunFn := func(ctx context.Context, cmd *exec.Command) error {
					return nil // Simulate successful fetch
				}
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					require.Equal(t, ChromiumUrl, repoUrl)
					require.Equal(t, testChromiumParentDir, workdir)
					return mockCheckout, nil
				}
				return cf, dc, execRunFn
			},
			expectError:      false,
			expectedCheckout: true,
			expectedCmdName:  filepath.Join(testDepotToolsPath, "fetch"),
			expectedCmdArgs:  []string{"--nohooks", "chromium"},
			expectedCmdDir:   testChromiumParentDir,
		},
		{
			name: "directory creator fails",
			setupMocks: func(t *testing.T, mockCmdCollector *exec.CommandCollector, mockCheckout *MockCheckout) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error) {
				dc := func(path string, perm os.FileMode) error {
					return errors.New("mkdir failed")
				}
				execRunFn := func(ctx context.Context, cmd *exec.Command) error {
					return errors.New("exec.Run should not be called")
				}
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return nil, errors.New("cf should not be called")
				}
				return cf, dc, execRunFn
			},
			expectError:      true,
			expectedCheckout: false,
			errorMsgContains: "mkdir failed",
		},
		{
			name: "exec.Run fails (fetch command fails)",
			setupMocks: func(t *testing.T, mockCmdCollector *exec.CommandCollector, mockCheckout *MockCheckout) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error) {
				dc := func(path string, perm os.FileMode) error {
					return nil
				}
				execRunFn := func(ctx context.Context, cmd *exec.Command) error {
					return errors.New("fetch command failed")
				}
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return nil, errors.New("cf should not be called")
				}
				return cf, dc, execRunFn
			},
			expectError:      true,
			expectedCheckout: false,
			errorMsgContains: "Failed to fetch Chromium",
			expectedCmdName:  filepath.Join(testDepotToolsPath, "fetch"),
			expectedCmdArgs:  []string{"--nohooks", "chromium"},
			expectedCmdDir:   testChromiumParentDir,
		},
		{
			name: "checkout factory fails",
			setupMocks: func(t *testing.T, mockCmdCollector *exec.CommandCollector, mockCheckout *MockCheckout) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error) {
				dc := func(path string, perm os.FileMode) error {
					return nil
				}
				execRunFn := func(ctx context.Context, cmd *exec.Command) error {
					return nil // Simulate successful fetch
				}
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return nil, errors.New("checkout factory failed")
				}
				return cf, dc, execRunFn
			},
			expectError:      true,
			expectedCheckout: false,
			errorMsgContains: "checkout factory failed",
			expectedCmdName:  filepath.Join(testDepotToolsPath, "fetch"),
			expectedCmdArgs:  []string{"--nohooks", "chromium"},
			expectedCmdDir:   testChromiumParentDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupService()
			mockFS := vfs_mocks.NewFS(t)
			mockCmdCollector := &exec.CommandCollector{}
			mockCheckout := NewMockCheckout(t, testChromiumParentDir)

			cf, dc, execRunFn := tt.setupMocks(t, mockCmdCollector, mockCheckout)
			mockCmdCollector.SetDelegateRun(execRunFn)
			runCtx := exec.NewContext(context.Background(), mockCmdCollector.Run)

			err := s.handleMissingChromiumCheckout(runCtx, mockFS, cf, dc)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsgContains != "" {
					require.Contains(t, err.Error(), tt.errorMsgContains)
				}
			} else {
				require.NoError(t, err)
			}

			if tt.expectedCheckout {
				require.NotNil(t, s.chromiumCheckout)
				if !tt.expectError {
					require.Equal(t, mockCheckout, s.chromiumCheckout)
				}
			} else {
				require.Nil(t, s.chromiumCheckout)
			}

			commands := mockCmdCollector.Commands()
			if tt.expectedCmdName != "" {
				require.Len(t, commands, 1)
				cmd := commands[0]
				require.Equal(t, tt.expectedCmdName, cmd.Name)
				require.Equal(t, tt.expectedCmdArgs, cmd.Args)
				require.Equal(t, tt.expectedCmdDir, cmd.Dir)
				require.NotNil(t, cmd.CombinedOutput)
			} else {
				require.Len(t, commands, 0)
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

const (
	testDepotToolsPath      = "/test/depot_tools"
	testDepotToolsParentDir = "/test"
	testChromiumPath        = "/test/chromium/src"
	testChromiumParentDir   = "/test/chromium"
)

var validServiceArgs = fmt.Sprintf("%s=%s,%s=%s", ArgDepotToolsPath, testDepotToolsPath, ArgChromiumPath, testChromiumPath)

func TestChromiumBuilderService_initImpl(t *testing.T) {
	baseCtx := context.Background()

	tests := []struct {
		name             string
		serviceArgs      string
		setupMocks       func(t *testing.T, mockFS *vfs_mocks.FS, mockCmdCollector *exec.CommandCollector) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error)
		expectError      bool
		errorMsgContains string
	}{
		{
			name:        "happy path - both checkouts missing and successfully created",
			serviceArgs: validServiceArgs,
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCmdCollector *exec.CommandCollector) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error) {
				// Depot Tools Mocks (missing checkout)
				mockFS.On("Open", mock.AnythingOfType("*context.valueCtx"), testDepotToolsPath).Return(nil, os.ErrNotExist).Once()
				dc := func(path string, perm os.FileMode) error {
					if path == testDepotToolsParentDir {
						require.Equal(t, os.FileMode(0o750), perm)
						return nil
					}
					if path == testChromiumParentDir {
						require.Equal(t, os.FileMode(0o750), perm)
						return nil
					}
					return errors.New("unexpected dc call")
				}
				mockDtCheckout := NewMockCheckout(t, testDepotToolsParentDir)
				mockDtCheckout.On("Update", mock.AnythingOfType("*context.valueCtx")).Return(nil).Once()

				// Chromium Mocks (missing checkout)
				mockFS.On("Open", mock.AnythingOfType("*context.valueCtx"), testChromiumPath).Return(nil, os.ErrNotExist).Once()
				execRunFn := func(ctx context.Context, cmd *exec.Command) error {
					require.Equal(t, filepath.Join(testDepotToolsPath, "fetch"), cmd.Name)
					require.Equal(t, []string{"--nohooks", "chromium"}, cmd.Args)
					require.Equal(t, testChromiumParentDir, cmd.Dir)
					return nil
				}
				mockCrCheckout := NewMockCheckout(t, testChromiumParentDir)
				// No Update call needed for chromium if fetched new

				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					if repoUrl == DepotToolsUrl && workdir == testDepotToolsParentDir {
						return mockDtCheckout, nil
					}
					if repoUrl == ChromiumUrl && workdir == testChromiumParentDir {
						return mockCrCheckout, nil
					}
					return nil, errors.New("unexpected cf call")
				}
				return cf, dc, execRunFn
			},
			expectError: false,
		},
		{
			name:        "parseServiceArgs fails - missing depot_tools_path",
			serviceArgs: fmt.Sprintf("%s=%s", ArgChromiumPath, testChromiumPath),
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCmdCollector *exec.CommandCollector) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error) {
				return nil, nil, nil
			},
			expectError:      true,
			errorMsgContains: "Did not receive a depot_tools_path argument",
		},
		{
			name:        "handleDepotToolsSetup fails - dc fails for missing checkout",
			serviceArgs: validServiceArgs,
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCmdCollector *exec.CommandCollector) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error) {
				mockFS.On("Open", mock.AnythingOfType("*context.valueCtx"), testDepotToolsPath).Return(nil, os.ErrNotExist).Once()
				dc := func(path string, perm os.FileMode) error {
					if path == testDepotToolsParentDir {
						return errors.New("dc failed for depot_tools")
					}
					return errors.New("unexpected dc call")
				}
				return nil, dc, nil // cf and execRunFn not reached
			},
			expectError:      true,
			errorMsgContains: "dc failed for depot_tools",
		},
		{
			name:        "handleChromiumSetup fails - dc fails for missing checkout",
			serviceArgs: validServiceArgs,
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCmdCollector *exec.CommandCollector) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error) {
				// Depot Tools Mocks (missing checkout, success)
				mockFS.On("Open", mock.AnythingOfType("*context.valueCtx"), testDepotToolsPath).Return(nil, os.ErrNotExist).Once()
				mockDtCheckout := NewMockCheckout(t, testDepotToolsParentDir)
				mockDtCheckout.On("Update", mock.AnythingOfType("*context.valueCtx")).Return(nil).Once()

				// Chromium Mocks (missing checkout, dc fails)
				mockFS.On("Open", mock.AnythingOfType("*context.valueCtx"), testChromiumPath).Return(nil, os.ErrNotExist).Once()

				dc := func(path string, perm os.FileMode) error {
					if path == testDepotToolsParentDir {
						return nil
					}
					if path == testChromiumParentDir {
						return errors.New("dc failed for chromium")
					}
					return errors.New("unexpected dc call")
				}
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					if repoUrl == DepotToolsUrl && workdir == testDepotToolsParentDir {
						return mockDtCheckout, nil
					}
					return nil, errors.New("unexpected cf call for chromium")
				}
				return cf, dc, nil // execRunFn not reached
			},
			expectError:      true,
			errorMsgContains: "dc failed for chromium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ChromiumBuilderService{}
			mockFS := vfs_mocks.NewFS(t)
			mockCmdCollector := &exec.CommandCollector{}

			cf, dc, execRunFn := tt.setupMocks(t, mockFS, mockCmdCollector)
			mockCmdCollector.SetDelegateRun(execRunFn)
			runCtx := exec.NewContext(baseCtx, mockCmdCollector.Run)

			err := s.initImpl(runCtx, tt.serviceArgs, mockFS, cf, dc) // Pass runCtx here

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsgContains != "" {
					require.Contains(t, err.Error(), tt.errorMsgContains)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, testDepotToolsPath, s.depotToolsPath)
				require.Equal(t, testChromiumPath, s.chromiumPath)
				require.NotNil(t, s.depotToolsCheckout)
				require.NotNil(t, s.chromiumCheckout)
			}
		})
	}
}

func TestChromiumBuilderService_handleDepotToolsSetup(t *testing.T) {
	ctx := context.Background()
	setupService := func() *ChromiumBuilderService {
		return &ChromiumBuilderService{depotToolsPath: testDepotToolsPath}
	}

	tests := []struct {
		name             string
		setupMocks       func(t *testing.T, mockFS *vfs_mocks.FS) (checkoutFactory, directoryCreator)
		expectError      bool
		errorMsgContains string
		expectDtCheckout bool
	}{
		{
			name: "happy path - existing checkout",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS) (checkoutFactory, directoryCreator) {
				// For the first Open in handleDepotToolsSetup for testDepotToolsPath
				dtFileOpenedBySetup := vfs_mocks.NewFile(t)
				mockFS.On("Open", ctx, testDepotToolsPath).Return(dtFileOpenedBySetup, nil).Once()
				dtFileOpenedBySetup.On("Close", ctx).Return(nil).Once() // Closed by defer in handleDepotToolsSetup

				// Mocks for handleExistingDepotToolsCheckout's success
				// 1. checkIfPathIsDirectory(testDepotToolsPath)
				dtFileOpenedByCheck := vfs_mocks.NewFile(t)
				mockFS.On("Open", ctx, testDepotToolsPath).Return(dtFileOpenedByCheck, nil).Once() // Second Open for testDepotToolsPath
				dtFileInfo := &MockFileInfo{FIsDir: true}
				dtFileOpenedByCheck.On("Stat", ctx).Return(dtFileInfo, nil).Once()
				dtFileOpenedByCheck.On("Close", ctx).Return(nil).Once()

				// 2. Open lucicfg
				lucicfgFile := vfs_mocks.NewFile(t)
				mockFS.On("Open", ctx, filepath.Join(testDepotToolsPath, "lucicfg")).Return(lucicfgFile, nil).Once()
				lucicfgFile.On("Close", ctx).Return(nil).Once()

				// 3. checkIfPathIsDirectory for .git
				dotGitFile := vfs_mocks.NewFile(t)
				mockFS.On("Open", ctx, filepath.Join(testDepotToolsPath, ".git")).Return(dotGitFile, nil).Once()
				dotGitFileInfo := &MockFileInfo{FIsDir: true}
				dotGitFile.On("Stat", ctx).Return(dotGitFileInfo, nil).Once()
				dotGitFile.On("Close", ctx).Return(nil).Once()

				mockCheckout := NewMockCheckout(t, testDepotToolsParentDir)
				mockCheckout.On("Update", ctx).Return(nil).Once()
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
				return cf, nil
			},
			expectError:      false,
			expectDtCheckout: true,
		},
		{
			name: "happy path - missing checkout",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS) (checkoutFactory, directoryCreator) {
				mockFS.On("Open", ctx, testDepotToolsPath).Return(nil, os.ErrNotExist).Once()
				// Mocks for handleMissingDepotToolsCheckout's success
				dc := func(path string, perm os.FileMode) error { return nil }
				mockCheckout := NewMockCheckout(t, testDepotToolsParentDir)
				mockCheckout.On("Update", ctx).Return(nil).Once()
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
				return cf, dc
			},
			expectError:      false,
			expectDtCheckout: true,
		},
		{
			name: "error - fs.Open fails with non-NotExist error",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS) (checkoutFactory, directoryCreator) {
				mockFS.On("Open", ctx, testDepotToolsPath).Return(nil, errors.New("generic open error")).Once()
				return nil, nil
			},
			expectError:      true,
			errorMsgContains: "generic open error",
			expectDtCheckout: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupService()
			mockFS := vfs_mocks.NewFS(t)
			cf, dc := tt.setupMocks(t, mockFS)

			err := s.handleDepotToolsSetup(ctx, mockFS, cf, dc)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsgContains != "" {
					require.Contains(t, err.Error(), tt.errorMsgContains)
				}
			} else {
				require.NoError(t, err)
			}
			if tt.expectDtCheckout {
				require.NotNil(t, s.depotToolsCheckout)
			} else {
				require.Nil(t, s.depotToolsCheckout)
			}
		})
	}
}

func TestChromiumBuilderService_handleChromiumSetup(t *testing.T) {
	baseCtx := context.Background() // Base context
	setupService := func() *ChromiumBuilderService {
		return &ChromiumBuilderService{chromiumPath: testChromiumPath, depotToolsPath: testDepotToolsPath}
	}

	tests := []struct {
		name              string
		setupMocks        func(t *testing.T, mockFS *vfs_mocks.FS, mockCmdCollector *exec.CommandCollector) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error)
		expectError       bool
		errorMsgContains  string
		expectCrCheckout  bool
		expectedCmdsCount int
	}{
		{
			name: "happy path - existing checkout",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCmdCollector *exec.CommandCollector) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error) {
				// For the first Open in handleChromiumSetup for testChromiumPath
				crFileOpenedBySetup := vfs_mocks.NewFile(t)
				mockFS.On("Open", mock.AnythingOfType("*context.valueCtx"), testChromiumPath).Return(crFileOpenedBySetup, nil).Once()
				crFileOpenedBySetup.On("Close", mock.AnythingOfType("*context.valueCtx")).Return(nil).Once() // Closed by defer in handleChromiumSetup

				// Mocks for handleExistingChromiumCheckout's success
				// 1. checkIfPathIsDirectory(testChromiumPath)
				crFileOpenedByCheck := vfs_mocks.NewFile(t)
				mockFS.On("Open", mock.AnythingOfType("*context.valueCtx"), testChromiumPath).Return(crFileOpenedByCheck, nil).Once() // Second Open for testChromiumPath
				crFileInfo := &MockFileInfo{FIsDir: true}
				crFileOpenedByCheck.On("Stat", mock.AnythingOfType("*context.valueCtx")).Return(crFileInfo, nil).Once()
				crFileOpenedByCheck.On("Close", mock.AnythingOfType("*context.valueCtx")).Return(nil).Once()

				// 2. checkIfPathIsDirectory for .git
				dotGitFile := vfs_mocks.NewFile(t)
				mockFS.On("Open", mock.AnythingOfType("*context.valueCtx"), filepath.Join(testChromiumPath, ".git")).Return(dotGitFile, nil).Once()
				dotGitFileInfo := &MockFileInfo{FIsDir: true}
				dotGitFile.On("Stat", mock.AnythingOfType("*context.valueCtx")).Return(dotGitFileInfo, nil).Once()
				dotGitFile.On("Close", mock.AnythingOfType("*context.valueCtx")).Return(nil).Once()

				mockCheckout := NewMockCheckout(t, testChromiumParentDir)
				mockCheckout.On("Update", mock.AnythingOfType("*context.valueCtx")).Return(nil).Once()
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
				return cf, nil, nil
			},
			expectError:       false,
			expectCrCheckout:  true,
			expectedCmdsCount: 0,
		},
		{
			name: "happy path - missing checkout",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCmdCollector *exec.CommandCollector) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error) {
				mockFS.On("Open", mock.AnythingOfType("*context.valueCtx"), testChromiumPath).Return(nil, os.ErrNotExist).Once()
				// Mocks for handleMissingChromiumCheckout's success
				dc := func(path string, perm os.FileMode) error { return nil }
				execRunFn := func(ctx context.Context, cmd *exec.Command) error { return nil }
				mockCheckout := NewMockCheckout(t, testChromiumParentDir)
				// No Update call for new checkout
				cf := func(c context.Context, repoUrl string, workdir string) (git.Checkout, error) {
					return mockCheckout, nil
				}
				return cf, dc, execRunFn
			},
			expectError:       false,
			expectCrCheckout:  true,
			expectedCmdsCount: 1, // fetch command
		},
		{
			name: "error - fs.Open fails with non-NotExist error",
			setupMocks: func(t *testing.T, mockFS *vfs_mocks.FS, mockCmdCollector *exec.CommandCollector) (checkoutFactory, directoryCreator, func(context.Context, *exec.Command) error) {
				mockFS.On("Open", mock.AnythingOfType("*context.valueCtx"), testChromiumPath).Return(nil, errors.New("generic open error")).Once()
				return nil, nil, nil
			},
			expectError:       true,
			errorMsgContains:  "generic open error",
			expectCrCheckout:  false,
			expectedCmdsCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupService()
			mockFS := vfs_mocks.NewFS(t)
			mockCmdCollector := &exec.CommandCollector{}

			cf, dc, execRunFn := tt.setupMocks(t, mockFS, mockCmdCollector)
			mockCmdCollector.SetDelegateRun(execRunFn)
			runCtx := exec.NewContext(baseCtx, mockCmdCollector.Run)

			err := s.handleChromiumSetup(runCtx, mockFS, cf, dc)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsgContains != "" {
					require.Contains(t, err.Error(), tt.errorMsgContains)
				}
			} else {
				require.NoError(t, err)
			}

			if tt.expectCrCheckout {
				require.NotNil(t, s.chromiumCheckout)
			} else {
				require.Nil(t, s.chromiumCheckout)
			}
			require.Len(t, mockCmdCollector.Commands(), tt.expectedCmdsCount)
		})
	}
}
