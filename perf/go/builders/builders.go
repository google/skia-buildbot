// Package builders builds objects from config.InstanceConfig objects.
//
// These are functions separate from config.InstanceConfig so that we don't end
// up with cyclical import issues.
package builders

import (
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/config"
)

// NewAlertStoreFromConfig creates a new alerts.AlertStore from the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewAlertStoreFromConfig(local bool, cfg *config.InstanceConfig) (alerts.AlertStore, error) {
	if local {
		// Should we forcibly change the namespace?
	}
	return alerts.NewAlertStoreDS(), nil
}
