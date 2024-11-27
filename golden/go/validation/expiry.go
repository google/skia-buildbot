package validation

import (
	"context"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/validation/data_manager"
)

// ExpirationMonitor provides a struct to perform expiration monitoring tasks.
type ExpirationMonitor struct {
	expiryDataManager data_manager.ExpiryDataManager
}

// New returns a new instance of the ExpirationMonitor.
func NewExpirationMonitor(dataManager data_manager.ExpiryDataManager) *ExpirationMonitor {
	return &ExpirationMonitor{
		expiryDataManager: dataManager,
	}
}

// UpdateTriagedExpectationsExpiry retrieves positively triaged Expectations that are about
// to expire in a month and updates them to never expire.
//
// Positively triaged expectations are used as the baseline to compare images and need to
// exist for as long as needed. While Expectations that get updated post checkin have their
// expiry auto updated, the ones that are triaged during the CL (i.e updated in ExpectationDeltas
// and then copied over to Expectations in gitilesFollower) get an expiry of 2 months (default on
// the Expectations table rows for ON CREATE operation). This function will capture those and
// update their expiry accordingly.
func (m *ExpirationMonitor) UpdateTriagedExpectationsExpiry(ctx context.Context) error {
	rowsToUpdate, err := m.expiryDataManager.GetExpiringExpectations(ctx)
	if err != nil {
		return err
	}
	targetExpiry := time.Now().AddDate(1000, 0, 0)
	sklog.Infof("Retrieved %d positive expectations to update expiry.", len(rowsToUpdate))

	if len(rowsToUpdate) > 0 {
		return m.expiryDataManager.UpdateExpectationsExpiry(ctx, rowsToUpdate, targetExpiry)
	}

	return nil
}
