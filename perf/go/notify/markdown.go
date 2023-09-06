package notify

import (
	"bytes"
	"context"
	"strings"
	"text/template"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/provider"
)

const (
	defaultRegressionMarkdownSubject = `{{ .Alert.DisplayName }} - Regression found for {{ .Commit.Subject }}`
	defaultRegressionMarkdown        = `A Perf Regression ({{.Cluster.StepFit.Status}}) has been found at:

  {{.URL}}/g/t/{{.Commit.GitHash}}

For:

  Commit {{.CommitURL}}

With:

  - {{.Cluster.Num}} matching traces.
  - Direction {{.Cluster.StepFit.Status}}.

From Alert [{{ .Alert.DisplayName }}]({{.URL}}/a/?{{ .Alert.IDAsString }})
`
	defaultRegressionMissingMarkdownSubject = `{{ .Alert.DisplayName }} - Regression no longer found for {{ .Commit.Subject }}`
	defaultRegressionMissingMarkdown        = `The Perf Regression can no longer be detected. This issue is being automatically closed.
`
)

// MarkdownFormatter implement Formatter.
type MarkdownFormatter struct {
	commitRangeURITemplate                   string
	markdownTemplateNewRegression            *template.Template
	markdownTemplateNewRegressionSubject     *template.Template
	markdownTemplateRegressionMissing        *template.Template
	markdownTemplateRegressionMissingSubject *template.Template
}

// buildIDFromSubject is a template func for notify templates.
//
// Note that this is very specific to the android-x git repo where each commit
// subject is formatted as a single URL, for example:
//
//	https://android-build.googleplex.com/builds/jump-to-build/10768702
//
//	And the template func will extract "10768702" from the above subject.
//
// The implementation is robust and if the subject isn't in the right format
// then the empty string is returned.
func buildIDFromSubject(subject string) string {
	parts := strings.Split(strings.TrimSpace(subject), "/")
	n := len(parts)
	if n == 0 {
		return ""
	}
	return parts[n-1]
}

// NewMarkdownFormatter return a new MarkdownFormatter.
func NewMarkdownFormatter(commitRangeURITemplate string, notifyConfig *config.NotifyConfig) (MarkdownFormatter, error) {
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
		"buildIDFromSubject": buildIDFromSubject,
	}

	markdownTemplateNewRegression, err := template.New("newRegressionMarkdown").Funcs(funcMap).Parse(body)
	if err != nil {
		return MarkdownFormatter{}, skerr.Wrapf(err, "compiling markdownTemplateNewRegression")
	}
	markdownTemplateNewRegressionSubject, err := template.New("newRegressionMarkdown").Funcs(funcMap).Parse(subject)
	if err != nil {
		return MarkdownFormatter{}, skerr.Wrapf(err, "compiling markdownTemplateNewRegressionSubject")
	}
	markdownTemplateRegressionMissing, err := template.New("regressionMissingMarkdown").Funcs(funcMap).Parse(missingBody)
	if err != nil {
		return MarkdownFormatter{}, skerr.Wrapf(err, "compiling markdownTemplateRegressionMissing")
	}
	markdownTemplateRegressionMissingSubject, err := template.New("regressionMissingMarkdown").Funcs(funcMap).Parse(missingSubject)
	if err != nil {
		return MarkdownFormatter{}, skerr.Wrapf(err, "compiling markdownTemplateRegressionMissingSubject")
	}

	return MarkdownFormatter{
		commitRangeURITemplate:                   commitRangeURITemplate,
		markdownTemplateNewRegression:            markdownTemplateNewRegression,
		markdownTemplateNewRegressionSubject:     markdownTemplateNewRegressionSubject,
		markdownTemplateRegressionMissing:        markdownTemplateRegressionMissing,
		markdownTemplateRegressionMissingSubject: markdownTemplateRegressionMissingSubject,
	}, nil
}

// FormatNewRegression implements Formatter.
func (h MarkdownFormatter) FormatNewRegression(ctx context.Context, commit, previousCommit provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, URL string) (string, string, error) {
	templateContext := &TemplateContext{
		URL:            URL,
		PreviousCommit: previousCommit,
		Commit:         commit,
		CommitURL:      URLFromCommitRange(commit, previousCommit, h.commitRangeURITemplate),
		Alert:          alert,
		Cluster:        cl,
	}

	var body bytes.Buffer
	if err := h.markdownTemplateNewRegression.Execute(&body, templateContext); err != nil {
		return "", "", skerr.Wrapf(err, "format Markdown body for a new regression")
	}
	var subject bytes.Buffer
	if err := h.markdownTemplateNewRegressionSubject.Execute(&subject, templateContext); err != nil {
		return "", "", skerr.Wrapf(err, "format Markdown subject for a new regression")
	}

	return body.String(), subject.String(), nil
}

// FormatRegressionMissing implements Formatter.
func (h MarkdownFormatter) FormatRegressionMissing(ctx context.Context, commit, previousCommit provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, URL string) (string, string, error) {
	templateContext := &TemplateContext{
		URL:            URL,
		PreviousCommit: previousCommit,
		Commit:         commit,
		CommitURL:      URLFromCommitRange(commit, previousCommit, h.commitRangeURITemplate),
		Alert:          alert,
		Cluster:        cl,
	}

	var body bytes.Buffer
	if err := h.markdownTemplateRegressionMissing.Execute(&body, templateContext); err != nil {
		return "", "", skerr.Wrapf(err, "format Markdown body for a regression that has gone missing")
	}
	var subject bytes.Buffer
	if err := h.markdownTemplateRegressionMissingSubject.Execute(&subject, templateContext); err != nil {
		return "", "", skerr.Wrapf(err, "format Markdown subject for regression that has gone missing")
	}
	return body.String(), subject.String(), nil
}

var _ Formatter = MarkdownFormatter{}
