package anomalies_impl

import (
	"context"

	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/anomalies"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/chromeperf/compat"
	"go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/types"
)

// sqlAnomaliesStore implements anomalies.Store.
type sqlAnomaliesStore struct {
	regStore regression.Store
	git      git.Git
}

// NewSqlAnomaliesStore returns a new anomalies.Store instance that wraps a
// regression.Store and perfgit.Git.
func NewSqlAnomaliesStore(regStore regression.Store, perfGit git.Git) (*sqlAnomaliesStore, error) {
	return &sqlAnomaliesStore{
		regStore: regStore,
		git:      perfGit,
	}, nil
}

// GetAnomalies implements anomalies.Store.
// It delegates to the underlying regression.Store.
func (s *sqlAnomaliesStore) GetAnomalies(ctx context.Context, traceNames []string, startCommitPosition int, endCommitPosition int) (chromeperf.AnomalyMap, error) {
	ctx, span := trace.StartSpan(ctx, "anomalies.sqlAnomaliesStore.GetAnomalies")
	defer span.End()
	result := chromeperf.AnomalyMap{}

	if startCommitPosition < 0 || endCommitPosition < startCommitPosition {
		return nil, skerr.Fmt("invalid commit range for GetAnomalies: [%d, %d]", startCommitPosition, endCommitPosition)
	}

	var regressions []*regression.Regression
	var err error
	if len(traceNames) == 0 {
		regressionsMap, err := s.regStore.Range(ctx, types.CommitNumber(startCommitPosition), types.CommitNumber(endCommitPosition))
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to load regressions from database")
		}
		for _, allRegressionsForCommit := range regressionsMap {
			for _, reg := range allRegressionsForCommit.ByAlertID {
				regressions = append(regressions, reg)
			}
		}
	} else {
		regressions, err = s.regStore.RangeFiltered(ctx, types.CommitNumber(startCommitPosition), types.CommitNumber(endCommitPosition), traceNames)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to load regressions from database")
		}
	}

	multiplicities := map[string]map[types.CommitNumber]int{}
	for _, reg := range regressions {
		convertedAnomalies, err := compat.ConvertRegressionToAnomalies(reg)
		if err != nil {
			sklog.Warningf("Could not convert regression with id %s to anomalies: %s", reg.Id, err)
			continue
		}

		for traceKey, commitMap := range convertedAnomalies {
			if _, ok := result[traceKey]; !ok {
				result[traceKey] = chromeperf.CommitNumberAnomalyMap{}
				multiplicities[traceKey] = map[types.CommitNumber]int{}
			}
			for commitNumber, anomaly := range commitMap {
				multiplicities[traceKey][commitNumber]++
				anomaly.Multiplicity = multiplicities[traceKey][commitNumber]
				result[traceKey][commitNumber] = anomaly
				sklog.Debugf("Constructed anomaly for trace %s: %+v", traceKey, anomaly)
			}
		}
	}
	sklog.Debugf("Found %d anomalies for traceNames: %v, startCommitPosition: %d, endCommitPosition: %d", len(result), traceNames, startCommitPosition, endCommitPosition)
	return result, nil
}

// GetAnomaliesInTimeRange implements anomalies.Store.
// It uses perfgit.Git to convert time range to commit range, then calls GetAnomalies.
func (s *sqlAnomaliesStore) GetAnomaliesInTimeRange(ctx context.Context, traceNames []string, startTime time.Time, endTime time.Time) (chromeperf.AnomalyMap, error) {
	ctx, span := trace.StartSpan(ctx, "anomalies.sqlAnomaliesStore.GetAnomaliesInTimeRange")
	defer span.End()

	if s.git == nil {
		return nil, skerr.Fmt("Git provider is not initialized for sqlAnomaliesStore")
	}

	if startTime.After(endTime) {
		return nil, skerr.Fmt("invalid time range: start %v is after end %v", startTime, endTime)
	}

	commits, err := s.git.CommitSliceFromTimeRange(ctx, startTime, endTime)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get commits from time range %v to %v", startTime, endTime)
	}

	if len(commits) == 0 {
		sklog.Debugf("No commits found in time range %v to %v", startTime, endTime)
		return chromeperf.AnomalyMap{}, nil
	}

	startCommitNumber := commits[0].CommitNumber
	endCommitNumber := commits[len(commits)-1].CommitNumber

	sklog.Debugf("Found %d commits in time range %v to %v. Converted to commit range [%d, %d]", len(commits), startTime, endTime, startCommitNumber, endCommitNumber)
	return s.GetAnomalies(ctx, traceNames, int(startCommitNumber), int(endCommitNumber))
}

// GetAnomaliesAroundRevision implements anomalies.Store.
// It defines a window around the given revision, fetches anomalies in that window,
// and then transforms them.
func (s *sqlAnomaliesStore) GetAnomaliesAroundRevision(ctx context.Context, revision int) ([]chromeperf.AnomalyForRevision, error) {
	ctx, span := trace.StartSpan(ctx, "anomalies.sqlAnomaliesStore.GetAnomaliesAroundRevision")
	defer span.End()

	if s.git == nil {
		return nil, skerr.Fmt("Git provider is not initialized for sqlAnomaliesStore")
	}

	const windowSize = 500
	startCommitPosition := revision - windowSize
	if startCommitPosition < 0 {
		startCommitPosition = 0
	}
	endCommitPosition := revision + windowSize

	anomalyMap, err := s.GetAnomalies(ctx, []string{}, startCommitPosition, endCommitPosition)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get anomalies for revision window [%d, %d]", startCommitPosition, endCommitPosition)
	}

	// Transform AnomalyMap to []chromeperf.AnomalyForRevision.
	var result []chromeperf.AnomalyForRevision
	for traceKey, commitMap := range anomalyMap {
		for _, anomaly := range commitMap {
			testPath := anomaly.TestPath
			if testPath == "" {
				testPath = traceKey
			}
			result = append(result, chromeperf.AnomalyForRevision{
				StartRevision: anomaly.StartRevision,
				EndRevision:   anomaly.EndRevision,
				Anomaly:       anomaly,
				TestPath:      testPath,
			})
		}
	}

	sklog.Debugf("Found %d anomalies around revision %d (window [%d, %d])", len(result), revision, startCommitPosition, endCommitPosition)
	return result, nil
}

// Verify sqlAnomaliesStore implements anomalies.Store.
var _ anomalies.Store = (*sqlAnomaliesStore)(nil)
