package validate

import (
	"context"
	"encoding/json"
	"io"
	"regexp"
	"time"

	_ "embed" // For embed functionality.

	"go.skia.org/infra/go/jsonschema"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/notify"
	"go.skia.org/infra/perf/go/notifytypes"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

// schema is a json schema for InstanceConfig, it is created by
// running go generate on ./generate/main.go.
//
//go:embed instanceConfigSchema.json
var schema []byte

// InstanceConfigFromFile returns the deserialized JSON of an InstanceConfig
// found in filename.
//
// If there was an error loading the file a list of schema violations may be
// returned also.
func InstanceConfigFromFile(filename string) (*config.InstanceConfig, []string, error) {
	var instanceConfig config.InstanceConfig
	var schemaViolations []string = nil

	// Validate config here.
	err := util.WithReadFile(filename, func(r io.Reader) error {
		b, err := io.ReadAll(r)
		if err != nil {
			return skerr.Wrapf(err, "failed to read bytes")
		}
		schemaViolations, err = jsonschema.Validate(b, schema)
		if err != nil {
			return skerr.Wrapf(err, "file does not conform to schema")
		}
		return json.Unmarshal(b, &instanceConfig)
	})
	if err != nil {
		return nil, schemaViolations, skerr.Wrapf(err, "Filename: %s", filename)
	}
	err = Validate(instanceConfig)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Filename: %s", filename)
	}
	return &instanceConfig, nil, nil
}

// Validate the config.
func Validate(i config.InstanceConfig) error {
	if i.NotifyConfig.Notifications == notifytypes.MarkdownIssueTracker {
		if i.NotifyConfig.IssueTrackerAPIKeySecretProject == "" {
			return skerr.Fmt("issue_tracker_api_key_secret_project must be supplied when `notifications` is set to %q", i.NotifyConfig.Notifications)
		}
		if i.NotifyConfig.IssueTrackerAPIKeySecretName == "" {
			return skerr.Fmt("issue_tracker_api_key_secret_name must be supplied when `notifications` is set to %q", i.NotifyConfig.Notifications)
		}
	}

	if i.CulpritNotifyConfig.NotificationType == types.IssueNotify {
		if i.CulpritNotifyConfig.IssueTrackerAPIKeySecretProject == "" {
			return skerr.Fmt("issue_tracker_api_key_secret_project must be supplied when `notifications` is set to %q", i.CulpritNotifyConfig.NotificationType)
		}
		if i.CulpritNotifyConfig.IssueTrackerAPIKeySecretName == "" {
			return skerr.Fmt("issue_tracker_api_key_secret_name must be supplied when `notifications` is set to %q", i.CulpritNotifyConfig.NotificationType)
		}
	}

	if i.InvalidParamCharRegex != "" {
		re, err := regexp.Compile(i.InvalidParamCharRegex)
		if err != nil {
			return skerr.Wrapf(err, "compiling invalid_param_char_regex: %q", i.InvalidParamCharRegex)
		}
		if !re.MatchString(",") {
			return skerr.Fmt("invalid_param_char_regex must match ',' (comma).")
		}
		if !re.MatchString("=") {
			return skerr.Fmt("invalid_param_char_regex must match '=' (equals).")
		}
	}

	// Validate the Notify Config.
	if i.NotifyConfig.Notifications == notifytypes.MarkdownIssueTracker && (len(i.NotifyConfig.Body) > 0 || i.NotifyConfig.Subject != "" || len(i.NotifyConfig.MissingBody) > 0 || i.NotifyConfig.MissingSubject != "") {
		f, err := notify.NewMarkdownFormatter("", &(i.NotifyConfig))
		if err != nil {
			return skerr.Wrapf(err, "creating MarkdownFormatter")
		}

		// Validate the templates in the config by passing in valid data and
		// confirming the templates expand w/o error.
		ctx := context.Background()
		commit := provider.Commit{
			CommitNumber: 100,
			GitHash:      "0f9e50daa87997d376bf5fb60b06ab5b15c63ed9",
			Timestamp:    1693173794,
			Author:       "author@example.com",
			Subject:      "Fix a bug.",
			URL:          "https://skia.googlesource.com/skia/+/0f9e50daa87997d376bf5fb60b06ab5b15c63ed9",
		}
		prevCommit := provider.Commit{
			CommitNumber: 101,
			GitHash:      "26c9cb69e1211bb47dc3a03af69d7d0796557253",
			Timestamp:    1693170794,
			Author:       "author2@example.com",
			Subject:      "Fix another bug.",
			URL:          "https://skia.googlesource.com/skia/+/26c9cb69e1211bb47dc3a03af69d7d0796557253",
		}
		alert := &alerts.Alert{
			DisplayName:           "My Alert",
			Query:                 "test=foo",
			IssueTrackerComponent: 1234567890,
			Algo:                  types.KMeansGrouping,
			Step:                  types.MannWhitneyU,
			StepUpOnly:            true,
			DirectionAsString:     alerts.BOTH,
			Radius:                7,
			K:                     0,
			GroupBy:               "",
			Sparse:                true,
			MinimumNum:            3,
			Category:              "Prod",
		}
		clusterSummary := &clustering2.ClusterSummary{
			Keys:     []string{",test=foo,"},
			Shortcut: "12343212123123",
			ParamSummaries: []clustering2.ValuePercent{
				{
					Value:   "test",
					Percent: 90,
				},
			},
			StepFit: &stepfit.StepFit{
				LeastSquares: 0.87,
				TurningPoint: 101,
				StepSize:     1.2,
				Regression:   2.3,
				Status:       stepfit.HIGH,
			},
			StepPoint: &dataframe.ColumnHeader{
				Offset:    101,
				Timestamp: 1693173794,
			},
			Num:       50,
			Timestamp: time.Now(),
		}
		frame := &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				Header: []*dataframe.ColumnHeader{
					{Offset: 1, Timestamp: 1687824470},
					{Offset: 2, Timestamp: 1498176000},
				},
			},
		}
		_, _, err = f.FormatNewRegression(ctx, prevCommit, commit, alert, clusterSummary, "https://example.com", frame)
		if err != nil {
			return skerr.Wrapf(err, "formatting regression")
		}
		_, _, err = f.FormatRegressionMissing(ctx, prevCommit, commit, alert, clusterSummary, "https://example.com", frame)
		if err != nil {
			return skerr.Wrapf(err, "formatting regression missing")
		}
	}

	return nil
}

// LoadAndValidate loads the selected config by name into config.Config.
func LoadAndValidate(filename string) error {
	cfg, schemaViolations, err := InstanceConfigFromFile(filename)
	if err != nil {
		for _, v := range schemaViolations {
			sklog.Error(v)
		}
		return skerr.Wrap(err)
	}
	config.Config = cfg
	return nil
}
