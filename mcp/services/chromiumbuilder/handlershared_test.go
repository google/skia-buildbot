package chromiumbuilder

// Tests for chromiumbuilder code that is shared by a multiple tool handlers.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
)

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
