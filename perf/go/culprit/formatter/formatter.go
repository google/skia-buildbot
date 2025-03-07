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
)

const (
	defaultNewCulpritSubject = `{{ .Subscription.Name }} - Regression Detected & Culprit Found`
	defaultNewCulpritBody    = `Culprit commit: {{ buildCommitURL .CommitUrl .Commit.Revision}}`
	defaultNewReportSubject  = `[{{ .Subscription.Name }}]: [{{ len .AnomalyGroup.AnomalyIds }}] regressions in {{ .AnomalyGroup.BenchmarkName }}`
	defaultNewReportBody     = `
	Dashboard URLs:
		TO ADD WITH GROUP ID
	Top {{ len .TopAnomalies }} anomalies in this group:
	{{ range .TopAnomalies }}
	 - {{ . }}
	{{end}}
	`
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
}

func buildCommitURL(url, commit string) string {
	return fmt.Sprintf(url, commit)
}

// NewMarkdownFormatter return a new MarkdownFormatter.
func NewMarkdownFormatter(commitURLTemplate string, notifyConfig *config.IssueTrackerConfig) (*MarkdownFormatter, error) {
	culpritSubject := notifyConfig.CulpritSubject
	if culpritSubject == "" {
		culpritSubject = defaultNewCulpritSubject
	}
	culpritBody := strings.Join(notifyConfig.CulpritBody, "\n")
	if culpritBody == "" {
		culpritBody = defaultNewCulpritBody
	}
	newCulpritSubjectTemplate, err := template.New("newCulpritMarkdown").Parse(culpritSubject)
	if err != nil {
		return nil, skerr.Wrapf(err, "compiling newCulpritSubjectTemplate")
	}
	funcMap := template.FuncMap{
		"buildCommitURL": buildCommitURL,
	}
	newCulpritBodyTemplate, err := template.New("newCulpritMarkdown").Funcs(funcMap).Parse(culpritBody)
	if err != nil {
		return nil, skerr.Wrapf(err, "compiling newCulpritBodyTemplate")
	}

	reportSubject := notifyConfig.AnomalyReportSubject
	if reportSubject == "" {
		reportSubject = defaultNewReportSubject
	}
	reportBody := strings.Join(notifyConfig.AnomalyReportBody, "\n")
	if reportBody == "" {
		reportBody = defaultNewReportBody
	}
	newReportSubjectTemplate, err := template.New("newReportSubjectMarkdown").Parse(reportSubject)
	if err != nil {
		return nil, skerr.Wrapf(err, "compiling newReportSubjectTemplate")
	}
	newReportBodyTemplate, err := template.New("newReporBodytMarkdown").Parse(reportBody)
	if err != nil {
		return nil, skerr.Wrapf(err, "compiling newReportBodyTemplate")
	}

	return &MarkdownFormatter{
		commitURLTemplate:         commitURLTemplate,
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
