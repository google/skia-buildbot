package refiner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/alerts"
	alerts_mock "go.skia.org/infra/perf/go/alerts/mock"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/sqlregression2store"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

type performanceMockTraceStore struct {
	tracestore.TraceStore
}

func (m *performanceMockTraceStore) ReadTracesForCommitRange(ctx context.Context, keys []string, begin types.CommitNumber, end types.CommitNumber) (types.TraceSet, []provider.Commit, map[string]*types.TraceSourceInfo, error) {
	traceSet := types.TraceSet{}
	var commits []provider.Commit
	for _, key := range keys {
		length := int(end - begin + 1)
		trace := make(types.Trace, length)
		for i := 0; i < length; i++ {
			trace[i] = 10.0 // steady baseline value
		}
		traceSet[key] = trace
	}

	length := int(end - begin + 1)
	commits = make([]provider.Commit, length)
	for i := 0; i < length; i++ {
		commits[i] = provider.Commit{
			CommitNumber: begin + types.CommitNumber(i),
		}
	}
	return traceSet, commits, nil, nil
}

func (m *performanceMockTraceStore) ReadTracesForCommitRanges(ctx context.Context, requests map[string]tracestore.TraceRangeRequest) (map[string]types.Trace, map[string][]types.CommitNumber, error) {
	traces := map[string]types.Trace{}
	commits := map[string][]types.CommitNumber{}
	for traceName, req := range requests {
		length := int(req.EndCommit - req.BeginCommit + 1)
		trace := make(types.Trace, length)
		for i := 0; i < length; i++ {
			trace[i] = 10.0 // steady baseline value
		}
		traces[traceName] = trace

		commitList := make([]types.CommitNumber, length)
		for i := 0; i < length; i++ {
			commitList[i] = req.BeginCommit + types.CommitNumber(i)
		}
		commits[traceName] = commitList
	}
	return traces, commits, nil
}

func generateRegressionForTrace(traceName string, commit types.CommitNumber, prevCommit types.CommitNumber) *regression.Regression {
	r := regression.NewRegression()
	r.Id = uuid.NewString()
	r.CommitNumber = commit
	r.PrevCommitNumber = prevCommit
	r.DisplayCommitNumber = commit
	r.AlertId = 1111
	r.SubscriptionName = "test-sub"
	r.CreationTime = time.Now()
	r.IsImprovement = false
	r.MedianBefore = 10.0
	r.MedianAfter = 20.0

	r.Frame = &frame.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			Header: []*dataframe.ColumnHeader{
				{Offset: prevCommit},
				{Offset: prevCommit + 2},
				{Offset: prevCommit + 5},
				{Offset: commit},
			},
			TraceSet: types.TraceSet{traceName: {}},
		},
	}

	r.High = &clustering2.ClusterSummary{
		StepFit: &stepfit.StepFit{
			TurningPoint: 2,
			Status:       stepfit.HIGH,
		},
		Timestamp: time.Now(),
		Centroid:  []float32{10.0, 10.0, 20.0, 20.0},
		Keys:      []string{traceName},
		StepPoint: &dataframe.ColumnHeader{Offset: commit},
	}
	r.HighStatus = regression.TriageStatus{
		Status: regression.Untriaged,
	}
	return r
}

func batchInsertRegressions(ctx context.Context, t *testing.T, db pool.Pool, regs []*regression.Regression) error {
	const insertQueryHeader = `
		INSERT INTO Regressions2 (
			id, commit_number, prev_commit_number, display_commit_number,
			alert_id, sub_name, bug_id, creation_time, median_before,
			median_after, is_improvement, cluster_type, cluster_summary,
			frame, legacy_key, trace_id, triage_status, triage_message
		) VALUES `

	const numCols = 18

	batchSize := 50
	for i := 0; i < len(regs); i += batchSize {
		end := i + batchSize
		if end > len(regs) {
			end = len(regs)
		}
		batch := regs[i:end]

		var placeholders []string
		var args []interface{}
		paramIdx := 1

		for _, r := range batch {
			var rowPlaceholders []string
			for col := 0; col < numCols; col++ {
				rowPlaceholders = append(rowPlaceholders, fmt.Sprintf("$%d", paramIdx))
				paramIdx++
			}
			placeholders = append(placeholders, "("+strings.Join(rowPlaceholders, ", ")+")")

			clusterType, clusterSummary, triage := r.GetClusterTypeAndSummaryAndTriageStatus()

			var manualTriageBugId *int64
			for _, b := range r.Bugs {
				if b.Type == types.ManualTriage {
					bug, err := strconv.Atoi(b.BugId)
					if err == nil {
						val := int64(bug)
						manualTriageBugId = &val
						break
					}
				}
			}

			clusterSummaryJSON, err := json.Marshal(clusterSummary)
			require.NoError(t, err)
			frameJSON, err := json.Marshal(r.Frame)
			require.NoError(t, err)

			traceName := ""
			for k := range r.Frame.DataFrame.TraceSet {
				traceName = k
				break
			}
			tid := types.TraceIDForSQLInBytesFromTraceName(traceName)
			traceIdBytes := tid[:]

			args = append(args,
				r.Id,
				int64(r.CommitNumber),
				int64(r.PrevCommitNumber),
				int64(r.DisplayCommitNumber),
				r.AlertId,
				r.SubscriptionName,
				manualTriageBugId,
				r.CreationTime,
				r.MedianBefore,
				r.MedianAfter,
				r.IsImprovement,
				string(clusterType),
				clusterSummaryJSON,
				frameJSON,
				r.LegacyKey,
				traceIdBytes,
				triage.Status,
				triage.Message,
			)
		}

		query := insertQueryHeader + strings.Join(placeholders, ", ") + `
			ON CONFLICT (id) DO UPDATE SET
				median_before=EXCLUDED.median_before,
				median_after=EXCLUDED.median_after,
				frame=EXCLUDED.frame,
				triage_status=EXCLUDED.triage_status,
				triage_message=EXCLUDED.triage_message,
				alert_id=EXCLUDED.alert_id,
				bug_id=EXCLUDED.bug_id,
				cluster_summary=EXCLUDED.cluster_summary,
				cluster_type=EXCLUDED.cluster_type,
				commit_number=EXCLUDED.commit_number,
				creation_time=EXCLUDED.creation_time,
				is_improvement=EXCLUDED.is_improvement,
				prev_commit_number=EXCLUDED.prev_commit_number,
				sub_name=EXCLUDED.sub_name,
				trace_id=EXCLUDED.trace_id,
				display_commit_number=EXCLUDED.display_commit_number,
				legacy_key=EXCLUDED.legacy_key`

		_, err := db.Exec(ctx, query, args...)
		if err != nil {
			return err
		}
	}
	return nil
}

func TestProcess_Performance(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize real Spanner Emulator database.
	db := sqltest.NewSpannerDBForTests(t, "refiner_integration")

	// 2. Set up alert config.
	alert := &alerts.Alert{
		IDAsString:       "11111",
		DisplayName:      "Test Alert",
		Algo:             types.StepFitGrouping,
		Step:             types.CohenStep,
		Interesting:      1.0,
		Radius:           10,
		SubscriptionName: "test-sub",
	}

	alertsProvider := alerts_mock.NewConfigProvider(t)
	alertsProvider.On("GetAlertConfig", int64(1111)).Return(alert, nil).Maybe()

	// 3. Initialize real sqlregression2store.
	instanceConfig := &config.InstanceConfig{
		AllowMultipleRegressionsPerAlertId: true,
		Experiments:                        config.Experiments{RegressionsTraceIdField: true},
	}
	regStore, err := sqlregression2store.New(db, alertsProvider, instanceConfig)
	require.NoError(t, err)

	// 4. Generate 4440 regressions evenly distributed across 37 traces.
	// 37 traces, 120 regressions per trace.
	numTraces := 37
	regressionsPerTrace := 120
	allRegressions := make([]*regression.Regression, 0, numTraces*regressionsPerTrace)

	for i := 0; i < numTraces; i++ {
		traceName := fmt.Sprintf("trace_%d", i)
		for j := 1; j <= regressionsPerTrace; j++ {
			// e.g. commit 10, 20, 30...
			commit := types.CommitNumber(j * 10)
			prevCommit := types.CommitNumber((j - 1) * 10)
			reg := generateRegressionForTrace(traceName, commit, prevCommit)
			allRegressions = append(allRegressions, reg)
		}
	}

	// 5. Batch write the regressions using custom batch insert.
	t.Logf("Writing %d regressions to database...", len(allRegressions))
	writeStart := time.Now()
	err = batchInsertRegressions(ctx, t, db, allRegressions)
	require.NoError(t, err)
	t.Logf("Database setup finished in %s", time.Since(writeStart))

	// 6. Instantiate ImprovedAnomalyBoundsRefiner with mocked TraceStore.
	refiner := NewImprovedAnomalyBoundsRefiner(nil, regStore, &performanceMockTraceStore{}, nil, 0.001, false)

	// 7. Prepare candidate responses input containing all regressions.
	responses := make([]*regression.RegressionDetectionResponse, len(allRegressions))
	for i, reg := range allRegressions {
		responses[i] = &regression.RegressionDetectionResponse{
			Summary: &clustering2.ClusterSummaries{
				Clusters: []*clustering2.ClusterSummary{reg.High},
			},
			Frame:     reg.Frame,
			TraceName: reg.High.Keys[0],
		}
	}

	// 8. Execute and measure the performance of Process function.
	t.Log("Running Process function...")
	processStart := time.Now()
	confirmed, err := refiner.Process(ctx, alert, responses)
	processDuration := time.Since(processStart)

	require.NoError(t, err)
	t.Logf("Process execution took: %s", processDuration)
	assert.NotEmpty(t, confirmed)

	// Verify that Process didn't take an excessive amount of time.
	// 15 seconds is a conservative upper bound for processed candidates.
	assert.Less(t, processDuration, 15*time.Second)
}
