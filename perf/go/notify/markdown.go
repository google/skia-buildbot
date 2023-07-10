package notify

import (
	"bytes"
	"context"
	"fmt"
	"html/template"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/git/provider"
)

const (
	newRegressionMarkdown = `A Perf Regression ({{.Cluster.StepFit.Status}}) has been found at:

  {{.URL}}/g/t/{{.Commit.GitHash}}

For:

  Commit {{.Commit.URL}}

With:

  - {{.Cluster.Num}} matching traces.
  - Direction {{.Cluster.StepFit.Status}}.

From Alert [{{ .Alert.DisplayName }}]({{.URL}}/a/?{{ .Alert.IDAsString }})
`
	regressionMissingMarkdown = `The Perf Regression can no longer be detected. This issue is being automatically closed.
`
)

var (
	markdownTemplateNewRegression     = template.Must(template.New("newRegressionMarkdown").Parse(newRegressionMarkdown))
	markdownTemplateRegressionMissing = template.Must(template.New("regressionMissingMarkdown").Parse(regressionMissingMarkdown))
)

// MarkdownFormatter implement Formatter.
type MarkdownFormatter struct{}

// NewMarkdownFormatter return a new MarkdownFormatter.
func NewMarkdownFormatter() MarkdownFormatter {
	return MarkdownFormatter{}
}

// FormatNewRegression implements Formatter.
func (h MarkdownFormatter) FormatNewRegression(ctx context.Context, c provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, URL string) (string, string, error) {
	templateContext := &templateContext{
		URL:     URL,
		Commit:  c,
		Alert:   alert,
		Cluster: cl,
	}

	var b bytes.Buffer
	if err := markdownTemplateNewRegression.Execute(&b, templateContext); err != nil {
		return "", "", skerr.Wrapf(err, "format Markdown body for a new regression")
	}
	subject := fmt.Sprintf("%s - Regression found for %s", alert.DisplayName, c.Display(now.Now(ctx)))

	return b.String(), subject, nil
}

// FormatRegressionMissing implements Formatter.
func (h MarkdownFormatter) FormatRegressionMissing(ctx context.Context, c provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, URL string) (string, string, error) {
	templateContext := &templateContext{
		URL:     URL,
		Commit:  c,
		Alert:   alert,
		Cluster: cl,
	}

	var b bytes.Buffer
	if err := markdownTemplateRegressionMissing.Execute(&b, templateContext); err != nil {
		return "", "", skerr.Wrapf(err, "format Markdown body for a regression that has gone missing")
	}
	subject := fmt.Sprintf("%s - Regression no longer found for %s", alert.DisplayName, c.Display(now.Now(ctx)))
	return b.String(), subject, nil
}

var _ Formatter = MarkdownFormatter{}
