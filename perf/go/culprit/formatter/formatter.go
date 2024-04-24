package formatter

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/config"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

const (
	defaultNewCulpritSubject = `{{ .Subscription.Name }} - Regression Detected & Culprit Found`
	defaultNewCulpritBody    = `Culprit commit: {{ buildCommitURL .CommitUrl .Commit.Revision}}`
)

// Formatter controls how the notification looks like
type Formatter interface {
	// Return body and subject.
	GetSubjectAndBody(ctx context.Context, culprit *pb.Culprit, subscription *sub_pb.Subscription) (string, string, error)
}

// MarkdownFormatter implement Formatter.
type MarkdownFormatter struct {
	commitURLTemplate         string
	newCulpritBodyTemplate    *template.Template
	newCulpritSubjectTemplate *template.Template
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

func buildCommitURL(url, commit string) string {
	return fmt.Sprintf(url, commit)
}

// NewMarkdownFormatter return a new MarkdownFormatter.
func NewMarkdownFormatter(commitURLTemplate string, notifyConfig *config.CulpritNotifyConfig) (*MarkdownFormatter, error) {
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
	return &MarkdownFormatter{
		commitURLTemplate:         commitURLTemplate,
		newCulpritSubjectTemplate: newCulpritSubjectTemplate,
		newCulpritBodyTemplate:    newCulpritBodyTemplate,
	}, nil
}

// FormatNewCulprit implements Formatter.
func (f MarkdownFormatter) GetSubjectAndBody(ctx context.Context, culprit *pb.Culprit,
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
