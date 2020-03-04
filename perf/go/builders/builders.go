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
	"go.skia.org/infra/perf/go/alerts/dsalertstore"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/dsregressionstore"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/shortcut/dsshortcutstore"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/btts"
)

// NewTraceStoreFromConfig creates a new TraceStore from the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewTraceStoreFromConfig(ctx context.Context, local bool, cfg *config.InstanceConfig) (tracestore.TraceStore, error) {
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

// NewAlertStoreFromConfig creates a new alerts.Store from the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewAlertStoreFromConfig(local bool, cfg *config.InstanceConfig) (alerts.Store, error) {
	if local {
		// Should we forcibly change the namespace?
	}
	return dsalertstore.New(), nil
}

// NewRegressionStoreFromConfig creates a new regression.RegressionStore from
// the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewRegressionStoreFromConfig(local bool, cidl *cid.CommitIDLookup, cfg *config.InstanceConfig) (regression.Store, error) {
	lookup := func(ctx context.Context, c *cid.CommitID) (*cid.CommitDetail, error) {
		details, err := cidl.Lookup(ctx, []*cid.CommitID{c})
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return details[0], nil
	}
	return dsregressionstore.NewRegressionStoreDS(lookup), nil
}

// NewShortcutStoreFromConfig creates a new shortcut.Store from the
// InstanceConfig.
func NewShortcutStoreFromConfig(cfg *config.InstanceConfig) (shortcut.Store, error) {
	return dsshortcutstore.New(), nil
}
