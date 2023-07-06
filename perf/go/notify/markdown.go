package notify

import "html/template"

const (
	newRegressionMarkdown     = ``
	regressionMissingMarkdown = ``
)

var (
	markdownTemplateNewRegression     = template.Must(template.New("newRegressionMarkdown").Parse(newRegressionMarkdown))
	markdownTemplateRegressionMissing = template.Must(template.New("regressionMissingMarkdown").Parse(regressionMissingMarkdown))
)
