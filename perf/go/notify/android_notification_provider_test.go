package notify

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/notify/common"
	"go.skia.org/infra/perf/go/ui/frame"
)

func TestProvider_RegressionFound_HappyPath(t *testing.T) {
	cfg := config.NotifyConfig{
		Body: []string{
			`Link: {{index .RegressionCommitLinks "link"}}`,
		},
		Subject: "Regression found at commit {{ .RegressionCommit.CommitNumber }}",
	}

	prov, err := NewAndroidNotificationDataProvider("", &cfg)
	require.NoError(t, err)
	metadata := common.RegressionMetadata{
		RegressionCommitLinks: map[string]string{
			"link": "http://google.com",
		},
		Cl: &clustering2.ClusterSummary{
			Keys:     []string{"k1=v1", "k2=v2"},
			Shortcut: "shortcut1",
			StepPoint: &dataframe.ColumnHeader{
				Offset: 3,
			},
		},
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				ParamSet: paramtools.ReadOnlyParamSet{
					"k1": []string{"v1"},
				},
			},
		},
		RegressionCommit: provider.Commit{CommitNumber: 1},
		PreviousCommit:   provider.Commit{CommitNumber: 0},
		AlertConfig:      &alerts.Alert{},
	}
	notificationData, err := prov.GetNotificationDataRegressionFound(context.Background(), metadata)
	require.NoError(t, err)
	require.NotNil(t, notificationData)
	assert.Equal(t, "Link: http://google.com", notificationData.Body)
}

func TestProvider_RegressionMissing_HappyPath(t *testing.T) {
	cfg := config.NotifyConfig{
		MissingBody: []string{
			`Link: {{index .RegressionCommitLinks "link"}}`,
		},
		MissingSubject: "Regression found at commit {{ .RegressionCommit.CommitNumber }}",
	}

	prov, err := NewAndroidNotificationDataProvider("", &cfg)
	require.NoError(t, err)
	metadata := common.RegressionMetadata{
		RegressionCommitLinks: map[string]string{
			"link": "http://google.com",
		},
		Cl: &clustering2.ClusterSummary{
			Keys:     []string{"k1=v1", "k2=v2"},
			Shortcut: "shortcut1",
			StepPoint: &dataframe.ColumnHeader{
				Offset: 3,
			},
		},
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				ParamSet: paramtools.ReadOnlyParamSet{
					"k1": []string{"v1"},
				},
			},
		},
		RegressionCommit: provider.Commit{CommitNumber: 1},
		PreviousCommit:   provider.Commit{CommitNumber: 0},
		AlertConfig:      &alerts.Alert{},
	}
	notificationData, err := prov.GetNotificationDataRegressionMissing(context.Background(), metadata)
	require.NoError(t, err)
	require.NotNil(t, notificationData)
	assert.Equal(t, "Link: http://google.com", notificationData.Body)
}

func TestProvider_RegressionFound_BuildUrlDiff(t *testing.T) {
	cfg := config.NotifyConfig{
		Body: []string{
			`Link: {{ .GetBuildIdUrlDiff }}`,
		},
		Subject: "Regression found at commit {{ .RegressionCommit.CommitNumber }}",
	}

	prov, err := NewAndroidNotificationDataProvider("", &cfg)
	require.NoError(t, err)
	metadata := common.RegressionMetadata{
		RegressionCommitLinks: map[string]string{
			"Build ID": "12345",
		},
		PreviousCommitLinks: map[string]string{
			"Build ID": "67890",
		},
		Cl: &clustering2.ClusterSummary{
			Keys:     []string{"k1=v1", "k2=v2"},
			Shortcut: "shortcut1",
			StepPoint: &dataframe.ColumnHeader{
				Offset: 3,
			},
		},
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				ParamSet: paramtools.ReadOnlyParamSet{
					"k1": []string{"v1"},
				},
			},
		},
		RegressionCommit: provider.Commit{CommitNumber: 1},
		PreviousCommit:   provider.Commit{CommitNumber: 0},
		AlertConfig:      &alerts.Alert{},
	}
	notificationData, err := prov.GetNotificationDataRegressionFound(context.Background(), metadata)
	require.NoError(t, err)
	require.NotNil(t, notificationData)
	assert.Equal(t, "Link: https://android-build.corp.google.com/range_search/cls/from_id/12345/to_id/67890/?s=menu&includeTo=0&includeFrom=1", notificationData.Body)
}
