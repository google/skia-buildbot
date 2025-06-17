package chromiumbuilder

// Git checkout-related code that is used in multiple places in the package.

import (
	"context"
	"path/filepath"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	ChromiumUrl   string = "https://chromium.googlesource.com/chromium/src"
	DepotToolsUrl string = "https://chromium.googlesource.com/chromium/tools/depot_tools"
)

// createDepotToolsCheckout creates and stores a re-usable reference to the
// depot_tools checkout.
func (s *ChromiumBuilderService) createDepotToolsCheckout(ctx context.Context, cf checkoutFactory) error {
	s.depotToolsCheckoutLock.Lock()
	defer s.depotToolsCheckoutLock.Unlock()

	if s.shuttingDown.Load() {
		return skerr.Fmt("Server is shutting down, not proceeding with depot_tools checkout.")
	}
	var err error
	s.depotToolsCheckout, err = cf(ctx, DepotToolsUrl, filepath.Dir(s.depotToolsPath))
	if err != nil {
		return err
	}

	return nil
}

func (s *ChromiumBuilderService) createChromiumCheckout(ctx context.Context, cf checkoutFactory) error {
	s.chromiumCheckoutLock.Lock()
	defer s.chromiumCheckoutLock.Unlock()

	if s.shuttingDown.Load() {
		return skerr.Fmt("Server is shutting down, not proceeding with Chromium checkout.")
	}
	var err error
	s.chromiumCheckout, err = cf(ctx, ChromiumUrl, filepath.Dir(s.chromiumPath))
	if err != nil {
		return err
	}

	return nil
}

// updateDepotToolsCheckout ensures that depot_tools is up to date with
// origin/main.
func (s *ChromiumBuilderService) updateDepotToolsCheckout(ctx context.Context) error {
	sklog.Info("Updating depot_tools checkout")
	s.depotToolsCheckoutLock.Lock()
	defer s.depotToolsCheckoutLock.Unlock()

	if s.shuttingDown.Load() {
		return skerr.Fmt("Server is shutting down, not proceeding with depot_tools update")
	}
	err := s.depotToolsCheckout.Update(ctx)
	if err != nil {
		return err
	}
	return nil
}

// updateChromiumCheckout ensures that Chromium is up to date with origin/main.
// This does *not* interact with gclient, as DEPS should not be needed for
// interacting with //infra/config.
func (s *ChromiumBuilderService) updateChromiumCheckout(ctx context.Context) error {
	sklog.Info("Updating Chromium checkout")
	s.chromiumCheckoutLock.Lock()
	defer s.chromiumCheckoutLock.Unlock()

	if s.shuttingDown.Load() {
		return skerr.Fmt("Server is shutting down, not proceeding with Chromium update")
	}

	err := s.chromiumCheckout.Update(ctx)
	if err != nil {
		return err
	}

	return nil
}
