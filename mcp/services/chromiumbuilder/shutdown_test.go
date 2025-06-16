package chromiumbuilder

// Tests for code in service.go related to Shutdown().

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChromiumBuilderService_shutdownImpl(t *testing.T) {
	const testChromiumDir = "/test/chromium"

	tests := []struct {
		name             string
		setupService     func(s *ChromiumBuilderService)
		setupMocks       func(t *testing.T, mockDirRemover *mockDirectoryRemover, mockProcess *MockProcess)
		expectError      bool
		errorMsgContains string
		expectDirRemoval bool
	}{
		{
			name: "happy path - no locks held, no processes running",
			setupService: func(s *ChromiumBuilderService) {
				s.chromiumPath = filepath.Join(testChromiumDir, "src")
			},
			setupMocks: func(t *testing.T, mockDirRemover *mockDirectoryRemover, mockProcess *MockProcess) {
				// No mocks needed as no processes should be killed or dirs removed if not fetching.
			},
			expectError:      false,
			expectDirRemoval: false,
		},
		{
			name: "safe command running - should be killed",
			setupService: func(s *ChromiumBuilderService) {
				s.chromiumPath = filepath.Join(testChromiumDir, "src")
				s.safeCancellableCommandLock.Lock() // Simulate command running
			},
			setupMocks: func(t *testing.T, mockDirRemover *mockDirectoryRemover, mockProcess *MockProcess) {
				mockProcess.On("Kill").Return(nil).Once()
			},
			expectError:      false,
			expectDirRemoval: false,
		},
		{
			name: "chromium fetch running - should be killed and dir removed",
			setupService: func(s *ChromiumBuilderService) {
				s.chromiumPath = filepath.Join(testChromiumDir, "src")
				s.chromiumFetchLock.Lock() // Simulate fetch running
			},
			setupMocks: func(t *testing.T, mockDirRemover *mockDirectoryRemover, mockProcess *MockProcess) {
				mockProcess.On("Kill").Return(nil).Once()
				mockDirRemover.On("Execute", testChromiumDir).Return(nil).Once()
			},
			expectError:      false,
			expectDirRemoval: true,
		},
		{
			name: "chromium fetch running - dir removal fails",
			setupService: func(s *ChromiumBuilderService) {
				s.chromiumPath = filepath.Join(testChromiumDir, "src")
				s.chromiumFetchLock.Lock() // Simulate fetch running
			},
			setupMocks: func(t *testing.T, mockDirRemover *mockDirectoryRemover, mockProcess *MockProcess) {
				mockProcess.On("Kill").Return(nil).Once()
				mockDirRemover.On("Execute", testChromiumDir).Return(errors.New("remove failed")).Once()
			},
			expectError:      true,
			errorMsgContains: "remove failed",
			expectDirRemoval: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ChromiumBuilderService{}
			mockProcess := NewMockProcess(t) // Create mock process

			// Apply service setup, which might lock command locks
			if tt.setupService != nil {
				tt.setupService(s)
			}

			// Determine if currentProcess should be non-nil for the test.
			// This simulates the state where runCancellableCommand would have set currentProcess
			// because a command was started and its specific lock (safeCancellableCommandLock or chromiumFetchLock)
			// is being held by runCancellableCommand.
			var isAnyCommandIntendedToRun bool
			if !s.safeCancellableCommandLock.TryLock() {
				// If TryLock fails, it means setupService locked it and it's still locked.
				// This is the state cancelSafeCommands expects if a command is running.
				isAnyCommandIntendedToRun = true
				// We DO NOT unlock it here. cancelSafeCommands needs to see it locked.
			} else {
				// TryLock succeeded, meaning setupService did NOT lock it, or it was locked and then unlocked by setupService.
				// We must unlock it as TryLock acquired it.
				s.safeCancellableCommandLock.Unlock()
			}

			if !s.chromiumFetchLock.TryLock() {
				isAnyCommandIntendedToRun = true // Could be true from safe command already
				// DO NOT unlock. cancelChromiumFetch needs to see it locked.
			} else {
				s.chromiumFetchLock.Unlock() // Unlock if TryLock succeeded
			}

			if isAnyCommandIntendedToRun {
				s.currentProcess = mockProcess
			} else {
				s.currentProcess = nil
			}

			mockDirRemover := &mockDirectoryRemover{}
			mockDirRemover.Test(t)

			// Setup mocks for Kill or RemoveAll based on the test case
			if tt.setupMocks != nil {
				tt.setupMocks(t, mockDirRemover, mockProcess)
			}

			err := s.shutdownImpl(mockDirRemover.Execute)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsgContains != "" {
					require.Contains(t, err.Error(), tt.errorMsgContains)
				}
			} else {
				require.NoError(t, err)
			}
			require.True(t, s.shuttingDown.Load())
			mockDirRemover.AssertExpectations(t)
			mockProcess.AssertExpectations(t)
		})
	}
}

func TestChromiumBuilderService_ensureDepotToolsCheckoutNotInUse(t *testing.T) {
	s := &ChromiumBuilderService{}

	// Test error if not shutting down
	s.shuttingDown.Store(false)
	err := s.ensureDepotToolsCheckoutNotInUse()
	require.Error(t, err)
	require.Contains(t, err.Error(), "must only be called during shutdown")

	// Test happy path when shutting down
	s.shuttingDown.Store(true)
	err = s.ensureDepotToolsCheckoutNotInUse()
	require.NoError(t, err)

	// Test that it waits for the lock
	s.shuttingDown.Store(true)
	s.depotToolsCheckoutLock.Lock()
	done := make(chan bool)
	go func() {
		err = s.ensureDepotToolsCheckoutNotInUse()
		require.NoError(t, err)
		done <- true
	}()
	time.Sleep(10 * time.Millisecond) // Give goroutine a chance to block
	s.depotToolsCheckoutLock.Unlock()
	<-done
}

func TestChromiumBuilderService_ensureChromiumCheckoutNotInUse(t *testing.T) {
	s := &ChromiumBuilderService{}

	// Test error if not shutting down
	s.shuttingDown.Store(false)
	err := s.ensureChromiumCheckoutNotInUse()
	require.Error(t, err)
	require.Contains(t, err.Error(), "must only be called during shutdown")

	// Test happy path when shutting down
	s.shuttingDown.Store(true)
	err = s.ensureChromiumCheckoutNotInUse()
	require.NoError(t, err)

	// Test that it waits for the lock
	s.shuttingDown.Store(true)
	s.chromiumCheckoutLock.Lock()
	done := make(chan bool)
	go func() {
		err = s.ensureChromiumCheckoutNotInUse()
		require.NoError(t, err)
		done <- true
	}()
	time.Sleep(10 * time.Millisecond) // Give goroutine a chance to block
	s.chromiumCheckoutLock.Unlock()
	<-done
}

func TestChromiumBuilderService_cancelSafeCommands(t *testing.T) {
	s := &ChromiumBuilderService{}
	mockProcess := NewMockProcess(t)

	// Test error if not shutting down
	s.shuttingDown.Store(false)
	err := s.cancelSafeCommands()
	require.Error(t, err)
	require.Contains(t, err.Error(), "must only be called during shutdown")

	// Test no command running
	s.shuttingDown.Store(true)
	s.currentProcess = nil
	err = s.cancelSafeCommands()
	require.NoError(t, err)

	// Test command running, currentProcess is nil (should not happen if lock is held, but test defensively)
	s.shuttingDown.Store(true)
	s.safeCancellableCommandLock.Lock()
	s.currentProcess = nil
	err = s.cancelSafeCommands()
	s.safeCancellableCommandLock.Unlock() // Release for next test
	require.NoError(t, err)

	// Test command running, currentProcess is set
	s.shuttingDown.Store(true)
	s.safeCancellableCommandLock.Lock()
	s.currentProcess = mockProcess
	mockProcess.On("Kill").Return(nil).Once()
	err = s.cancelSafeCommands()
	s.safeCancellableCommandLock.Unlock()
	require.NoError(t, err)
	mockProcess.AssertExpectations(t)

	// Test command running, Kill fails
	s.shuttingDown.Store(true)
	s.safeCancellableCommandLock.Lock()
	s.currentProcess = mockProcess
	mockProcess.On("Kill").Return(errors.New("kill failed")).Once()
	err = s.cancelSafeCommands()
	s.safeCancellableCommandLock.Unlock()
	require.NoError(t, err) // Error from Kill is logged but not returned
	mockProcess.AssertExpectations(t)
}

func TestChromiumBuilderService_cancelChromiumFetch(t *testing.T) {
	s := &ChromiumBuilderService{chromiumPath: "/test/chromium/src"}
	mockProcess := NewMockProcess(t)
	mockRemover := &mockDirectoryRemover{}
	mockRemover.Test(t)

	// Test error if not shutting down
	s.shuttingDown.Store(false)
	err := s.cancelChromiumFetch(mockRemover.Execute)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must only be called during shutdown")

	// Test no fetch running
	s.shuttingDown.Store(true)
	s.currentProcess = nil
	err = s.cancelChromiumFetch(mockRemover.Execute)
	require.NoError(t, err)

	// Test fetch running, currentProcess is nil
	s.shuttingDown.Store(true)
	s.chromiumFetchLock.Lock()
	s.currentProcess = nil
	err = s.cancelChromiumFetch(mockRemover.Execute)
	s.chromiumFetchLock.Unlock()
	require.NoError(t, err)

	// Test fetch running, currentProcess is set, remove succeeds
	s.shuttingDown.Store(true)
	s.chromiumFetchLock.Lock()
	s.currentProcess = mockProcess
	mockProcess.On("Kill").Return(nil).Once()
	mockRemover.On("Execute", "/test/chromium").Return(nil).Once()
	err = s.cancelChromiumFetch(mockRemover.Execute)
	s.chromiumFetchLock.Unlock()
	require.NoError(t, err)
	mockProcess.AssertExpectations(t)
	mockRemover.AssertExpectations(t)

	// Test fetch running, Kill fails
	s.shuttingDown.Store(true)
	s.chromiumFetchLock.Lock()
	s.currentProcess = mockProcess
	mockProcess.On("Kill").Return(errors.New("kill failed")).Once()
	mockRemover.On("Execute", "/test/chromium").Return(nil).Once() // Still expect removal
	err = s.cancelChromiumFetch(mockRemover.Execute)
	s.chromiumFetchLock.Unlock()
	require.NoError(t, err) // Error from Kill is logged
	mockProcess.AssertExpectations(t)
	mockRemover.AssertExpectations(t)

	// Test fetch running, remove fails
	s.shuttingDown.Store(true)
	s.chromiumFetchLock.Lock()
	s.currentProcess = mockProcess
	mockProcess.On("Kill").Return(nil).Once()
	mockRemover.On("Execute", "/test/chromium").Return(errors.New("remove failed")).Once()
	err = s.cancelChromiumFetch(mockRemover.Execute)
	s.chromiumFetchLock.Unlock()
	require.Error(t, err)
	require.Contains(t, err.Error(), "remove failed")
	mockProcess.AssertExpectations(t)
	mockRemover.AssertExpectations(t)
}
