package regression

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/types"
)

// RegressionsForAlert looks for regressions to the given alert over the last
// domain.N commits with data and periodically calls clusterResponseProcessor
// with the results of checking each commit.
func RegressionsForAlert(
	ctx context.Context,
	alert *alerts.Alert,
	domain types.Domain,
	ps paramtools.ParamSet,
	shortcutStore shortcut.Store,
	clusterResponseProcessor RegressionDetectionResponseProcessor,
	perfGit *perfgit.Git,
	dfBuilder dataframe.DataFrameBuilder,
	progressCallback ProgressCallback,
) {
	queriesCounter := metrics2.GetCounter("perf_clustering_queries", nil)
	sklog.Infof("About to cluster for: %#v", *alert)

	// This set of queries is restricted by the incoming set of trace ids, if
	// that's the kind of loop we're doing, by restricting 'ps' to just the
	// trace ids.
	queries, err := alert.QueriesFromParamset(ps)
	if err != nil {
		sklog.Errorf("Failed to build GroupBy combinations: %s", err)
		return
	}
	sklog.Infof("Config expanded into %d queries.", len(queries))
	for step, q := range queries {
		if progressCallback != nil {
			progressCallback(fmt.Sprintf("Step %d/%d\nQuery: %q", step+1, len(queries), q))
		}
		sklog.Infof("Clustering for query: %q", q)

		// Create RegressionDetectionRequest and run.
		req := &RegressionDetectionRequest{
			Alert:        alert,
			Domain:       domain,
			Query:        q,
			Step:         step,
			TotalQueries: len(queries),
		}
		_, err := Run(ctx, req, perfGit, dfBuilder, shortcutStore, clusterResponseProcessor, progressCallback)
		if err != nil {
			sklog.Warningf("Failed while clustering %v %s", *req, err)
			continue
		}
		queriesCounter.Inc(1)
	}
	sklog.Infof("Finished clustering for: %#v", *alert)
}
