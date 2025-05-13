package notify

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/notify/common"
)

// AndroidBugTemplateContext provides a struct containing data necessary
// for generating body and subject for android instance bugs.
type AndroidBugTemplateContext struct {
	// URL is the root URL of the Perf instance.
	URL string

	// DashboardUrl is the URL to view the regressing traces on the explore
	// page.
	DashboardUrl string

	// PreviousCommit is the previous commit the regression was found at.
	//
	// All commits that might be blamed for causing the regression
	// are in the range `(PreviousCommit, Commit]`, that is inclusive of
	// Commit but exclusive of PreviousCommit.
	PreviousCommit provider.Commit

	// RegressionCommit is the commit the regression was found at.
	RegressionCommit provider.Commit

	// CommitURL is a URL that points to the above Commit. The value of this URL
	// can be controlled via the `--commit_range_url` flag.
	CommitURL string

	// Alert is the configuration for the alert that found the regression.
	Alert *alerts.Alert

	// Cluster is all the information found about the regression.
	Cluster *clustering2.ClusterSummary

	// ParamSet for all the matching traces in the query.
	ParamSet paramtools.ReadOnlyParamSet

	// TraceID for the trace where regression is detected. Only available when the
	// detection is done individually and not KMeans.
	TraceID string

	// RegressionCommitLinks contain the links for the regression commit data point for the trace.
	RegressionCommitLinks map[string]string

	// PreviousCommitLinks contain the links for the commit data point before the regression for the trace.
	PreviousCommitLinks map[string]string

	// Contains formatted names of affected test(s). Formatted as
	// "{{test_class}}#{{test_method}}#{{device_name}}#{{os_version}}"
	Tests []string
}

// GetBuildIdUrlDiff returns a url that contains the diff between build ids for androidx.
func (context AndroidBugTemplateContext) GetBuildIdUrlDiff() string {
	const buildIdKey = "Build ID"
	var fromBuildId, toBuildId string
	var ok bool
	if fromBuildId, ok = context.RegressionCommitLinks[buildIdKey]; !ok {
		return "No build id in current commit"
	}
	if toBuildId, ok = context.PreviousCommitLinks[buildIdKey]; !ok {
		return "No build id in previous commit"
	}

	return buildIdsToUrlDiff(fromBuildId, toBuildId)
}

// androidNotificationProvider provides functionality to generate data for android bugs.
type androidNotificationProvider struct {
	formatter *MarkdownFormatter
}

// NewAndroidNotificationDataProvider returns a new instance of the androidNotificationProvider.
func NewAndroidNotificationDataProvider(commitRangeURITemplate string, notifyConfig *config.NotifyConfig) (*androidNotificationProvider, error) {
	body := strings.Join(notifyConfig.Body, "\n")
	if body == "" {
		body = defaultRegressionMarkdown
	}
	subject := notifyConfig.Subject
	if subject == "" {
		subject = defaultRegressionMarkdownSubject
	}

	missingBody := strings.Join(notifyConfig.MissingBody, "\n")
	if missingBody == "" {
		missingBody = defaultRegressionMissingMarkdown
	}

	missingSubject := notifyConfig.MissingSubject
	if missingSubject == "" {
		missingSubject = defaultRegressionMissingMarkdownSubject
	}

	funcMap := template.FuncMap{
		"buildIdsToUrlDiff": buildIdsToUrlDiff,
	}

	markdownTemplateNewRegression, err := template.New("newRegressionMarkdown").Funcs(funcMap).Parse(body)
	if err != nil {
		return nil, skerr.Wrapf(err, "compiling markdownTemplateNewRegression")
	}
	markdownTemplateNewRegressionSubject, err := template.New("newRegressionMarkdown").Funcs(funcMap).Parse(subject)
	if err != nil {
		return nil, skerr.Wrapf(err, "compiling markdownTemplateNewRegressionSubject")
	}
	markdownTemplateRegressionMissing, err := template.New("regressionMissingMarkdown").Funcs(funcMap).Parse(missingBody)
	if err != nil {
		return nil, skerr.Wrapf(err, "compiling markdownTemplateRegressionMissing")
	}
	markdownTemplateRegressionMissingSubject, err := template.New("regressionMissingMarkdown").Funcs(funcMap).Parse(missingSubject)
	if err != nil {
		return nil, skerr.Wrapf(err, "compiling markdownTemplateRegressionMissingSubject")
	}

	formatter := MarkdownFormatter{
		commitRangeURITemplate:                   commitRangeURITemplate,
		markdownTemplateNewRegression:            markdownTemplateNewRegression,
		markdownTemplateNewRegressionSubject:     markdownTemplateNewRegressionSubject,
		markdownTemplateRegressionMissing:        markdownTemplateRegressionMissing,
		markdownTemplateRegressionMissingSubject: markdownTemplateRegressionMissingSubject,
	}

	return &androidNotificationProvider{
		formatter: &formatter,
	}, nil
}

// GetNotificationDataRegressionFound returns the notification data for a new regression.
func (prov *androidNotificationProvider) GetNotificationDataRegressionFound(ctx context.Context, metadata common.RegressionMetadata) (*common.NotificationData, error) {
	if prov.formatter != nil {
		templateContext := prov.getTemplateContext(metadata)
		body, subject, err := prov.formatter.FormatNewRegressionWithContext(templateContext)
		if err != nil {
			return nil, err
		}

		return &common.NotificationData{
			Body:    body,
			Subject: subject,
		}, nil
	}

	return &common.NotificationData{
		Body:    "",
		Subject: "",
	}, nil
}

// GetNotificationDataRegressionMissing returns the notification data for a missing regression.
func (prov *androidNotificationProvider) GetNotificationDataRegressionMissing(ctx context.Context, metadata common.RegressionMetadata) (*common.NotificationData, error) {
	if prov.formatter != nil {
		templateContext := prov.getTemplateContext(metadata)
		body, subject, err := prov.formatter.FormatRegressionMissingWithContext(templateContext)
		if err != nil {
			return nil, err
		}

		return &common.NotificationData{
			Body:    body,
			Subject: subject,
		}, nil
	}

	return &common.NotificationData{
		Body:    "",
		Subject: "",
	}, nil
}

func formatTests(metadata common.RegressionMetadata) []string {
	formattedTests := make([]string, 0)
	if metadata.Frame != nil && metadata.Frame.DataFrame != nil && metadata.Frame.DataFrame.TraceSet != nil {
		if len(metadata.Frame.DataFrame.TraceSet) > 0 {
			for traceKey := range metadata.Frame.DataFrame.TraceSet {
				params := paramtools.NewParams(traceKey)
				testClass := params["test_class"]
				testMethod := params["test_method"]
				deviceName := params["device_name"]
				osVersion := params["os_version"]
				formattedString := fmt.Sprintf("%s#%s#%s#%s", testClass, testMethod, deviceName, osVersion)
				formattedTests = append(formattedTests, formattedString)
			}
		}
	}
	sort.Strings(formattedTests)
	return formattedTests
}

// getTemplateContext returns a template context object to be used for the bug data formatting.
func (prov *androidNotificationProvider) getTemplateContext(metadata common.RegressionMetadata) AndroidBugTemplateContext {

	return AndroidBugTemplateContext{
		URL:                   metadata.InstanceUrl,
		DashboardUrl:          viewOnDashboard(metadata.Cl, metadata.InstanceUrl, metadata.Frame),
		PreviousCommit:        metadata.PreviousCommit,
		RegressionCommit:      metadata.RegressionCommit,
		CommitURL:             URLFromCommitRange(metadata.RegressionCommit, metadata.PreviousCommit, prov.formatter.commitRangeURITemplate),
		Alert:                 metadata.AlertConfig,
		Cluster:               metadata.Cl,
		ParamSet:              metadata.Frame.DataFrame.ParamSet,
		TraceID:               metadata.TraceID,
		RegressionCommitLinks: metadata.RegressionCommitLinks,
		PreviousCommitLinks:   metadata.PreviousCommitLinks,
		Tests:                 formatTests(metadata),
	}
}

func buildIdsToUrlDiff(fromBuildId string, toBuildId string) string {
	const urlFormat = "https://android-build.corp.google.com/range_search/cls/from_id/%s/to_id/%s/?s=menu&includeTo=0&includeFrom=1"
	return fmt.Sprintf(urlFormat, fromBuildId, toBuildId)
}
