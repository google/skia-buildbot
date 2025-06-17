package chromiumbuilder

// Code related to running commands that can be cancelled mid-run by the
// service.

import (
	"sync"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
)

// runSafeCancellableCommand runs the provided Command in such a way that it can
// be cancelled mid-run. Any commands run this way must not result in bad state
// being left on disk in the event of the command being cancelled.
func (s *ChromiumBuilderService) runSafeCancellableCommand(cmd *exec.Command, ccr concurrentCommandRunner) error {
	return s.runCancellableCommand(cmd, ccr, &(s.safeCancellableCommandLock))
}

// runCancellableCommand runs the provided Command in such a way that it can be
// cancelled mid-run. The sync.Mutex argument will be locked for the duration
// of the function to signal that some cancellable command is being run.
func (s *ChromiumBuilderService) runCancellableCommand(cmd *exec.Command, ccr concurrentCommandRunner, lock *sync.Mutex) error {
	lock.Lock()
	defer lock.Unlock()
	// This is manually unlocked later so we can release it sooner.
	s.currentProcessLock.Lock()

	if s.shuttingDown.Load() {
		s.currentProcessLock.Unlock()
		return skerr.Fmt("Server is shutting down, not starting cancellable command.")
	}

	process, doneChan, err := ccr(cmd)
	s.currentProcess = process
	s.currentProcessLock.Unlock()
	if err != nil {
		return err
	}
	err = <-doneChan
	s.currentProcessLock.Lock()
	s.currentProcess = nil
	s.currentProcessLock.Unlock()
	if err != nil {
		return err
	}

	return nil
}
