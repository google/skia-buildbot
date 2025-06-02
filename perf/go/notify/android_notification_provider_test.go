package notify

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/notify/common"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
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

func TestProvider_Android2Config_Success(t *testing.T) {
	// Read the value from the checked in config for android2 instance.
	android2Config := filepath.Join("..", "..", "configs", "spanner", "android2.json")
	var cfg config.InstanceConfig
	err := util.WithReadFile(android2Config, func(r io.Reader) error {
		b, err := io.ReadAll(r)
		if err != nil {
			return skerr.Wrapf(err, "failed to read bytes")
		}

		err = json.Unmarshal(b, &cfg)
		if err != nil {
			return skerr.Wrapf(err, "failed to unmarshal json.")
		}
		return nil
	})
	require.NoError(t, err)
	prov, err := NewAndroidNotificationDataProvider("", &cfg.NotifyConfig)
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
			StepFit: &stepfit.StepFit{
				Status: stepfit.HIGH,
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
	assert.Contains(t, notificationData.Body, "[CLs in range](https://android-build.corp.google.com/range_search/cls/from_id/12345/to_id/67890/?s=menu&includeTo=0&includeFrom=1)")
	require.NoError(t, err)
	require.NotNil(t, notificationData)
}

func TestProvider_Android2Config_Format(t *testing.T) {
	// Read the value from the checked in config for android2 instance.
	android2Config := filepath.Join("..", "..", "configs", "spanner", "android2.json")
	var cfg config.InstanceConfig
	err := util.WithReadFile(android2Config, func(r io.Reader) error {
		b, err := io.ReadAll(r)
		if err != nil {
			return skerr.Wrapf(err, "failed to read bytes")
		}

		err = json.Unmarshal(b, &cfg)
		if err != nil {
			return skerr.Wrapf(err, "failed to unmarshal json.")
		}
		return nil
	})
	require.NoError(t, err)
	prov, err := NewAndroidNotificationDataProvider("", &cfg.NotifyConfig)
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
			StepFit: &stepfit.StepFit{
				Status: stepfit.HIGH,
			},
		},
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				ParamSet: paramtools.ReadOnlyParamSet{
					"k1": []string{"v1"},
				},
				TraceSet: types.TraceSet{
					",os_version=d,device_name=c,test_method=b,test_class=a,": []float32{3.0, 3.1},
				},
			},
		},
		RegressionCommit: provider.Commit{CommitNumber: 1},
		PreviousCommit:   provider.Commit{CommitNumber: 0},
		AlertConfig:      &alerts.Alert{},
	}
	notificationData, err := prov.GetNotificationDataRegressionFound(context.Background(), metadata)
	assert.Contains(t, notificationData.Body, "a#b (c d)")
	require.NoError(t, err)
	require.NotNil(t, notificationData)
}

func TestProvider_Android2Config_MultipleTraces_Format(t *testing.T) {
	android2Config := filepath.Join("..", "..", "configs", "spanner", "android2.json")
	var cfg config.InstanceConfig
	err := util.WithReadFile(android2Config, func(r io.Reader) error {
		b, err := io.ReadAll(r)
		if err != nil {
			return skerr.Wrapf(err, "failed to read bytes")
		}

		err = json.Unmarshal(b, &cfg)
		if err != nil {
			return skerr.Wrapf(err, "failed to unmarshal json.")
		}
		return nil
	})
	require.NoError(t, err)

	prov, err := NewAndroidNotificationDataProvider("", &cfg.NotifyConfig)
	require.NoError(t, err)
	require.NotNil(t, prov)

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
			StepFit: &stepfit.StepFit{
				Status: stepfit.HIGH,
			},
		},
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				ParamSet: paramtools.ReadOnlyParamSet{
					"k1":          []string{"v1"},
					"os_version":  []string{"d", "e"},
					"device_name": []string{"c", "f"},
					"test_method": []string{"b", "g"},
					"test_class":  []string{"a", "h"},
				},
				TraceSet: types.TraceSet{
					",os_version=d,device_name=c,test_method=b,test_class=a,": []float32{3.0, 3.1},
					",os_version=e,device_name=f,test_method=g,test_class=h,": []float32{4.0, 4.2},
					",os_version=d,device_name=f,test_method=b,test_class=h,": []float32{5.0, 5.5},
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

	assert.Contains(t, notificationData.Body, "a#b (c d)")
	assert.Contains(t, notificationData.Body, "h#g (f e)")
	assert.Contains(t, notificationData.Body, "h#b (f d)")
}
