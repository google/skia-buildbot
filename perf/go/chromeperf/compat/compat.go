package compat

import (
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/regression"
)

// ConvertRegressionToAnomalies converts a regression.Regression to a chromeperf.AnomalyMap.
func ConvertRegressionToAnomalies(reg *regression.Regression) (chromeperf.AnomalyMap, error) {
	anomalyMap := chromeperf.AnomalyMap{}
	if reg.Frame == nil || reg.Frame.DataFrame == nil {
		return anomalyMap, nil
	}

	for key := range reg.Frame.DataFrame.TraceSet {
		testPath, err := chromeperf.TraceNameToTestPath(key, false)
		if err != nil {
			// If we fail to convert one trace, we continue with others.
			sklog.Errorf("Failed to get test path from trace name %s: %v", key, err)
			continue
		}
		anomaly := chromeperf.Anomaly{
			Id: reg.Id,
			// TODO(mordeckimarcin) add remaining fields that bug_id can come from.
			BugId:               int(reg.BugId),
			TestPath:            testPath,
			StartRevision:       int(reg.PrevCommitNumber),
			EndRevision:         int(reg.CommitNumber),
			IsImprovement:       reg.IsImprovement,
			MedianBeforeAnomaly: float64(reg.MedianBefore),
			MedianAfterAnomaly:  float64(reg.MedianAfter),
		}

		commitMap := chromeperf.CommitNumberAnomalyMap{
			reg.CommitNumber: anomaly,
		}
		anomalyMap[key] = commitMap
	}
	return anomalyMap, nil
}
