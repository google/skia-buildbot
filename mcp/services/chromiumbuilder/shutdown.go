package chromiumbuilder

// Code related to ChromiumBuilderService.Shutdown()

import (
	"path/filepath"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// shutdownImpl is the actual implementation for Shutdown(), broken out to
// support dependency injection.
func (s *ChromiumBuilderService) shutdownImpl(dr directoryRemover) error {
	sklog.Infof("Shutting down Chromium Builder service")
	s.shuttingDown.Store(true)

	err := s.ensureDepotToolsCheckoutNotInUse()
	if err != nil {
		return err
	}

	err = s.ensureChromiumCheckoutNotInUse()
	if err != nil {
		return err
	}

	err = s.cancelSafeCommands()
	if err != nil {
		return err
	}

	err = s.cancelChromiumFetch(dr)
	if err != nil {
		return err
	}

	return nil
}

// ensureDepotToolsCheckoutNotInUse ensures that the depot_tools checkout is not
// actively being used before continuing with shutdown. Killing the server while
// it is in use, e.g. mid-update, could leave the checkout in an unusable state
// which would affect the server the next time it is deployed.
func (s *ChromiumBuilderService) ensureDepotToolsCheckoutNotInUse() error {
	if !s.shuttingDown.Load() {
		return skerr.Fmt("ensureDepotToolsCheckoutNotInUse() must only be called during shutdown.")
	}

	s.depotToolsCheckoutLock.Lock()
	defer s.depotToolsCheckoutLock.Unlock()

	// Both the initial checkout and updating of depot_tools is very quick, so
	// just let them run their course. Both are handled via the git package
	// rather than the exec package anyways, so we would not be able to cancel
	// them mid-run.
	return nil
}

// ensureChromiumCheckoutNotInUse ensures that the Chromium checkout is not
// actively being used before continuing with shutdown. Killing the server while
// it is in use, e.g. mid-update, could leave the checkout in an unusable state
// which would affect the server the next time it is deployed.
func (s *ChromiumBuilderService) ensureChromiumCheckoutNotInUse() error {
	if !s.shuttingDown.Load() {
		return skerr.Fmt("ensureChromiumCheckoutNotInUse() must only be called during shutdown.")
	}

	s.chromiumCheckoutLock.Lock()
	defer s.chromiumCheckoutLock.Unlock()

	// Initial checkout setup is handled via fetch, which can be cancelled
	// in another shutdown helper. Updating the Chromium checkout should not
	// take too long, and isn't cancellable anyways due to use of the git
	// package instead of the exec package.
	return nil
}

// cancelSafeCommands cancels any in-progress commands which are safe to cancel
// without any additional cleanup.
func (s *ChromiumBuilderService) cancelSafeCommands() error {
	if !s.shuttingDown.Load() {
		return skerr.Fmt("cancelSafeCommands() must only be called during shutdown.")
	}

	notCurrentlyRunning := s.safeCancellableCommandLock.TryLock()
	if notCurrentlyRunning {
		s.safeCancellableCommandLock.Unlock()
		return nil
	}

	s.currentProcessLock.Lock()
	defer s.currentProcessLock.Unlock()

	if s.currentProcess == nil {
		// This can happen in one of two ways:
		//   1. We tried to acquire safeCallableCommandLock just as it was
		//      acquired by the function running the command. In this case, we
		//      can safely assume that the current process won't be set later
		//      since that function will detect that the server is shutting down
		//      and not start the process.
		//   2. We tried to acquire safeCallableCommandLock as the function
		//      running the command was finishing. In this case, the process has
		//      already finished.
		// In both cases, it is safe to not do anything else.
		return nil
	}

	err := s.currentProcess.Kill()
	if err != nil {
		// We don't return this error since we want shutdown to continue. It
		// seems likely that we are going to hit this during normal operation
		// anyways if the process is already finished by the time we try to kill
		// it.
		sklog.Errorf("Got the following error when trying to kill the current running safe command: %v", err)
	}
	return nil
}

// cancelChromiumFetch cancels the in-progress Chromium fetch, if there is one.
// In the event that there is an in-progress fetch, the directories potentially
// containing checkout data will be wiped in order to ensure it is not left
// in a bad state that will affect future deployments.
func (s *ChromiumBuilderService) cancelChromiumFetch(dr directoryRemover) error {
	if !s.shuttingDown.Load() {
		return skerr.Fmt("cancelChromiumFetch() must only be called during shutdown.")
	}

	notCurrentlyFetching := s.chromiumFetchLock.TryLock()
	if notCurrentlyFetching {
		s.chromiumFetchLock.Unlock()
		return nil
	}

	s.currentProcessLock.Lock()
	defer s.currentProcessLock.Unlock()

	if s.currentProcess == nil {
		// See cancelSafeCommands for explanation on why we can safely do
		// nothing here.
		return nil
	}

	err := s.currentProcess.Kill()
	if err != nil {
		sklog.Errorf("Got the following error when trying to kill the Chromium fetch process: %v", err)
	}

	// We remove the parent directory since the stored path is to the src
	// directory, but gclient information is stored in the directory above that.
	// We want to wipe any gclient information as well so that the next
	// deployment will have a clean slate.
	err = dr(filepath.Dir(s.chromiumPath))
	if err != nil {
		sklog.Errorf(("Failed to delete in-progress Chromium checkout, future deployments will likely fail until " +
			"this is cleaned up. Error: %v"), err)
		return err
	}

	return nil
}
