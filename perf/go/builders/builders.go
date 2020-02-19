// Package builders builds objects from config.InstanceConfig objects.
//
// These are functions separate from config.InstanceConfig so that we don't end
// up with cyclical import issues.
package builders

import (
	"context"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/alerts/alertstores"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/dsregressionstore"
	"go.skia.org/infra/perf/go/types"
)

// NewTraceStoreFromConfig creates a new TraceStore from the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewTraceStoreFromConfig(ctx context.Context, local bool, cfg *config.InstanceConfig) (types.TraceStore, error) {
	sklog.Info("About to create token source.")
	ts, err := auth.NewDefaultTokenSource(local, bigtable.Scope)
	if err != nil {
		sklog.Fatalf("Failed to get TokenSource: %s", err)
	}

	traceStore, err := btts.NewBigTableTraceStoreFromConfig(ctx, cfg, ts, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to open trace store")
	}
	return traceStore, nil
}

// NewAlertStoreFromConfig creates a new alerts.AlertStore from the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewAlertStoreFromConfig(local bool, cfg *config.InstanceConfig) (alerts.AlertStore, error) {
	if local {
		// Should we forcibly change the namespace?
	}
	return alertstores.NewAlertStoreDS(), nil
}

// NewRegressionStoreFromConfig creates a new regression.RegressionStore from
// the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewRegressionStoreFromConfig(local bool, cfg *config.InstanceConfig) (regression.Store, error) {
	return dsregressionstore.NewRegressionStoreDS(), nil
}
