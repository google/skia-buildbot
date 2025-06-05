package anomalies_impl

import (
	"context"

	"hash/fnv"
	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/anomalies"
	"go.skia.org/infra/perf/go/chromeperf"
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

	regressionsMap, err := s.regStore.Range(ctx, types.CommitNumber(startCommitPosition), types.CommitNumber(endCommitPosition))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to load regressions from database")
	}

	targetTraceSet := make(map[string]bool, len(traceNames))
	for _, tn := range traceNames {
		targetTraceSet[tn] = true
	}

	for _, allRegressionsForCommit := range regressionsMap {
		for _, reg := range allRegressionsForCommit.ByAlertID {
			if reg.Frame == nil || reg.Frame.DataFrame == nil {
				continue
			}

			for traceKeyInFrame := range reg.Frame.DataFrame.TraceSet {
				if len(targetTraceSet) > 0 && !targetTraceSet[traceKeyInFrame] {
					continue
				}

				// In ChromePerf, the id is integer, but in the database, it is string.
				// Temporarily, hash the string id to get a deterministic integer id.
				// TODO(ansid): converge these two (by making id a string everywhere).
				h := fnv.New32a()
				h.Write([]byte(reg.Id))
				id := int(h.Sum32())

				anomaly := chromeperf.Anomaly{
					Id:                  id,
					TestPath:            traceKeyInFrame,
					StartRevision:       int(reg.PrevCommitNumber),
					EndRevision:         int(reg.CommitNumber),
					IsImprovement:       reg.IsImprovement,
					MedianBeforeAnomaly: float64(reg.MedianBefore),
					MedianAfterAnomaly:  float64(reg.MedianAfter),
				}
				sklog.Debugf("Constructed anomaly for trace %s: %+v", traceKeyInFrame, anomaly)

				if _, ok := result[traceKeyInFrame]; !ok {
					result[traceKeyInFrame] = chromeperf.CommitNumberAnomalyMap{}
				}
				result[traceKeyInFrame][reg.CommitNumber] = anomaly
			}
		}
	}
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

	return result, nil
}

// Verify sqlAnomaliesStore implements anomalies.Store.
var _ anomalies.Store = (*sqlAnomaliesStore)(nil)
