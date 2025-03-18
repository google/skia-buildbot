package formatter

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"go.skia.org/infra/go/skerr"
	ag "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	"go.skia.org/infra/perf/go/config"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/urlprovider"
)

const (
	defaultNewCulpritSubject = `{{ .Subscription.Name }} - Regression Detected & Culprit Found`
	defaultNewCulpritBody    = `Culprit commit: {{ buildCommitURL .CommitUrl .Commit.Revision}}`
	defaultNewReportSubject  = `[{{ .Subscription.Name }}]: [{{ len .AnomalyGroup.AnomalyIds }}] regressions in {{ .AnomalyGroup.BenchmarkName }}`
	defaultNewReportBody     = `
	Dashboard URLs:
	 {{ buildGroupUrl .HostUrl .AnomalyGroup.GroupId }}
	Top {{ len .TopAnomalies }} anomalies in this group:{{ range .TopAnomalies }}
	 - {{ buildAnomalyDetails . }}
	{{end}}`
)

// Formatter controls how the notification looks like
type Formatter interface {
	// Return body and subject of a bug for new found culprit.
	GetCulpritSubjectAndBody(ctx context.Context, culprit *pb.Culprit, subscription *sub_pb.Subscription) (string, string, error)

	// Return body and subject of a bug for reporting a new anomaly group.
	GetReportSubjectAndBody(ctx context.Context, anomalyGroup *ag.AnomalyGroup, subscription *sub_pb.Subscription, anomalyList []*pb.Anomaly) (string, string, error)
}

// MarkdownFormatter implement Formatter.
type MarkdownFormatter struct {
	commitURLTemplate         string
	instanceUrl               string
	newCulpritSubjectTemplate *template.Template
	newCulpritBodyTemplate    *template.Template
	newReportSubjectTempalte  *template.Template
	newReportBodyTemplate     *template.Template
}

// TemplateContext is used in expanding the message templates.
type TemplateContext struct {
	// Commit is the Commit the regression was found at.
	Commit *pb.Commit

	// CommitURL is a URL that points to the above Commit.
	// e.g.
	CommitUrl string

	// Subscription is the configuration which tells where/how to file the bug
	Subscription *sub_pb.Subscription
}

type ReportTemplateContext struct {
	// The anomaly group newly found.
	AnomalyGroup *ag.AnomalyGroup

	// The subcsription based on which the anomaly group is created.
	Subscription *sub_pb.Subscription

	// The anomalies with the most significant performance change.
	TopAnomalies []*pb.Anomaly

	// The url of the current instance
	HostUrl string
}

func buildCommitURL(url, commit string) string {
	return fmt.Sprintf(url, commit)
}

func buildAnomalyGroupUrl(url, groupId string) string {
	return fmt.Sprintf("%s%s", url, urlprovider.GroupReport("anomalyGroupID", groupId))
}

func buildAnomalyDetails(anomaly *pb.Anomaly) string {
	return fmt.Sprintf(`Bot: %s, Benchmark: %s, Measurement: %s, Story: %s,
	   Change: %.4f -> %.4f (%.2f%%); Commit range: %d -> %d`,
		anomaly.Paramset["bot"], anomaly.Paramset["benchmark"], anomaly.Paramset["measurement"], anomaly.Paramset["story"],
		anomaly.MedianBefore, anomaly.MedianAfter, 100*(anomaly.MedianAfter-anomaly.MedianBefore)/anomaly.MedianBefore,
		anomaly.StartCommit, anomaly.EndCommit)
}

// NewMarkdownFormatter return a new MarkdownFormatter.
func NewMarkdownFormatter(commitURLTemplate string, instanceConfig *config.InstanceConfig) (*MarkdownFormatter, error) {
	culpritSubject := instanceConfig.IssueTrackerConfig.CulpritSubject
	if culpritSubject == "" {
		culpritSubject = defaultNewCulpritSubject
	}
	culpritBody := strings.Join(instanceConfig.IssueTrackerConfig.CulpritBody, "\n")
	if culpritBody == "" {
		culpritBody = defaultNewCulpritBody
	}
	newCulpritSubjectTemplate, err := template.New("newCulpritMarkdown").Parse(culpritSubject)
	if err != nil {
		return nil, skerr.Wrapf(err, "compiling newCulpritSubjectTemplate")
	}
	culpritFuncMap := template.FuncMap{
		"buildCommitURL": buildCommitURL,
	}
	newCulpritBodyTemplate, err := template.New("newCulpritMarkdown").Funcs(culpritFuncMap).Parse(culpritBody)
	if err != nil {
		return nil, skerr.Wrapf(err, "compiling newCulpritBodyTemplate")
	}

	reportSubject := instanceConfig.IssueTrackerConfig.AnomalyReportSubject
	if reportSubject == "" {
		reportSubject = defaultNewReportSubject
	}
	reportBody := strings.Join(instanceConfig.IssueTrackerConfig.AnomalyReportBody, "\n")
	if reportBody == "" {
		reportBody = defaultNewReportBody
	}
	anomalyReportFuncMap := template.FuncMap{
		"buildGroupUrl":       buildAnomalyGroupUrl,
		"buildAnomalyDetails": buildAnomalyDetails,
	}
	newReportSubjectTemplate, err := template.New("newReportSubjectMarkdown").Parse(reportSubject)
	if err != nil {
		return nil, skerr.Wrapf(err, "compiling newReportSubjectTemplate")
	}
	newReportBodyTemplate, err := template.New("newReporBodytMarkdown").Funcs(anomalyReportFuncMap).Parse(reportBody)
	if err != nil {
		return nil, skerr.Wrapf(err, "compiling newReportBodyTemplate")
	}

	return &MarkdownFormatter{
		commitURLTemplate:         commitURLTemplate,
		instanceUrl:               instanceConfig.URL,
		newCulpritSubjectTemplate: newCulpritSubjectTemplate,
		newCulpritBodyTemplate:    newCulpritBodyTemplate,
		newReportSubjectTempalte:  newReportSubjectTemplate,
		newReportBodyTemplate:     newReportBodyTemplate,
	}, nil
}

// FormatNewCulprit implements Formatter.
func (f MarkdownFormatter) GetCulpritSubjectAndBody(ctx context.Context, culprit *pb.Culprit,
	subscription *sub_pb.Subscription) (string, string, error) {
	templateContext := &TemplateContext{
		CommitUrl:    f.commitURLTemplate,
		Commit:       culprit.Commit,
		Subscription: subscription,
	}
	var bodyb bytes.Buffer
	if err := f.newCulpritBodyTemplate.Execute(&bodyb, templateContext); err != nil {
		return "", "", skerr.Wrapf(err, "format Markdown body for a new culprit")
	}
	var subjectb bytes.Buffer
	if err := f.newCulpritSubjectTemplate.Execute(&subjectb, templateContext); err != nil {
		return "", "", skerr.Wrapf(err, "format Markdown subject for a new culprit")
	}
	return string(subjectb.String()), string(bodyb.String()), nil
}

// FormatNewReportSubjectAndBody implements Formatter
func (f MarkdownFormatter) GetReportSubjectAndBody(ctx context.Context, anomalyGroup *ag.AnomalyGroup,
	subscription *sub_pb.Subscription, anomalyList []*pb.Anomaly) (string, string, error) {
	templateContext := &ReportTemplateContext{
		AnomalyGroup: anomalyGroup,
		Subscription: subscription,
		TopAnomalies: anomalyList,
		HostUrl:      f.instanceUrl,
	}
	var subjectb bytes.Buffer
	if err := f.newReportSubjectTempalte.Execute(&subjectb, templateContext); err != nil {
		return "", "", skerr.Wrapf(err, "format Markdown subject for a new anomaly group report")
	}
	var bodyb bytes.Buffer
	if err := f.newReportBodyTemplate.Execute(&bodyb, templateContext); err != nil {
		return "", "", skerr.Wrapf(err, "format Markdown body for a new anomaly group report")
	}
	return string(subjectb.String()), string(bodyb.String()), nil
}
