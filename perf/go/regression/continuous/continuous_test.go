package continuous

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/alerts"
	alertconfigmocks "go.skia.org/infra/perf/go/alerts/mock"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dataframe/mocks"
	gitmocks "go.skia.org/infra/perf/go/git/mocks"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/ingestevents"
	notifymocks "go.skia.org/infra/perf/go/notify/mocks"
	"go.skia.org/infra/perf/go/regression"
	regressionmocks "go.skia.org/infra/perf/go/regression/mocks"
	shortcutmocks "go.skia.org/infra/perf/go/shortcut/mocks"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
	"golang.org/x/exp/slices"
)

func TestBuildConfigsAndParamSet(t *testing.T) {
	mockConfigProvider := alertconfigmocks.NewConfigProvider(t)
	mockConfigProvider.On("GetAllAlertConfigs", testutils.AnyContext, false).Return(
		[]*alerts.Alert{
			{
				IDAsString: "1",
			},
			{
				IDAsString: "3",
			},
		}, nil)
	c := Continuous{
		provider: mockConfigProvider,
		paramsProvider: func() paramtools.ReadOnlyParamSet {
			return paramtools.ReadOnlyParamSet{
				"config": []string{"8888", "565"},
			}
		},
		pollingDelay: time.Nanosecond,
		instanceConfig: &config.InstanceConfig{
			DataStoreConfig: config.DataStoreConfig{},
			GitRepoConfig:   config.GitRepoConfig{},
			IngestionConfig: config.IngestionConfig{},
		},
		flags: &config.FrontendFlags{},
	}

	// Build channel.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := c.buildConfigAndParamsetChannel(ctx)

	// Read value.
	cnp := <-ch

	// Confirm it conforms to expectations.
	assert.Equal(t, c.paramsProvider(), cnp.paramset)
	assert.Len(t, cnp.configs, 2)
	ids := []string{}
	for _, cfg := range cnp.configs {
		ids = append(ids, cfg.IDAsString)
	}
	assert.Subset(t, []string{"1", "3"}, ids)

	// Confirm we continue to get items from the channel.
	cnp = <-ch
	assert.Equal(t, c.paramsProvider(), cnp.paramset)
}

func TestMatchingConfigsFromTraceIDs_TraceIDSliceIsEmpty_ReturnsEmptySlice(t *testing.T) {
	config := alerts.NewConfig()
	config.Query = "foo=bar"
	traceIDs := []string{}
	matchingConfigs := matchingConfigsFromTraceIDs(traceIDs, []*alerts.Alert{config})
	require.Empty(t, matchingConfigs)
}

func TestMatchingConfigsFromTraceIDs_OneConfigThatMatchesZeroTraces_ReturnsEmptySlice(t *testing.T) {
	config := alerts.NewConfig()
	config.Query = "arch=some-unknown-arch"
	traceIDs := []string{
		",arch=x86,config=8888,",
		",arch=arm,config=8888,",
	}
	matchingConfigs := matchingConfigsFromTraceIDs(traceIDs, []*alerts.Alert{config})
	require.Empty(t, matchingConfigs)
}

func TestMatchingConfigsFromTraceIDs_OneConfigThatMatchesOneTrace_ReturnsTheOneConfig(t *testing.T) {
	config := alerts.NewConfig()
	config.Query = "arch=x86"
	traceIDs := []string{
		",arch=x86,config=8888,",
		",arch=arm,config=8888,",
	}
	matchingConfigs := matchingConfigsFromTraceIDs(traceIDs, []*alerts.Alert{config})
	require.Len(t, matchingConfigs, 1)
}

func TestMatchingConfigsFromTraceIDs_TwoConfigsThatMatchesOneTrace_ReturnsBothConfigs(t *testing.T) {
	config1 := alerts.NewConfig()
	config1.Query = "arch=x86"
	config2 := alerts.NewConfig()
	config2.Query = "arch=arm"
	traceIDs := []string{
		",arch=x86,config=8888,",
		",arch=arm,config=8888,",
	}
	matchingConfigs := matchingConfigsFromTraceIDs(traceIDs, []*alerts.Alert{config1, config2})
	require.Len(t, matchingConfigs, 2)
}

func TestMatchingConfigsFromTraceIDs_GroupByMatchesTrace_ReturnsConfigWithRestrictedQuery(t *testing.T) {
	config1 := alerts.NewConfig()
	config1.Query = "arch=x86"
	config1.SetIDFromInt64(123)
	config1.GroupBy = "config"
	traceIDs := []string{
		",arch=x86,config=8888,",
		",arch=arm,config=8888,",
	}
	matchingConfigs := matchingConfigsFromTraceIDs(traceIDs, []*alerts.Alert{config1})
	require.Len(t, matchingConfigs, 1)

	for config, traces := range matchingConfigs {
		assert.Equal(t, config1.IDAsString, config.IDAsString)
		assert.Equal(t, config1.GroupBy, config.GroupBy)

		// Ensure that the query is updated inside the alert config
		assert.NotEqual(t, config1.Query, config.Query)
		assert.Equal(t, "arch=x86&config=8888", config.Query)

		assert.Equal(t, traceIDs[0], traces[0])
	}
}

func TestMatchingConfigsFromTraceIDs_MultipleGroupByPartsMatchTrace_ReturnsConfigWithRestrictedQueryUsingAllMatchingGroupByKeys(t *testing.T) {
	config := alerts.NewConfig()
	config.Query = "arch=x86"
	config.GroupBy = "config,device"
	traceIDs := []string{
		",arch=x86,config=8888,device=Pixel4,",
		",arch=arm,config=8888,device=Pixel4,",
	}
	matchingConfigs := matchingConfigsFromTraceIDs(traceIDs, []*alerts.Alert{config})
	require.Len(t, matchingConfigs, 1)
	for config := range matchingConfigs {
		assert.Equal(t, "arch=x86&config=8888&device=Pixel4", config.Query)
		_, err := url.ParseQuery(config.Query)
		require.NoError(t, err)
	}
}

type allMocks struct {
	perfGit          *gitmocks.Git
	shortcutStore    *shortcutmocks.Store
	regressionStore  *regressionmocks.Store
	notifier         *notifymocks.Notifier
	dataFrameBuilder *mocks.DataFrameBuilder
	configProvider   *alertconfigmocks.ConfigProvider
}

func createArgsForReportRegressions(t *testing.T) (*Continuous, *regression.RegressionDetectionRequest, []*regression.RegressionDetectionResponse, *alerts.Alert, allMocks) {
	pg := gitmocks.NewGit(t)
	ss := shortcutmocks.NewStore(t)
	rs := regressionmocks.NewStore(t)
	cp := alertconfigmocks.NewConfigProvider(t)
	n := notifymocks.NewNotifier(t)
	pp := func() paramtools.ReadOnlyParamSet {
		return nil
	}
	dfb := mocks.NewDataFrameBuilder(t)
	i := &config.InstanceConfig{}
	f := &config.FrontendFlags{}

	req := &regression.RegressionDetectionRequest{}
	resp := []*regression.RegressionDetectionResponse{}
	cfg := &alerts.Alert{}

	c := &Continuous{
		perfGit:           pg,
		shortcutStore:     ss,
		store:             rs,
		provider:          cp,
		notifier:          n,
		paramsProvider:    pp,
		dfBuilder:         dfb,
		pollingDelay:      time.Microsecond,
		instanceConfig:    i,
		flags:             f,
		current:           &alerts.Alert{},
		regressionCounter: metrics2.GetCounter("continuous_regression_found"),
	}

	allMocks := allMocks{
		perfGit:          pg,
		shortcutStore:    ss,
		regressionStore:  rs,
		notifier:         n,
		dataFrameBuilder: dfb,
		configProvider:   cp,
	}

	return c, req, resp, cfg, allMocks

}

func TestReportRegressions_EmptyRegressionDetectionResponse_NoRegressionsReported(t *testing.T) {
	c, req, resp, cfg, _ := createArgsForReportRegressions(t)
	// We know this works since we didn't need to supply any implementations for any of the mocks.
	c.reportRegressions(context.Background(), req, resp, cfg)
}

func TestReportRegressions_OneNewStepDownRegressionFound_OneRegressionStoredAndNotified(t *testing.T) {
	ctx := context.Background()
	c, req, resp, cfg, allMocks := createArgsForReportRegressions(t)

	const regressionCommitNumber = types.CommitNumber(2)
	resp = append(resp, &regression.RegressionDetectionResponse{
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				Header: []*dataframe.ColumnHeader{
					{Offset: 1},
					{Offset: regressionCommitNumber},
				},
				ParamSet: paramtools.ReadOnlyParamSet{
					"device_name": []string{"sailfish", "sargo", "wembley"},
				},
			},
		},
		Summary: &clustering2.ClusterSummaries{
			Clusters: []*clustering2.ClusterSummary{
				{
					Keys: []string{
						",device_name=sailfish",
						",device_name=sargo",
						",device_name=wembley",
					},
					Shortcut: "some-shortcut-id",
					StepFit: &stepfit.StepFit{
						Status: stepfit.LOW,
					},
					StepPoint: &dataframe.ColumnHeader{
						Offset: regressionCommitNumber,
					},
				},
			},
		},
	})

	commitAtStep := provider.Commit{
		Subject: "The subject of the commit where a regression occurred.",
	}
	previousCommit := provider.Commit{
		Subject: "The subject of the commit right before where a regression occurred.",
	}

	const notificationID = "some-notification-id"

	// First call to CommitFromCommitNumber.
	allMocks.perfGit.On("CommitFromCommitNumber", testutils.AnyContext, types.CommitNumber(2)).Return(commitAtStep, nil)

	// First call to CommitFromCommitNumber is for the previous commit.
	allMocks.perfGit.On("CommitFromCommitNumber", testutils.AnyContext, types.CommitNumber(1)).Return(previousCommit, nil)
	cfg.DirectionAsString = alerts.DOWN

	// Returns true to indicate that this is a newly found regression. Note that
	// this is called twice, first to store the regression since it's new, then
	// called again to store the notification ID.
	allMocks.regressionStore.On("GetRegression", testutils.AnyContext, regressionCommitNumber, cfg.IDAsString).Return(nil, nil)
	allMocks.regressionStore.On("SetLow", testutils.AnyContext, regressionCommitNumber, cfg.IDAsString, resp[0].Frame, resp[0].Summary.Clusters[0]).Return(true, "", nil).Twice()
	allMocks.notifier.On("RegressionFound", testutils.AnyContext, commitAtStep, previousCommit, cfg, resp[0].Summary.Clusters[0], resp[0].Frame, mock.Anything).Return(notificationID, nil)

	c.reportRegressions(ctx, req, resp, cfg)

	require.Equal(t, notificationID, resp[0].Summary.Clusters[0].NotificationID)
}

func TestTraceIdForIngestEvent_Matching(t *testing.T) {
	c, _, _, _, allMocks := createArgsForReportRegressions(t)

	allConfigs := []*alerts.Alert{
		{
			IDAsString: "1",
			Query:      "&id=trace1&id=trace3",
		},
		{
			IDAsString: "3",
			Query:      "&id=trace3",
		},
	}
	allMocks.configProvider.On("GetAllAlertConfigs", testutils.AnyContext, false).Return(
		allConfigs, nil)
	ctx := context.Background()
	ie := &ingestevents.IngestEvent{
		TraceIDs: []string{",id=trace1,", ",id=trace2,"},
	}
	configTracesMap, err := c.getTraceIdConfigsForIngestEvent(ctx, ie)
	assert.Nil(t, err)
	assert.NotNil(t, configTracesMap)
	for _, traces := range configTracesMap {
		assert.Equal(t, ie.TraceIDs[0], traces[0], "Expect the first config to match first trace.")
		assert.False(t, slices.Contains(traces, ie.TraceIDs[1]), "No match expected for second trace.")
	}

}

func TestTraceIdForIngestEvent_MultipleConfigs_Matching(t *testing.T) {
	c, _, _, _, allMocks := createArgsForReportRegressions(t)

	allConfigs := []*alerts.Alert{
		{
			IDAsString: "1",
			Query:      "&id=trace1&id=trace3",
		},
		{
			IDAsString: "3",
			Query:      "&id=trace3",
		},
	}
	allMocks.configProvider.On("GetAllAlertConfigs", testutils.AnyContext, false).Return(
		allConfigs, nil)
	ctx := context.Background()
	ie := &ingestevents.IngestEvent{
		TraceIDs: []string{",id=trace3,"},
	}
	configTracesMap, err := c.getTraceIdConfigsForIngestEvent(ctx, ie)
	assert.Nil(t, err)
	assert.NotNil(t, configTracesMap)
	assert.Equal(t, 2, len(configTracesMap))
}

func TestReportRegressions_OneNewStepDownRegressionFound_OneHighRegressionFoundAndNotified(t *testing.T) {
	ctx := context.Background()
	c, req, resp, cfg, allMocks := createArgsForReportRegressions(t)

	const regressionCommitNumber = types.CommitNumber(2)
	resp = append(resp, &regression.RegressionDetectionResponse{
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				Header: []*dataframe.ColumnHeader{
					{Offset: 1},
					{Offset: regressionCommitNumber},
				},
				ParamSet: paramtools.ReadOnlyParamSet{
					"device_name": []string{"sailfish", "sargo", "wembley"},
				},
			},
		},
		Summary: &clustering2.ClusterSummaries{
			Clusters: []*clustering2.ClusterSummary{
				{
					Keys: []string{
						",device_name=sailfish",
						",device_name=sargo",
						",device_name=wembley",
					},
					Shortcut: "some-shortcut-id",
					StepFit: &stepfit.StepFit{
						Status: stepfit.LOW,
					},
					StepPoint: &dataframe.ColumnHeader{
						Offset: regressionCommitNumber,
					},
				},
			},
		},
	}, &regression.RegressionDetectionResponse{
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				Header: []*dataframe.ColumnHeader{
					{Offset: 1},
					{Offset: regressionCommitNumber},
				},
				ParamSet: paramtools.ReadOnlyParamSet{
					"device_name": []string{"sailfish", "sargo", "wembley"},
				},
			},
		},
		Summary: &clustering2.ClusterSummaries{
			Clusters: []*clustering2.ClusterSummary{
				{
					Keys: []string{
						",device_name=sailfish",
						",device_name=sargo",
						",device_name=wembley",
					},
					Shortcut: "some-shortcut-id2",
					StepFit: &stepfit.StepFit{
						Status: stepfit.HIGH,
					},
					StepPoint: &dataframe.ColumnHeader{
						Offset: regressionCommitNumber,
					},
				},
			},
		},
	})

	commitAtStep := provider.Commit{
		Subject: "The subject of the commit where a regression occurred.",
	}
	previousCommit := provider.Commit{
		Subject: "The subject of the commit right before where a regression occurred.",
	}

	const notificationID = "some-notification-id"

	// First call to CommitFromCommitNumber.
	allMocks.perfGit.On("CommitFromCommitNumber", testutils.AnyContext, types.CommitNumber(2)).Return(commitAtStep, nil)

	// First call to CommitFromCommitNumber is for the previous commit.
	allMocks.perfGit.On("CommitFromCommitNumber", testutils.AnyContext, types.CommitNumber(1)).Return(previousCommit, nil)
	cfg.DirectionAsString = alerts.BOTH

	// Returns true to indicate that this is a newly found regression. Note that
	// this is called twice, first to store the regression since it's new, then
	// called again to store the notification ID.
	allMocks.regressionStore.On("GetRegression", testutils.AnyContext, regressionCommitNumber, cfg.IDAsString).Return(nil, nil).Once()
	allMocks.regressionStore.On("SetLow", testutils.AnyContext, regressionCommitNumber, cfg.IDAsString, resp[0].Frame, resp[0].Summary.Clusters[0]).Return(true, "", nil).Twice()
	allMocks.notifier.On("RegressionFound", testutils.AnyContext, commitAtStep, previousCommit, cfg, resp[0].Summary.Clusters[0], resp[0].Frame, mock.Anything).Return(notificationID, nil)

	allMocks.regressionStore.On("GetRegression", testutils.AnyContext, regressionCommitNumber, cfg.IDAsString).Return(nil, nil).Once()
	allMocks.regressionStore.On("SetHigh", testutils.AnyContext, regressionCommitNumber, cfg.IDAsString, resp[1].Frame, resp[1].Summary.Clusters[0]).Return(false, "", nil)
	allMocks.notifier.On("RegressionFound", testutils.AnyContext, commitAtStep, previousCommit, cfg, resp[0].Summary.Clusters[0], resp[0].Frame, mock.Anything).Return(notificationID, nil)

	c.reportRegressions(ctx, req, resp, cfg)

	require.Equal(t, notificationID, resp[0].Summary.Clusters[0].NotificationID)
}

func TestGetQueryWithDefaultsIfNeeded(t *testing.T) {
	// Backup original config.
	originalConfig := config.Config
	defer func() { config.Config = originalConfig }()

	// Test case 1: With defaults.
	config.Config = &config.InstanceConfig{
		QueryConfig: config.QueryConfig{
			DefaultParamSelections: map[string][]string{
				"stat": {"value"},
			},
		},
	}
	q, err := getQueryWithDefaultsIfNeeded("test=1")
	require.NoError(t, err)
	assert.Equal(t, "stat=[value]test=[1]", q.KeyValueString())

	// Test case 2: Without defaults.
	config.Config = &config.InstanceConfig{
		QueryConfig: config.QueryConfig{
			DefaultParamSelections: nil,
		},
	}
	q, err = getQueryWithDefaultsIfNeeded("test=1")
	require.NoError(t, err)
	assert.Equal(t, "test=[1]", q.KeyValueString())

	// Test case 3: Query containing some params that are present in default.
	config.Config = &config.InstanceConfig{
		QueryConfig: config.QueryConfig{
			DefaultParamSelections: map[string][]string{
				"stat": {"value"},
				"type": {"canvas"},
			},
		},
	}
	q, err = getQueryWithDefaultsIfNeeded("test=1&stat=other")
	require.NoError(t, err)
	assert.Equal(t, "stat=[other]test=[1]type=[canvas]", q.KeyValueString())
}
