package chromiumbuilder

// Tests for chromiumbuilder code that is shared by a multiple tool handlers.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
)

func TestChromiumBuilderService_prepareCheckoutsForStarlarkModification(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		setupMocks       func(t *testing.T, mockDtCheckout *MockCheckout, mockCrCheckout *MockCheckout)
		expectError      bool
		errorMsgContains string
	}{
		{
			name: "happy path",
			setupMocks: func(t *testing.T, mockDtCheckout *MockCheckout, mockCrCheckout *MockCheckout) {
				mockDtCheckout.On("Update", ctx).Return(nil).Once()
				mockCrCheckout.On("Update", ctx).Return(nil).Once()
				mockCrCheckout.On("Git", ctx, mock.MatchedBy(func(cmd []string) bool {
					return len(cmd) == 3 && cmd[0] == "checkout" && cmd[1] == "-b"
				})).Return("", nil).Once()
			},
			expectError: false,
		},
		{
			name: "update depot_tools fails",
			setupMocks: func(t *testing.T, mockDtCheckout *MockCheckout, mockCrCheckout *MockCheckout) {
				mockDtCheckout.On("Update", ctx).Return(errors.New("dt update failed")).Once()
			},
			expectError:      true,
			errorMsgContains: "dt update failed",
		},
		{
			name: "update chromium fails",
			setupMocks: func(t *testing.T, mockDtCheckout *MockCheckout, mockCrCheckout *MockCheckout) {
				mockDtCheckout.On("Update", ctx).Return(nil).Once()
				mockCrCheckout.On("Update", ctx).Return(errors.New("cr update failed")).Once()
			},
			expectError:      true,
			errorMsgContains: "cr update failed",
		},
		{
			name: "switch to temporary branch fails",
			setupMocks: func(t *testing.T, mockDtCheckout *MockCheckout, mockCrCheckout *MockCheckout) {
				mockDtCheckout.On("Update", ctx).Return(nil).Once()
				mockCrCheckout.On("Update", ctx).Return(nil).Once()
				mockCrCheckout.On("Git", ctx, mock.MatchedBy(func(cmd []string) bool {
					return len(cmd) == 3 && cmd[0] == "checkout" && cmd[1] == "-b"
				})).Return("", errors.New("git checkout failed")).Once()
			},
			expectError:      true,
			errorMsgContains: "git checkout failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ChromiumBuilderService{}
			mockDtCheckout := NewMockCheckout(t, "/fake/depot_tools")
			mockCrCheckout := NewMockCheckout(t, "/fake/chromium/src")
			s.depotToolsCheckout = mockDtCheckout
			s.chromiumCheckout = mockCrCheckout

			tt.setupMocks(t, mockDtCheckout, mockCrCheckout)

			branchName, err := s.prepareCheckoutsForStarlarkModification(ctx)

			if tt.expectError {
				require.Error(t, err)
				require.Empty(t, branchName)
				if tt.errorMsgContains != "" {
					require.Contains(t, err.Error(), tt.errorMsgContains)
				}
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, branchName)
			}
		})
	}
}

func TestChromiumBuilderService_updateCheckouts(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		setupService     func(s *ChromiumBuilderService)
		setupMocks       func(t *testing.T, mockDtCheckout *MockCheckout, mockCrCheckout *MockCheckout)
		expectError      bool
		errorMsgContains string
	}{
		{
			name: "happy path",
			setupMocks: func(t *testing.T, mockDtCheckout *MockCheckout, mockCrCheckout *MockCheckout) {
				mockDtCheckout.On("Update", ctx).Return(nil).Once()
				mockCrCheckout.On("Update", ctx).Return(nil).Once()
			},
			expectError: false,
		},
		{
			name: "update depot_tools fails",
			setupMocks: func(t *testing.T, mockDtCheckout *MockCheckout, mockCrCheckout *MockCheckout) {
				mockDtCheckout.On("Update", ctx).Return(errors.New("dt update failed")).Once()
			},
			expectError:      true,
			errorMsgContains: "dt update failed",
		},
		{
			name: "update chromium fails",
			setupMocks: func(t *testing.T, mockDtCheckout *MockCheckout, mockCrCheckout *MockCheckout) {
				mockDtCheckout.On("Update", ctx).Return(nil).Once()
				mockCrCheckout.On("Update", ctx).Return(errors.New("cr update failed")).Once()
			},
			expectError:      true,
			errorMsgContains: "cr update failed",
		},
		{
			name: "server shutting down",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(true)
			},
			setupMocks: func(t *testing.T, mockDtCheckout *MockCheckout, mockCrCheckout *MockCheckout) {
				// No calls to Update should be made.
			},
			expectError:      true,
			errorMsgContains: "Server is shutting down, not proceeding with depot_tools update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ChromiumBuilderService{}
			mockDtCheckout := NewMockCheckout(t, "/fake/depot_tools")
			mockCrCheckout := NewMockCheckout(t, "/fake/chromium/src")
			s.depotToolsCheckout = mockDtCheckout
			s.chromiumCheckout = mockCrCheckout

			if tt.setupService != nil {
				tt.setupService(s)
			} else {
				s.shuttingDown.Store(false)
			}
			tt.setupMocks(t, mockDtCheckout, mockCrCheckout)

			err := s.updateCheckouts(ctx)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsgContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChromiumBuilderService_switchToTemporaryBranch(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		setupService     func(s *ChromiumBuilderService)
		setupMocks       func(t *testing.T, mockCrCheckout *MockCheckout)
		expectError      bool
		errorMsgContains string
	}{
		{
			name: "happy path",
			setupMocks: func(t *testing.T, mockCrCheckout *MockCheckout) {
				mockCrCheckout.On("Git", ctx, mock.MatchedBy(func(cmd []string) bool {
					return len(cmd) == 3 && cmd[0] == "checkout" && cmd[1] == "-b"
				})).Return("", nil).Once()
			},
			expectError: false,
		},
		{
			name: "server shutting down",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(true)
			},
			setupMocks: func(t *testing.T, mockCrCheckout *MockCheckout) {
				// No Git call expected.
			},
			expectError:      true,
			errorMsgContains: "Server is shutting down, not proceeding with branch switch",
		},
		{
			name: "git checkout fails",
			setupMocks: func(t *testing.T, mockCrCheckout *MockCheckout) {
				mockCrCheckout.On("Git", ctx, mock.MatchedBy(func(cmd []string) bool {
					return len(cmd) == 3 && cmd[0] == "checkout" && cmd[1] == "-b"
				})).Return("", errors.New("git checkout failed")).Once()
			},
			expectError:      true,
			errorMsgContains: "git checkout failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ChromiumBuilderService{}
			mockCrCheckout := NewMockCheckout(t, "/fake/chromium/src")
			s.chromiumCheckout = mockCrCheckout

			if tt.setupService != nil {
				tt.setupService(s)
			} else {
				s.shuttingDown.Store(false)
			}
			tt.setupMocks(t, mockCrCheckout)

			branchName, err := s.switchToTemporaryBranch(ctx)

			if tt.expectError {
				require.Error(t, err)
				require.Empty(t, branchName)
				require.Contains(t, err.Error(), tt.errorMsgContains)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, branchName)
			}
		})
	}
}

func TestChromiumBuilderService_cleanUpBranch(t *testing.T) {
	ctx := context.Background()
	const testBranchName = "test-branch-123"

	tests := []struct {
		name             string
		setupService     func(s *ChromiumBuilderService)
		setupMocks       func(t *testing.T, mockCheckout *MockCheckout)
		expectError      bool
		errorMsgContains string
	}{
		{
			name: "happy path",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(false)
			},
			setupMocks: func(t *testing.T, mockCheckout *MockCheckout) {
				mockCheckout.On("Git", ctx, []string{"checkout", "main"}).Return("", nil).Once()
				mockCheckout.On("Git", ctx, []string{"branch", "-D", testBranchName}).Return("", nil).Once()
			},
			expectError: false,
		},
		{
			name: "server shutting down",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(true)
			},
			setupMocks:       func(t *testing.T, mockCheckout *MockCheckout) {},
			expectError:      true,
			errorMsgContains: "Server is shutting down, not proceeding with branch cleanup",
		},
		{
			name: "checkout main fails",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(false)
			},
			setupMocks: func(t *testing.T, mockCheckout *MockCheckout) {
				mockCheckout.On("Git", ctx, []string{"checkout", "main"}).Return("", errors.New("checkout main failed")).Once()
			},
			expectError:      true,
			errorMsgContains: "checkout main failed",
		},
		{
			name: "delete branch fails",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(false)
			},
			setupMocks: func(t *testing.T, mockCheckout *MockCheckout) {
				mockCheckout.On("Git", ctx, []string{"checkout", "main"}).Return("", nil).Once()
				mockCheckout.On("Git", ctx, []string{"branch", "-D", testBranchName}).Return("", errors.New("delete branch failed")).Once()
			},
			expectError:      true,
			errorMsgContains: "delete branch failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ChromiumBuilderService{}
			mockCheckout := NewMockCheckout(t, "/fake/chromium/src") // workdir doesn't matter for this test
			s.chromiumCheckout = mockCheckout

			if tt.setupService != nil {
				tt.setupService(s)
			}
			tt.setupMocks(t, mockCheckout)

			err := s.cleanUpBranch(ctx, testBranchName)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsgContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChromiumBuilderService_cleanUpBranchDeferred(t *testing.T) {
	ctx := context.Background()
	const testBranchName = "test-branch-deferred-123"

	tests := []struct {
		name       string
		setupMocks func(t *testing.T, mockCheckout *MockCheckout)
	}{
		{
			name: "underlying cleanUpBranch succeeds",
			setupMocks: func(t *testing.T, mockCheckout *MockCheckout) {
				// Mocks for a successful cleanUpBranch call.
				mockCheckout.On("Git", ctx, []string{"checkout", "main"}).Return("", nil).Once()
				mockCheckout.On("Git", ctx, []string{"branch", "-D", testBranchName}).Return("", nil).Once()
			},
		},
		{
			name: "underlying cleanUpBranch fails",
			setupMocks: func(t *testing.T, mockCheckout *MockCheckout) {
				// Mock a failure in the first git command of cleanUpBranch.
				mockCheckout.On("Git", ctx, []string{"checkout", "main"}).Return("", errors.New("checkout main failed")).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ChromiumBuilderService{}
			mockCheckout := NewMockCheckout(t, "/fake/chromium/src")
			s.chromiumCheckout = mockCheckout
			s.shuttingDown.Store(false) // cleanUpBranch checks this.

			tt.setupMocks(t, mockCheckout)

			// The deferred function should never panic or return an error.
			// The mock framework will assert that the expected calls were made.
			s.cleanUpBranchDeferred(ctx, testBranchName)
		})
	}
}

func Test_determineBuildConfig(t *testing.T) {
	tests := []struct {
		name             string
		buildConfig      string
		expected         string
		expectError      bool
		errorMsgContains string
	}{
		{
			name:        "Debug",
			buildConfig: BuilderConfigDebug,
			expected:    "builder_config.build_config.DEBUG",
			expectError: false,
		},
		{
			name:        "Release",
			buildConfig: BuilderConfigRelease,
			expected:    "builder_config.build_config.RELEASE",
			expectError: false,
		},
		{
			name:             "unhandled",
			buildConfig:      "unhandled",
			expectError:      true,
			errorMsgContains: "Unhandled builder config unhandled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := determineBuildConfig(tt.buildConfig)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsgContains)
				require.Empty(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func Test_determineTargetArch(t *testing.T) {
	tests := []struct {
		name             string
		targetArch       string
		expected         string
		expectError      bool
		errorMsgContains string
	}{
		{
			name:        "Arm",
			targetArch:  TargetArchArm,
			expected:    "builder_config.target_arch.ARM",
			expectError: false,
		},
		{
			name:        "Intel",
			targetArch:  TargetArchIntel,
			expected:    "builder_config.target_arch.INTEL",
			expectError: false,
		},
		{
			name:             "unhandled",
			targetArch:       "unhandled",
			expectError:      true,
			errorMsgContains: "Unhandled target architecture unhandled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := determineTargetArch(tt.targetArch)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsgContains)
				require.Empty(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func Test_determineTargetOs(t *testing.T) {
	tests := []struct {
		name             string
		targetOs         string
		expected         string
		expectError      bool
		errorMsgContains string
	}{
		{
			name:        "Android",
			targetOs:    TargetOsAndroid,
			expected:    "builder_config.target_platform.ANDROID",
			expectError: false,
		},
		{
			name:        "Linux",
			targetOs:    TargetOsLinux,
			expected:    "builder_config.target_platform.LINUX",
			expectError: false,
		},
		{
			name:        "Mac",
			targetOs:    TargetOsMac,
			expected:    "builder_config.target_platform.MAC",
			expectError: false,
		},
		{
			name:        "Windows",
			targetOs:    TargetOsWin,
			expected:    "builder_config.target_platform.WIN",
			expectError: false,
		},
		{
			name:             "unhandled",
			targetOs:         "unhandled",
			expectError:      true,
			errorMsgContains: "Unhandled target OS unhandled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := determineTargetOs(tt.targetOs)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsgContains)
				require.Empty(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func Test_determineAdditionalConfigs(t *testing.T) {
	tests := []struct {
		name     string
		targetOs string
		expected string
	}{
		{
			name:     "Android",
			targetOs: TargetOsAndroid,
			expected: `android_config = builder_config.android_config(config = "base_config"),`,
		},
		{
			name:     "Linux",
			targetOs: TargetOsLinux,
			expected: "",
		},
		{
			name:     "Other non-android",
			targetOs: "some-other-os",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := determineAdditionalConfigs(tt.targetOs)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func Test_quoteAndCommaSeparate(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: "",
		},
		{
			name:     "single element",
			input:    []string{"one"},
			expected: `"one"`,
		},
		{
			name:     "multiple elements",
			input:    []string{"one", "two", "three"},
			expected: `"one", "two", "three"`,
		},
		{
			name:     "slice with empty string",
			input:    []string{"one", "", "three"},
			expected: `"one", "", "three"`,
		},
		{
			name:     "slice with only empty string",
			input:    []string{""},
			expected: `""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := quoteAndCommaSeparate(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// Tests for determineGnArgs, determineTests, and determineSwarmingDimensions
// intentionally omitted for now since they just call quoteAndCommaSeparate.

func Test_formatString(t *testing.T) {
	tests := []struct {
		name             string
		format           string
		data             map[string]string
		expected         string
		expectError      bool
		errorMsgContains string
	}{
		{
			name:        "happy path",
			format:      "Hello, {{.Name}}!",
			data:        map[string]string{"Name": "World"},
			expected:    "Hello, World!",
			expectError: false,
		},
		{
			name:        "multiple replacements",
			format:      "Key1: {{.Key1}}, Key2: {{.Key2}}",
			data:        map[string]string{"Key1": "Value1", "Key2": "Value2"},
			expected:    "Key1: Value1, Key2: Value2",
			expectError: false,
		},
		{
			name:        "empty template",
			format:      "",
			data:        map[string]string{"Name": "World"},
			expected:    "",
			expectError: false,
		},
		{
			name:        "empty data map with template expecting data",
			format:      "Hello, {{.Name}}!",
			data:        map[string]string{},
			expected:    "Hello, <no value>!",
			expectError: false,
		},
		{
			name:             "invalid template syntax",
			format:           "Hello, {{.Name",
			data:             map[string]string{"Name": "World"},
			expectError:      true,
			errorMsgContains: "unclosed action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatString(tt.format, tt.data)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsgContains)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestChromiumBuilderService_handleStarlarkFormattingAndGeneration(t *testing.T) {
	ctx := context.Background()
	const testDepotToolsPath = "/fake/depot_tools"
	const testChromiumPath = "/fake/chromium/src"

	setupService := func() *ChromiumBuilderService {
		return &ChromiumBuilderService{
			depotToolsPath: testDepotToolsPath,
			chromiumPath:   testChromiumPath,
		}
	}

	tests := []struct {
		name             string
		setupService     func(s *ChromiumBuilderService)
		setupMocks       func(t *testing.T) concurrentCommandRunner
		expectError      bool
		errorMsgContains string
	}{
		{
			name: "happy path",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(false)
			},
			setupMocks: func(t *testing.T) concurrentCommandRunner {
				return func(cmd *exec.Command) (exec.Process, <-chan error, error) {
					errCh := make(chan error, 1)
					errCh <- nil
					close(errCh)
					return NewMockProcess(t), errCh, nil
				}
			},
			expectError: false,
		},
		{
			name: "formatStarlark fails",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(false)
			},
			setupMocks: func(t *testing.T) concurrentCommandRunner {
				return func(cmd *exec.Command) (exec.Process, <-chan error, error) {
					errCh := make(chan error, 1)
					errCh <- errors.New("lucicfg failed")
					close(errCh)
					return NewMockProcess(t), errCh, nil
				}
			},
			expectError:      true,
			errorMsgContains: "Failed to format Starlark",
		},
		{
			name: "generateFilesFromStarlark fails",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(false)
			},
			setupMocks: func(t *testing.T) concurrentCommandRunner {
				callCount := 0
				return func(cmd *exec.Command) (exec.Process, <-chan error, error) {
					callCount++
					errCh := make(chan error, 1)
					if callCount == 1 { // formatStarlark succeeds
						errCh <- nil
					} else { // generateFilesFromStarlark fails
						errCh <- errors.New("main.star failed")
					}
					close(errCh)
					return NewMockProcess(t), errCh, nil
				}
			},
			expectError:      true,
			errorMsgContains: "Failed to generate files from Starlark",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupService()
			tt.setupService(s)
			ccr := tt.setupMocks(t)

			err := s.handleStarlarkFormattingAndGeneration(ctx, ccr)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsgContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChromiumBuilderService_formatStarlark(t *testing.T) {
	ctx := context.Background()
	const testDepotToolsPath = "/fake/depot_tools"
	const testChromiumPath = "/fake/chromium/src"
	expectedLucicfgPath := filepath.Join(testDepotToolsPath, "lucicfg")
	expectedInfraConfigPath := filepath.Join(testChromiumPath, InfraConfigSubdirectory)

	setupService := func() *ChromiumBuilderService {
		return &ChromiumBuilderService{
			depotToolsPath: testDepotToolsPath,
			chromiumPath:   testChromiumPath,
		}
	}

	tests := []struct {
		name             string
		setupService     func(s *ChromiumBuilderService)
		setupMocks       func(t *testing.T) concurrentCommandRunner
		expectError      bool
		errorMsgContains string
	}{
		{
			name: "happy path",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(false)
			},
			setupMocks: func(t *testing.T) concurrentCommandRunner {
				return func(cmd *exec.Command) (exec.Process, <-chan error, error) {
					require.Equal(t, expectedLucicfgPath, cmd.Name)
					require.Equal(t, []string{"fmt", expectedInfraConfigPath}, cmd.Args)
					require.NotNil(t, cmd.CombinedOutput)

					errCh := make(chan error, 1)
					errCh <- nil
					close(errCh)
					return NewMockProcess(t), errCh, nil
				}
			},
			expectError: false,
		},
		{
			name: "command fails",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(false)
			},
			setupMocks: func(t *testing.T) concurrentCommandRunner {
				return func(cmd *exec.Command) (exec.Process, <-chan error, error) {
					errCh := make(chan error, 1)
					errCh <- errors.New("lucicfg failed")
					close(errCh)
					return NewMockProcess(t), errCh, nil
				}
			},
			expectError:      true,
			errorMsgContains: "Failed to format Starlark. Original error: lucicfg failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupService()
			tt.setupService(s)
			ccr := tt.setupMocks(t)

			err := s.formatStarlark(ctx, ccr)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsgContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChromiumBuilderService_generateFilesFromStarlark(t *testing.T) {
	ctx := context.Background()
	const testChromiumPath = "/fake/chromium/src"
	expectedStarlarkMainPath := filepath.Join(testChromiumPath, InfraConfigSubdirectory, "main.star")

	setupService := func() *ChromiumBuilderService {
		return &ChromiumBuilderService{
			chromiumPath: testChromiumPath,
		}
	}

	tests := []struct {
		name             string
		setupService     func(s *ChromiumBuilderService)
		setupMocks       func(t *testing.T) concurrentCommandRunner
		expectError      bool
		errorMsgContains string
	}{
		{
			name: "happy path",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(false)
			},
			setupMocks: func(t *testing.T) concurrentCommandRunner {
				return func(cmd *exec.Command) (exec.Process, <-chan error, error) {
					require.Equal(t, expectedStarlarkMainPath, cmd.Name)
					require.Equal(t, []string{}, cmd.Args)
					require.NotNil(t, cmd.CombinedOutput)

					errCh := make(chan error, 1)
					errCh <- nil
					close(errCh)
					return NewMockProcess(t), errCh, nil
				}
			},
			expectError: false,
		},
		{
			name: "command fails",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(false)
			},
			setupMocks: func(t *testing.T) concurrentCommandRunner {
				return func(cmd *exec.Command) (exec.Process, <-chan error, error) {
					errCh := make(chan error, 1)
					errCh <- errors.New("main.star failed")
					close(errCh)
					return NewMockProcess(t), errCh, nil
				}
			},
			expectError:      true,
			errorMsgContains: "Failed to generate files from Starlark. Original error: main.star failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupService()
			tt.setupService(s)
			ccr := tt.setupMocks(t)

			err := s.generateFilesFromStarlark(ctx, ccr)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsgContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChromiumBuilderService_handleCommitAndUpload(t *testing.T) {
	ctx := context.Background()
	const testBuilderName = "test-builder"
	const testBuilderGroup = "test.group"
	const testChromiumPath = "/fake/chromium/src"
	const testDepotToolsPath = "/fake/depot_tools"
	const gerritLink = "https://chromium-review.googlesource.com/c/12345"

	setupService := func() *ChromiumBuilderService {
		s := &ChromiumBuilderService{
			chromiumPath:   testChromiumPath,
			depotToolsPath: testDepotToolsPath,
		}
		s.shuttingDown.Store(false)
		return s
	}

	tests := []struct {
		name             string
		setupService     func(s *ChromiumBuilderService)
		setupMocks       func(t *testing.T, mockCrCheckout *MockCheckout) (concurrentCommandRunner, environmentGetter)
		expectError      bool
		errorMsgContains string
		expectedLink     string
	}{
		{
			name: "happy path",
			setupMocks: func(t *testing.T, mockCrCheckout *MockCheckout) (concurrentCommandRunner, environmentGetter) {
				// Mocks for addAndCommitFiles
				infraConfigPath := filepath.Join(testChromiumPath, InfraConfigSubdirectory)
				expectedTitle := fmt.Sprintf("Add new builder %s", testBuilderName)
				expectedDescription := fmt.Sprintf("Adds a new builder %s in the %s group. This CL was auto-generated.", testBuilderName, testBuilderGroup)
				mockCrCheckout.On("Git", ctx, []string{"add", infraConfigPath}).Return("", nil).Once()
				mockCrCheckout.On("Git", ctx, []string{"commit", "-m", expectedTitle, "-m", expectedDescription}).Return("", nil).Once()

				// Mocks for uploadCl
				ccr := func(cmd *exec.Command) (exec.Process, <-chan error, error) {
					if cmd.Name == filepath.Join(testDepotToolsPath, "git_cl.py") {
						if cmd.CombinedOutput != nil {
							_, err := cmd.CombinedOutput.Write([]byte(fmt.Sprintf("Issue created. URL: %s", gerritLink)))
							require.NoError(t, err)
						}
						errCh := make(chan error, 1)
						errCh <- nil
						close(errCh)
						return NewMockProcess(t), errCh, nil
					}
					t.Fatalf("Unexpected command: %s", cmd.Name)
					return nil, nil, errors.New("unexpected command")
				}
				eg := func(key string) string { return "" } // Simple mock for env getter
				return ccr, eg
			},
			expectError:  false,
			expectedLink: gerritLink,
		},
		{
			name: "addAndCommitFiles fails on add",
			setupMocks: func(t *testing.T, mockCrCheckout *MockCheckout) (concurrentCommandRunner, environmentGetter) {
				infraConfigPath := filepath.Join(testChromiumPath, InfraConfigSubdirectory)
				mockCrCheckout.On("Git", ctx, []string{"add", infraConfigPath}).Return("", errors.New("git add failed")).Once()
				return nil, nil
			},
			expectError:      true,
			errorMsgContains: "git add failed",
		},
		{
			name: "addAndCommitFiles fails on commit",
			setupMocks: func(t *testing.T, mockCrCheckout *MockCheckout) (concurrentCommandRunner, environmentGetter) {
				infraConfigPath := filepath.Join(testChromiumPath, InfraConfigSubdirectory)
				expectedTitle := fmt.Sprintf("Add new builder %s", testBuilderName)
				expectedDescription := fmt.Sprintf("Adds a new builder %s in the %s group. This CL was auto-generated.", testBuilderName, testBuilderGroup)
				mockCrCheckout.On("Git", ctx, []string{"add", infraConfigPath}).Return("", nil).Once()
				mockCrCheckout.On("Git", ctx, []string{"commit", "-m", expectedTitle, "-m", expectedDescription}).Return("", errors.New("git commit failed")).Once()
				return nil, nil
			},
			expectError:      true,
			errorMsgContains: "git commit failed",
		},
		{
			name: "uploadCl fails",
			setupMocks: func(t *testing.T, mockCrCheckout *MockCheckout) (concurrentCommandRunner, environmentGetter) {
				infraConfigPath := filepath.Join(testChromiumPath, InfraConfigSubdirectory)
				expectedTitle := fmt.Sprintf("Add new builder %s", testBuilderName)
				expectedDescription := fmt.Sprintf("Adds a new builder %s in the %s group. This CL was auto-generated.", testBuilderName, testBuilderGroup)
				mockCrCheckout.On("Git", ctx, []string{"add", infraConfigPath}).Return("", nil).Once()
				mockCrCheckout.On("Git", ctx, []string{"commit", "-m", expectedTitle, "-m", expectedDescription}).Return("", nil).Once()

				ccr := func(cmd *exec.Command) (exec.Process, <-chan error, error) {
					errCh := make(chan error, 1)
					errCh <- errors.New("git cl upload failed")
					close(errCh)
					return NewMockProcess(t), errCh, nil
				}
				eg := func(key string) string { return "" }
				return ccr, eg
			},
			expectError:      true,
			errorMsgContains: "git cl upload failed",
		},
		{
			name: "server shutting down",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(true)
			},
			setupMocks: func(t *testing.T, mockCrCheckout *MockCheckout) (concurrentCommandRunner, environmentGetter) {
				return nil, nil
			},
			expectError:      true,
			errorMsgContains: "Server is shutting down, not proceeding with adding/committing files.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupService()
			if tt.setupService != nil {
				tt.setupService(s)
			}
			mockCrCheckout := NewMockCheckout(t, testChromiumPath)
			s.chromiumCheckout = mockCrCheckout

			ccr, eg := tt.setupMocks(t, mockCrCheckout)

			link, err := s.handleCommitAndUpload(ctx, testBuilderName, testBuilderGroup, ccr, eg)

			if tt.expectError {
				require.Error(t, err)
				require.Empty(t, link)
				if tt.errorMsgContains != "" {
					require.Contains(t, err.Error(), tt.errorMsgContains)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedLink, link)
			}
		})
	}
}

func TestChromiumBuilderService_addAndCommitFiles(t *testing.T) {
	ctx := context.Background()
	const testBuilderName = "test-builder"
	const testBuilderGroup = "test.group"
	const testChromiumPath = "/fake/chromium/src"

	setupService := func() *ChromiumBuilderService {
		s := &ChromiumBuilderService{
			chromiumPath: testChromiumPath,
		}
		s.shuttingDown.Store(false)
		return s
	}

	tests := []struct {
		name             string
		setupService     func(s *ChromiumBuilderService)
		setupMocks       func(t *testing.T, mockCrCheckout *MockCheckout)
		expectError      bool
		errorMsgContains string
	}{
		{
			name: "happy path",
			setupMocks: func(t *testing.T, mockCrCheckout *MockCheckout) {
				infraConfigPath := filepath.Join(testChromiumPath, InfraConfigSubdirectory)
				mockCrCheckout.On("Git", ctx, []string{"add", infraConfigPath}).Return("", nil).Once()

				expectedTitle := fmt.Sprintf("Add new builder %s", testBuilderName)
				expectedDescription := fmt.Sprintf("Adds a new builder %s in the %s group. This CL was auto-generated.", testBuilderName, testBuilderGroup)
				mockCrCheckout.On("Git", ctx, []string{"commit", "-m", expectedTitle, "-m", expectedDescription}).Return("", nil).Once()
			},
			expectError: false,
		},
		{
			name: "server shutting down",
			setupService: func(s *ChromiumBuilderService) {
				s.shuttingDown.Store(true)
			},
			setupMocks: func(t *testing.T, mockCrCheckout *MockCheckout) {
				// No calls to Git should be made.
			},
			expectError:      true,
			errorMsgContains: "Server is shutting down, not proceeding with adding/committing files.",
		},
		{
			name: "git add fails",
			setupMocks: func(t *testing.T, mockCrCheckout *MockCheckout) {
				infraConfigPath := filepath.Join(testChromiumPath, InfraConfigSubdirectory)
				mockCrCheckout.On("Git", ctx, []string{"add", infraConfigPath}).Return("", errors.New("git add failed")).Once()
			},
			expectError:      true,
			errorMsgContains: "git add failed",
		},
		{
			name: "git commit fails",
			setupMocks: func(t *testing.T, mockCrCheckout *MockCheckout) {
				infraConfigPath := filepath.Join(testChromiumPath, InfraConfigSubdirectory)
				mockCrCheckout.On("Git", ctx, []string{"add", infraConfigPath}).Return("", nil).Once()

				expectedTitle := fmt.Sprintf("Add new builder %s", testBuilderName)
				expectedDescription := fmt.Sprintf("Adds a new builder %s in the %s group. This CL was auto-generated.", testBuilderName, testBuilderGroup)
				mockCrCheckout.On("Git", ctx, []string{"commit", "-m", expectedTitle, "-m", expectedDescription}).Return("", errors.New("git commit failed")).Once()
			},
			expectError:      true,
			errorMsgContains: "git commit failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupService()
			if tt.setupService != nil {
				tt.setupService(s)
			}
			mockCrCheckout := NewMockCheckout(t, testChromiumPath)
			s.chromiumCheckout = mockCrCheckout

			tt.setupMocks(t, mockCrCheckout)

			err := s.addAndCommitFiles(ctx, testBuilderName, testBuilderGroup)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsgContains != "" {
					require.Contains(t, err.Error(), tt.errorMsgContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChromiumBuilderService_uploadCl(t *testing.T) {
	ctx := context.Background()
	const testDepotToolsPath = "/fake/depot_tools"
	const testChromiumPath = "/fake/chromium/src"
	const expectedGitClPath = "/fake/depot_tools/git_cl.py"
	const gerritLink = "https://chromium-review.googlesource.com/c/12345"

	setupService := func() *ChromiumBuilderService {
		return &ChromiumBuilderService{
			depotToolsPath: testDepotToolsPath,
			chromiumPath:   testChromiumPath,
		}
	}

	tests := []struct {
		name             string
		existingPathEnv  string
		ccrReturnError   error
		ccrOutput        string
		expectError      bool
		errorMsgContains string
		expectedLink     string
	}{
		{
			name:            "happy path - no existing PATH",
			existingPathEnv: "",
			ccrOutput:       fmt.Sprintf("Issue created. URL: %s", gerritLink),
			expectedLink:    gerritLink,
		},
		{
			name:            "happy path - with existing PATH",
			existingPathEnv: "/usr/bin:/bin",
			ccrOutput:       fmt.Sprintf("Issue created. URL: %s some other output", gerritLink),
			expectedLink:    gerritLink,
		},
		{
			name:             "ccr returns error",
			ccrReturnError:   errors.New("git cl failed"),
			expectError:      true,
			errorMsgContains: "Failed to upload CL to Gerrit. Original error: git cl failed",
		},
		{
			name:             "ccr output does not contain gerrit link",
			ccrOutput:        "Something went wrong, no link here.",
			expectError:      true,
			errorMsgContains: "Unable to extract Gerrit link",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupService()

			mockCCR := func(cmd *exec.Command) (exec.Process, <-chan error, error) {
				require.Equal(t, expectedGitClPath, cmd.Name)
				require.Equal(t, []string{"upload", "--skip-title", "--bypass-hooks", "--force"}, cmd.Args)
				require.Equal(t, testChromiumPath, cmd.Dir)
				require.NotNil(t, cmd.CombinedOutput)
				require.True(t, cmd.InheritEnv)
				require.WithinDuration(t, time.Now().Add(5*time.Minute), time.Now().Add(cmd.Timeout), 1*time.Second)

				expectedEnvPath := fmt.Sprintf("PATH=%s", testDepotToolsPath)
				if tt.existingPathEnv != "" {
					expectedEnvPath = fmt.Sprintf("%s:%s", expectedEnvPath, tt.existingPathEnv)
				}
				require.Contains(t, cmd.Env, expectedEnvPath)

				if cmd.CombinedOutput != nil {
					_, err := cmd.CombinedOutput.Write([]byte(tt.ccrOutput))
					require.NoError(t, err)
				}

				errCh := make(chan error, 1)
				errCh <- tt.ccrReturnError
				close(errCh)
				mp := NewMockProcess(t) // MockProcess can be simple as it's not directly used by uploadCl after runSafeCancellableCommand
				return mp, errCh, nil
			}

			mockEG := func(key string) string {
				if key == "PATH" {
					return tt.existingPathEnv
				}
				return ""
			}

			link, err := s.uploadCl(ctx, mockCCR, mockEG)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsgContains != "" {
					require.Contains(t, err.Error(), tt.errorMsgContains)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedLink, link)
			}
		})
	}
}
