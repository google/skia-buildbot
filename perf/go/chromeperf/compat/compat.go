package compat

import (
	"strconv"

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
			Id:                  reg.Id,
			TestPath:            testPath,
			StartRevision:       int(reg.PrevCommitNumber),
			EndRevision:         int(reg.CommitNumber),
			IsImprovement:       reg.IsImprovement,
			MedianBeforeAnomaly: float64(reg.MedianBefore),
			MedianAfterAnomaly:  float64(reg.MedianAfter),
			Bugs:                reg.Bugs,
		}

		_, _, triageStatus := reg.GetClusterTypeAndSummaryAndTriageStatus()
		anomaly.State = string(triageStatus.Status)
		if triageStatus.Status == regression.Ignored {
			anomaly.BugId = chromeperf.IgnoreBugIDFlag
		}

		arbitraryBugIdSelectedWarningDisplayed := false
		// TODO(b/462782068) change anomalymap to contain all bug ids.
		// This is a temporary logic.
		if len(reg.Bugs) > 0 {
			anomaly.BugId, err = strconv.Atoi(reg.Bugs[0].BugId)
			if err != nil {
				// Again, if one conversion fails, we continue with others.
				sklog.Errorf("Failed to convert bug id from %s", reg.Bugs[0].BugId)
			}
			// Let's not display this warning too often.
			if len(reg.Bugs) > 1 && !arbitraryBugIdSelectedWarningDisplayed {
				arbitraryBugIdSelectedWarningDisplayed = true
				sklog.Warningf("Some regression has %d bug ids to choose from, we selected the first one from the list.", len(reg.Bugs))
				sklog.Warningf("Showing up to 5 first bugs from the list:")
				maxCount := len(reg.Bugs)
				if maxCount > 5 {
					maxCount = 5
				}
				for i := range maxCount {
					sklog.Warningf("bug %d out of %d has id %s and is of type %s", i, len(reg.Bugs), reg.Bugs[i].BugId, reg.Bugs[i].Type)
				}
			}
		}

		commitMap := chromeperf.CommitNumberAnomalyMap{
			reg.CommitNumber: anomaly,
		}
		anomalyMap[key] = commitMap
	}
	return anomalyMap, nil
}
